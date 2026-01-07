package main

import (
	"fmt"
	"slices"
	"strconv"

	"fiatjaf.com/nostr/schema"
	qt "github.com/mappu/miqt/qt6"
)

type buildVars struct {
	kindsCombo *qt.QComboBox
}

type kindItem struct {
	kind uint16
	schema.KindSchema
}

var build = &buildVars{}

func setupBuildTab() *qt.QWidget {
	tab := qt.NewQWidget(window.QWidget)
	layout := qt.NewQVBoxLayout2()
	tab.SetLayout(layout.QLayout)

	// label
	label := qt.NewQLabel2()
	label.SetText("select a kind to build an event:")
	layout.AddWidget(label.QWidget)

	// dropdown
	build.kindsCombo = qt.NewQComboBox(tab)
	layout.AddWidget(build.kindsCombo.QWidget)

	// load kinds
	loadKinds()

	return tab
}

func loadKinds() {
	sch, err := schema.FetchSchemaFromURL(schema.DefaultSchemaURL)
	if err != nil {
		fmt.Println("error loading schema:", err)
		setStatus(tabs.build, "error loading schema: %s", err)
		return
	}

	items := make([]kindItem, 0, len(sch.Kinds))
	for k, v := range sch.Kinds {
		num := -1
		fmt.Sscanf(k, "%d", &num)
		items = append(items, kindItem{kind: uint16(num), KindSchema: v})
	}
	slices.SortFunc(items, func(a, b kindItem) int { return int(a.kind - b.kind) })

	model := qt.NewQStandardItemModel2(len(items)+2, 2)
	inUseCategory := qt.NewQStandardItem2("common")
	inUseCategory.SetFlags(inUseCategory.Flags() ^ qt.ItemIsSelectable) // remove selectable flag
	inUseCategory.SetData(qt.NewQVariant(), int(qt.FontRole))           // bold font
	model.AppendRowWithItem(inUseCategory)

	notInUseCategory := qt.NewQStandardItem2("weird")
	notInUseCategory.SetFlags(inUseCategory.Flags() ^ qt.ItemIsSelectable) // remove selectable flag
	notInUseCategory.SetData(qt.NewQVariant(), int(qt.FontRole))           // bold font
	model.AppendRowWithItem(notInUseCategory)

	for _, item := range items {
		qitem := []*qt.QStandardItem{
			qt.NewQStandardItem2(strconv.Itoa(int(item.kind))),
			qt.NewQStandardItem2(item.Description),
		}
		if item.InUse {
			inUseCategory.AppendRow(qitem)
		} else {
			notInUseCategory.AppendRow(qitem)
		}
	}

	proxy := qt.NewQSortFilterProxyModel()
	proxy.SetSourceModel(model.QAbstractItemModel)
	proxy.SetFilterCaseSensitivity(qt.CaseInsensitive)
	build.kindsCombo.SetModel(proxy.QAbstractItemModel)
	build.kindsCombo.SetEditable(true)

	tableView := qt.NewQAbstractItemView2()
	tableView.SetSelectionBehavior(qt.QAbstractItemView__SelectRows)
	tableView.SetSelectionMode(qt.QAbstractItemView__SingleSelection)
	build.kindsCombo.SetView(tableView)

	build.kindsCombo.OnCurrentIndexChanged(func(index int) {
		sourceIndex := proxy.MapToSource(proxy.Index(index, 0, qt.NewQModelIndex()))
		actualIndex := sourceIndex.Row()
		selectedItem := items[actualIndex]
		fmt.Println("selected", selectedItem)
	})

	build.kindsCombo.OnEditTextChanged(func(text string) {
		fmt.Println("########", text)

		proxy.SetFilterFixedString(text)
	})
}
