package main

import (
	"encoding/json"
	"strconv"
	"strings"

	"fiatjaf.com/nostr"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

type reqVars struct {
	authorsEdits []*widgets.QLineEdit
	idsEdits     []*widgets.QLineEdit
	kindsEdits   []*widgets.QLineEdit
	kindsLabels  []*widgets.QLabel
	relaysEdits  []*widgets.QLineEdit
	sinceEdit    *widgets.QDateTimeEdit
	untilEdit    *widgets.QDateTimeEdit
	limitSpin    *widgets.QSpinBox

	filter nostr.Filter

	outputEdit  *widgets.QTextEdit
	resultsEdit *widgets.QTextEdit
}

var req = reqVars{}

func setupReqTab() *widgets.QWidget {
	tab := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQVBoxLayout()
	tab.SetLayout(layout)

	// authors
	authorsLabel := widgets.NewQLabel2("authors:", nil, 0)
	layout.AddWidget(authorsLabel, 0, 0)
	authorsVBox := widgets.NewQVBoxLayout()
	layout.AddLayout(authorsVBox, 0)
	req.authorsEdits = []*widgets.QLineEdit{}
	var addAuthorEdit func()
	addAuthorEdit = func() {
		edit := widgets.NewQLineEdit(nil)
		req.authorsEdits = append(req.authorsEdits, edit)
		authorsVBox.AddWidget(edit, 0, 0)
		edit.ConnectTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.authorsEdits[len(req.authorsEdits)-1] {
					addAuthorEdit()
				}
			} else {
				n := len(req.authorsEdits)
				if n >= 2 && strings.TrimSpace(req.authorsEdits[n-1].Text()) == "" && strings.TrimSpace(req.authorsEdits[n-2].Text()) == "" {
					authorsVBox.Layout().RemoveWidget(req.authorsEdits[n-1])
					req.authorsEdits[n-1].DeleteLater()
					req.authorsEdits = req.authorsEdits[0 : n-1]
				}
			}
			updateReq()
		})
	}
	addAuthorEdit()

	// ids
	idsLabel := widgets.NewQLabel2("ids:", nil, 0)
	layout.AddWidget(idsLabel, 0, 0)
	idsVBox := widgets.NewQVBoxLayout()
	layout.AddLayout(idsVBox, 0)
	req.idsEdits = []*widgets.QLineEdit{}
	var addIdEdit func()
	addIdEdit = func() {
		edit := widgets.NewQLineEdit(nil)
		req.idsEdits = append(req.idsEdits, edit)
		idsVBox.AddWidget(edit, 0, 0)
		edit.ConnectTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.idsEdits[len(req.idsEdits)-1] {
					addIdEdit()
				}
			} else {
				n := len(req.idsEdits)
				if n >= 2 && strings.TrimSpace(req.idsEdits[n-1].Text()) == "" && strings.TrimSpace(req.idsEdits[n-2].Text()) == "" {
					idsVBox.Layout().RemoveWidget(req.idsEdits[n-1])
					req.idsEdits[n-1].DeleteLater()
					req.idsEdits = req.idsEdits[0 : n-1]
				}
			}
			updateReq()
		})
	}
	addIdEdit()

	// kinds
	kindsLabel := widgets.NewQLabel2("kinds:", nil, 0)
	layout.AddWidget(kindsLabel, 0, 0)
	kindsVBox := widgets.NewQVBoxLayout()
	layout.AddLayout(kindsVBox, 0)
	req.kindsEdits = []*widgets.QLineEdit{}
	req.kindsLabels = []*widgets.QLabel{}
	var addKindEdit func()
	addKindEdit = func() {
		hbox := widgets.NewQHBoxLayout()
		kindsVBox.AddLayout(hbox, 0)
		edit := widgets.NewQLineEdit(nil)
		req.kindsEdits = append(req.kindsEdits, edit)
		hbox.AddWidget(edit, 0, 0)
		label := widgets.NewQLabel2("", nil, 0)
		req.kindsLabels = append(req.kindsLabels, label)
		hbox.AddWidget(label, 0, 0)
		edit.ConnectTextChanged(func(text string) {
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
					lastHBox.RemoveWidget(req.kindsEdits[n-1])
					lastHBox.RemoveWidget(req.kindsLabels[n-1])
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
	sinceHBox := widgets.NewQHBoxLayout()
	layout.AddLayout(sinceHBox, 0)
	sinceLabel := widgets.NewQLabel2("since:", nil, 0)
	sinceHBox.AddWidget(sinceLabel, 0, 0)
	req.sinceEdit = widgets.NewQDateTimeEdit(nil)
	{
		time := core.NewQDateTime3(core.NewQDate3(0, 0, 0), core.NewQTime3(0, 0, 0, 0), 0)
		time.SetMSecsSinceEpoch(0)
		req.sinceEdit.SetDateTime(time)
	}
	sinceHBox.AddWidget(req.sinceEdit, 0, 0)
	req.sinceEdit.ConnectDateTimeChanged(func(*core.QDateTime) {
		updateReq()
	})

	// until
	untilHBox := widgets.NewQHBoxLayout()
	layout.AddLayout(untilHBox, 0)
	untilLabel := widgets.NewQLabel2("until:", nil, 0)
	untilHBox.AddWidget(untilLabel, 0, 0)
	req.untilEdit = widgets.NewQDateTimeEdit(nil)
	{
		time := core.NewQDateTime3(core.NewQDate3(0, 0, 0), core.NewQTime3(0, 0, 0, 0), 0)
		time.SetMSecsSinceEpoch(0)
		req.untilEdit.SetDateTime(time)
	}
	untilHBox.AddWidget(req.untilEdit, 0, 0)
	req.untilEdit.ConnectDateTimeChanged(func(*core.QDateTime) {
		updateReq()
	})

	// limit
	limitHBox := widgets.NewQHBoxLayout()
	layout.AddLayout(limitHBox, 0)
	limitLabel := widgets.NewQLabel2("limit:", nil, 0)
	limitHBox.AddWidget(limitLabel, 0, 0)
	req.limitSpin = widgets.NewQSpinBox(nil)
	req.limitSpin.SetMinimum(0)
	req.limitSpin.SetMaximum(1000)
	limitHBox.AddWidget(req.limitSpin, 0, 0)
	req.limitSpin.ConnectValueChanged(func(int) {
		updateReq()
	})

	// output
	outputLabel := widgets.NewQLabel2("filter:", nil, 0)
	layout.AddWidget(outputLabel, 0, 0)
	req.outputEdit = widgets.NewQTextEdit(nil)
	req.outputEdit.SetReadOnly(true)
	layout.AddWidget(req.outputEdit, 0, 0)

	// relays
	relaysLabel := widgets.NewQLabel2("relays:", nil, 0)
	layout.AddWidget(relaysLabel, 0, 0)
	relaysVBox := widgets.NewQVBoxLayout()
	layout.AddLayout(relaysVBox, 0)
	req.relaysEdits = []*widgets.QLineEdit{}
	var addRelayEdit func()
	addRelayEdit = func() {
		edit := widgets.NewQLineEdit(nil)
		req.relaysEdits = append(req.relaysEdits, edit)
		relaysVBox.AddWidget(edit, 0, 0)
		edit.ConnectTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.relaysEdits[len(req.relaysEdits)-1] {
					addRelayEdit()
				}
			} else {
				n := len(req.relaysEdits)
				if n >= 2 && strings.TrimSpace(req.relaysEdits[n-1].Text()) == "" && strings.TrimSpace(req.relaysEdits[n-2].Text()) == "" {
					relaysVBox.Layout().RemoveWidget(req.relaysEdits[n-1])
					req.relaysEdits[n-1].DeleteLater()
					req.relaysEdits = req.relaysEdits[0 : n-1]
				}
			}
		})
	}
	addRelayEdit()

	// send button
	buttonHBox := widgets.NewQHBoxLayout()
	layout.AddLayout(buttonHBox, 0)
	sendButton := widgets.NewQPushButton2("send request", nil)
	buttonHBox.AddWidget(sendButton, 0, 0)
	buttonHBox.AddStretch(1)

	// results
	resultsLabel := widgets.NewQLabel2("results:", nil, 0)
	layout.AddWidget(resultsLabel, 0, 0)
	req.resultsEdit = widgets.NewQTextEdit(nil)
	req.resultsEdit.SetReadOnly(true)
	layout.AddWidget(req.resultsEdit, 0, 0)

	sendButton.ConnectClicked(func(checked bool) {
		req.subscribe()
	})

	return tab
}

func updateReq() {
	req.filter = nostr.Filter{}

	// collect authors
	authors := []nostr.PubKey{}
	for _, edit := range req.authorsEdits {
		if pk, err := nostr.PubKeyFromHex(strings.TrimSpace(edit.Text())); err == nil {
			authors = append(authors, pk)
		}
	}
	if len(authors) > 0 {
		req.filter.Authors = authors
	}

	// collect ids
	ids := []nostr.ID{}
	for _, edit := range req.idsEdits {
		if id, err := nostr.IDFromHex(strings.TrimSpace(edit.Text())); err == nil {
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
	statusLabel.SetText("subscribed to " + strings.Join(relays, " "))
	eoseChan := make(chan struct{})
	eventsChan := sys.Pool.SubscribeManyNotifyEOSE(ctx, relays, req.filter, eoseChan, nostr.SubscriptionOptions{
		Label: "nakv-req",
	})

	// collect events
	go func() {
		eosed := false
		for {
			select {
			case ie, ok := <-eventsChan:
				if !ok {
					statusLabel.SetText("subscription ended")
					return
				}

				jsonBytes, _ := json.Marshal(ie.Event)
				if eosed {
					req.resultsEdit.SetPlainText(string(jsonBytes) + "\n" + req.resultsEdit.ToPlainText())
				} else {
					req.resultsEdit.InsertPlainText("\n" + string(jsonBytes))
				}
			case <-eoseChan:
				eosed = true
			}
		}
	}()
}
