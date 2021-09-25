package gui

import (
	"os"

	"github.com/therecipe/qt/core"
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
	window.SetMinimumSize2(500, 700)
	window.SetWindowTitle("tableview Example")

	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(widgets.NewQVBoxLayout())
	window.SetCentralWidget(widget)

	tableview := widgets.NewQTableView(nil)
	ui.model = NewCustomTableModel(nil)
	go ui.ShowData(ui.ch)
	tableview.SetModel(ui.model)
	widget.Layout().AddWidget(tableview)

	window.Show()
	app.Exec()
}

func (ui *ui) ShowData(ch chan *TableItem) {
	for {
		if data, ok := <-ch; ok {
			ui.model.Add(*data)
		}
	}
}
