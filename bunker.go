package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip46"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

type bunkerVars struct {
	tab *qt.QWidget

	loadProfileButton *qt.QPushButton
	startButton       *qt.QPushButton
	stopButton        *qt.QPushButton

	authorizedKeysList *qt.QListWidget
	logsList           *qt.QListWidget
	bunkerURIInput     *qt.QLineEdit

	relaysHBox  *qt.QHBoxLayout
	relaysEdits []*qt.QLineEdit

	config struct {
		AuthorizedKeys []nostr.PubKey      `json:"authorized-keys"`
		Secret         plainOrEncryptedKey `json:"sec"`
		Relays         []string            `json:"relays"`
	}

	relays    []*nostr.Relay
	signer    nip46.StaticKeySigner
	handlerWg sync.WaitGroup
	cancel    context.CancelFunc
	newSecret string
}

var bunkerTab = &bunkerVars{}

type plainOrEncryptedKey struct {
	Plain     *nostr.SecretKey
	Encrypted *string
}

func (pe plainOrEncryptedKey) MarshalJSON() ([]byte, error) {
	if pe.Plain != nil {
		res := make([]byte, 66)
		hex.Encode(res[1:], (*pe.Plain)[:])
		res[0] = '"'
		res[65] = '"'
		return res, nil
	} else if pe.Encrypted != nil {
		return json.Marshal(*pe.Encrypted)
	}

	return nil, fmt.Errorf("no key to marshal")
}

func (pe *plainOrEncryptedKey) UnmarshalJSON(buf []byte) error {
	if len(buf) == 66 {
		sk, err := nostr.SecretKeyFromHex(string(buf[1 : 1+64]))
		if err != nil {
			return err
		}
		pe.Plain = &sk
		return nil
	} else if bytes.HasPrefix(buf, []byte("\"nsec")) {
		_, v, err := nip19.Decode(string(buf[1 : len(buf)-1]))
		if err != nil {
			return err
		}
		sk := v.(nostr.SecretKey)
		pe.Plain = &sk
		return nil
	} else if bytes.HasPrefix(buf, []byte("\"ncryptsec1")) {
		ncryptsec := string(buf[1 : len(buf)-1])
		pe.Encrypted = &ncryptsec
		return nil
	}

	return fmt.Errorf("unrecognized key format '%s'", string(buf))
}

func setupBunkerTab() *qt.QWidget {
	bunkerTab.tab = qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	bunkerTab.tab.SetLayout(layout.QLayout)

	// buttons row
	buttonsHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(buttonsHBox.QLayout)

	bunkerTab.loadProfileButton = qt.NewQPushButton5("load profile", bunkerTab.tab)
	buttonsHBox.AddWidget(bunkerTab.loadProfileButton.QWidget)

	bunkerTab.startButton = qt.NewQPushButton5("start", bunkerTab.tab)
	buttonsHBox.AddWidget(bunkerTab.startButton.QWidget)

	bunkerTab.stopButton = qt.NewQPushButton5("stop", bunkerTab.tab)
	bunkerTab.stopButton.SetEnabled(false)
	buttonsHBox.AddWidget(bunkerTab.stopButton.QWidget)

	// bunker URI input
	uriHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(uriHBox.QLayout)

	uriLabel := qt.NewQLabel2()
	uriLabel.SetText("bunker URI:")
	uriHBox.AddWidget(uriLabel.QWidget)

	bunkerTab.bunkerURIInput = qt.NewQLineEdit(bunkerTab.tab)
	bunkerTab.bunkerURIInput.SetReadOnly(true)
	uriHBox.AddWidget(bunkerTab.bunkerURIInput.QWidget)

	// relays
	relaysHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(relaysHBox.QLayout)
	bunkerTab.relaysHBox = relaysHBox
	relaysLabel := qt.NewQLabel2()
	relaysLabel.SetText("relays:")
	relaysHBox.AddWidget(relaysLabel.QWidget)

	bunkerTab.relaysEdits = []*qt.QLineEdit{}
	bunkerTab.addRelay("")

	// tables
	tablesHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(tablesHBox.QLayout)

	// authorized keys list
	keysVBox := qt.NewQVBoxLayout2()
	keysLabel := qt.NewQLabel2()
	keysLabel.SetText("authorized keys:")
	keysVBox.AddWidget(keysLabel.QWidget)

	bunkerTab.authorizedKeysList = qt.NewQListWidget(bunkerTab.tab)
	keysVBox.AddWidget(bunkerTab.authorizedKeysList.QWidget)
	tablesHBox.AddLayout(keysVBox.QLayout)

	// logs list
	logsVBox := qt.NewQVBoxLayout2()
	logsLabel := qt.NewQLabel2()
	logsLabel.SetText("logs:")
	logsVBox.AddWidget(logsLabel.QWidget)

	bunkerTab.logsList = qt.NewQListWidget(bunkerTab.tab)
	logsVBox.AddWidget(bunkerTab.logsList.QWidget)
	tablesHBox.AddLayout(logsVBox.QLayout)

	// connect signals
	bunkerTab.loadProfileButton.OnClicked(bunkerTab.loadProfile)
	bunkerTab.startButton.OnClicked(bunkerTab.startBunker)
	bunkerTab.stopButton.OnClicked(bunkerTab.stopBunker)

	return bunkerTab.tab
}

func (b *bunkerVars) loadProfile() {
	fileDialog := qt.NewQFileDialog(window.QWidget)
	fileDialog.SetFileMode(qt.QFileDialog__ExistingFile)
	fileDialog.SetWindowTitle("select bunker profile")
	fileDialog.SetAcceptDrops(true)

	if fileDialog.Exec() == int(qt.QDialog__Accepted) {
		selectedFiles := fileDialog.SelectedFiles()
		if len(selectedFiles) > 0 {
			filename := selectedFiles[0]
			b.loadProfileFromFile(filename)
		}
	}
}

func (b *bunkerVars) loadProfileFromFile(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		setStatus(tabs.bunker, "failed to read profile: %v", err)
		return
	}

	if err := json.Unmarshal(data, &b.config); err != nil {
		setStatus(tabs.bunker, "failed to parse profile: %v", err)
		return
	}

	// normalize relays
	for i, url := range b.config.Relays {
		b.config.Relays[i] = nostr.NormalizeURL(url)
	}

	// set currentSec and currentKeyer if secret is available
	if b.config.Secret.Plain != nil {
		secEdit.SetText(b.config.Secret.Plain.Hex())
	} else if b.config.Secret.Encrypted != nil {
		secEdit.SetText(*b.config.Secret.Encrypted)
	}
	keyChanged(secEdit.Text())

	b.updateAuthorizedKeysTable()
	b.updateRelaysUI()
	setStatus(tabs.bunker, "loaded profile from %s", filepath.Base(filename))
}

func (b *bunkerVars) updateAuthorizedKeysTable() {
	mainthread.Start(func() {
		b.authorizedKeysList.Clear()
		for _, pubkey := range b.config.AuthorizedKeys {
			npub := nip19.EncodeNpub(pubkey)
			item := qt.NewQListWidgetItem2(fmt.Sprintf("%s %s", pubkey.Hex(), npub))
			b.authorizedKeysList.AddItemWithItem(item)
		}
	})
}

func (b *bunkerVars) addRelay(value string) {
	edit := qt.NewQLineEdit(b.tab)
	edit.SetText(value)
	b.relaysEdits = append(b.relaysEdits, edit)
	b.relaysHBox.AddWidget(edit.QWidget)
	edit.OnTextChanged(func(text string) {
		if strings.TrimSpace(text) != "" {
			if edit == b.relaysEdits[len(b.relaysEdits)-1] {
				b.addRelay("")
			}
		} else {
			n := len(b.relaysEdits)
			if n >= 2 && strings.TrimSpace(b.relaysEdits[n-1].Text()) == "" && strings.TrimSpace(b.relaysEdits[n-2].Text()) == "" {
				b.relaysHBox.RemoveWidget(b.relaysEdits[n-1].QWidget)
				b.relaysEdits[n-1].DeleteLater()
				b.relaysEdits = b.relaysEdits[0 : n-1]
			}
		}
		b.updateConfigRelays()
	})
}

func (b *bunkerVars) updateConfigRelays() {
	b.config.Relays = []string{}
	for _, edit := range b.relaysEdits {
		text := strings.TrimSpace(edit.Text())
		if text != "" {
			b.config.Relays = append(b.config.Relays, text)
		}
	}
}

func (b *bunkerVars) updateRelaysUI() {
	mainthread.Start(func() {
		// clear existing
		for _, edit := range b.relaysEdits {
			b.relaysHBox.RemoveWidget(edit.QWidget)
			edit.DeleteLater()
		}
		b.relaysEdits = []*qt.QLineEdit{}
		// add from config
		for _, relay := range b.config.Relays {
			b.addRelay(relay)
		}
		b.addRelay("") // extra
	})
}

func (bunker *bunkerVars) log(message string, args ...any) {
	msg := fmt.Sprintf(message, args...)
	timestamp := time.Now().Format("15:04:05")
	mainthread.Start(func() {
		pos := bunker.logsList.VerticalScrollBar().SliderPosition()
		bunker.logsList.InsertItem(0, qt.NewQListWidgetItem2(fmt.Sprintf("%s %s", timestamp, msg)))
		if pos == 0 {
			bunker.logsList.ScrollToTop()
		}
	})
}

func (bunker *bunkerVars) startBunker() {
	if len(bunker.config.Relays) == 0 {
		setStatus(tabs.bunker, "no relays configured")
		return
	}

	if currentSec == [32]byte{} {
		setStatus(tabs.bunker, "no secret key configured")
		return
	}

	bunker.startButton.SetEnabled(false)
	bunker.stopButton.SetEnabled(true)

	ctx, cancel := context.WithCancel(context.Background())
	bunker.cancel = cancel

	// generate new secret for bunker URI
	bunker.newSecret = randString(12)

	// connect to relays
	bunker.relays = connectToAllRelays(ctx, bunker, bunker.config.Relays, nil, nostr.PoolOptions{})
	if len(bunker.relays) == 0 {
		setStatus(tabs.bunker, "failed to connect to any relays")
		bunker.stopBunker()
		return
	}

	// update bunker URI
	bunker.updateBunkerURI()

	// setup signer
	bunker.signer = nip46.NewStaticKeySigner(currentSec)

	// setup authorization
	bunker.signer.AuthorizeRequest = func(harmless bool, from nostr.PubKey, secret string) bool {
		if secret == bunker.newSecret {
			// store this key
			bunker.config.AuthorizedKeys = appendUnique(bunker.config.AuthorizedKeys, from)
			// discard this and generate a new secret
			bunker.newSecret = randString(12)
			// update URI
			bunker.updateBunkerURI()
			// update table
			bunker.updateAuthorizedKeysTable()
			bunker.log("new client authorized: %s", from.Hex())
			return true
		}

		authorized := slices.Contains(bunker.config.AuthorizedKeys, from) || slices.Contains([]string{}, secret) // TODO: authorized secrets
		if authorized {
			if harmless {
				bunker.log("harmless request from %s", from.Hex())
			} else {
				bunker.log("signing request from %s", from.Hex())
			}
		}
		return authorized
	}

	// subscribe to requests
	pubkey := currentSec.Public()
	relayURLs := make([]string, len(bunker.relays))
	for i, relay := range bunker.relays {
		relayURLs[i] = relay.URL
	}
	events := sys.Pool.SubscribeMany(ctx, relayURLs, nostr.Filter{
		Kinds:     []nostr.Kind{nostr.KindNostrConnect},
		Tags:      nostr.TagMap{"p": []string{pubkey.Hex()}},
		Since:     nostr.Now(),
		LimitZero: true,
	}, nostr.SubscriptionOptions{
		Label: "vnak-bunker",
	})

	bunker.log("bunker started")

	go func() {
		for ie := range events {
			_, _, eventResponse, err := bunker.signer.HandleRequest(ctx, ie.Event)
			if err != nil {
				bunker.log("failed to handle request from %s: %s", ie.Event.PubKey, err.Error())
				continue
			}

			bunker.log("request from %s", ie.Event.PubKey.Hex())

			bunker.handlerWg.Add(len(bunker.relays))
			for _, relay := range bunker.relays {
				go func(relay *nostr.Relay) {
					defer bunker.handlerWg.Done()
					if relay != nil {
						err := relay.Publish(ctx, eventResponse)
						if err != nil {
							bunker.log("failed to send response: %s", err)
						} else {
							bunker.log("sent response through %s", relay.URL)
						}
					}
				}(relay)
			}
			bunker.handlerWg.Wait()
		}
	}()
}

func (bunker *bunkerVars) stopBunker() {
	if bunker.cancel != nil {
		bunker.cancel()
	}
	bunker.startButton.SetEnabled(true)
	bunker.stopButton.SetEnabled(false)
	bunker.bunkerURIInput.SetText("")
	bunker.log("bunker stopped")
}

func (bunker *bunkerVars) updateBunkerURI() {
	if currentSec == [32]byte{} {
		return
	}

	pubkey := currentSec.Public()
	qs := url.Values{}
	for _, relay := range bunker.relays {
		qs.Add("relay", relay.URL)
	}
	qs.Set("secret", bunker.newSecret)
	bunkerURI := fmt.Sprintf("bunker://%s?%s", pubkey.Hex(), qs.Encode())

	mainthread.Start(func() {
		bunker.bunkerURIInput.SetText(bunkerURI)
	})
}

func connectToAllRelays(ctx context.Context, b *bunkerVars, relayURLs []string, onConnect func(*nostr.Relay), opts nostr.PoolOptions) []*nostr.Relay {
	relays := make([]*nostr.Relay, 0, len(relayURLs))
	for _, relayURL := range relayURLs {
		relay, err := sys.Pool.EnsureRelay(relayURL)
		if err != nil {
			b.log("failed to connect to %s: %v", relayURL, err)
			continue
		}
		relays = append(relays, relay)
		if onConnect != nil {
			onConnect(relay)
		}
		b.log("connected to %s", relayURL)
	}
	return relays
}
