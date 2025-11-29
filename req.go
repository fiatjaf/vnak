package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	qt "github.com/mappu/miqt/qt6"
)

type reqVars struct {
	authorsEdits []*qt.QLineEdit
	idsEdits     []*qt.QLineEdit
	kindsEdits   []*qt.QLineEdit
	kindsLabels  []*qt.QLabel
	relaysEdits  []*qt.QLineEdit
	sinceEdit    *qt.QDateTimeEdit
	untilEdit    *qt.QDateTimeEdit
	limitSpin    *qt.QSpinBox

	filter nostr.Filter

	outputEdit  *qt.QTextEdit
	resultsEdit *qt.QTextEdit
}

var req = reqVars{}

func setupReqTab() *qt.QWidget {
	tab := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	tab.SetLayout(layout.QLayout)

	// authors
	authorsLabel := qt.NewQLabel2()
	authorsLabel.SetText("authors:")
	layout.AddWidget(authorsLabel.QWidget)
	authorsVBox := qt.NewQVBoxLayout2()
	layout.AddLayout(authorsVBox.QLayout)
	req.authorsEdits = []*qt.QLineEdit{}
	var addAuthorEdit func()
	addAuthorEdit = func() {
		edit := qt.NewQLineEdit(tab)
		req.authorsEdits = append(req.authorsEdits, edit)
		authorsVBox.AddWidget(edit.QWidget)
		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.authorsEdits[len(req.authorsEdits)-1] {
					addAuthorEdit()
				}
			} else {
				n := len(req.authorsEdits)
				if n >= 2 && strings.TrimSpace(req.authorsEdits[n-1].Text()) == "" && strings.TrimSpace(req.authorsEdits[n-2].Text()) == "" {
					authorsVBox.RemoveWidget(req.authorsEdits[n-1].QWidget)
					req.authorsEdits[n-1].DeleteLater()
					req.authorsEdits = req.authorsEdits[0 : n-1]
				}
			}
			updateReq()
		})
	}
	addAuthorEdit()

	// ids
	idsLabel := qt.NewQLabel2()
	idsLabel.SetText("ids:")
	layout.AddWidget(idsLabel.QWidget)
	idsVBox := qt.NewQVBoxLayout2()
	layout.AddLayout(idsVBox.QLayout)
	req.idsEdits = []*qt.QLineEdit{}
	var addIdEdit func()
	addIdEdit = func() {
		edit := qt.NewQLineEdit(tab)
		req.idsEdits = append(req.idsEdits, edit)
		idsVBox.AddWidget(edit.QWidget)
		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.idsEdits[len(req.idsEdits)-1] {
					addIdEdit()
				}
			} else {
				n := len(req.idsEdits)
				if n >= 2 && strings.TrimSpace(req.idsEdits[n-1].Text()) == "" && strings.TrimSpace(req.idsEdits[n-2].Text()) == "" {
					idsVBox.RemoveWidget(req.idsEdits[n-1].QWidget)
					req.idsEdits[n-1].DeleteLater()
					req.idsEdits = req.idsEdits[0 : n-1]
				}
			}
			updateReq()
		})
	}
	addIdEdit()

	// kinds
	kindsLabel := qt.NewQLabel2()
	kindsLabel.SetText("kinds:")
	layout.AddWidget(kindsLabel.QWidget)
	kindsVBox := qt.NewQVBoxLayout2()
	layout.AddLayout(kindsVBox.QLayout)
	req.kindsEdits = []*qt.QLineEdit{}
	req.kindsLabels = []*qt.QLabel{}
	var addKindEdit func()
	addKindEdit = func() {
		hbox := qt.NewQHBoxLayout2()
		kindsVBox.AddLayout(hbox.QLayout)
		edit := qt.NewQLineEdit(tab)
		req.kindsEdits = append(req.kindsEdits, edit)
		hbox.AddWidget(edit.QWidget)
		label := qt.NewQLabel2()
		req.kindsLabels = append(req.kindsLabels, label)
		hbox.AddWidget(label.QWidget)
		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.kindsEdits[len(req.kindsEdits)-1] {
					addKindEdit()
				}
			} else {
				n := len(req.kindsEdits)
				if n >= 2 && strings.TrimSpace(req.kindsEdits[n-1].Text()) == "" && strings.TrimSpace(req.kindsEdits[n-2].Text()) == "" {
					lastItem := kindsVBox.ItemAt(kindsVBox.Count() - 1)
					kindsVBox.RemoveItem(lastItem)
					lastHBox := lastItem.Layout()
					lastHBox.RemoveWidget(req.kindsEdits[n-1].QWidget)
					lastHBox.RemoveWidget(req.kindsLabels[n-1].QWidget)
					req.kindsEdits[n-1].DeleteLater()
					req.kindsLabels[n-1].DeleteLater()
					lastHBox.DeleteLater()
					req.kindsEdits = req.kindsEdits[0 : n-1]
					req.kindsLabels = req.kindsLabels[0 : n-1]
				}
			}
			updateReq()
		})
	}
	addKindEdit()

	// since
	sinceHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(sinceHBox.QLayout)
	sinceLabel := qt.NewQLabel2()
	sinceLabel.SetText("since:")
	sinceHBox.AddWidget(sinceLabel.QWidget)
	req.sinceEdit = qt.NewQDateTimeEdit(tab)
	{
		time := qt.NewQDateTime()
		time.SetMSecsSinceEpoch(0)
		req.sinceEdit.SetDateTime(time)
	}
	sinceHBox.AddWidget(req.sinceEdit.QWidget)
	req.sinceEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		updateReq()
	})

	// until
	untilHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(untilHBox.QLayout)
	untilLabel := qt.NewQLabel2()
	untilLabel.SetText("until:")
	untilHBox.AddWidget(untilLabel.QWidget)
	req.untilEdit = qt.NewQDateTimeEdit(tab)
	{
		time := qt.NewQDateTime()
		time.SetMSecsSinceEpoch(0)
		req.untilEdit.SetDateTime(time)
	}
	untilHBox.AddWidget(req.untilEdit.QWidget)
	req.untilEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		updateReq()
	})

	// limit
	limitHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(limitHBox.QLayout)
	limitLabel := qt.NewQLabel2()
	limitLabel.SetText("limit:")
	limitHBox.AddWidget(limitLabel.QWidget)
	req.limitSpin = qt.NewQSpinBox(tab)
	req.limitSpin.SetMinimum(0)
	req.limitSpin.SetMaximum(1000)
	limitHBox.AddWidget(req.limitSpin.QWidget)
	req.limitSpin.OnValueChanged(func(int) {
		updateReq()
	})

	// output
	outputLabel := qt.NewQLabel2()
	outputLabel.SetText("filter:")
	layout.AddWidget(outputLabel.QWidget)
	req.outputEdit = qt.NewQTextEdit(tab)
	req.outputEdit.SetReadOnly(true)
	layout.AddWidget(req.outputEdit.QWidget)

	// relays
	relaysLabel := qt.NewQLabel2()
	relaysLabel.SetText("relays:")
	layout.AddWidget(relaysLabel.QWidget)
	relaysVBox := qt.NewQVBoxLayout2()
	layout.AddLayout(relaysVBox.QLayout)
	req.relaysEdits = []*qt.QLineEdit{}
	var addRelayEdit func()
	addRelayEdit = func() {
		edit := qt.NewQLineEdit(tab)
		req.relaysEdits = append(req.relaysEdits, edit)
		relaysVBox.AddWidget(edit.QWidget)
		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.relaysEdits[len(req.relaysEdits)-1] {
					addRelayEdit()
				}
			} else {
				n := len(req.relaysEdits)
				if n >= 2 && strings.TrimSpace(req.relaysEdits[n-1].Text()) == "" && strings.TrimSpace(req.relaysEdits[n-2].Text()) == "" {
					relaysVBox.RemoveWidget(req.relaysEdits[n-1].QWidget)
					req.relaysEdits[n-1].DeleteLater()
					req.relaysEdits = req.relaysEdits[0 : n-1]
				}
			}
		})
	}
	addRelayEdit()

	// send button
	buttonHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(buttonHBox.QLayout)
	sendButton := qt.NewQPushButton5("send request", tab)
	buttonHBox.AddWidget(sendButton.QWidget)
	buttonHBox.AddStretch()

	// results
	resultsLabel := qt.NewQLabel2()
	resultsLabel.SetText("results:")
	layout.AddWidget(resultsLabel.QWidget)
	req.resultsEdit = qt.NewQTextEdit(tab)
	req.resultsEdit.SetReadOnly(true)
	layout.AddWidget(req.resultsEdit.QWidget)

	sendButton.OnClicked(func() {
		req.subscribe()
	})

	return tab
}

func updateReq() {
	req.filter = nostr.Filter{}

	// collect authors
	authors := []nostr.PubKey{}
	for _, edit := range req.authorsEdits {
		if pk, err := parsePubKey(strings.TrimSpace(edit.Text())); err == nil {
			authors = append(authors, pk)
		}
	}
	if len(authors) > 0 {
		req.filter.Authors = authors
	}

	// collect ids
	ids := []nostr.ID{}
	for _, edit := range req.idsEdits {
		if id, err := parseEventID(strings.TrimSpace(edit.Text())); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		req.filter.IDs = ids
	}

	// collect kinds
	kinds := []nostr.Kind{}
	for _, edit := range req.kindsEdits {
		text := strings.TrimSpace(edit.Text())
		if k, err := strconv.Atoi(text); err == nil {
			kinds = append(kinds, nostr.Kind(k))
		}
	}
	if len(kinds) > 0 {
		req.filter.Kinds = kinds
	}

	// update kind labels
	for i, kind := range kinds {
		if i < len(req.kindsLabels) {
			name := kind.Name()
			if name != "unknown" {
				req.kindsLabels[i].SetText(name)
			} else {
				req.kindsLabels[i].SetText("")
			}
		}
	}
	for i := len(kinds); i < len(req.kindsLabels); i++ {
		req.kindsLabels[i].SetText("")
	}

	// since
	if req.sinceEdit.DateTime().IsValid() {
		ts := nostr.Timestamp(req.sinceEdit.DateTime().ToMSecsSinceEpoch() / 1000)
		req.filter.Since = ts
	}

	// until
	if req.untilEdit.DateTime().IsValid() {
		ts := nostr.Timestamp(req.untilEdit.DateTime().ToMSecsSinceEpoch() / 1000)
		req.filter.Until = ts
	}

	// limit
	if req.limitSpin.Value() > 0 {
		req.filter.Limit = req.limitSpin.Value()
	}

	jsonBytes, _ := json.Marshal(req.filter)
	req.outputEdit.SetPlainText(string(jsonBytes))
}

func (req reqVars) subscribe() {
	// collect relays
	relays := []string{}
	for _, edit := range req.relaysEdits {
		url := strings.TrimSpace(edit.Text())
		if url != "" {
			relays = append(relays, url)
		}
	}
	if len(relays) == 0 {
		statusLabel.SetText("no relays specified")
		return
	}

	// subscribe
	var eoseChan chan struct{}
	var eventsChan chan nostr.Event

	if len(relays) == 1 {
		relay, err := sys.Pool.EnsureRelay(relays[0])
		if err != nil {
			statusLabel.SetText(fmt.Sprintf("failed to connect to %s: %s", niceRelayURL(relays[0]), err))
			return
		}

		statusLabel.SetText("subscribed to " + niceRelayURL(relay.URL))
		sub, err := relay.Subscribe(ctx, req.filter, nostr.SubscriptionOptions{
			Label: "nakv-req-1",
		})
		if err != nil {
			statusLabel.SetText(fmt.Sprintf("failed to subscribe to %s: %s", niceRelayURL(relay.URL), err))
			return
		}

		eventsChan = sub.Events
		eoseChan = sub.EndOfStoredEvents

		go func() {
			reason := <-sub.ClosedReason
			time.Sleep(time.Second)
			statusLabel.SetText(fmt.Sprintf("subscription closed: %s", reason))
		}()
	} else {
		statusLabel.SetText("subscribed to " + strings.Join(niceRelayURLs(relays), ", "))
		eoseChan = make(chan struct{})
		eventsChan = make(chan nostr.Event)

		go func() {
			for ie := range sys.Pool.SubscribeManyNotifyEOSE(ctx, relays, req.filter, eoseChan,
				nostr.SubscriptionOptions{
					Label: "nakv-req",
				},
			) {
				eventsChan <- ie.Event
			}
		}()
	}

	// collect events
	eosed := false
	go func() {
		for event := range eventsChan {
			jsonBytes, _ := json.Marshal(event)
			if eosed {
				req.resultsEdit.SetPlainText(string(jsonBytes) + "\n" + req.resultsEdit.ToPlainText())
			} else {
				req.resultsEdit.InsertPlainText("\n" + string(jsonBytes))
			}
		}
		statusLabel.SetText("subscription ended")
	}()

	go func() {
		<-eoseChan
		eosed = true
	}()
}
