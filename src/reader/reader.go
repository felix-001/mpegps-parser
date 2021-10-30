package reader

type M map[string]interface{}

type PktInfo struct {
	Typ    string
	Offset uint64
	Status string
}

type DetailReader interface {
	ParseDetail(offset int, typ string) (M, error)
}
