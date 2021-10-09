package ui

import (
	"fmt"
	"log"
	"os"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

type TableItem struct {
	Offset  int64
	PktType string
	Status  string
}

type CustomTableModel struct {
	core.QAbstractTableModel
	_         func()               `constructor:"init"`
	_         func(item TableItem) `signal:"add,auto"`
	modelData []TableItem
}

func (m *CustomTableModel) init() {
	m.ConnectHeaderData(m.headerData)
	m.ConnectRowCount(m.rowCount)
	m.ConnectColumnCount(m.columnCount)
	m.ConnectData(m.data)
}

func (m *CustomTableModel) headerData(section int, orientation core.Qt__Orientation, role int) *core.QVariant {
	if role != int(core.Qt__DisplayRole) || orientation == core.Qt__Vertical {
		return m.HeaderDataDefault(section, orientation, role)
	}

	// todo 加入length字段
	switch section {
	case 0:
		return core.NewQVariant1("offset")
	case 1:
		return core.NewQVariant1("包类型")
	case 2:
		return core.NewQVariant1("状态")
	}
	return core.NewQVariant()
}

func (m *CustomTableModel) rowCount(*core.QModelIndex) int {
	return len(m.modelData)
}

func (m *CustomTableModel) columnCount(*core.QModelIndex) int {
	return 3
}

func (m *CustomTableModel) data(index *core.QModelIndex, role int) *core.QVariant {
	if role != int(core.Qt__DisplayRole) {
		return core.NewQVariant()
	}

	item := m.modelData[index.Row()]
	switch m.HeaderData(index.Column(), core.Qt__Horizontal, role).ToString() {
	case "offset":
		return core.NewQVariant1(item.Offset)
	case "包类型":
		return core.NewQVariant1(item.PktType)
	case "状态":
		return core.NewQVariant1(item.Status)
	}
	return core.NewQVariant()
}

func (m *CustomTableModel) add(item TableItem) {
	m.BeginInsertRows(core.NewQModelIndex(), len(m.modelData), len(m.modelData))
	m.modelData = append(m.modelData, item)
	m.EndInsertRows()
}

type ui struct {
	model *CustomTableModel
	ch    chan *TableItem
}

func New(ch chan *TableItem) *ui {
	return &ui{ch: ch}
}

func (ui *ui) Disp() {

	app := widgets.NewQApplication(len(os.Args), os.Args)

	window := widgets.NewQMainWindow(nil, 0)
	window.SetMinimumSize2(800, 700)
	window.SetWindowTitle("mpegps解析")

	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(widgets.NewQVBoxLayout())
	window.SetCentralWidget(widget)

	treeview := widgets.NewQTreeView(nil)
	//model := NewCustomTreeModel(nil)
	model2 := gui.NewQStandardItemModel(nil)
	item1 := gui.NewQStandardItem2("vahi-daemon")
	model2.SetItem2(0, item1)
	item2 := gui.NewQStandardItem2("hello")
	item1.AppendRow2(item2)
	item3 := gui.NewQStandardItem2("world")
	item1.AppendRow2(item3)
	item4 := gui.NewQStandardItem2("111")
	item3.AppendRow2(item4)
	item5 := gui.NewQStandardItem2("change")
	model2.SetItem2(0, item5)
	item6 := gui.NewQStandardItem2("222")
	item5.AppendRow2(item6)
	treeview.SetModel(model2)

	tableview := widgets.NewQTableView(nil)
	tableview.SetSelectionMode(widgets.QAbstractItemView__SingleSelection)
	tableview.SetSelectionBehavior(widgets.QAbstractItemView__SelectRows)
	ui.model = NewCustomTableModel(nil)
	tableview.ConnectClicked(func(index *core.QModelIndex) {
		log.Println("ConnectClicked", index)
		row := index.Row()
		log.Println("row", row)
		idx := ui.model.Index(row, 0, nil)
		d1 := idx.Data(0)
		log.Println("d1", d1.ToLongLong(nil))
		data := index.Data(0)
		s := data.TypeName()
		log.Println("typename:", s)
		s1 := data.ToString()
		log.Println(s1)
		item1 := gui.NewQStandardItem2(fmt.Sprintf("%d", d1.ToLongLong(nil)))
		model2.SetItem2(0, item1)

	})
	tableview.SetModel(ui.model)

	textedit := widgets.NewQTextEdit(nil)

	layout := widgets.NewQGridLayout2()
	layout.AddWidget(tableview)
	layout.AddWidget3(treeview, 0, 1, 2, 1, 0)
	layout.AddWidget2(textedit, 1, 0, 0)

	centralWidget := widgets.NewQWidget(nil, 0)
	centralWidget.SetLayout(layout)
	window.SetCentralWidget(centralWidget)
	go ui.ShowData(ui.ch)
	window.Show()
	app.Exec()
}

func (ui *ui) ShowData(ch chan *TableItem) {
	for {
		if data, ok := <-ch; ok {
			// todo tabview 也使用standrand model
			ui.model.Add(*data)
		}
	}
}
