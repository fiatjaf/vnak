package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

type reqVars struct {
	authorsEdits []*qt.QLineEdit
	idsEdits     []*qt.QLineEdit
	kindsEdits   []*qt.QLineEdit
	kindsLabels  []*qt.QLabel
	tagRows      [][]*qt.QLineEdit
	tagRowHBoxes []*qt.QHBoxLayout
	tagsLayout   *qt.QVBoxLayout
	relaysEdits  []*qt.QLineEdit
	sinceEdit    *qt.QDateTimeEdit
	sinceCheck   *qt.QCheckBox
	untilEdit    *qt.QDateTimeEdit
	untilCheck   *qt.QCheckBox
	limitSpin    *qt.QSpinBox
	limitCheck   *qt.QCheckBox

	filter nostr.Filter

	outputEdit  *qt.QTextEdit
	resultsList *qt.QListWidget
}

var req = &reqVars{}

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
			req.updateReq()
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
			req.updateReq()
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
			req.updateReq()
		})
	}
	addKindEdit()

	// tags
	tagsLabel := qt.NewQLabel2()
	tagsLabel.SetText("tags:")
	layout.AddWidget(tagsLabel.QWidget)
	req.tagsLayout = qt.NewQVBoxLayout2()
	layout.AddLayout(req.tagsLayout.QLayout)
	req.tagRows = make([][]*qt.QLineEdit, 0, 2)
	req.tagRowHBoxes = make([]*qt.QHBoxLayout, 0, 2)
	var addTagRow func()
	addTagRow = func() {
		hbox := qt.NewQHBoxLayout2()
		req.tagRowHBoxes = append(req.tagRowHBoxes, hbox)
		req.tagsLayout.AddLayout(hbox.QLayout)
		tagItems := []*qt.QLineEdit{}
		y := len(req.tagRows)
		req.tagRows = append(req.tagRows, tagItems)

		var addItem func()
		addItem = func() {
			edit := qt.NewQLineEdit(tab)
			hbox.AddWidget(edit.QWidget)
			x := len(tagItems)
			tagItems = append(tagItems, edit)
			req.tagRows[y] = tagItems
			edit.OnTextChanged(func(text string) {
				if strings.TrimSpace(text) != "" {
					if y == len(req.tagRows)-1 && x == 0 {
						addTagRow()
					}
					if x == len(tagItems)-1 {
						addItem()
					}
				} else {
					nItems := len(tagItems)
					if nItems >= 2 && strings.TrimSpace(tagItems[nItems-1].Text()) == "" && strings.TrimSpace(tagItems[nItems-2].Text()) == "" {
						hbox.RemoveWidget(tagItems[nItems-1].QWidget)
						tagItems[nItems-1].DeleteLater()
						tagItems = tagItems[0 : nItems-1]
						req.tagRows[y] = tagItems
					}

					nRows := len(req.tagRows)
					if nRows >= 2 && len(req.tagRows[nRows-1]) == 0 && len(req.tagRows[nRows-2]) == 0 {
						req.tagsLayout.RemoveItem(hbox.QLayoutItem)
						hbox.DeleteLater()
						req.tagRows = req.tagRows[0 : nRows-1]
						req.tagRowHBoxes = req.tagRowHBoxes[0 : nRows-1]
					}
				}
				req.updateReq()
			})
		}
		addItem()
	}
	addTagRow()

	// since
	sinceHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(sinceHBox.QLayout)
	sinceLabel := qt.NewQLabel2()
	sinceLabel.SetText("since:")
	sinceHBox.AddWidget(sinceLabel.QWidget)
	req.sinceCheck = qt.NewQCheckBox(tab)
	req.sinceCheck.SetChecked(false)
	sinceHBox.AddWidget(req.sinceCheck.QWidget)
	req.sinceEdit = qt.NewQDateTimeEdit(tab)
	req.sinceEdit.SetEnabled(false)
	{
		qtime := qt.NewQDateTime()
		qtime.SetMSecsSinceEpoch(time.Now().UnixMilli())
		req.sinceEdit.SetDateTime(qtime)
	}
	sinceHBox.AddWidget(req.sinceEdit.QWidget)
	req.sinceCheck.OnStateChanged(func(state int) {
		req.sinceEdit.SetEnabled(state == 2) // 2 is checked
		if state == 2 {
			qtime := qt.NewQDateTime()
			qtime.SetMSecsSinceEpoch(time.Now().UnixMilli())
			req.sinceEdit.SetDateTime(qtime)
		}
		req.updateReq()
	})
	req.sinceEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		req.updateReq()
	})

	// until
	untilHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(untilHBox.QLayout)
	untilLabel := qt.NewQLabel2()
	untilLabel.SetText("until:")
	untilHBox.AddWidget(untilLabel.QWidget)
	req.untilCheck = qt.NewQCheckBox(tab)
	req.untilCheck.SetChecked(false)
	untilHBox.AddWidget(req.untilCheck.QWidget)
	req.untilEdit = qt.NewQDateTimeEdit(tab)
	req.untilEdit.SetEnabled(false)
	{
		qtime := qt.NewQDateTime()
		qtime.SetMSecsSinceEpoch(time.Now().UnixMilli())
		req.untilEdit.SetDateTime(qtime)
	}
	untilHBox.AddWidget(req.untilEdit.QWidget)
	req.untilCheck.OnStateChanged(func(state int) {
		req.untilEdit.SetEnabled(state == 2) // 2 is checked
		if state == 2 {
			qtime := qt.NewQDateTime()
			qtime.SetMSecsSinceEpoch(time.Now().UnixMilli())
			req.untilEdit.SetDateTime(qtime)
		}
		req.updateReq()
	})
	req.untilEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		req.updateReq()
	})

	// limit
	limitHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(limitHBox.QLayout)
	limitLabel := qt.NewQLabel2()
	limitLabel.SetText("limit:")
	limitHBox.AddWidget(limitLabel.QWidget)
	req.limitCheck = qt.NewQCheckBox(tab)
	req.limitCheck.SetChecked(false)
	limitHBox.AddWidget(req.limitCheck.QWidget)
	req.limitSpin = qt.NewQSpinBox(tab)
	req.limitSpin.SetEnabled(false)
	req.limitSpin.SetMinimum(0)
	req.limitSpin.SetMaximum(1000)
	limitHBox.AddWidget(req.limitSpin.QWidget)
	req.limitCheck.OnStateChanged(func(state int) {
		req.limitSpin.SetEnabled(state == 2) // 2 is checked
		req.updateReq()
	})
	req.limitSpin.OnValueChanged(func(int) {
		req.updateReq()
	})

	// output
	outputLabel := qt.NewQLabel2()
	outputLabel.SetText("filter:")
	layout.AddWidget(outputLabel.QWidget)
	req.outputEdit = qt.NewQTextEdit(tab)
	req.outputEdit.SetReadOnly(true)
	req.outputEdit.SetMaximumHeight(100)
	layout.AddWidget(req.outputEdit.QWidget)

	// send button
	buttonHBox := qt.NewQHBoxLayout2()
	sendButton := qt.NewQPushButton5("send request", tab)
	buttonHBox.AddWidget(sendButton.QWidget)
	buttonHBox.AddStretch()

	// relays
	relaysLabel := qt.NewQLabel2()
	relaysLabel.SetText("relays:")
	layout.AddWidget(relaysLabel.QWidget)
	relaysHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(relaysHBox.QLayout)
	req.relaysEdits = []*qt.QLineEdit{}
	var addRelayEdit func()
	addRelayEdit = func() {
		edit := qt.NewQLineEdit(tab)
		req.relaysEdits = append(req.relaysEdits, edit)
		relaysHBox.AddWidget(edit.QWidget)
		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if edit == req.relaysEdits[len(req.relaysEdits)-1] {
					addRelayEdit()
				}
			} else {
				n := len(req.relaysEdits)
				if n >= 2 && strings.TrimSpace(req.relaysEdits[n-1].Text()) == "" && strings.TrimSpace(req.relaysEdits[n-2].Text()) == "" {
					relaysHBox.RemoveWidget(req.relaysEdits[n-1].QWidget)
					req.relaysEdits[n-1].DeleteLater()
					req.relaysEdits = req.relaysEdits[0 : n-1]
				}
			}
		})
		edit.OnReturnPressed(func() {
			sendButton.Click()
		})
	}
	addRelayEdit()

	layout.AddLayout(buttonHBox.QLayout)

	// results
	resultsLabel := qt.NewQLabel2()
	resultsLabel.SetText("results:")
	layout.AddWidget(resultsLabel.QWidget)
	req.resultsList = qt.NewQListWidget(tab)
	req.resultsList.SetMinimumHeight(200)
	layout.AddWidget(req.resultsList.QWidget)

	// double-click to show pretty JSON
	req.resultsList.OnItemDoubleClicked(func(item *qt.QListWidgetItem) {
		var event nostr.Event
		if err := json.Unmarshal([]byte(item.Text()), &event); err != nil {
			return
		}
		pretty, _ := json.MarshalIndent(event, "", "  ")
		dialog := qt.NewQDialog(window.QWidget)
		dialog.SetWindowTitle("Event Details")
		dlayout := qt.NewQVBoxLayout2()
		dialog.SetLayout(dlayout.QLayout)
		textEdit := qt.NewQTextEdit(dialog.QWidget)
		textEdit.SetReadOnly(true)
		textEdit.SetPlainText(string(pretty))
		dlayout.AddWidget(textEdit.QWidget)
		closeButton := qt.NewQPushButton5("Close", dialog.QWidget)
		closeButton.OnClicked(func() { dialog.Close() })
		dlayout.AddWidget(closeButton.QWidget)
		dialog.Exec()
	})

	sendButton.OnClicked(func() {
		req.subscribe()
	})

	return tab
}

func (req *reqVars) updateReq() {
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

	// collect tags
	tags := make(map[string][]string)
	for _, tagItems := range req.tagRows {
		if len(tagItems) == 0 || strings.TrimSpace(tagItems[0].Text()) == "" {
			continue
		}
		key := strings.TrimSpace(tagItems[0].Text())
		values := []string{}
		for _, edit := range tagItems[1:] {
			text := strings.TrimSpace(edit.Text())
			if text != "" {
				values = append(values, text)
			}
		}
		if len(values) > 0 {
			tags[key] = values
		}
	}
	if len(tags) > 0 {
		req.filter.Tags = tags
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
	if req.sinceCheck.IsChecked() && req.sinceEdit.DateTime().IsValid() {
		ts := nostr.Timestamp(req.sinceEdit.DateTime().ToMSecsSinceEpoch() / 1000)
		req.filter.Since = ts
	}

	// until
	if req.untilCheck.IsChecked() && req.untilEdit.DateTime().IsValid() {
		ts := nostr.Timestamp(req.untilEdit.DateTime().ToMSecsSinceEpoch() / 1000)
		req.filter.Until = ts
	}

	// limit
	if req.limitCheck.IsChecked() {
		if req.limitSpin.Value() > 0 {
			req.filter.Limit = req.limitSpin.Value()
		} else {
			req.filter.LimitZero = true
		}
	}

	jsonBytes, _ := json.Marshal(req.filter)
	req.outputEdit.SetPlainText(string(jsonBytes))
}

func (req *reqVars) subscribe() {
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
			mainthread.Wait(func() {
				statusLabel.SetText(fmt.Sprintf("subscription closed: %s", reason))
			})
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
			mainthread.Wait(func() {
				item := qt.NewQListWidgetItem2(string(jsonBytes))

				if eosed {
					req.resultsList.InsertItem(0, item)
				} else {
					req.resultsList.AddItemWithItem(item)
				}
			})
		}
		statusLabel.SetText("subscription ended")
	}()

	go func() {
		<-eoseChan
		eosed = true
	}()
}

func (req *reqVars) populate(filter nostr.Filter) {
	// clear all authors, kinds, ids fields
	// For simplicity, clear all and add new ones
	// But since the UI is dynamic, perhaps set the first one and clear others

	// For authors
	if len(filter.Authors) > 0 {
		req.authorsEdits[0].SetText(filter.Authors[0].String())
		for i := 1; i < len(req.authorsEdits); i++ {
			req.authorsEdits[i].SetText("")
		}
	} else {
		for _, edit := range req.authorsEdits {
			edit.SetText("")
		}
	}

	// For ids
	if len(filter.IDs) > 0 {
		req.idsEdits[0].SetText(filter.IDs[0].String())
		for i := 1; i < len(req.idsEdits); i++ {
			req.idsEdits[i].SetText("")
		}
	} else {
		for _, edit := range req.idsEdits {
			edit.SetText("")
		}
	}

	// For kinds
	if len(filter.Kinds) > 0 {
		req.kindsEdits[0].SetText(strconv.Itoa(int(filter.Kinds[0])))
		for i := 1; i < len(req.kindsEdits); i++ {
			if i < len(filter.Kinds) {
				req.kindsEdits[i].SetText(strconv.Itoa(int(filter.Kinds[i])))
			} else {
				req.kindsEdits[i].SetText("")
			}
		}
	} else {
		for _, edit := range req.kindsEdits {
			edit.SetText("")
		}
	}

	// clear all tag items and rows
	for _, hbox := range req.tagRowHBoxes {
		for _, item := range req.tagRows[len(req.tagRows)-1] {
			item.DeleteLater()
		}
		hbox.DeleteLater()
	}
	req.tagRows = req.tagRows[:0]
	req.tagRowHBoxes = req.tagRowHBoxes[:0]

	// add tags according to filter
	for key, values := range filter.Tags {
		hbox := qt.NewQHBoxLayout2()
		req.tagRowHBoxes = append(req.tagRowHBoxes, hbox)
		req.tagsLayout.AddLayout(hbox.QLayout)
		tagItems := []*qt.QLineEdit{}
		edit := qt.NewQLineEdit(nil)
		edit.SetText(key)
		hbox.AddWidget(edit.QWidget)
		tagItems = append(tagItems, edit)
		for _, value := range values {
			edit := qt.NewQLineEdit(nil)
			edit.SetText(value)
			hbox.AddWidget(edit.QWidget)
			tagItems = append(tagItems, edit)
		}
		req.tagRows = append(req.tagRows, tagItems)
	}

	// set since, until, limit
	if filter.Since != 0 {
		req.sinceCheck.SetChecked(true)
		dt := qt.NewQDateTime()
		dt.SetMSecsSinceEpoch(int64(filter.Since) * 1000)
		req.sinceEdit.SetDateTime(dt)
	} else {
		req.sinceCheck.SetChecked(false)
	}

	if filter.Until != 0 {
		req.untilCheck.SetChecked(true)
		dt := qt.NewQDateTime()
		dt.SetMSecsSinceEpoch(int64(filter.Until) * 1000)
		req.untilEdit.SetDateTime(dt)
	} else {
		req.untilCheck.SetChecked(false)
	}

	if filter.Limit != 0 {
		req.limitCheck.SetChecked(true)
		req.limitSpin.SetValue(filter.Limit)
	} else if filter.LimitZero {
		req.limitCheck.SetChecked(true)
		req.limitSpin.SetValue(0)
	} else {
		req.limitCheck.SetChecked(false)
	}

	req.updateReq()
}
