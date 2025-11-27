package main

import (
	"context"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"fiatjaf.com/lib/debouncer"
	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip49"
	"fiatjaf.com/nostr/sdk"
	"github.com/therecipe/qt/widgets"
)

var (
	currentSec   nostr.SecretKey
	currentKeyer nostr.Keyer
	statusLabel  *widgets.QLabel
	debounced    = debouncer.New(800 * time.Millisecond)
	sys          = sdk.NewSystem()
	ctx          = context.Background()
)

func main() {
	app := widgets.NewQApplication(len(os.Args), os.Args)

	window := widgets.NewQMainWindow(nil, 0)
	window.SetMinimumSize2(800, 600)
	window.SetWindowTitle("nakv")

	centralWidget := widgets.NewQWidget(nil, 0)
	window.SetCentralWidget(centralWidget)

	mainLayout := widgets.NewQVBoxLayout()
	centralWidget.SetLayout(mainLayout)

	// private key input
	secLabel := widgets.NewQLabel2("private key (hex or nsec):", nil, 0)
	mainLayout.AddWidget(secLabel, 0, 0)

	secHBox := widgets.NewQHBoxLayout()
	mainLayout.AddLayout(secHBox, 0)
	secEdit := widgets.NewQLineEdit(nil)
	secHBox.AddWidget(secEdit, 0, 0)
	generateButton := widgets.NewQPushButton2("generate", nil)
	secHBox.AddWidget(generateButton, 0, 0)

	// password input
	passwordHBox := widgets.NewQHBoxLayout()
	passwordWidget := widgets.NewQWidget(nil, 0)
	passwordWidget.SetLayout(passwordHBox)
	passwordWidget.SetVisible(false)
	mainLayout.AddWidget(passwordWidget, 0, 0)
	passwordLabel := widgets.NewQLabel2("password:", nil, 0)
	passwordHBox.AddWidget(passwordLabel, 0, 0)
	secPasswordEdit := widgets.NewQLineEdit(nil)
	secPasswordEdit.SetEchoMode(widgets.QLineEdit__Password)
	passwordHBox.AddWidget(secPasswordEdit, 0, 0)
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
		updateEvent()
		return

	empty:
		currentSec = nostr.SecretKey{}
		currentKeyer = nil
		statusLabel.SetText("")
		return
	}
	secEdit.ConnectTextChanged(keyChanged)
	secPasswordEdit.ConnectTextChanged(keyChanged)
	generateButton.ConnectClicked(func(bool) {
		sk := nostr.Generate()
		nsec := nip19.EncodeNsec(sk)
		secEdit.SetText(nsec)
		keyChanged(nsec)
	})

	tabWidget := widgets.NewQTabWidget(nil)

	eventTab := setupEventTab()
	reqTab := setupReqTab()

	tabWidget.AddTab(eventTab, "event")
	tabWidget.AddTab(reqTab, "req")

	mainLayout.AddWidget(tabWidget, 0, 0)

	statusLabel = widgets.NewQLabel2("", nil, 0)
	mainLayout.AddWidget(statusLabel, 0, 0)

	// initial render
	updateEvent()
	updateReq()

	window.Show()
	app.Exec()
}
