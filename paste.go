package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip05"
	"fiatjaf.com/nostr/nip19"
	"github.com/btcsuite/btcd/btcec/v2"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
)

type pasteVars struct {
	inputEdit  *qt.QTextEdit
	outputVBox *qt.QVBoxLayout

	nip05ctxCancel context.CancelFunc
	nip05ctxAbort  error
}

var paste = pasteVars{
	nip05ctxAbort: errors.New("aborted"),
}

func setupPasteTab() *qt.QWidget {
	tab := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	tab.SetLayout(layout.QLayout)

	// input
	inputLabel := qt.NewQLabel2()
	inputLabel.SetText("paste:")
	layout.AddWidget(inputLabel.QWidget)
	paste.inputEdit = qt.NewQTextEdit(tab)
	layout.AddWidget(paste.inputEdit.QWidget)
	paste.inputEdit.OnTextChanged(updatePaste)

	// output
	paste.outputVBox = qt.NewQVBoxLayout2()
	layout.AddLayout(paste.outputVBox.QLayout)

	return tab
}

func deleteLayoutRecursively(layout *qt.QLayout) {
	for layout.Count() > 0 {
		layoutItem := layout.ItemAt(0)
		if w := layoutItem.Widget(); w != nil {
			w.DeleteLater()
		} else if subLayout := layoutItem.Layout(); subLayout != nil {
			deleteLayoutRecursively(subLayout)
		}
		layout.RemoveItem(layoutItem)
	}
	layout.DeleteLater()
}

func updatePaste() {
	// clear previous output
	for paste.outputVBox.Count() > 0 {
		item := paste.outputVBox.ItemAt(0)
		if widget := item.Widget(); widget != nil {
			widget.DeleteLater()
		} else if layout := item.Layout(); layout != nil {
			deleteLayoutRecursively(layout)
		}
		paste.outputVBox.RemoveItem(item)
	}

	text := strings.TrimSpace(paste.inputEdit.ToPlainText())
	if text == "" {
		return
	}

	// try nip19 decode
	if prefix, decoded, err := nip19.Decode(text); err == nil {
		paste.displayNip19Decoded(prefix, decoded)
		return
	}

	// try nip05
	if nip05.IsValidIdentifier(text) {
		debounced.Call(func() {
			if nip05.IsValidIdentifier(text) {
				paste.displayNip05(text)
			}
		})
		return
	}

	// try JSON event
	var event nostr.Event
	if err := json.Unmarshal([]byte(text), &event); err == nil {
		paste.displayEventButton(event)
		return
	}

	// try JSON filter
	var filter nostr.Filter
	if err := json.Unmarshal([]byte(text), &filter); err == nil {
		paste.displayFilterButton(filter)
		return
	}

	// if nothing worked, show error
	errorLabel := qt.NewQLabel2()
	errorLabel.SetText("could not decode input")
	paste.outputVBox.AddWidget(errorLabel.QWidget)
}

func (p *pasteVars) displayNip19Decoded(prefix string, decoded interface{}) {
	switch prefix {
	case "nsec":
		if sk, ok := decoded.(nostr.SecretKey); ok {
			p.displayNsec(sk)
		}
	case "npub":
		if pk, ok := decoded.(nostr.PubKey); ok {
			p.displayPubKey(pk)
		}
	case "note":
		if id, ok := decoded.(nostr.ID); ok {
			p.displayEventID(id)
		}
	case "nevent":
		if ep, ok := decoded.(nostr.EventPointer); ok {
			p.displayEventPointer(ep)
			p.displayPointerTag(ep)
		}
	case "nprofile":
		if pp, ok := decoded.(nostr.ProfilePointer); ok {
			p.displayProfilePointer(pp)
			p.displayPointerTag(pp)
		}
	case "naddr":
		if ap, ok := decoded.(nostr.EntityPointer); ok {
			p.displayAddressPointer(ap)
			p.displayPointerTag(ap)
		}
	default:
		label := qt.NewQLabel2()
		label.SetText(fmt.Sprintf("decoded %s: %v", prefix, decoded))
		p.outputVBox.AddWidget(label.QWidget)
	}
}

func (p *pasteVars) displayNsec(sk nostr.SecretKey) {
	// nsec
	nsecLabel := qt.NewQLabel2()
	nsecLabel.SetText("nsec:")
	p.outputVBox.AddWidget(nsecLabel.QWidget)
	nsec := nip19.EncodeNsec(sk)
	nsecEdit := qt.NewQLineEdit(window.QWidget)
	nsecEdit.SetText(nsec)
	nsecEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(nsecEdit.QWidget)

	// hex
	hexLabel := qt.NewQLabel2()
	hexLabel.SetText("hex:")
	p.outputVBox.AddWidget(hexLabel.QWidget)
	hexEdit := qt.NewQLineEdit(window.QWidget)
	hexEdit.SetText(sk.Hex())
	hexEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(hexEdit.QWidget)

	// npub
	npubLabel := qt.NewQLabel2()
	npubLabel.SetText("corresponding npub:")
	p.outputVBox.AddWidget(npubLabel.QWidget)
	_, pub := btcec.PrivKeyFromBytes(sk[:])
	pk := nostr.PubKey(pub.SerializeCompressed()[1:])
	npub := nip19.EncodeNpub(pk)
	npubEdit := qt.NewQLineEdit(window.QWidget)
	npubEdit.SetText(npub)
	npubEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(npubEdit.QWidget)
}

func (p *pasteVars) displayPubKey(pk nostr.PubKey) {
	npubLabel := qt.NewQLabel2()
	npubLabel.SetText("npub:")
	p.outputVBox.AddWidget(npubLabel.QWidget)
	npub := nip19.EncodeNpub(pk)
	npubEdit := qt.NewQLineEdit(window.QWidget)
	npubEdit.SetText(npub)
	npubEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(npubEdit.QWidget)

	hexLabel := qt.NewQLabel2()
	hexLabel.SetText("hex:")
	p.outputVBox.AddWidget(hexLabel.QWidget)
	hexEdit := qt.NewQLineEdit(window.QWidget)
	hexEdit.SetText(pk.Hex())
	hexEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(hexEdit.QWidget)
}

func (p *pasteVars) displayEventID(id nostr.ID) {
	hexLabel := qt.NewQLabel2()
	hexLabel.SetText("id (hex):")
	p.outputVBox.AddWidget(hexLabel.QWidget)
	hexEdit := qt.NewQLineEdit(window.QWidget)
	hexEdit.SetText(id.Hex())
	hexEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(hexEdit.QWidget)
}

func (p *pasteVars) displayPointerTag(pointer nostr.Pointer) {
	tagLabel := qt.NewQLabel2()
	tagLabel.SetText("tag reference:")
	p.outputVBox.AddWidget(tagLabel.QWidget)
	tagEdit := qt.NewQLineEdit(window.QWidget)
	tagj, _ := json.Marshal(pointer.AsTag())
	tagEdit.SetText(string(tagj))
	tagEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(tagEdit.QWidget)
}

func (p *pasteVars) displayEventPointer(ep nostr.EventPointer) {
	p.displayEventID(ep.ID)
	if ep.Author != nostr.ZeroPK {
		authorLabel := qt.NewQLabel2()
		authorLabel.SetText("author:")
		p.outputVBox.AddWidget(authorLabel.QWidget)
		authorEdit := qt.NewQLineEdit(window.QWidget)
		authorEdit.SetText(ep.Author.Hex())
		authorEdit.SetReadOnly(true)
		p.outputVBox.AddWidget(authorEdit.QWidget)
	}
	p.displayRelays(ep.Relays)
}

func (p *pasteVars) displayProfilePointer(pp nostr.ProfilePointer) {
	p.displayPubKey(pp.PublicKey)
	p.displayRelays(pp.Relays)
}

func (p *pasteVars) displayRelays(relays []string) {
	if len(relays) > 0 {
		relaysLabel := qt.NewQLabel2()
		relaysLabel.SetText("relays:")
		p.outputVBox.AddWidget(relaysLabel.QWidget)
		relaysVBox := qt.NewQVBoxLayout2()
		for i := 0; i < len(relays); i += 5 {
			rowHBox := qt.NewQHBoxLayout2()
			for j := 0; j < 5 && i+j < len(relays); j++ {
				relayEdit := qt.NewQLineEdit(window.QWidget)
				relayEdit.SetText(relays[i+j])
				relayEdit.SetReadOnly(true)
				rowHBox.AddWidget(relayEdit.QWidget)
			}
			relaysVBox.AddLayout(rowHBox.QLayout)
		}
		p.outputVBox.AddLayout(relaysVBox.QLayout)
	}
}

func (p *pasteVars) displayAddressPointer(ap nostr.EntityPointer) {
	kindLabel := qt.NewQLabel2()
	kindLabel.SetText("kind:")
	p.outputVBox.AddWidget(kindLabel.QWidget)
	kindEdit := qt.NewQLineEdit(window.QWidget)
	kindEdit.SetText(fmt.Sprintf("%d", ap.Kind))
	kindEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(kindEdit.QWidget)

	p.displayPubKey(ap.PublicKey)

	identifierLabel := qt.NewQLabel2()
	identifierLabel.SetText("identifier:")
	p.outputVBox.AddWidget(identifierLabel.QWidget)
	identifierEdit := qt.NewQLineEdit(window.QWidget)
	identifierEdit.SetText(ap.Identifier)
	identifierEdit.SetReadOnly(true)
	p.outputVBox.AddWidget(identifierEdit.QWidget)

	p.displayRelays(ap.Relays)
}

func (p *pasteVars) displayNip05(identifier string) {
	mainthread.Wait(func() {
		label := qt.NewQLabel2()
		label.SetText("nip05: " + identifier)
		p.outputVBox.AddWidget(label.QWidget)
	})

	// try to query
	if paste.nip05ctxCancel != nil {
		paste.nip05ctxCancel()
	}
	nip05ctx, cancel := context.WithTimeoutCause(ctx, time.Second*3, paste.nip05ctxAbort)
	paste.nip05ctxCancel = cancel
	defer cancel()
	pp, err := nip05.QueryIdentifier(nip05ctx, identifier)
	if err != nil && err != paste.nip05ctxAbort {
		mainthread.Wait(func() {
			errorLabel := qt.NewQLabel2()
			errorLabel.SetText("failed to query nip05: " + err.Error())
			p.outputVBox.AddWidget(errorLabel.QWidget)
		})
		return
	}

	mainthread.Wait(func() {
		nprofileLabel := qt.NewQLabel2()
		nprofileLabel.SetText("nprofile")
		p.outputVBox.AddWidget(nprofileLabel.QWidget)
		nprofileEdit := qt.NewQLineEdit(window.QWidget)
		nprofileEdit.SetText(nip19.EncodeNprofile(pp.PublicKey, pp.Relays))
		nprofileEdit.SetReadOnly(true)
		p.outputVBox.AddWidget(nprofileEdit.QWidget)

		p.displayProfilePointer(*pp)
		p.displayPointerTag(*pp)
	})
}

func (p *pasteVars) displayEventButton(evt nostr.Event) {
	button := qt.NewQPushButton5("event", window.QWidget)
	button.OnClicked(func() {
		// populate event tab
		event.kindSpin.SetValue(int(evt.Kind))
		event.contentEdit.SetPlainText(evt.Content)
		// created_at
		dt := qt.NewQDateTime()
		dt.SetMSecsSinceEpoch(int64(evt.CreatedAt) * 1000)
		event.createdAtEdit.SetDateTime(dt)
		// tags
		// clear existing
		for len(event.tagRows) > 1 {
			// remove last row
			lastRow := event.tagRows[len(event.tagRows)-1]
			for _, edit := range lastRow {
				edit.DeleteLater()
			}
			event.tagRows = event.tagRows[:len(event.tagRows)-1]
		}
		// set tags
		for i, tag := range evt.Tags {
			if i >= len(event.tagRows) {
				// add more rows if needed
				// but for simplicity, just set what we can
				break
			}
			for j, value := range tag {
				if j >= len(event.tagRows[i]) {
					break
				}
				event.tagRows[i][j].SetText(value)
			}
		}
		// switch to event tab
		tabWidget.SetCurrentIndex(0) // assuming event is first
	})
	p.outputVBox.AddWidget(button.QWidget)
}

func (p *pasteVars) displayFilterButton(filt nostr.Filter) {
	button := qt.NewQPushButton5("filter", window.QWidget)
	button.OnClicked(func() {
		// populate req tab
		// authors
		for i, author := range filt.Authors {
			if i < len(req.authorsEdits) {
				req.authorsEdits[i].SetText(nip19.EncodeNpub(author))
			}
		}
		// ids
		for i, id := range filt.IDs {
			if i < len(req.idsEdits) {
				req.idsEdits[i].SetText(id.String())
			}
		}
		// kinds
		for i, kind := range filt.Kinds {
			if i < len(req.kindsEdits) {
				req.kindsEdits[i].SetText(fmt.Sprintf("%d", kind))
			}
		}
		// since
		if filt.Since != 0 {
			dt := qt.NewQDateTime()
			dt.SetMSecsSinceEpoch(int64(filt.Since) * 1000)
			req.sinceEdit.SetDateTime(dt)
		}
		// until
		if filt.Until != 0 {
			dt := qt.NewQDateTime()
			dt.SetMSecsSinceEpoch(int64(filt.Until) * 1000)
			req.untilEdit.SetDateTime(dt)
		}
		// limit
		req.limitSpin.SetValue(filt.Limit)
		// switch to req tab
		tabWidget.SetCurrentIndex(1) // assuming req is second
	})
	p.outputVBox.AddWidget(button.QWidget)
}
