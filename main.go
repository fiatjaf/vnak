package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"fiatjaf.com/lib/debouncer"
	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip49"
	"fiatjaf.com/nostr/sdk"
	qt "github.com/mappu/miqt/qt6"
)

var (
	app       *qt.QApplication
	window    *qt.QMainWindow
	tabWidget *qt.QTabWidget

	currentSec   nostr.SecretKey
	currentKeyer nostr.Keyer
	tabIndexes   struct {
		event int
		req   int
		paste int
		serve int
	}
	statusLabel *qt.QLabel

	debounced = debouncer.New(950 * time.Millisecond)
	sys       = sdk.NewSystem()
	ctx       = context.Background()

	debug      = flag.Bool("debug", false, "enable debug mode")
	initialTab = flag.String("tab", "paste", "tab to open initially")
)

func main() {
	flag.Parse()

	// nostr setup
	httpHeader := http.Header{}
	httpHeader.Set("User-Agent", "vnak")
	sys.Pool = nostr.NewPool(nostr.PoolOptions{
		AuthorKindQueryMiddleware: sys.TrackQueryAttempts,
		EventMiddleware:           sys.TrackEventHintsAndRelays,
		DuplicateMiddleware:       sys.TrackEventRelaysD,
		PenaltyBox:                false,
		AuthHandler: func(ctx context.Context, evt *nostr.Event) error {
			if currentKeyer != nil {
				err := currentKeyer.SignEvent(ctx, evt)
				if err != nil {
					return fmt.Errorf("failed to sign auth event: %w", err)
				}
				return nil
			}
			return fmt.Errorf("can't sign auth event, no key")
		},
		RelayOptions: nostr.RelayOptions{
			RequestHeader: httpHeader,
		},
	})

	// UI setup
	app = qt.NewQApplication(os.Args)

	window = qt.NewQMainWindow2()

	if *debug {
		window.SetWindowFlag(qt.WindowStaysOnTopHint | qt.Dialog)
	}

	window.SetMinimumSize2(800, 600)
	window.SetWindowTitle("vnak")

	centralWidget := qt.NewQWidget(window.QWidget)
	window.SetCentralWidget(centralWidget)

	mainLayout := qt.NewQVBoxLayout2()
	centralWidget.SetLayout(mainLayout.QLayout)

	// private key input
	secLabel := qt.NewQLabel2()
	secLabel.SetText("private key (hex or nsec):")
	mainLayout.AddWidget(secLabel.QWidget)

	secHBox := qt.NewQHBoxLayout2()
	mainLayout.AddLayout(secHBox.QLayout)
	secEdit := qt.NewQLineEdit(centralWidget)
	secHBox.AddWidget(secEdit.QWidget)
	generateButton := qt.NewQPushButton5("generate", centralWidget)
	secHBox.AddWidget(generateButton.QWidget)

	// password input
	passwordHBox := qt.NewQHBoxLayout2()
	passwordWidget := qt.NewQWidget(centralWidget)
	passwordWidget.SetLayout(passwordHBox.QLayout)
	passwordWidget.SetVisible(false)
	mainLayout.AddWidget(passwordWidget)
	passwordLabel := qt.NewQLabel2()
	passwordLabel.SetText("password:")
	passwordHBox.AddWidget(passwordLabel.QWidget)
	secPasswordEdit := qt.NewQLineEdit(passwordWidget)
	secPasswordEdit.SetEchoMode(qt.QLineEdit__Password)
	passwordHBox.AddWidget(secPasswordEdit.QWidget)
	keyChanged := func(text string) {
		text = strings.TrimSpace(text)

		var sk nostr.SecretKey
		var keyer nostr.Keyer
		var err error

		if text == "" {
			passwordWidget.SetVisible(false)
			goto empty
		}

		if strings.HasPrefix(text, "ncryptsec1") {
			passwordWidget.SetVisible(true)
			password := secPasswordEdit.Text()
			if password != "" {
				sk, err = nip49.Decrypt(text, password)
				if err != nil {
					statusLabel.SetText("decryption failed: " + err.Error())
					goto empty
				}
				text = hex.EncodeToString(sk[:])
			} else {
				goto empty
			}
		} else {
			passwordWidget.SetVisible(false)
		}

		sk, keyer, err = handleSecretKeyOrBunker(text)
		if err != nil {
			statusLabel.SetText(err.Error())
			currentSec = nostr.SecretKey{}
			currentKeyer = nil
			return
		}

		currentSec = sk
		currentKeyer = keyer
		statusLabel.SetText("")
		event.updateEvent()
		return

	empty:
		currentSec = nostr.SecretKey{}
		currentKeyer = nil
		statusLabel.SetText("")
		return
	}
	secEdit.OnTextChanged(keyChanged)
	secPasswordEdit.OnTextChanged(keyChanged)
	generateButton.OnClicked(func() {
		sk := nostr.Generate()
		nsec := nip19.EncodeNsec(sk)
		secEdit.SetText(nsec)
		keyChanged(nsec)
	})

	tabWidget = qt.NewQTabWidget(centralWidget)

	eventTab := setupEventTab()
	reqTab := setupReqTab()
	pasteTab := setupPasteTab()
	serveTab := setupServeTab()

	tabWidget.AddTab(eventTab, "event")
	tabIndexes.event = 0

	tabWidget.AddTab(reqTab, "req")
	tabIndexes.req = 1

	tabWidget.AddTab(pasteTab, "paste")
	tabIndexes.paste = 2

	tabWidget.AddTab(serveTab, "serve")
	tabIndexes.serve = 3

	switch *initialTab {
	case "event":
		tabWidget.SetCurrentIndex(tabIndexes.event)
	case "req":
		tabWidget.SetCurrentIndex(tabIndexes.req)
	case "paste":
		tabWidget.SetCurrentIndex(tabIndexes.paste)
	case "serve":
		tabWidget.SetCurrentIndex(tabIndexes.serve)
	default:
		tabWidget.SetCurrentIndex(0)
	}

	mainLayout.AddWidget(tabWidget.QWidget)

	statusLabel = qt.NewQLabel2()
	mainLayout.AddWidget(statusLabel.QWidget)

	// initial render
	event.updateEvent()
	req.updateReq()
	paste.updatePaste()

	window.Show()
	qt.QApplication_Exec()
}
