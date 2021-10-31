package reader

import "ntree"

type PktInfo struct {
	Typ    string
	Offset uint64
	Status string
}

type DetailReader interface {
	ParseDetail(offset int, typ string) (*ntree.NTree, error)
}
