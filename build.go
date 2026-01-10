package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/schema"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
	"github.com/sahilm/fuzzy"
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

type listFuzzySource struct{ *qt.QListWidget }

func (l listFuzzySource) String(i int) string { return strings.ToLower(l.Item(i).Text()) }
func (l listFuzzySource) Len() int            { return l.Count() }

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
	bool       *qt.QCheckBox
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
		matches := fuzzy.FindFrom(filterText, listFuzzySource{build.kindsList})

		if filterText == "" {
			// show all
			for i := 0; i < build.kindsList.Count(); i++ {
				build.kindsList.SetRowHidden(i, false)
			}
		} else {
			// hide all
			for i := 0; i < build.kindsList.Count(); i++ {
				build.kindsList.SetRowHidden(i, true)
			}

			// except the matched
			for _, match := range matches {
				build.kindsList.SetRowHidden(match.Index, false)
			}
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

		// check for missing required tags
		missing := []string{}
		for _, ts := range build.tagSections {
			if ts.isRequired {
				tagName := strings.TrimSuffix(
					strings.SplitN(ts.label.Text(), " ", 2)[0],
					":",
				)

				hasValidTag := false
				if ts.isBoolean {
					hasValidTag = ts.bool != nil && ts.bool.CheckState() == qt.Checked
				} else {
					for _, edits := range ts.edits {
						hasValidValue := false
						for _, edit := range edits {
							if strings.TrimSpace(edit.Text()) != "" {
								hasValidValue = true
								break
							}
						}
						if hasValidValue {
							hasValidTag = true
							break
						}
					}
				}

				if !hasValidTag {
					missing = append(missing, tagName)
				}
			}
		}

		if len(missing) > 0 {
			// show validation failure alert
			msgBox := qt.NewQMessageBox2()
			msgBox.SetWindowTitle("validation failed")
			msgBox.SetText(fmt.Sprintf("missing required tags: %s", strings.Join(missing, ", ")))
			msgBox.SetInformativeText("you can continue to manual event editing or cancel to fix the issues.")
			msgBox.SetStandardButtons(qt.QMessageBox__Ok | qt.QMessageBox__Cancel)
			if msgBox.Exec() == int(qt.QMessageBox__Cancel) {
				return
			}
		}

		// check each tag to see if it has all the necessary items and they match each of the types
		validationErrors := build.validateTags()
		if len(validationErrors) > 0 {
			msgBox := qt.NewQMessageBox2()
			msgBox.SetWindowTitle("tag validation errors")
			msgBox.SetText(strings.Join(validationErrors, "\n"))
			msgBox.SetInformativeText("you can continue to manual event editing or cancel to fix the issues")
			msgBox.SetStandardButtons(qt.QMessageBox__Ok | qt.QMessageBox__Cancel)
			if msgBox.Exec() == int(qt.QMessageBox__Cancel) {
				return
			}
		}

		event.populate(eventBuilt)
		tabWidget.SetCurrentIndex(tabs.event)
	})

	// spacer at bottom
	layout.AddStretch()

	// load kinds
	go func() {
		sch, err := schema.FetchSchemaFromURL("https://raw.githubusercontent.com/nostr-protocol/registry-of-kinds/refs/heads/master/schema.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading schema:", err)
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

					// all addressable events need a 'd' tag
					if nostr.Kind(kindItem.kind).IsAddressable() {
						build.addTag(&schema.TagSpec{
							Name: "d",
							Next: &schema.ContentSpec{
								Type:     "free",
								Required: true,
							},
						}, true, false)
					}

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
		build.addTagRow(ts, tagSpec, 0)
	} else {
		// boolean tag - add checkbox
		ts.bool = qt.NewQCheckBox2()
		ts.sectionBox.AddWidget(ts.bool.QWidget)
	}
}

var (
	gitURLRegex = regexp.MustCompile(`^(git@[\w.-]+:[\w./-]+(\.git)?|(https?|git|ssh)://[\w.-]+/[\w./-]+(\.git)?)$`)
	addrRegex   = regexp.MustCompile(`^\d+:[0-9a-fA-F]{64}:.+$`)
	dateRegex   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func validateTagInput(text string, spec *schema.ContentSpec) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true // empty is always valid (required check is separate)
	}

	switch spec.Type {
	case "free":
		return true

	case "timestamp":
		n, err := strconv.ParseInt(text, 10, 64)
		return err == nil && n >= 0

	case "constrained":
		return slices.Contains(spec.Either, text)

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

func (b *buildVars) addTagRow(ts *tagSection, tagSpec *schema.TagSpec, rowIdx int) {
	hbox := qt.NewQHBoxLayout2()
	ts.rowsBox.AddLayout(hbox.QLayout)
	ts.rows = append(ts.rows, hbox)
	ts.edits = append(ts.edits, make([]*qt.QLineEdit, 0, 3))

	if ts.isBoolean {
		// TODO: in this case only add one field: a checkbox, save it to ts.bool
	} else {
		itemIdx := 0
		for curr := tagSpec.Next; curr != nil; curr = curr.Next {
			b.addInput(ts, tagSpec, rowIdx, hbox, curr, itemIdx)
			itemIdx++
		}
	}
}

func (b *buildVars) addInput(
	ts *tagSection,
	tagSpec *schema.TagSpec,
	rowIdx int,
	row *qt.QHBoxLayout,
	itemSpec *schema.ContentSpec,
	itemIdx int,
) {
	// input for the tag value
	edit := qt.NewQLineEdit(b.tab)
	configureInputForType(edit, itemSpec)

	row.AddWidget(edit.QWidget)
	ts.edits[rowIdx] = append(ts.edits[rowIdx], edit)

	edit.OnTextChanged(func(text string) {
		if !ts.isMultiple {
			return
		}

		if strings.TrimSpace(text) != "" {
			// if this is the last row and it has content and this tag accepts multiple values and add a new row
			if rowIdx == ts.rowsBox.Count()-1 {
				b.addTagRow(ts, tagSpec, rowIdx+1)
			}
		} else {
			// now if this is the second to last row and the last row and it is fully empty we remove the last
			if len(ts.rows) >= 2 && rowIdx == len(ts.rows)-2 {
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

	if itemSpec.Variadic {
		// variadic field - add dynamic add/remove functionality
		edit.OnTextChanged(func(text string) {
			if strings.TrimSpace(text) != "" {
				// if this is the last edit and has content, add a new one
				lastRowIdx := len(ts.edits) - 1
				lastEditIdx := len(ts.edits[lastRowIdx]) - 1
				if itemIdx == lastEditIdx {
					b.addInput(ts, tagSpec, rowIdx, row, itemSpec, itemIdx+1)
				}
			} else {
				// if this is second to last and the last one is empty delete the last
				if itemIdx == len(ts.edits[rowIdx])-2 {
					if strings.TrimSpace(ts.edits[rowIdx][itemIdx+1].Text()) == "" {
						edit := ts.edits[rowIdx][itemIdx+1]
						row.RemoveWidget(edit.QWidget)
						edit.DeleteLater()
						ts.edits[rowIdx] = ts.edits[rowIdx][:itemIdx+1]
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

		if ts.isBoolean {
			if ts.bool.CheckState() == qt.Checked {
				base.Tags = append(base.Tags, nostr.Tag{tagName})
			}
		} else {
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
	}

	return base
}

func (b *buildVars) validateTags() []string {
	var errors []string

	for _, ts := range b.tagSections {
		tagName := strings.TrimSuffix(
			strings.SplitN(ts.label.Text(), " ", 2)[0],
			":",
		)

		if ts.isBoolean {
			// boolean tags don't need type validation
			continue
		}

		for rowIdx, edits := range ts.edits {
			for itemIdx, edit := range edits {
				text := strings.TrimSpace(edit.Text())
				if text == "" {
					continue // skip empty fields
				}

				// find the tag spec for this tag
				for _, tagSpec := range b.selected.Tags {
					// try only tags with matching names, of course
					if tagSpec.Name != tagName {
						continue
					}

					// navigate to the correct position in the spec
					curr := tagSpec.Next
					for range itemIdx {
						if curr.Variadic {
							// if we reach a variadic that's the end of it, all edits further on will be of this type
							break
						}

						if curr.Next == nil {
							// this should only happen when we are the last iteration step anyway,
							// but let's exit here instead of crashing
							break
						}

						curr = curr.Next
					}

					// validate against expected type
					if !validateTagInput(text, curr) {
						errors = append(errors, fmt.Sprintf("%s[%d][%d]: invalid value for type '%s'", tagName, rowIdx, itemIdx, curr.Type))
					}
				}
			}
		}
	}

	return errors
}

func (b *buildVars) clearForm() {
	// clear all tag layouts and inputs
	for _, ts := range b.tagSections {
		ts.sectionBox.RemoveWidget(ts.label.QWidget)
		ts.label.DeleteLater()

		if ts.isBoolean {
			ts.sectionBox.RemoveWidget(ts.bool.QWidget)
			ts.bool.DeleteLater()
		} else {
			for r, row := range ts.rows {
				for _, edit := range ts.edits[r] {
					row.RemoveWidget(edit.QWidget)
					edit.DeleteLater()
				}
				ts.rowsBox.RemoveItem(row.QLayoutItem)
				row.DeleteLater()
			}
			ts.sectionBox.RemoveItem(ts.rowsBox.QLayoutItem)
			ts.rowsBox.DeleteLater()
		}

		b.formLayout.RemoveItem(ts.sectionBox.QLayoutItem)
		ts.sectionBox.DeleteLater()
	}
	b.tagSections = b.tagSections[:0]

	// but keep the content just in case
	// b.contentEdit.SetPlainText("")
}

func configureInputForType(edit *qt.QLineEdit, spec *schema.ContentSpec) {
	switch spec.Type {
	case "constrained":
		edit.SetPlaceholderText("\"" + strings.Join(spec.Either, "\" or \"") + "\"")
	case "pubkey":
		edit.SetPlaceholderText("pubkey: npub1... (hex or npub)")
	case "url":
		edit.SetPlaceholderText("url: https://example.com")
	case "relay":
		edit.SetPlaceholderText("relay url: wss://relay.example.com")
	case "event":
		edit.SetPlaceholderText("event id: nevent1... (hex or nevent)")
	case "hex":
		edit.SetPlaceholderText("hex: 0a1b2c3d...")
	case "timestamp":
		edit.SetPlaceholderText("unix timestamp: " + strconv.Itoa(int(nostr.Now())))
	case "kind":
		edit.SetPlaceholderText("kind: 1")
	case "addr":
		edit.SetPlaceholderText("address: <kind>:<hex-pubkey>:<identifier>")
	case "date":
		edit.SetPlaceholderText("date: " + time.Now().Format(time.DateOnly))
	case "giturl":
		edit.SetPlaceholderText("git url: https://gitserver.com/repo.git")
	case "json":
		edit.SetPlaceholderText("json: {\"key\": \"value\"}")
	default:
		edit.SetPlaceholderText(spec.Type)
	}

	edit.OnTextChanged(func(text string) {
		if validateTagInput(text, spec) {
			edit.SetStyleSheet("")
		} else {
			edit.SetStyleSheet("border: 1px solid red;")
		}
	})
}
