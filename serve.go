package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/slicestore"
	"fiatjaf.com/nostr/khatru"
	"fiatjaf.com/nostr/khatru/blossom"
	"fiatjaf.com/nostr/khatru/grasp"
	"github.com/mailru/easyjson"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
	"github.com/puzpuzpuz/xsync/v3"
)

type serveVars struct {
	tab *qt.QWidget

	graspCheck      *qt.QCheckBox
	blossomCheck    *qt.QCheckBox
	negentropyCheck *qt.QCheckBox

	serverAddressInput *qt.QLineEdit

	startButton *qt.QPushButton
	stopButton  *qt.QPushButton

	logsList         *qt.QListWidget
	eventsList       *qt.QListWidget
	graspReposList   *serveSpecialBox
	blossomBlobsList *serveSpecialBox

	bottomHBox *qt.QHBoxLayout

	relay     *khatru.Relay
	db        *slicestore.SliceStore
	blobStore *xsync.MapOf[string, []byte]
	repoDir   string
}

type serveSpecialBox struct {
	vbox  *qt.QVBoxLayout
	list  *qt.QListWidget
	label *qt.QLabel
}

var serve = &serveVars{}

func setupServeTab() *qt.QWidget {
	serve.tab = qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	serve.tab.SetLayout(layout.QLayout)

	// checkboxes
	optionsHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(optionsHBox.QLayout)

	serve.negentropyCheck = qt.NewQCheckBox(serve.tab)
	serve.negentropyCheck.SetText("negentropy")
	optionsHBox.AddWidget(serve.negentropyCheck.QWidget)

	serve.graspCheck = qt.NewQCheckBox(serve.tab)
	serve.graspCheck.SetText("grasp")
	optionsHBox.AddWidget(serve.graspCheck.QWidget)

	serve.blossomCheck = qt.NewQCheckBox(serve.tab)
	serve.blossomCheck.SetText("blossom")
	optionsHBox.AddWidget(serve.blossomCheck.QWidget)

	serve.serverAddressInput = qt.NewQLineEdit(serve.tab)
	serve.serverAddressInput.SetReadOnly(true)
	optionsHBox.AddWidget(serve.serverAddressInput.QWidget)

	// buttons
	buttonsHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(buttonsHBox.QLayout)

	serve.startButton = qt.NewQPushButton5("start", serve.tab)
	buttonsHBox.AddWidget(serve.startButton.QWidget)

	serve.stopButton = qt.NewQPushButton5("stop", serve.tab)
	serve.stopButton.SetEnabled(false)
	buttonsHBox.AddWidget(serve.stopButton.QWidget)

	// logs
	logsLabel := qt.NewQLabel2()
	logsLabel.SetText("logs:")
	layout.AddWidget(logsLabel.QWidget)
	serve.logsList = qt.NewQListWidget(serve.tab)
	serve.logsList.SetMinimumHeight(150)
	layout.AddWidget(serve.logsList.QWidget)

	// bottom layout with columns
	serve.bottomHBox = qt.NewQHBoxLayout2()
	layout.AddLayout(serve.bottomHBox.QLayout)

	// events column
	eventsVBox := qt.NewQVBoxLayout2()
	eventsLabel := qt.NewQLabel2()
	eventsLabel.SetText("events:")
	eventsVBox.AddWidget(eventsLabel.QWidget)
	serve.eventsList = qt.NewQListWidget(serve.tab)
	serve.eventsList.SetMinimumHeight(200)
	eventsVBox.AddWidget(serve.eventsList.QWidget)
	serve.bottomHBox.AddLayout(eventsVBox.QLayout)

	// double-click events
	serve.eventsList.OnItemDoubleClicked(func(item *qt.QListWidgetItem) {
		var event nostr.Event
		if err := json.Unmarshal([]byte(item.Text()), &event); err != nil {
			return
		}
		pretty, _ := json.MarshalIndent(event, "", "  ")
		dialog := qt.NewQDialog(window.QWidget)
		dialog.SetWindowTitle("event")
		dialog.SetMinimumWidth(400)
		dialog.SetMinimumHeight(500)
		dlayout := qt.NewQVBoxLayout2()
		dialog.SetLayout(dlayout.QLayout)
		textEdit := qt.NewQTextEdit(dialog.QWidget)
		textEdit.SetReadOnly(true)
		textEdit.SetPlainText(string(pretty))
		dlayout.AddWidget(textEdit.QWidget)
		closeButton := qt.NewQPushButton5("close", dialog.QWidget)
		closeButton.OnClicked(func() { dialog.Close() })
		dlayout.AddWidget(closeButton.QWidget)
		dialog.Exec()
	})

	serve.startButton.OnClicked(serve.startRelay)
	serve.stopButton.OnClicked(serve.stopRelay)

	return serve.tab
}

func (serve *serveVars) startRelay() {
	serve.startButton.SetEnabled(false)
	serve.stopButton.SetEnabled(true)
	serve.negentropyCheck.SetEnabled(false)
	serve.blossomCheck.SetEnabled(false)
	serve.graspCheck.SetEnabled(false)

	// clear blossom and grasp boxes
	if serve.blossomBlobsList != nil {
		serve.blossomBlobsList.vbox.RemoveWidget(serve.blossomBlobsList.label.QWidget)
		serve.blossomBlobsList.label.DeleteLater()

		serve.blossomBlobsList.vbox.RemoveWidget(serve.blossomBlobsList.list.QWidget)
		serve.blossomBlobsList.list.DeleteLater()

		serve.bottomHBox.RemoveItem(serve.blossomBlobsList.vbox.QLayoutItem)
		serve.blossomBlobsList.vbox.DeleteLater()

		serve.blossomBlobsList = nil
	}

	if serve.graspReposList != nil {
		serve.graspReposList.vbox.RemoveWidget(serve.graspReposList.label.QWidget)
		serve.graspReposList.label.DeleteLater()

		serve.graspReposList.vbox.RemoveWidget(serve.graspReposList.list.QWidget)
		serve.graspReposList.list.DeleteLater()

		serve.bottomHBox.RemoveItem(serve.graspReposList.vbox.QLayoutItem)
		serve.graspReposList.vbox.DeleteLater()

		serve.graspReposList = nil
	}

	// setup relay
	if serve.db == nil {
		serve.db = &slicestore.SliceStore{}
	}

	serve.relay = khatru.NewRelay()
	serve.relay.Info.Name = "vnak serve"
	serve.relay.Info.Description = "a local relay for testing, debugging and development."
	serve.relay.Info.Software = "https://github.com/fiatjaf/vnak"
	serve.relay.Info.Version = "dev"

	serve.relay.UseEventstore(serve.db, 500)

	if serve.negentropyCheck.IsChecked() {
		serve.relay.Negentropy = true
	}

	started := make(chan bool)
	exited := make(chan error)

	hostname := "localhost"
	port := 10547

	if serve.blossomCheck.IsChecked() {
		// setup blossom
		if serve.blobStore == nil {
			serve.blobStore = xsync.NewMapOf[string, []byte]()
		}

		bs := blossom.New(serve.relay, fmt.Sprintf("http://%s:%d", hostname, port))
		bs.Store = blossom.NewMemoryBlobIndex()

		bs.StoreBlob = func(ctx context.Context, sha256 string, ext string, body []byte) error {
			serve.blobStore.Store(sha256+ext, body)
			serve.log("blob stored: %s", sha256+ext)
			serve.updateBlossomBlobsList()
			return nil
		}
		bs.LoadBlob = func(ctx context.Context, sha256 string, ext string) (io.ReadSeeker, *url.URL, error) {
			if body, ok := serve.blobStore.Load(sha256 + ext); ok {
				serve.log("blob download: %s", sha256+ext)
				return bytes.NewReader(body), nil, nil
			}
			return nil, nil, nil
		}
		bs.DeleteBlob = func(ctx context.Context, sha256 string, ext string) error {
			serve.blobStore.Delete(sha256 + ext)
			serve.log("blob delete: %s", sha256+ext)
			serve.updateBlossomBlobsList()
			return nil
		}

		// display blossom box
		serve.blossomBlobsList = &serveSpecialBox{
			vbox:  qt.NewQVBoxLayout2(),
			label: qt.NewQLabel2(),
			list:  qt.NewQListWidget(serve.tab),
		}
		serve.blossomBlobsList.list.SetMinimumWidth(300)
		serve.blossomBlobsList.label.SetText("blossom blobs:")
		serve.blossomBlobsList.vbox.AddWidget(serve.blossomBlobsList.label.QWidget)
		serve.blossomBlobsList.vbox.AddWidget(serve.blossomBlobsList.list.QWidget)
		serve.bottomHBox.AddLayout(serve.blossomBlobsList.vbox.QLayout)
		serve.updateBlossomBlobsList()
	}

	if serve.graspCheck.IsChecked() {
		// setup grasp
		if serve.repoDir == "" {
			var err error
			serve.repoDir, err = os.MkdirTemp("", "vnak-serve-grasp-repos-")
			if err != nil {
				serve.log("failed to create grasp repos directory: %w", err)
				return
			}
		}
		g := grasp.New(serve.relay, serve.repoDir)
		g.OnRead = func(ctx context.Context, pubkey nostr.PubKey, repo string) (reject bool, reason string) {
			serve.log("git read by '%s' at '%s'", pubkey.Hex(), repo)
			serve.updateGraspReposList()
			return false, ""
		}
		g.OnWrite = func(ctx context.Context, pubkey nostr.PubKey, repo string) (reject bool, reason string) {
			serve.log("git write by '%s' at '%s'", pubkey.Hex(), repo)
			serve.updateGraspReposList()
			return false, ""
		}

		// display grasp vbox
		serve.graspReposList = &serveSpecialBox{
			vbox:  qt.NewQVBoxLayout2(),
			label: qt.NewQLabel2(),
			list:  qt.NewQListWidget(serve.tab),
		}
		serve.graspReposList.list.SetMinimumWidth(300)
		serve.graspReposList.label.SetText("grasp repos:")
		serve.graspReposList.vbox.AddWidget(serve.graspReposList.label.QWidget)
		serve.graspReposList.vbox.AddWidget(serve.graspReposList.list.QWidget)
		serve.bottomHBox.AddLayout(serve.graspReposList.vbox.QLayout)
		serve.updateGraspReposList()
	}

	go func() {
		err := serve.relay.Start(hostname, port, started)
		exited <- err
	}()

	// relay logging
	serve.relay.OnRequest = func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		negentropy := ""
		if khatru.IsNegentropySession(ctx) {
			negentropy = "negentropy "
		}

		serve.log("%srequest: %s", negentropy, filter)
		return false, ""
	}

	serve.relay.OnCount = func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		serve.log("count request: %s", filter)
		return false, ""
	}

	serve.relay.OnEvent = func(ctx context.Context, event nostr.Event) (reject bool, msg string) {
		serve.log("event: %s", event)
		return false, ""
	}

	serve.relay.OnEventSaved = func(ctx context.Context, event nostr.Event) {
		serve.updateEventsList()
	}

	totalConnections := atomic.Int32{}
	serve.relay.OnConnect = func(ctx context.Context) {
		totalConnections.Add(1)
		go func() {
			<-ctx.Done()
			totalConnections.Add(-1)
		}()
	}

	<-started
	serve.log("relay running at %s", fmt.Sprintf("ws://%s:%d", hostname, port))
	mainthread.Start(func() {
		serve.serverAddressInput.SetText(fmt.Sprintf("ws://%s:%d", hostname, port))
		if serve.graspCheck.IsChecked() {
			serve.updateGraspReposList()
		}
		if serve.blossomCheck.IsChecked() {
			serve.updateBlossomBlobsList()
		}
	})
	if serve.graspCheck.IsChecked() {
		serve.log("grasp repos at %s", serve.repoDir)
	}

	go func() {
		err := <-exited
		if err != nil {
			serve.log("relay exited with error: %s", err)
		}
		mainthread.Wait(func() {
			serve.startButton.SetEnabled(true)
			serve.stopButton.SetEnabled(false)
		})
	}()
}

func (serve *serveVars) stopRelay() {
	if serve.relay != nil {
		serve.relay.Shutdown(ctx)
	}
	serve.startButton.SetEnabled(true)
	serve.stopButton.SetEnabled(false)
	serve.negentropyCheck.SetEnabled(true)
	serve.blossomCheck.SetEnabled(true)
	serve.graspCheck.SetEnabled(true)
	serve.serverAddressInput.SetText("")
	serve.log("relay stopped")
}

func (serve *serveVars) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	mainthread.Start(func() {
		item := qt.NewQListWidgetItem2(msg)
		pos := serve.logsList.VerticalScrollBar().SliderPosition()
		serve.logsList.InsertItem(0, item)
		if pos == 0 {
			serve.logsList.ScrollToTop()
		}
	})
}

func calculateDirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}

func getHeadCommit(repoPath string) string {
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "no commits"
	}
	return strings.TrimSpace(string(out))
}

func (serve *serveVars) updateGraspReposList() {
	mainthread.Start(func() {
		serve.graspReposList.list.Clear()
		if serve.repoDir == "" {
			return
		}
		entries, err := os.ReadDir(serve.repoDir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			d := entry.Name()
			repoPath := filepath.Join(serve.repoDir, d)
			size := calculateDirSize(repoPath)
			head := getHeadCommit(repoPath)
			item := qt.NewQListWidgetItem2(fmt.Sprintf("d: %s\npath: %s\nsize: %d bytes\nhead: %s", d, repoPath, size, head))
			serve.graspReposList.list.AddItemWithItem(item)
		}
	})
}

func (serve *serveVars) updateBlossomBlobsList() {
	mainthread.Start(func() {
		serve.blossomBlobsList.list.Clear()
		for key, value := range serve.blobStore.Range {
			item := qt.NewQListWidgetItem2(fmt.Sprintf("%s (%d bytes)", key, len(value)))
			serve.blossomBlobsList.list.AddItemWithItem(item)
		}
	})
}

func (serve *serveVars) updateEventsList() {
	mainthread.Start(func() {
		serve.eventsList.Clear()
		for evt := range serve.db.QueryEvents(nostr.Filter{}, 5000) {
			evtj, _ := easyjson.Marshal(evt)
			item := qt.NewQListWidgetItem2(string(evtj))
			serve.eventsList.AddItemWithItem(item)
		}
	})
}
