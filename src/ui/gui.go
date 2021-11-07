package ui

import (
	"fmt"
	"log"
	"ntree"
	"os"
	"reader"

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
		return core.NewQVariant1("偏移")
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
	case "偏移":
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
	model        *CustomTableModel
	ch           chan *reader.PktInfo
	treeModel    *gui.QStandardItemModel
	detailReader reader.DetailReader
}

func New(ch chan *reader.PktInfo, detailReader reader.DetailReader) *ui {
	return &ui{ch: ch, detailReader: detailReader}
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
	ui.treeModel = gui.NewQStandardItemModel(nil)
	treeview.SetModel(ui.treeModel)

	tableview := widgets.NewQTableView(nil)
	tableview.SetSelectionMode(widgets.QAbstractItemView__SingleSelection)
	tableview.SetSelectionBehavior(widgets.QAbstractItemView__SelectRows)
	ui.model = NewCustomTableModel(nil)
	tableview.ConnectClicked(func(index *core.QModelIndex) {
		offset := ui.model.Index(index.Row(), 0, nil).Data(0).ToLongLong(nil)
		typ := ui.model.Index(index.Row(), 1, nil).Data(0).ToString()
		tree, err := ui.detailReader.ParseDetail(offset, typ)
		if err != nil {
			log.Println(err)
			return
		}
		var item *gui.QStandardItem
		ret := tree.Traverse(callback, item)
		item = ret.(*gui.QStandardItem)
		ui.treeModel.SetItem2(0, item)
		treeview.ExpandAllDefault()

	})
	tableview.SetModel(ui.model)

	textedit := widgets.NewQTextEdit(nil)
	// TODO: 待实现
	// 1.点击表格中某一个条目，对应的hex view能显示
	// 2.点击详细信息的某一个条目，对应的hex view能显示
	// 3.滚动条向下滑动，相应的内容跟着动
	// 4.支持搜索hex
	// 5.支持指定offset查看
	// 6.点击详细信息的某一个条目，对应的hex view高亮

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

func (ui *ui) ShowData(ch chan *reader.PktInfo) {
	for {
		if data, ok := <-ch; ok {
			// TODO: tabview 也使用standrand model
			item := TableItem{
				Offset:  int64(data.Offset),
				PktType: data.Typ,
				Status:  data.Status,
			}
			ui.model.Add(item)
		}
	}
}

func callback(node *ntree.NTree, levelChange bool, opaque interface{}) interface{} {
	ptr := opaque.(*gui.QStandardItem)
	data := node.Data.(*reader.Item)
	s := fmt.Sprintf("%s : 0x%x", data.K, data.V)
	if data.V == 0xFFFF {
		s = fmt.Sprintf("%s", data.K)
	}
	item := gui.NewQStandardItem2(s)
	if ptr == nil {
		return item
	}
	ptr.AppendRow2(item)
	return item
}
