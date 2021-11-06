package reader

import "ntree"

// map遍历是无序的
type Item struct {
	K string
	V uint64
}

type PktInfo struct {
	Typ    string
	Offset uint64
	Status string
}

type DetailReader interface {
	ParseDetail(offset int64, typ string) (*ntree.NTree, error)
}
