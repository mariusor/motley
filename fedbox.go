package motley

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/processing"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
)

type fedbox struct {
	tree  map[pub.IRI]pub.Item
	iri   pub.IRI
	s     processing.Store
	o     osin.Storage
	logFn func(string, ...interface{})
}

func FedBOX(base pub.IRI, r processing.Store, o osin.Storage, l *logrus.Logger) *fedbox {
	return &fedbox{tree: make(map[pub.IRI]pub.Item), iri: base, s: r, o: o, logFn: l.Infof}
}

func (f *fedbox) getService() pub.Item {
	col, err := f.s.Load(f.iri)
	if err != nil {
		return nil
	}
	var service pub.Item
	pub.OnObject(col, func(o *pub.Object) error {
		service = o
		return nil
	})
	return service
}

type apNode struct {
	pub.Item
}

func node(it pub.Item) tree.Node {
	return &apNode{it}
}

func (f *apNode) Parent() tree.Node {
	return nil
}
func (f *apNode) Name() string {
	return f.GetID().String()
}

func (f *apNode) Children() tree.Nodes {
	return getItemElements(f.Item)
}

func (f *apNode) State() tree.NodeState {
	curNode := f.Item
	if curNode == nil {
		return tree.NodeNone
	}
	var st tree.NodeState = tree.NodeVisible
	if pub.IsItemCollection(curNode) {
		st |= tree.NodeCollapsible
	}
	if _, col := pub.Split(curNode.GetLink()); col != "" {
		st |= tree.NodeCollapsible
	}
	//	f.logFn("%s state %d", what, st)
	return st
}

func (f *apNode) SetState(tree.NodeState) {}

func getObjectElements(ob pub.Object) tree.Nodes {
	result := make([]tree.Node, 0)
	if ob.Likes != nil {
		result = append(result, node(ob.Likes))
	}
	if ob.Shares != nil {
		result = append(result, node(ob.Shares))
	}
	if ob.Replies != nil {
		result = append(result, node(ob.Replies))
	}
	return result
}

func getActorElements(act pub.Actor) tree.Nodes {
	result := make([]tree.Node, 0)
	pub.OnObject(&act, func(o *pub.Object) error {
		result = append(result, getObjectElements(*o)...)
		return nil
	})
	if act.Inbox != nil {
		result = append(result, node(act.Inbox))
	}
	if act.Outbox != nil {
		result = append(result, node(act.Outbox))
	}
	if act.Liked != nil {
		result = append(result, node(act.Liked))
	}
	return result
}

func getItemElements(it pub.Item) tree.Nodes {
	result := make([]tree.Node, 0)
	if pub.IsItemCollection(it) {
		pub.OnItemCollection(it, func(c *pub.ItemCollection) error {
			for _, it := range c.Collection() {
				result = append(result, node(it))
			}
			return nil
		})
	}
	if pub.ActorTypes.Contains(it.GetType()) {
		pub.OnActor(it, func(act *pub.Actor) error {
			result = append(result, getActorElements(*act)...)
			return nil
		})
	}
	if pub.ActivityTypes.Contains(it.GetType()) || pub.ObjectTypes.Contains(it.GetType()) {
		pub.OnObject(it, func(act *pub.Object) error {
			result = append(result, getObjectElements(*act)...)
			return nil
		})
	}
	return result
}
