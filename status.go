package main

import (
	"fmt"
	"reflect"

	qt "github.com/mappu/miqt/qt6"
)

var statusLabel *qt.QLabel

var statuses []string = make([]string, reflect.TypeOf(tabs).NumField())

func setStatus(fromTab int, msg string, args ...any) {
	msg = fmt.Sprintf(msg, args...)

	statuses[fromTab] = msg
	if tabWidget.CurrentIndex() == fromTab {
		statusLabel.SetText(msg)
	}
}

func maybeSetStatusOnTab(tab int) {
	if statusLabel != nil && statuses[tab] != "" {
		statusLabel.SetText(statuses[tab])
	}
}
