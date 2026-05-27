package motley

import (
	"context"
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/config"
	"git.sr.ht/~mariusor/motley/internal/env"
	"git.sr.ht/~mariusor/storage-all"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	tree "github.com/mariusor/bubbles-tree"
)

const (
	//HasChanges   = "⧆"
	//Locked       = "⚿"
	//Unexpandable = "⊠"

	Collapsed    = "⊞"
	Expanded     = "⊟"
	Unexpandable = "⬚"
	Attention    = "⊡"
)

const (
	NodeSyncing = tree.NodeMaxState << (iota + 1)
	NodeSynced
	NodeError
)

type loggerFn func(string, ...interface{})

var logFn = func(string, ...interface{}) {}

type Store struct {
	root pub.Item
	env  env.Type
	s    storage.FullStorage
}

type fedbox struct {
	tree   map[pub.IRI]pub.Item
	items  pub.IRIs
	stores []Store
	logFn  loggerFn
}

func WithStore(st storage.FullStorage, root pub.Item, environment string) Store {
	return Store{
		root: root,
		env:  env.Type(environment),
		s:    st,
	}
}

func fedBOX(rootIRIs []string, st []config.Storage, l lw.Logger) (*fedbox, error) {
	logFn = l.Infof
	stores := make([]Store, 0)
	var appendStore = func(stores *[]Store, db storage.FullStorage, e env.Type, it pub.Item) {
		if pub.IsNil(it) {
			return
		}
		*stores = append(*stores, Store{root: it, s: db, env: e})
	}
	errs := make([]error, 0)
	for _, s := range st {
		found := false
		for _, iri := range rootIRIs {
			s.Host = iri
			db, err := config.Open(s, s.Env, l)
			if err != nil {
				l.Debugf("unable to initialize %s storage %s: %+v", s.Type, s.Path, err)
				errs = append(errs, errors.Annotatef(err, "Unable to initialize %s storage %s", s.Type, s.Path))
				continue
			}
			if opener, ok := db.(interface{ Open() error }); ok {
				_ = opener.Open()
			}
			it, err := db.Load(pub.IRI(iri))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if it.IsCollection() {
				_ = pub.OnCollectionIntf(it, func(col pub.CollectionInterface) error {
					for _, it := range col.Collection() {
						appendStore(&stores, db, s.Env, it)
					}
					return nil
				})
			} else {
				appendStore(&stores, db, s.Env, it)
			}
			found = true
		}
		if !found {
			l.Debugf("unable to load main Actor for storage[%s] %s", s.Type, s.Path)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &fedbox{tree: make(map[pub.IRI]pub.Item), stores: stores, logFn: l.Debugf}, nil
}

func (f *fedbox) Load(iri pub.IRI, ff ...filters.Check) (pub.Item, error) {
	for _, st := range f.stores {
		if pub.IsNil(st.root) || !iri.Contains(st.root.GetLink(), true) {
			continue
		}
		col, err := st.s.Load(iri, ff...)
		if err != nil {
			f.logFn("Unable to load (%s)%s: %s", st.root.GetLink(), iri, err)
			continue
		}
		return col, nil
	}
	return nil, errors.NotFoundf("unable to load %s in any storage", iri)
}

func (f *fedbox) getRootNodes() pub.ItemCollection {
	rootNodes := make(pub.ItemCollection, len(f.stores))
	for i, st := range f.stores {
		rootNodes[i] = st.root
	}
	return rootNodes
}

func initNodes(f *fedbox) tree.Nodes {
	nodes := make(tree.Nodes, 0)
	var st tree.NodeState
	if len(f.getRootNodes()) == 1 {
		st = tree.NodeLastChild
	}
	for _, root := range f.getRootNodes() {
		nodes = append(nodes, node(
			root,
			withState(st),
		))
	}
	return nodes
}

// n is the basic treeModel node
type n struct {
	pub.Item
	n string
	p *n
	c []*n
	s tree.NodeState
}

func (n *n) startedSyncing() {
	n.s |= NodeSyncing
	n.s ^= NodeSynced
}

func (n *n) stoppedSyncing() {
	n.s ^= NodeSyncing
	n.s |= NodeSynced
}

func (n *n) Parent() tree.Node {
	if n.p == nil {
		return nil
	}
	return n.p
}

func (n *n) Init() tea.Cmd {
	return noop
}

func nodeIsError(n *n) bool {
	return n.s.Is(NodeError)
}

func nodeIsSynced(n *n) bool {
	return n.s.Is(NodeSynced)
}

func iriIsCollection(iri pub.IRI) bool {
	if _, typ := pub.Split(iri); len(typ) > 0 {
		return true
	}
	if _, typ := filters.FedBOXCollections.Split(iri); len(typ) > 0 {
		return true
	}
	return false
}

func nodeIsCollapsible(n *n) bool {
	it := n.Item
	if len(n.c) > 0 {
		return true
	}
	if iriIsCollection(it.GetLink()) {
		n.s |= tree.NodeCollapsed | tree.NodeCollapsible
	}
	return n.s.Is(tree.NodeCollapsible)
}

func (n *n) View() tea.View {
	if n == nil || n.s.Is(tree.NodeHidden) {
		return tea.View{}
	}
	hints := n.s
	annotation := ""
	st := lipgloss.NewStyle()
	if nodeIsError(n) {
		st = faintRedFg
		//annotation = Attention
	}

	if n.Item != nil && nodeIsCollapsible(n) {
		annotation = Expanded
		if hints.Is(tree.NodeCollapsed) {
			annotation = Collapsed
		}
		if len(n.c) == 0 {
			//annotation = Unexpandable
			st = st.Faint(true)
		}
	}
	if n.s.Is(NodeSyncing) {
		//annotation = Attention
		st = st.Blink(true)
	}

	return tea.NewView(fmt.Sprintf("%-1s %s", annotation, st.Render(n.n)))
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
	switch mt := msg.(type) {
	case tree.NodeState:
		n.s = mt
	}
	return n, noop
}

func (n *n) setChildren(c ...*n) {
	for i, nnn := range c {
		if i == len(c)-1 {
			nnn.s |= tree.NodeLastChild
		}
		nnn.p = n
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
	n := filepath.Base(it.GetLink().String())
	var err error
	typ := it.GetType()
	switch {
	case pub.LinkTypes.Match(typ):
		err = pub.OnLink(it, func(l *pub.Link) error {
			if nm := name(l); len(nm) > 0 {
				n = fmt.Sprintf("%s[%s]", nm, typ)
			}
			return nil
		})
	case pub.ActorTypes.Match(typ):
		err = pub.OnActor(it, func(act *pub.Actor) error {
			if nm := name(act); len(nm) > 0 {
				n = fmt.Sprintf("%s[%s]", nm, typ)
			}
			return nil
		})
	case pub.ActivityTypes.Match(typ), pub.IntransitiveActivityTypes.Match(typ):
		if tt := typ.AsTypes(); len(tt) > 0 {
			n = tt.String()
		}
		err = pub.OnActivity(it, func(act *pub.Activity) error {
			obType := ""
			_ = pub.OnObject(act.Object, func(ob *pub.Object) error {
				if tt := ob.GetType(); tt != nil {
					obType = tt.AsTypes().String()
				}
				return nil
			})
			if len(obType) > 0 {
				n = fmt.Sprintf("%s » %s", typ, obType)
			}
			return nil
		})
	case pub.ObjectTypes.Match(typ):
		err = pub.OnObject(it, func(ob *pub.Object) error {
			if nm := name(ob); len(nm) > 0 {
				n = fmt.Sprintf("%s[%s]", nm, typ)
			} else {
				if typ != nil {
					n = typ.AsTypes().String()
				}
			}
			return nil
		})
	case pub.NilType.Match(typ):
		err = pub.OnObject(it, func(ob *pub.Object) error {
			if nm := name(ob); len(nm) > 0 {
				n = nm
			}
			return nil
		})
	}
	if err != nil && len(n) == 0 {
		return err.Error()
	}
	return n
}

func node(it pub.Item, fns ...func(*n)) *n {
	n := &n{Item: it}

	if it == nil {
		n.s = NodeError
		n.n = "Invalid object"
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
	_ = pub.OnObject(&act, func(o *pub.Object) error {
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
		_ = pub.OnItemCollection(it, func(c *pub.ItemCollection) error {
			for _, ob := range c.Collection() {
				result = append(result, node(ob, withParent(parent)))
			}
			return nil
		})
	}
	if pub.ActorTypes.Match(it.GetType()) {
		_ = pub.OnActor(it, func(act *pub.Actor) error {
			result = append(result, getActorElements(*act, parent)...)
			return nil
		})
	}
	if pub.ActivityTypes.Match(it.GetType()) || pub.ObjectTypes.Match(it.GetType()) {
		_ = pub.OnObject(it, func(act *pub.Object) error {
			result = append(result, getObjectElements(*act, parent)...)
			return nil
		})
	}
	return result
}

func (m *model) loadNodeProperties(ctx context.Context, node *n) tea.Cmd {
	m.nodeLoading(node)

	if err := m.f.loadItemProperties(ctx, &node.Item); err != nil {
		m.logFn("error while loading attributes %s", err)
		node.s |= NodeError
	}

	m.logFn("Node properties loaded: %s", node.n)
	if m.timer != nil {
		m.timer.Stop()
	}

	node.stoppedSyncing()

	return noop
}

func (m *model) nodeLoading(node *n) {
	node.startedSyncing()
}

func (m *model) loadNodeCollection(ctx context.Context, nd *n) tea.Cmd {
	if !nd.IsCollection() {
		return noop
	}

	defer func() {
		m.logFn("Collection node loaded: %s", nd.n)
		nd.stoppedSyncing()
		if m.timer != nil {
			m.timer.Stop()
		}
	}()

	m.nodeLoading(nd)
	col, err := m.f.loadCollectionItems(ctx, nd, filters.WithMaxCount(m.height))
	if err != nil {
		m.logFn("error while loading children %s", err)
		nd.s |= NodeError
		return errCmd(err)
	}
	children := make([]*n, 0)
	for _, it := range col.Collection() {
		children = append(children, node(it, withState(tree.NodeCollapsed)))
	}
	nd.setChildren(children...)

	return noop
}

func name(it pub.Item) string {
	n := ""
	_ = pub.OnActor(it, func(a *pub.Actor) error {
		if a.PreferredUsername != nil {
			n = a.PreferredUsername.First().String()
		}
		return nil
	})
	if n != "" {
		return n
	}
	_ = pub.OnObject(it, func(o *pub.Object) error {
		if o.Name != nil {
			n = o.Name.First().String()
			return nil
		}
		if !pub.IsNil(o.URL) {
			if u, err := o.URL.GetLink().URL(); err == nil {
				n = u.Hostname()
				return nil
			}
			return nil
		}
		if u, err := o.ID.GetLink().URL(); err == nil {
			n = u.Hostname()
		}
		return nil
	})
	if n != "" {
		return n
	}
	_ = pub.OnLink(it, func(l *pub.Link) error {
		if l.Name != nil {
			n = l.Name.First().String()
		}
		return nil
	})
	return n
}
