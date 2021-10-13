package reader

import "ntree"

type DetailReader interface {
	ParseDetail(offset int, typ string) (*ntree.NTree, error)
}
