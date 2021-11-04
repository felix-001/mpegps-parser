package parser

import (
	"bitreader"
	"log"
	"ntree"
)

type DataManager struct {
	tree *ntree.NTree
	br   *bitreader.BitReader
}

func NewDataManager(br *bitreader.BitReader, tree *ntree.NTree) *DataManager {
	return &DataManager{br: br, tree: tree}
}

func (dm *DataManager) _decode(items Items, output *ntree.NTree) error {
	for _, item := range items {
		ret, err := dm.br.Read(uint(item.v))
		if err != nil {
			log.Println("read", item.k, item.v, "err", err)
			return err
		}
		data := &Item{
			k: item.k,
			v: ret,
		}
		t := ntree.New(data)
		output.Append(t)
	}
	return nil
}

func (dm *DataManager) decode(input Items) error {
	return dm._decode(input, dm.tree)
}

func (dm *DataManager) decodeChild(items Items, tree *ntree.NTree) error {
	return dm._decode(items, tree)
}

func (dm *DataManager) set(key string, val uint64) {
	item := &Item{
		k: key,
		v: val,
	}
	t := ntree.New(item)
	dm.tree.Append(t)
}

func (dm *DataManager) get(tree *ntree.NTree, key string) *ntree.NTree {
	t := tree.Get(func(data interface{}) bool {
		return data.(*Item).k == key
	})
	return t
}

func (dm *DataManager) getDataFromTree(tree *ntree.NTree, key string) uint64 {
	t := dm.get(tree, key)
	//log.Println(key)
	return t.GetData().(*Item).v
}

func (dm *DataManager) getData(key string) uint64 {
	return dm.getDataFromTree(dm.tree, key)
}

func (dm *DataManager) read(key string, len uint) uint64 {
	val, _ := dm.br.Read(len)
	dm.set(key, val)
	return val
}
