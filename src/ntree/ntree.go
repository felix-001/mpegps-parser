package ntree

type NTree struct {
	Data   interface{}
	Childs []*NTree
}

func New(data interface{}) *NTree {
	return &NTree{Data: data}
}

func (t *NTree) Append(node *NTree) {
	if t.Childs == nil {
		t.Childs = make([]*NTree, 0)
	}
	t.Childs = append(t.Childs, node)
}

type TraverseFunc func(node *NTree, levelChange bool, opaque interface{}) interface{}

func (t *NTree) Traverse(cb TraverseFunc, opaque interface{}) {
	opaque = cb(t, false, opaque)
	for _, node := range t.Childs {
		node.Traverse(cb, opaque)
	}
}
