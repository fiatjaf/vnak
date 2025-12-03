package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
	"golang.org/x/exp/slices"
)

type reqVars struct {
	tab *qt.QWidget

	authorsVBox  *qt.QVBoxLayout
	authorsEdits []*qt.QLineEdit
	idsVBox      *qt.QVBoxLayout
	idsEdits     []*qt.QLineEdit
	kindsVBox    *qt.QVBoxLayout
	kindRows     []reqKindRow
	tagsVBox     *qt.QVBoxLayout
	tagRows      []reqTagRow
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

type reqKindRow struct {
	hbox  *qt.QHBoxLayout
	edit  *qt.QLineEdit
	label *qt.QLabel
}

type reqTagRow struct {
	hbox      *qt.QHBoxLayout
	key       *qt.QLineEdit
	separator *qt.QLabel
	vals      []*qt.QLineEdit
}

var req = &reqVars{}

func setupReqTab() *qt.QWidget {
	req.tab = qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	req.tab.SetLayout(layout.QLayout)

	// ids
	idsLabel := qt.NewQLabel2()
	idsLabel.SetText("ids:")
	layout.AddWidget(idsLabel.QWidget)
	req.idsVBox = qt.NewQVBoxLayout2()
	layout.AddLayout(req.idsVBox.QLayout)
	req.idsEdits = []*qt.QLineEdit{}
	req.addId("")

	// authors
	authorsLabel := qt.NewQLabel2()
	authorsLabel.SetText("authors:")
	layout.AddWidget(authorsLabel.QWidget)
	req.authorsVBox = qt.NewQVBoxLayout2()
	layout.AddLayout(req.authorsVBox.QLayout)
	req.authorsEdits = []*qt.QLineEdit{}
	req.addAuthor("")

	// kinds
	kindsLabel := qt.NewQLabel2()
	kindsLabel.SetText("kinds:")
	layout.AddWidget(kindsLabel.QWidget)
	req.kindsVBox = qt.NewQVBoxLayout2()
	layout.AddLayout(req.kindsVBox.QLayout)
	req.kindRows = []reqKindRow{}
	req.addKind("")

	// tags
	tagsLabel := qt.NewQLabel2()
	tagsLabel.SetText("tags:")
	layout.AddWidget(tagsLabel.QWidget)
	req.tagsLayout = qt.NewQVBoxLayout2()
	layout.AddLayout(req.tagsLayout.QLayout)
	req.tagRows = make([]reqTagRow, 0, 2)
	req.addTagRow("", []string{""})

	// numeric and boolean inputs
	qtime := qt.NewQDateTime()
	qtime.SetMSecsSinceEpoch(time.Now().UnixMilli())
	inputsHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(inputsHBox.QLayout)

	// limit
	limitHBox := qt.NewQHBoxLayout2()
	inputsHBox.AddLayout(limitHBox.QLayout)
	limitLabel := qt.NewQLabel2()
	limitLabel.SetText("limit:")
	limitHBox.AddWidget(limitLabel.QWidget)
	req.limitCheck = qt.NewQCheckBox(req.tab)
	req.limitCheck.SetChecked(true)
	limitHBox.AddWidget(req.limitCheck.QWidget)
	req.limitSpin = qt.NewQSpinBox(req.tab)
	req.limitSpin.SetEnabled(true)
	req.limitSpin.SetMinimum(0)
	req.limitSpin.SetMaximum(1000)
	req.limitSpin.SetValue(10)
	limitHBox.AddWidget(req.limitSpin.QWidget)
	req.limitCheck.OnStateChanged(func(state int) {
		req.limitSpin.SetEnabled(state == 2) // 2 is checked
		req.updateReq()
	})
	req.limitSpin.OnValueChanged(func(int) {
		req.updateReq()
	})

	// since
	sinceHBox := qt.NewQHBoxLayout2()
	sinceHBox.AddStretch()
	inputsHBox.AddLayout(sinceHBox.QLayout)
	sinceLabel := qt.NewQLabel2()
	sinceLabel.SetText("since:")
	sinceHBox.AddWidget(sinceLabel.QWidget)
	req.sinceCheck = qt.NewQCheckBox(req.tab)
	req.sinceCheck.SetChecked(false)
	sinceHBox.AddWidget(req.sinceCheck.QWidget)
	req.sinceEdit = qt.NewQDateTimeEdit(req.tab)
	req.sinceEdit.SetEnabled(false)
	req.sinceEdit.SetDateTime(qtime)
	sinceHBox.AddWidget(req.sinceEdit.QWidget)
	req.sinceCheck.OnStateChanged(func(state int) {
		req.sinceEdit.SetEnabled(state == 2) // 2 is checked
		req.updateReq()
	})
	req.sinceEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		req.updateReq()
	})

	// until
	untilHBox := qt.NewQHBoxLayout2()
	untilHBox.AddStretch()
	inputsHBox.AddLayout(untilHBox.QLayout)
	untilLabel := qt.NewQLabel2()
	untilLabel.SetText("until:")
	untilHBox.AddWidget(untilLabel.QWidget)
	req.untilCheck = qt.NewQCheckBox(req.tab)
	req.untilCheck.SetChecked(false)
	untilHBox.AddWidget(req.untilCheck.QWidget)
	req.untilEdit = qt.NewQDateTimeEdit(req.tab)
	req.untilEdit.SetEnabled(false)
	req.untilEdit.SetDateTime(qtime)
	untilHBox.AddWidget(req.untilEdit.QWidget)
	req.untilCheck.OnStateChanged(func(state int) {
		req.untilEdit.SetEnabled(state == 2) // 2 is checked
		req.updateReq()
	})
	req.untilEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		req.updateReq()
	})

	// output
	outputHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(outputHBox.QLayout)
	outputLabel := qt.NewQLabel2()
	outputLabel.SetText("filter:")
	outputHBox.AddWidget(outputLabel.QWidget)
	req.outputEdit = qt.NewQTextEdit(req.tab)
	req.outputEdit.SetReadOnly(true)
	req.outputEdit.SetMaximumHeight(100)
	outputHBox.AddWidget(req.outputEdit.QWidget)

	sendButton := qt.NewQPushButton5("send request", req.tab)
	sendButton.OnClicked(func() {
		req.subscribe()
	})

	// relays
	relaysHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(relaysHBox.QLayout)
	relaysLabel := qt.NewQLabel2()
	relaysLabel.SetText("relays:")
	relaysHBox.AddWidget(relaysLabel.QWidget)

	req.relaysEdits = []*qt.QLineEdit{}
	var addRelayEdit func()
	addRelayEdit = func() {
		edit := qt.NewQLineEdit(req.tab)
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

	// results
	resultsVBox := qt.NewQVBoxLayout2()
	resultsLabel := qt.NewQLabel2()
	resultsLabel.SetText("results:")
	req.resultsList = qt.NewQListWidget(req.tab)
	resultsVBox.AddWidget(resultsLabel.QWidget)
	resultsVBox.AddWidget(req.resultsList.QWidget)

	subscribeHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(subscribeHBox.QLayout)
	subscribeHBox.AddWidget(sendButton.QWidget)
	subscribeHBox.AddLayout(resultsVBox.QLayout)

	// double-click to show pretty JSON
	req.resultsList.OnItemDoubleClicked(func(item *qt.QListWidgetItem) {
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

	return req.tab
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
	for _, kindRow := range req.kindRows {
		text := strings.TrimSpace(kindRow.edit.Text())
		kindRow.label.SetText("")
		if k, err := strconv.Atoi(text); err == nil {
			kind := nostr.Kind(k)
			kinds = append(kinds, kind)

			// update kind label
			name := kind.Name()
			if name != "unknown" {
				kindRow.label.SetText(name)
			}
		}
	}
	if len(kinds) > 0 {
		req.filter.Kinds = kinds
	}

	// collect tags
	tags := make(map[string][]string)
	for _, tagRow := range req.tagRows {
		key := strings.TrimSpace(tagRow.key.Text())
		if key == "" || len(tagRow.vals) == 0 {
			continue
		}

		values := make([]string, 0, len(tagRow.vals))
		for _, edit := range tagRow.vals {
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

		if currentKeyer != nil {
			err = relay.Auth(ctx, func(ctx context.Context, evt *nostr.Event) error {
				return currentKeyer.SignEvent(ctx, evt)
			})
			if err != nil {
				statusLabel.SetText(fmt.Sprintf("failed to auth to %s: %s", niceRelayURL(relay.URL), err))
				return
			}
		}

		statusLabel.SetText("subscribed to " + niceRelayURL(relay.URL))
		sub, err := relay.Subscribe(ctx, req.filter, nostr.SubscriptionOptions{
			Label: "vnak-req-1",
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
					Label: "vnak-req",
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
	// clear all authors except the first, set the first to ""
	for _, authorEdit := range req.authorsEdits {
		req.authorsVBox.RemoveWidget(authorEdit.QWidget)
		authorEdit.DeleteLater()
	}
	req.authorsEdits = req.authorsEdits[:0]

	// add authors
	for _, author := range filter.Authors {
		req.addAuthor(author.Hex())
	}
	req.addAuthor("") // extra

	// clear all ids except the first, set the first to ""
	for _, idEdit := range req.idsEdits {
		req.idsVBox.RemoveWidget(idEdit.QWidget)
		idEdit.DeleteLater()
	}
	req.idsEdits = req.idsEdits[:0]

	// add ids
	for _, id := range filter.IDs {
		req.addId(id.Hex())
	}
	req.addId("") // extra

	// clear all kinds except the first, set the first to ""
	for _, kindRow := range req.kindRows {
		kindRow.hbox.RemoveWidget(kindRow.label.QWidget)
		kindRow.hbox.RemoveWidget(kindRow.edit.QWidget)
		kindRow.label.DeleteLater()
		kindRow.edit.DeleteLater()
		req.kindsVBox.RemoveItem(kindRow.hbox.QLayoutItem)
		kindRow.hbox.DeleteLater()
	}
	req.kindRows = req.kindRows[:0]

	// add kinds
	for _, kind := range filter.Kinds {
		req.addKind(strconv.Itoa(int(kind)))
	}
	req.addKind("") // extra

	// clear all tag items and rows
	for _, tagRow := range req.tagRows {
		tagRow.hbox.RemoveWidget(tagRow.key.QWidget)
		tagRow.key.QWidget.DeleteLater()
		tagRow.hbox.RemoveWidget(tagRow.separator.QWidget)
		tagRow.separator.DeleteLater()

		for _, item := range tagRow.vals {
			tagRow.hbox.RemoveWidget(item.QWidget)
			item.DeleteLater()
		}
		req.tagsLayout.RemoveItem(tagRow.hbox.QLayoutItem)
		tagRow.hbox.DeleteLater()
	}
	req.tagRows = req.tagRows[:0]

	// add tags according to filter
	for key, values := range filter.Tags {
		req.addTagRow(key, values)
	}
	req.addTagRow("", []string{""}) // extra

	// set since, until, limit
	if filter.Since != 0 {
		req.sinceCheck.SetChecked(true)
		dt := qt.NewQDateTime()
		dt.SetMSecsSinceEpoch(int64(filter.Since) * 1000)
		req.sinceEdit.SetDateTime(dt)
	} else {
		req.sinceCheck.SetChecked(true)
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

func (req *reqVars) addAuthor(value string) {
	edit := qt.NewQLineEdit(req.tab)
	edit.SetText(value)
	req.authorsEdits = append(req.authorsEdits, edit)
	req.authorsVBox.AddWidget(edit.QWidget)
	edit.OnTextChanged(func(text string) {
		if strings.TrimSpace(text) != "" {
			if edit == req.authorsEdits[len(req.authorsEdits)-1] {
				req.addAuthor("")
			}
		} else {
			n := len(req.authorsEdits)
			if n >= 2 && strings.TrimSpace(req.authorsEdits[n-1].Text()) == "" && strings.TrimSpace(req.authorsEdits[n-2].Text()) == "" {
				req.authorsVBox.RemoveWidget(req.authorsEdits[n-1].QWidget)
				req.authorsEdits[n-1].DeleteLater()
				req.authorsEdits = req.authorsEdits[0 : n-1]
			}
		}
		req.updateReq()
	})
}

func (req *reqVars) addId(text string) {
	edit := qt.NewQLineEdit(req.tab)
	edit.SetText(text)
	req.idsEdits = append(req.idsEdits, edit)
	req.idsVBox.AddWidget(edit.QWidget)
	edit.OnTextChanged(func(text string) {
		if strings.TrimSpace(text) != "" {
			if edit == req.idsEdits[len(req.idsEdits)-1] {
				req.addId("")
			}
		} else {
			n := len(req.idsEdits)
			if n >= 2 && strings.TrimSpace(req.idsEdits[n-1].Text()) == "" && strings.TrimSpace(req.idsEdits[n-2].Text()) == "" {
				req.idsVBox.RemoveWidget(req.idsEdits[n-1].QWidget)
				req.idsEdits[n-1].DeleteLater()
				req.idsEdits = req.idsEdits[0 : n-1]
			}
		}
		req.updateReq()
	})
}

func (req *reqVars) addKind(text string) {
	hbox := qt.NewQHBoxLayout2()
	req.kindsVBox.AddLayout(hbox.QLayout)
	edit := qt.NewQLineEdit(req.tab)
	edit.SetText(text)
	hbox.AddWidget(edit.QWidget)
	label := qt.NewQLabel2()
	hbox.AddWidget(label.QWidget)

	req.kindRows = append(req.kindRows, reqKindRow{
		hbox:  hbox,
		label: label,
		edit:  edit,
	})

	edit.OnTextChanged(func(text string) {
		if strings.TrimSpace(text) != "" {
			if edit == req.kindRows[len(req.kindRows)-1].edit {
				req.addKind("")
			}
		} else {
			n := len(req.kindRows)
			if n >= 2 && strings.TrimSpace(req.kindRows[n-1].edit.Text()) == "" && strings.TrimSpace(req.kindRows[n-2].edit.Text()) == "" {
				last := req.kindRows[n-1]
				req.kindsVBox.RemoveItem(last.hbox.QLayoutItem)
				last.hbox.RemoveWidget(req.kindRows[n-1].edit.QWidget)
				last.hbox.RemoveWidget(req.kindRows[n-1].label.QWidget)
				last.edit.DeleteLater()
				last.label.DeleteLater()
				last.hbox.DeleteLater()
				req.kindRows = req.kindRows[:n-1]
			}
		}
		req.updateReq()
	})
}

func (req *reqVars) addTagRow(name string, values []string) {
	hbox := qt.NewQHBoxLayout2()
	req.tagsLayout.AddLayout(hbox.QLayout)
	valuesEdits := []*qt.QLineEdit{}
	y := len(req.tagRows)

	var perhapsDeleteRow func()

	// name
	editName := qt.NewQLineEdit(req.tab)
	editName.SetText(name)
	hbox.AddWidget(editName.QWidget)
	editName.OnTextChanged(func(text string) {
		if strings.TrimSpace(text) != "" {
			if y == len(req.tagRows)-1 {
				req.addTagRow("", []string{""})
			}
		} else {
			perhapsDeleteRow()
		}
		req.updateReq()
	})

	// separator
	separator := qt.NewQLabel2()
	separator.SetText(": ")
	hbox.AddWidget(separator.QWidget)

	// keep track of everything
	req.tagRows = append(req.tagRows, reqTagRow{
		hbox:      hbox,
		key:       editName,
		separator: separator,
	})

	var addValue func(string)
	addValue = func(val string) {
		edit := qt.NewQLineEdit(req.tab)
		edit.SetText(val)
		hbox.AddWidget(edit.QWidget)
		x := len(valuesEdits)
		valuesEdits = append(valuesEdits, edit)
		req.tagRows[y].vals = valuesEdits

		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				if x == len(valuesEdits)-1 {
					addValue("")
				}
			} else {
				nItems := len(valuesEdits)
				if nItems >= 2 &&
					strings.TrimSpace(valuesEdits[nItems-1].Text()) == "" &&
					strings.TrimSpace(valuesEdits[nItems-2].Text()) == "" {
					// when the last values of a tag row are empty remove the last one
					last := valuesEdits[nItems-1]
					hbox.RemoveWidget(last.QWidget)
					last.DeleteLater()
					valuesEdits = valuesEdits[0 : nItems-1]
					req.tagRows[y].vals = valuesEdits
				}

				perhapsDeleteRow()
			}
			req.updateReq()
		})
	}

	perhapsDeleteRow = func() {
		nRows := len(req.tagRows)
		hasValue := func(e *qt.QLineEdit) bool { return strings.TrimSpace(e.Text()) != "" }
		if nRows >= 2 &&
			!slices.ContainsFunc(req.tagRows[nRows-1].vals, hasValue) &&
			!slices.ContainsFunc(req.tagRows[nRows-2].vals, hasValue) &&
			strings.TrimSpace(req.tagRows[nRows-1].key.Text()) == "" &&
			strings.TrimSpace(req.tagRows[nRows-2].key.Text()) == "" {
			// when the two last rows are completely empty remove the last one:
			last := req.tagRows[nRows-1]
			last.hbox.RemoveWidget(last.separator.QWidget)
			last.separator.DeleteLater()
			last.hbox.RemoveWidget(last.key.QWidget)
			last.key.DeleteLater()
			for _, val := range last.vals {
				last.hbox.RemoveWidget(val.QWidget)
				val.DeleteLater()
			}

			req.tagsLayout.RemoveItem(last.hbox.QLayoutItem)
			req.tagRows = req.tagRows[0 : nRows-1]
		}
	}

	for _, val := range values {
		addValue(val)
	}
}
