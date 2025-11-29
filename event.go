package main

import (
	"encoding/json"
	"slices"
	"strings"

	"fiatjaf.com/nostr"
	qt "github.com/mappu/miqt/qt6"
)

var event struct {
	kindSpin      *qt.QSpinBox
	kindNameLabel *qt.QLabel
	tagRows       [][]*qt.QLineEdit
	contentEdit   *qt.QTextEdit
	createdAtEdit *qt.QDateTimeEdit
	outputEdit    *qt.QTextEdit
}

func setupEventTab() *qt.QWidget {
	tab := qt.NewQWidget(window.QWidget)

	// set up event tab
	layout := qt.NewQVBoxLayout2()
	tab.SetLayout(layout.QLayout)

	// kind input
	kindHBox := qt.NewQHBoxLayout2()
	layout.AddLayout(kindHBox.QLayout)
	kindLabel := qt.NewQLabel2()
	kindLabel.SetText("kind:")
	kindHBox.AddWidget(kindLabel.QWidget)
	event.kindSpin = qt.NewQSpinBox(tab)
	event.kindSpin.SetValue(1)
	event.kindSpin.SetMinimum(0)
	event.kindSpin.SetMaximum(1<<16 - 1)
	kindHBox.AddWidget(event.kindSpin.QWidget)
	event.kindSpin.OnValueChanged(func(int) {
		updateEvent()
	})
	event.kindNameLabel = qt.NewQLabel2()
	kindHBox.AddWidget(event.kindNameLabel.QWidget)

	// content input
	contentLabel := qt.NewQLabel2()
	contentLabel.SetText("content:")
	layout.AddWidget(contentLabel.QWidget)
	event.contentEdit = qt.NewQTextEdit(tab)
	layout.AddWidget(event.contentEdit.QWidget)
	event.contentEdit.OnTextChanged(updateEvent)

	// created_at input
	createdAtLabel := qt.NewQLabel2()
	createdAtLabel.SetText("created at:")
	layout.AddWidget(createdAtLabel.QWidget)
	event.createdAtEdit = qt.NewQDateTimeEdit(tab)
	event.createdAtEdit.SetDateTime(qt.QDateTime_CurrentDateTime())
	layout.AddWidget(event.createdAtEdit.QWidget)
	event.createdAtEdit.OnDateTimeChanged(func(*qt.QDateTime) {
		updateEvent()
	})

	// tags input
	tagsLabel := qt.NewQLabel2()
	tagsLabel.SetText("tags:")
	layout.AddWidget(tagsLabel.QWidget)
	tagsLayout := qt.NewQVBoxLayout2()
	tagRowHBoxes := make([]*qt.QHBoxLayout, 0, 2)
	event.tagRows = make([][]*qt.QLineEdit, 0, 2)
	layout.AddLayout(tagsLayout.QLayout)

	var addTagRow func()
	addTagRow = func() {
		hbox := qt.NewQHBoxLayout2()
		tagRowHBoxes = append(tagRowHBoxes, hbox)
		tagsLayout.AddLayout(hbox.QLayout)
		tagItems := []*qt.QLineEdit{}
		y := len(event.tagRows)
		event.tagRows = append(event.tagRows, tagItems)

		var addItem func()
		addItem = func() {
			edit := qt.NewQLineEdit(tab)
			hbox.AddWidget(edit.QWidget)
			x := len(tagItems)
			tagItems = append(tagItems, edit)
			event.tagRows[y] = tagItems
			edit.OnTextChanged(func(text string) {
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
						hbox.RemoveWidget(tagItems[nItems-1].QWidget)
						tagItems[nItems-1].DeleteLater()
						tagItems = tagItems[0 : nItems-1]
						event.tagRows[y] = tagItems
					}

					// check if we need to remove rows
					nRows := len(event.tagRows)
					itemIsFilled := func(edit *qt.QLineEdit) bool { return strings.TrimSpace(edit.Text()) != "" }
					if nRows >= 2 && !slices.ContainsFunc(event.tagRows[nRows-1], itemIsFilled) && !slices.ContainsFunc(event.tagRows[nRows-2], itemIsFilled) {
						// remove the last row if the last 2 are empty
						tagsLayout.RemoveItem(tagRowHBoxes[nRows-1].QLayoutItem)
						for _, tagItem := range event.tagRows[nRows-1] {
							tagItem.DeleteLater()
						}
						tagRowHBoxes[nRows-1].DeleteLater()
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
	outputLabel := qt.NewQLabel2()
	outputLabel.SetText("event:")
	layout.AddWidget(outputLabel.QWidget)
	event.outputEdit = qt.NewQTextEdit(tab)
	event.outputEdit.SetReadOnly(true)
	layout.AddWidget(event.outputEdit.QWidget)

	return tab
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
