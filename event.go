package main

import (
	"encoding/json"
	"slices"
	"strings"

	"fiatjaf.com/nostr"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

var event struct {
	kindSpin      *widgets.QSpinBox
	kindNameLabel *widgets.QLabel
	tagRows       [][]*widgets.QLineEdit
	contentEdit   *widgets.QTextEdit
	createdAtEdit *widgets.QDateTimeEdit
	outputEdit    *widgets.QTextEdit
}

func updateEvent() {
	kind := nostr.Kind(event.kindSpin.Value())
	kindName := kind.Name()
	if kindName != "unknown" {
		event.kindNameLabel.SetText(kindName)
	} else {
		event.kindNameLabel.SetText("")
	}
	tags := make(nostr.Tags, 0, len(event.tagRows))
	for y, tagItems := range event.tagRows {
		if y == len(event.tagRows)-1 && strings.TrimSpace(tagItems[0].Text()) == "" {
			continue
		}

		tag := make(nostr.Tag, 0, len(tagItems))
		for x, edit := range tagItems {
			text := strings.TrimSpace(edit.Text())
			if x == len(tagItems)-1 && text == "" {
				continue
			}
			text = decodeTagValue(text)
			tag = append(tag, text)
		}
		if len(tag) > 0 {
			tags = append(tags, tag)
		}
	}

	result := nostr.Event{
		Kind:      kind,
		Content:   event.contentEdit.ToPlainText(),
		CreatedAt: nostr.Timestamp(event.createdAtEdit.DateTime().ToMSecsSinceEpoch() / 1000),
		Tags:      tags,
	}

	finalize := func() {
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		event.outputEdit.SetPlainText(string(jsonBytes))
	}

	if currentKeyer != nil {
		signAndFinalize := func() {
			if currentKeyer != nil {
				if err := currentKeyer.SignEvent(ctx, &result); err == nil {
					finalize()
				} else {
					statusLabel.SetText("failed to sign: " + err.Error())
				}
			}
		}

		if currentSec == [32]byte{} {
			// empty key, we must have a bunker
			debounced.Call(signAndFinalize)
		} else {
			// we have a key, can sign immediately
			signAndFinalize()
			return
		}
	}

	finalize()
}

func setupEventTab() *widgets.QWidget {
	tab := widgets.NewQWidget(nil, 0)

	// set up event tab
	layout := widgets.NewQVBoxLayout()
	tab.SetLayout(layout)

	// kind input
	kindHBox := widgets.NewQHBoxLayout()
	layout.AddLayout(kindHBox, 0)
	kindLabel := widgets.NewQLabel2("kind:", nil, 0)
	kindHBox.AddWidget(kindLabel, 0, 0)
	event.kindSpin = widgets.NewQSpinBox(nil)
	event.kindSpin.SetValue(1)
	event.kindSpin.SetMinimum(0)
	event.kindSpin.SetMaximum(1<<16 - 1)
	kindHBox.AddWidget(event.kindSpin, 0, 0)
	event.kindSpin.ConnectValueChanged(func(int) {
		updateEvent()
	})
	event.kindNameLabel = widgets.NewQLabel2("", nil, 0)
	kindHBox.AddWidget(event.kindNameLabel, 0, 0)

	// content input
	contentLabel := widgets.NewQLabel2("content:", nil, 0)
	layout.AddWidget(contentLabel, 0, 0)
	event.contentEdit = widgets.NewQTextEdit(nil)
	layout.AddWidget(event.contentEdit, 0, 0)
	event.contentEdit.ConnectTextChanged(updateEvent)

	// created_at input
	createdAtLabel := widgets.NewQLabel2("created at:", nil, 0)
	layout.AddWidget(createdAtLabel, 0, 0)
	event.createdAtEdit = widgets.NewQDateTimeEdit(nil)
	event.createdAtEdit.SetDateTime(core.QDateTime_CurrentDateTime())
	layout.AddWidget(event.createdAtEdit, 0, 0)
	event.createdAtEdit.ConnectDateTimeChanged(func(*core.QDateTime) {
		updateEvent()
	})

	// tags input
	tagsLabel := widgets.NewQLabel2("tags:", nil, 0)
	layout.AddWidget(tagsLabel, 0, 0)
	tagsLayout := widgets.NewQVBoxLayout()
	tagRowHBoxes := make([]widgets.QLayout_ITF, 0, 2)
	event.tagRows = make([][]*widgets.QLineEdit, 0, 2)
	layout.AddLayout(tagsLayout, 0)

	var addTagRow func()
	addTagRow = func() {
		hbox := widgets.NewQHBoxLayout()
		tagRowHBoxes = append(tagRowHBoxes, hbox)
		tagsLayout.AddLayout(hbox, 0)
		tagItems := []*widgets.QLineEdit{}
		y := len(event.tagRows)
		event.tagRows = append(event.tagRows, tagItems)

		var addItem func()
		addItem = func() {
			edit := widgets.NewQLineEdit(nil)
			hbox.AddWidget(edit, 0, 0)
			x := len(tagItems)
			tagItems = append(tagItems, edit)
			event.tagRows[y] = tagItems
			edit.ConnectTextChanged(func(text string) {
				if strings.TrimSpace(text) != "" {
					// when an item input has been filled check if we have to show more
					if y == len(event.tagRows)-1 {
						addTagRow()
					}
					if x == len(tagItems)-1 {
						addItem()
					}
				} else {
					// do this when an item input has been emptied: check if we need to remove an item from this row
					nItems := len(tagItems)
					if nItems >= 2 && strings.TrimSpace(tagItems[nItems-1].Text()) == "" && strings.TrimSpace(tagItems[nItems-2].Text()) == "" {
						// remove last item if the last 2 are empty
						hbox.Layout().RemoveWidget(tagItems[nItems-1])
						tagItems[nItems-1].DeleteLater()
						tagItems = tagItems[0 : nItems-1]
						event.tagRows[y] = tagItems
					}

					// check if we need to remove rows
					nRows := len(event.tagRows)
					itemIsFilled := func(edit *widgets.QLineEdit) bool { return strings.TrimSpace(edit.Text()) != "" }
					if nRows >= 2 && !slices.ContainsFunc(event.tagRows[nRows-1], itemIsFilled) && !slices.ContainsFunc(event.tagRows[nRows-2], itemIsFilled) {
						// remove the last row if the last 2 are empty
						tagsLayout.RemoveItem(tagRowHBoxes[nRows-1])
						for _, tagItem := range event.tagRows[nRows-1] {
							tagItem.DeleteLater()
						}
						tagRowHBoxes[nRows-1].QLayout_PTR().DeleteLater()
						event.tagRows = event.tagRows[0 : nRows-1]
						tagRowHBoxes = tagRowHBoxes[0 : nRows-1]
					}
				}
				updateEvent()
			})
		}
		addItem()
	}

	// first
	addTagRow()

	// output JSON
	outputLabel := widgets.NewQLabel2("event:", nil, 0)
	layout.AddWidget(outputLabel, 0, 0)
	event.outputEdit = widgets.NewQTextEdit(nil)
	event.outputEdit.SetReadOnly(true)
	layout.AddWidget(event.outputEdit, 0, 0)

	return tab
}
