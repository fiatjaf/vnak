package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/schema"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

type buildVars struct {
	tab       *qt.QWidget
	kindsList *qt.QListWidget
	items     []kindItem
	selected  *kindItem

	// dynamic form elements
	descLabel  *qt.QLabel
	formLayout *qt.QVBoxLayout
	formWidget *qt.QWidget

	tagSections []*tagSection

	contentEdit  *qt.QTextEdit
	contentLabel *qt.QLabel
}

type kindItem struct {
	kind uint16
	schema.KindSchema
}

type tagSection struct {
	isMultiple bool
	isBoolean  bool
	isRequired bool

	sectionBox *qt.QHBoxLayout
	label      *qt.QLabel
	rowsBox    *qt.QVBoxLayout
	rows       []*qt.QHBoxLayout
	edits      [][]*qt.QLineEdit
}

var build = &buildVars{}

func setupBuildTab() *qt.QWidget {
	build.tab = qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	build.tab.SetLayout(layout.QLayout)

	// label
	label := qt.NewQLabel2()
	label.SetText("select a kind to build an event:")
	layout.AddWidget(label.QWidget)

	// search/filter input
	searchInput := qt.NewQLineEdit(build.tab)
	searchInput.SetPlaceholderText("filter kinds...")
	layout.AddWidget(searchInput.QWidget)

	// list widget for kind selection
	build.kindsList = qt.NewQListWidget(build.tab)
	build.kindsList.SetMaximumHeight(100)
	layout.AddWidget(build.kindsList.QWidget)

	// filter list items as user types
	searchInput.OnTextChanged(func(text string) {
		filterText := strings.ToLower(text)
		for i := 0; i < build.kindsList.Count(); i++ {
			item := build.kindsList.Item(i)
			itemText := strings.ToLower(item.Text())
			// TODO: instead of strings.Contains() use levenshtein distance fuzz matching
			build.kindsList.SetRowHidden(i, !strings.Contains(itemText, filterText))
		}
	})

	// description label
	build.descLabel = qt.NewQLabel2()
	build.descLabel.SetWordWrap(true)
	layout.AddWidget(build.descLabel.QWidget)

	// form container for dynamic tag inputs
	build.formWidget = qt.NewQWidget(build.tab)
	build.formLayout = qt.NewQVBoxLayout2()
	build.formWidget.SetLayout(build.formLayout.QLayout)
	layout.AddWidget(build.formWidget)

	// content input
	build.contentLabel = qt.NewQLabel2()
	build.contentLabel.SetText("content:")
	build.contentLabel.SetVisible(false)
	layout.AddWidget(build.contentLabel.QWidget)
	build.contentEdit = qt.NewQTextEdit(build.tab)
	build.contentEdit.SetVisible(false)
	build.contentEdit.SetMaximumHeight(40)
	layout.AddWidget(build.contentEdit.QWidget)

	// button to continue to event tab
	buttonHBox := qt.NewQHBoxLayout2()
	continueButton := qt.NewQPushButton5("event "+ARROW, build.tab)
	continueButton.SetVisible(false)
	buttonHBox.AddWidget(continueButton.QWidget)
	buttonHBox.AddStretch()
	layout.AddLayout(buttonHBox.QLayout)

	continueButton.OnClicked(func() {
		eventBuilt := build.buildEvent()

		// TODO: check for missing tags in event
		missing := []string{}
		if len(missing) > 0 {
			setStatus(tabs.build, "missing required tags: %s", strings.Join(missing, ", "))
			return
		}

		// TODO: check each tag to see if it has all the necessary items and they match each of the types
		// ('constrained', 'url', 'relay', 'id', 'addr', 'pubkey', 'timestamp', 'kind', 'giturl'. 'json', 'hex')
		// ...

		event.populate(eventBuilt)
		tabWidget.SetCurrentIndex(tabs.event)
	})

	// spacer at bottom
	layout.AddStretch()

	// load kinds
	go func() {
		sch, err := schema.FetchSchemaFromURL("https://raw.githubusercontent.com/nostr-protocol/registry-of-kinds/refs/heads/master/schema.yaml")
		if err != nil {
			fmt.Println("error loading schema:", err)
			mainthread.Wait(func() {
				setStatus(tabs.build, "error loading schema: %s", err)
			})
			return
		}

		build.items = make([]kindItem, 0, len(sch.Kinds))
		for k, v := range sch.Kinds {
			num := -1
			fmt.Sscanf(k, "%d", &num)
			if v.InUse {
				build.items = append(build.items, kindItem{kind: uint16(num), KindSchema: v})
			}
		}
		slices.SortFunc(build.items, func(a, b kindItem) int { return int(a.kind) - int(b.kind) })

		mainthread.Wait(func() {
			// populate list widget with items
			for _, item := range build.items {
				build.kindsList.AddItem(fmt.Sprintf("%d - %s", item.kind, item.Description))
			}

			// handle item selection
			build.kindsList.OnItemClicked(func(li *qt.QListWidgetItem) {
				row := build.kindsList.Row(li)
				if row >= 0 && row < len(build.items) {
					kindItem := build.items[row]
					build.selected = &kindItem
					item := kindItem.KindSchema

					build.clearForm()

					// show description
					build.descLabel.SetText(item.Description)

					// generate tag inputs based on schema
					for _, tagSpec := range item.Tags {
						isMultiple := slices.Contains(item.Multiple, tagSpec.Name)
						if tagSpec.Prefix != "" {
							isMultiple = isMultiple || slices.Contains(item.Multiple, tagSpec.Prefix)
						}
						build.addTag(&tagSpec, slices.Contains(item.Required, tagSpec.Name), isMultiple)
					}

					// show content input if the kind has content
					hasContent := item.Content.Type != "empty"
					build.contentLabel.SetVisible(hasContent)
					build.contentEdit.SetVisible(hasContent)
					if hasContent {
						build.contentLabel.SetText(fmt.Sprintf("content (%s):", item.Content.Type))
					}

					continueButton.SetVisible(true)
				}
			})
		})
	}()

	return build.tab
}

func (b *buildVars) addTag(tagSpec *schema.TagSpec, required bool, isMultiple bool) {
	ts := &tagSection{
		isMultiple: isMultiple,
		isBoolean:  tagSpec.Next == nil,
		isRequired: required,
	}

	ts.sectionBox = qt.NewQHBoxLayout2()
	b.formLayout.AddLayout(ts.sectionBox.QLayout)

	// label for the tag
	labelText := tagSpec.Name
	if required {
		labelText += " (required)"
	}

	ts.label = qt.NewQLabel2()
	ts.label.SetText(labelText + ":")
	ts.label.SetMinimumWidth(80)
	ts.sectionBox.AddWidget(ts.label.QWidget)

	b.tagSections = append(b.tagSections, ts)

	// input for the tag value
	if tagSpec.Next != nil {
		vbox := qt.NewQVBoxLayout2()
		ts.rowsBox = vbox
		ts.sectionBox.AddLayout(vbox.Layout())
		build.addTagRow(0, ts, tagSpec)
	}
}

var (
	gitURLRegex = regexp.MustCompile(`^(git@[\w.-]+:[\w./-]+(\.git)?|https?://[\w.-]+/[\w./-]+(\.git)?)$`)
	addrRegex   = regexp.MustCompile(`^\d+:[0-9a-fA-F]{64}:.+$`)
	dateRegex   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func validateTagInput(text string, inputType string, either []string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true // empty is always valid (required check is separate)
	}

	switch inputType {
	case "free":
		return true

	case "timestamp":
		n, err := strconv.ParseInt(text, 10, 64)
		return err == nil && n >= 0

	case "constrained":
		return slices.Contains(either, text)

	case "hex":
		_, err := hex.DecodeString(text)
		return err == nil

	case "url":
		u, err := url.Parse(text)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""

	case "giturl":
		return gitURLRegex.MatchString(text)

	case "relay":
		u, err := url.Parse(text)
		return err == nil && (u.Scheme == "ws" || u.Scheme == "wss") && u.Host != ""

	case "pubkey":
		_, err := nostr.PubKeyFromHex(text)
		return err == nil

	case "event":
		_, err := nostr.IDFromHex(text)
		return err == nil

	case "addr":
		return addrRegex.MatchString(text)

	case "date":
		return dateRegex.MatchString(text)

	case "json":
		var v any
		err := json.Unmarshal([]byte(text), &v)
		return err == nil

	case "kind":
		n, err := strconv.ParseUint(text, 10, 16)
		return err == nil && n <= 65535

	default:
		return true
	}
}

func (b *buildVars) addTagRow(i int, ts *tagSection, tagSpec *schema.TagSpec) {
	hbox := qt.NewQHBoxLayout2()
	ts.rowsBox.AddLayout(hbox.QLayout)
	ts.rows = append(ts.rows, hbox)
	ts.edits = append(ts.edits, make([]*qt.QLineEdit, 0, 3))

	for curr := tagSpec.Next; curr != nil; curr = curr.Next {
		// input for the tag value
		edit := qt.NewQLineEdit(b.tab)
		switch curr.Type {
		case "constrained":
			edit.SetPlaceholderText("\"" + strings.Join(curr.Either, "\" or \"") + "\"")
		default:
			edit.SetPlaceholderText(curr.Type)
		}
		hbox.AddWidget(edit.QWidget)
		ts.edits[i] = append(ts.edits[i], edit)

		inputType := curr.Type
		eitherValues := curr.Either

		edit.OnTextChanged(func(text string) {
			// validation
			if validateTagInput(text, inputType, eitherValues) {
				edit.SetStyleSheet("")
			} else {
				edit.SetStyleSheet("border: 1px solid red;")
			}

			if !ts.isMultiple {
				return
			}

			if strings.TrimSpace(text) != "" {
				// if this is the last row and it has content and this tag accepts multiple values and add a new row
				if i == len(ts.rowsBox.Children())-1 {
					b.addTagRow(i+1, ts, tagSpec)
				}
			} else {
				// now if this is the second to last row and the last row and it is fully empty we remove the last
				if len(ts.rows) >= 2 && i == len(ts.rows)-2 {
					shouldDelete := true
					// if all inputs are empty in the last two rows we will delete
				deletecheck:
					for r := len(ts.rows) - 2; r < len(ts.edits); r++ {
						for _, edit := range ts.edits[r] {
							if strings.TrimSpace(edit.Text()) != "" {
								shouldDelete = false
								break deletecheck
							}
						}
					}

					if shouldDelete {
						last := ts.rows[len(ts.rows)-1]
						for _, edit := range ts.edits[len(ts.rows)-1] {
							last.RemoveWidget(edit.QWidget)
							edit.DeleteLater()
						}
						ts.edits = ts.edits[0 : len(ts.edits)-1]

						ts.rowsBox.RemoveItem(last.QLayoutItem)
						last.DeleteLater()
						ts.rows = ts.rows[0 : len(ts.rows)-1]
					}
				}
			}
		})
	}
}

func (b *buildVars) buildEvent() nostr.Event {
	base := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      nostr.Kind(b.selected.kind),
		Tags:      make(nostr.Tags, 0, 3),
		Content:   b.contentEdit.ToPlainText(),
	}

	// of all the hboxes
	for _, ts := range b.tagSections {
		// if the second element of an hbox is a vbox that means it is a top-level hbox
		// get tag name from the hbox
		tagName := strings.TrimSuffix(
			strings.SplitN(ts.label.Text(), " ", 2)[0],
			":",
		)

		for e, edits := range ts.edits {
			tag := make(nostr.Tag, 1, 4)
			tag[0] = tagName

			hasValue := false
			for _, edit := range edits {
				value := strings.TrimSpace(edit.Text())
				tag = append(tag, value)
				if value != "" {
					hasValue = true
				}
			}

			if ts.isBoolean || (ts.isRequired && e == 0) || hasValue {
				base.Tags = append(base.Tags, tag)
			}
		}
	}

	return base
}

func (b *buildVars) clearForm() {
	// clear all tag layouts and inputs
	for _, ts := range b.tagSections {
		ts.sectionBox.RemoveWidget(ts.label.QWidget)
		ts.label.DeleteLater()

		for r, row := range ts.rows {
			for _, edit := range ts.edits[r] {
				row.RemoveWidget(edit.QWidget)
				edit.DeleteLater()
			}
			ts.rowsBox.RemoveItem(row.QLayoutItem)
			row.DeleteLater()
		}
		ts.rowsBox.DeleteLater()

		b.formLayout.RemoveItem(ts.sectionBox.QLayoutItem)
		ts.sectionBox.DeleteLater()
	}
	b.tagSections = b.tagSections[:0]

	// but keep the content just in case
	// b.contentEdit.SetPlainText("")
}
