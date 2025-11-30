package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync/atomic"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/slicestore"
	"fiatjaf.com/nostr/khatru"
	"fiatjaf.com/nostr/khatru/blossom"
	"fiatjaf.com/nostr/khatru/grasp"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
	"github.com/puzpuzpuz/xsync/v3"
)

var version = "dev"

func isPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

type serveVars struct {
	tab *qt.QWidget

	graspCheck      *qt.QCheckBox
	blossomCheck    *qt.QCheckBox
	negentropyCheck *qt.QCheckBox

	startButton *qt.QPushButton
	stopButton  *qt.QPushButton

	logsList   *qt.QListWidget
	eventsList *qt.QListWidget

	relay     *khatru.Relay
	db        *slicestore.SliceStore
	blobStore *xsync.MapOf[string, []byte]
	repoDir   string

	running bool
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

	// events
	eventsLabel := qt.NewQLabel2()
	eventsLabel.SetText("events:")
	layout.AddWidget(eventsLabel.QWidget)
	serve.eventsList = qt.NewQListWidget(serve.tab)
	serve.eventsList.SetMinimumHeight(200)
	layout.AddWidget(serve.eventsList.QWidget)

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

	serve.startButton.OnClicked(func() {
		serve.startRelay()
	})

	serve.stopButton.OnClicked(func() {
		serve.stopRelay()
	})

	return serve.tab
}

func (serve *serveVars) startRelay() {
	if serve.running {
		return
	}
	serve.running = true
	serve.startButton.SetEnabled(false)
	serve.stopButton.SetEnabled(true)

	serve.db = &slicestore.SliceStore{}
	serve.blobStore = xsync.NewMapOf[string, []byte]()

	rl := khatru.NewRelay()
	serve.relay = rl

	rl.Info.Name = "nak serve"
	rl.Info.Description = "a local relay for testing, debugging and development."
	rl.Info.Software = "https://github.com/fiatjaf/nak"
	rl.Info.Version = "dev"

	rl.UseEventstore(serve.db, 500)

	if serve.negentropyCheck.IsChecked() {
		rl.Negentropy = true
	}

	started := make(chan bool)
	exited := make(chan error)

	hostname := "localhost"
	port := 10547

	if serve.blossomCheck.IsChecked() {
		bs := blossom.New(rl, fmt.Sprintf("http://%s:%d", hostname, port))
		bs.Store = blossom.NewMemoryBlobIndex()

		bs.StoreBlob = func(ctx context.Context, sha256 string, ext string, body []byte) error {
			serve.blobStore.Store(sha256+ext, body)
			serve.log("blob stored: %s", sha256+ext)
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
			return nil
		}
	}

	if serve.graspCheck.IsChecked() {
		var err error
		serve.repoDir, err = os.MkdirTemp("", "vnak-serve-grasp-repos-")
		if err != nil {
			serve.log("failed to create grasp repos directory: %w", err)
			return
		}
		g := grasp.New(rl, serve.repoDir)
		g.OnRead = func(ctx context.Context, pubkey nostr.PubKey, repo string) (reject bool, reason string) {
			serve.log("git read by '%s' at '%s'", pubkey.Hex(), repo)
			return false, ""
		}
		g.OnWrite = func(ctx context.Context, pubkey nostr.PubKey, repo string) (reject bool, reason string) {
			serve.log("git write by '%s' at '%s'", pubkey.Hex(), repo)
			return false, ""
		}
	}

	go func() {
		err := rl.Start(hostname, port, started)
		exited <- err
	}()

	// relay logging
	rl.OnRequest = func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		negentropy := ""
		if khatru.IsNegentropySession(ctx) {
			negentropy = "negentropy "
		}

		serve.log("%srequest: %s", negentropy, filter)
		return false, ""
	}

	rl.OnCount = func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
		serve.log("count request: %s", filter)
		return false, ""
	}

	rl.OnEvent = func(ctx context.Context, event nostr.Event) (reject bool, msg string) {
		serve.log("event: %s", event)
		serve.addEvent(event)
		return false, ""
	}

	totalConnections := atomic.Int32{}
	rl.OnConnect = func(ctx context.Context) {
		totalConnections.Add(1)
		go func() {
			<-ctx.Done()
			totalConnections.Add(-1)
		}()
	}

	<-started
	serve.log("relay running at %s", fmt.Sprintf("ws://%s:%d", hostname, port))
	if serve.graspCheck.IsChecked() {
		serve.log("grasp repos at %s", serve.repoDir)
	}

	go func() {
		err := <-exited
		if err != nil {
			serve.log("relay exited with error: %s", err)
		}
		mainthread.Wait(func() {
			serve.running = false
			serve.startButton.SetEnabled(true)
			serve.stopButton.SetEnabled(false)
		})
	}()
}

func (serve *serveVars) stopRelay() {
	if !serve.running {
		return
	}
	if serve.relay != nil {
		serve.relay.Shutdown(ctx)
	}
	serve.running = false
	serve.startButton.SetEnabled(true)
	serve.stopButton.SetEnabled(false)
	statusLabel.SetText("relay stopped")
}

func (serve *serveVars) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	mainthread.Start(func() {
		item := qt.NewQListWidgetItem2(msg)
		serve.logsList.InsertItem(0, item)
		serve.logsList.ScrollToTop()
	})
}

func (serve *serveVars) addEvent(event nostr.Event) {
	jsonBytes, _ := json.Marshal(event)
	mainthread.Wait(func() {
		item := qt.NewQListWidgetItem2(string(jsonBytes))
		serve.eventsList.AddItemWithItem(item)
	})
}
