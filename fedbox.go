package motley

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	pub "github.com/go-ap/activitypub"
	fbox "github.com/go-ap/fedbox/activitypub"
	"github.com/go-ap/processing"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
)

var logFn = func(string, ...interface{}) {}

type fedbox struct {
	tree  map[pub.IRI]pub.Item
	iri   pub.IRI
	s     processing.Store
	o     osin.Storage
	logFn func(string, ...interface{})
}

func FedBOX(base pub.IRI, r processing.Store, o osin.Storage, l *logrus.Logger) *fedbox {
	logFn = l.Infof
	return &fedbox{tree: make(map[pub.IRI]pub.Item), iri: base, s: r, o: o, logFn: l.Infof}
}

func (f *fedbox) getService() pub.Item {
	col, err := f.s.Load(f.iri)
	if err != nil {
		return nil
	}
	var service pub.Item
	pub.OnActor(col, func(o *pub.Actor) error {
		service = o
		return nil
	})
	return service
}

func initNodes(f *fedbox) tree.Nodes {
	n := node(
		f.getService(),
		//withStorage(f),
		withState(tree.NodeLastChild|tree.NodeSelected),
	)

	return tree.Nodes{n}
}

type n struct {
	pub.Item
	n string
	p *n
	c []*n
	s tree.NodeState

	f *fedbox
}

func (n *n) Parent() tree.Node {
	if n.p == nil {
		return nil
	}
	return n.p
}
func (n *n) Init() tea.Cmd {
	return nil
}

const (
	Collapsed = "⊞"
	Expanded  = "⊟"
)

func nodeIsCollapsible(n *n) bool {
	st := false
	it := n.Item
	if len(n.c) > 0 {
		return true
	}
	_, typ := pub.Split(it.GetLink())
	st = pub.ValidCollection(typ) || fbox.FedBOXCollections.Contains(typ)
	return st
}

func (n *n) View() string {
	if n == nil || n.Item == nil || n.s.Is(tree.NodeHidden) {
		return ""
	}
	hints := n.s
	annotation := ""
	if nodeIsCollapsible(n) {
		annotation = Expanded
		if hints.Is(tree.NodeCollapsed) {
			annotation = Collapsed
		}
	}
	return fmt.Sprintf("%-1s %s", annotation, n.n)
}

func (n *n) Children() tree.Nodes {
	nodes := make(tree.Nodes, len(n.c))
	for i, nn := range n.c {
		nodes[i] = nn
	}
	return nodes
}

func (n *n) State() tree.NodeState {
	return n.s
}

func (n *n) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tree.NodeState:
		n.s = m
		if m.Is(tree.NodeSelected) && !m.Is(tree.NodeCollapsed) {
			//if pub.IsIRI(n.Item) && pub.ValidCollectionIRI(n.Item.GetLink()) {
			//	n
			//	it, err := n.f.s.Load(n.Item.GetLink())
			//	if err != nil {
			//		// TODO(marius): plug this into an error channel for the top model
			//		n.n = err.Error()
			//		n.s = n.s | NodeError
			//	}
			//	n.Item = it
			//}
			//n.c = getItemElements(n)
		}
	}
	return n, nil
}

func (n *n) setChildren(c ...*n) {
	for i, nnn := range c {
		if i == len(c)-1 {
			nnn.s |= tree.NodeLastChild
		}
		nnn.p = n
		//nnn.f = n.f
		n.c = append(n.c, nnn)
	}
}

func withName(name string) func(*n) {
	return func(nn *n) {
		nn.n = name
	}
}

func withParent(p *n) func(*n) {
	return func(nn *n) {
		//nn.f = p.f
		nn.p = p
	}
}

func withStorage(f *fedbox) func(*n) {
	return func(nn *n) {
		nn.f = f
	}
}

func withState(st tree.NodeState) func(*n) {
	return func(nn *n) {
		nn.s = st
	}
}

func withChildren(c ...*n) func(*n) {
	return func(nn *n) {
		nn.setChildren(c...)
	}
}

func getNameFromItem(it pub.Item) string {
	switch it.GetType() {
	default:
		return filepath.Base(it.GetLink().String())
	}
}

const (
	NodeError = tree.NodeLastChild << (iota + 1)
)

func node(it pub.Item, fns ...func(*n)) *n {
	n := &n{Item: it}

	if it == nil {
		n.s = NodeError
		n.n = "Invalid ActivityPub object"
		n.c = n.c[:0]
		return n
	}

	n.n = getNameFromItem(it)
	n.c = getItemElements(n)

	for _, fn := range fns {
		fn(n)
	}
	if len(n.c) > 0 || pub.IsItemCollection(it) || pub.ValidCollectionIRI(it.GetLink()) {
		n.s |= tree.NodeCollapsible
	}
	return n
}

func getObjectElements(ob pub.Object, parent *n) []*n {
	result := make([]*n, 0)
	if ob.Likes != nil {
		result = append(result, node(ob.Likes, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if ob.Shares != nil {
		result = append(result, node(ob.Shares, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if ob.Replies != nil {
		result = append(result, node(ob.Replies, withParent(parent), withState(tree.NodeCollapsed)))
	}
	return result
}

func getActorElements(act pub.Actor, parent *n) []*n {
	result := make([]*n, 0)
	pub.OnObject(&act, func(o *pub.Object) error {
		result = append(result, getObjectElements(*o, parent)...)
		return nil
	})
	if act.Inbox != nil {
		result = append(result, node(act.Inbox, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if act.Outbox != nil {
		result = append(result, node(act.Outbox, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if act.Liked != nil {
		result = append(result, node(act.Liked, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if act.Followers != nil {
		result = append(result, node(act.Followers, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if act.Following != nil {
		result = append(result, node(act.Following, withParent(parent), withState(tree.NodeCollapsed)))
	}
	if act.Streams != nil {
		result = append(result, node(act.Streams, withName("streams"), withParent(parent), withState(tree.NodeCollapsed)))
	}
	return result
}

func getItemElements(parent *n) []*n {
	result := make([]*n, 0)
	it := parent.Item

	if pub.IsItemCollection(it) {
		pub.OnItemCollection(it, func(c *pub.ItemCollection) error {
			for _, ob := range c.Collection() {
				result = append(result, node(ob, withParent(parent)))
			}
			return nil
		})
	}
	if pub.ActorTypes.Contains(it.GetType()) {
		pub.OnActor(it, func(act *pub.Actor) error {
			result = append(result, getActorElements(*act, parent)...)
			return nil
		})
	}
	if pub.ActivityTypes.Contains(it.GetType()) || pub.ObjectTypes.Contains(it.GetType()) {
		pub.OnObject(it, func(act *pub.Object) error {
			result = append(result, getObjectElements(*act, parent)...)
			return nil
		})
	}
	//if pub.IsIRI(it) {
	//	result = append(result)
	//}
	return result
}
