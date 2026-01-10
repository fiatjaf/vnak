package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fiatjaf.com/lib/debouncer"
	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip49"
	"fiatjaf.com/nostr/sdk"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

var (
	app       *qt.QApplication
	window    *qt.QMainWindow
	tabWidget *qt.QTabWidget

	keyChanged   func(string)
	secEdit      *qt.QLineEdit
	currentSec   nostr.SecretKey
	currentKeyer nostr.Keyer

	tabs struct {
		event  int
		req    int
		paste  int
		serve  int
		bunker int
		build  int
	}

	debounced = debouncer.New(950 * time.Millisecond)
	sys       = sdk.NewSystem()
	ctx       = context.Background()

	debug      = flag.Bool("debug", false, "enable debug mode")
	initialTab = flag.String("tab", "paste", "tab to open initially")
)

type AppState struct {
	Tab string `json:"tab"`
}

func getCacheFilePath() string {
	cacheDir, _ := os.UserCacheDir()
	os.MkdirAll(filepath.Join(cacheDir, "vnak"), 0755)
	return filepath.Join(cacheDir, "vnak", "_state.json")
}

func saveState(tabName string) error {
	state := AppState{Tab: tabName}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	cachePath := getCacheFilePath()
	return os.WriteFile(cachePath, data, 0644)
}

func loadState() (AppState, error) {
	cachePath := getCacheFilePath()
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return AppState{Tab: "paste"}, nil
		}
		return AppState{}, err
	}

	var state AppState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return AppState{}, err
	}

	return state, nil
}

func getTabName(index int) string {
	switch index {
	case tabs.event:
		return "event"
	case tabs.req:
		return "req"
	case tabs.paste:
		return "paste"
	case tabs.serve:
		return "serve"
	case tabs.bunker:
		return "bunker"
	case tabs.build:
		return "build"
	default:
		return "paste"
	}
}

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
		AuthRequiredHandler:       signAuthEvent,
		RelayOptions: nostr.RelayOptions{
			RequestHeader: httpHeader,
			AuthHandler:   signAuthEvent,
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
	secEdit = qt.NewQLineEdit(centralWidget)
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
	keyChanged = func(_ string) {
		debounced.Call(func() {
			mainthread.Start(func() {
				text := strings.TrimSpace(secEdit.Text())

				var sk nostr.SecretKey
				var keyer nostr.Keyer
				var err error

				if text == "" {
					passwordWidget.SetVisible(false)
					currentSec = nostr.SecretKey{}
					currentKeyer = nil
					statusLabel.SetText("")
				}

				if strings.HasPrefix(text, "ncryptsec1") {
					passwordWidget.SetVisible(true)
					password := secPasswordEdit.Text()
					if password != "" {
						sk, err = nip49.Decrypt(text, password)
						if err == nil {
							statusLabel.SetText(fmt.Sprintf("secret key decrypted, pubkey: " + sk.Public().Hex()))
						} else {
							currentSec = nostr.SecretKey{}
							currentKeyer = nil
							statusLabel.SetText("decryption failed: " + err.Error())
							return
						}
						text = hex.EncodeToString(sk[:])
					} else {
						currentSec = nostr.SecretKey{}
						currentKeyer = nil
						statusLabel.SetText("")
						return
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
				event.updateEvent()
				return
			})
		})
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
	tabWidget.OnCurrentChanged(func(index int) {
		tabName := getTabName(index)
		go saveState(tabName)
		maybeSetStatusOnTab(index)
	})

	eventTab := setupEventTab()
	reqTab := setupReqTab()
	pasteTab := setupPasteTab()
	serveTab := setupServeTab()
	bunkerTab := setupBunkerTab()
	buildTab := setupBuildTab()

	tabWidget.AddTab(eventTab, "event")
	tabs.event = 0

	tabWidget.AddTab(reqTab, "req")
	tabs.req = 1

	tabWidget.AddTab(pasteTab, "paste")
	tabs.paste = 2

	tabWidget.AddTab(serveTab, "serve")
	tabs.serve = 3

	tabWidget.AddTab(bunkerTab, "bunker")
	tabs.bunker = 4

	tabWidget.AddTab(buildTab, "build")
	tabs.build = 5

	// Load saved state or use initial tab
	savedState, err := loadState()
	if err == nil && savedState.Tab != "" {
		// Use saved tab
		switch savedState.Tab {
		case "event":
			tabWidget.SetCurrentIndex(tabs.event)
		case "req":
			tabWidget.SetCurrentIndex(tabs.req)
		case "paste":
			tabWidget.SetCurrentIndex(tabs.paste)
		case "serve":
			tabWidget.SetCurrentIndex(tabs.serve)
		case "bunker":
			tabWidget.SetCurrentIndex(tabs.bunker)
		case "build":
			tabWidget.SetCurrentIndex(tabs.build)
		default:
			tabWidget.SetCurrentIndex(tabs.paste)
		}
	} else {
		// Use initial tab flag
		switch *initialTab {
		case "event":
			tabWidget.SetCurrentIndex(tabs.event)
		case "req":
			tabWidget.SetCurrentIndex(tabs.req)
		case "paste":
			tabWidget.SetCurrentIndex(tabs.paste)
		case "serve":
			tabWidget.SetCurrentIndex(tabs.serve)
		case "bunker":
			tabWidget.SetCurrentIndex(tabs.bunker)
		case "build":
			tabWidget.SetCurrentIndex(tabs.build)
		default:
			tabWidget.SetCurrentIndex(0)
		}
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
