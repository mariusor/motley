package motley

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/config"
	"git.sr.ht/~mariusor/motley/internal/env"
	"git.sr.ht/~mariusor/motley/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/mariusor/qstring"
	"golang.org/x/sync/errgroup"
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

type store struct {
	root pub.Item
	env  env.Type
	s    config.FullStorage
}

type fedbox struct {
	tree   map[pub.IRI]pub.Item
	items  pub.IRIs
	stores []store
	logFn  loggerFn
}

func FedBOX(rootIRIs []string, st []config.Storage, l lw.Logger) (*fedbox, error) {
	logFn = l.Infof
	stores := make([]store, 0)
	var appendStore = func(stores *[]store, db config.FullStorage, e env.Type, it pub.Item) {
		if pub.IsNil(it) {
			return
		}
		*stores = append(*stores, store{root: it, s: db, env: e})
	}
	errs := make([]error, 0)
	for _, s := range st {
		found := false
		for _, iri := range rootIRIs {
			s.Host = iri
			db, err := storage.Storage(s, s.Env, l)
			if err != nil {
				l.Debugf("unable to initialize %s storage %s: %+v", s.Type, s.Path, err)
				errs = append(errs, errors.Annotatef(err, "Unable to initialize %s storage %s", s.Type, s.Path))
				continue
			}
			if it, err := db.Load(pub.IRI(iri)); err == nil {
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
		}
		if !found {
			l.Debugf("unable to load main Actor for storage[%s] %s", s.Type, s.Path)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &fedbox{tree: make(map[pub.IRI]pub.Item), stores: stores, logFn: l.Infof}, nil
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
	var state tree.NodeState
	if len(f.getRootNodes()) == 1 {
		state = tree.NodeLastChild
	}
	for _, root := range f.getRootNodes() {
		nodes = append(nodes, node(
			root,
			withState(state),
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

}

func (n *n) stoppedSyncing() {
	n.s ^= NodeSyncing
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

func (n *n) View() string {
	if n == nil || n.s.Is(tree.NodeHidden) {
		return ""
	}
	hints := n.s
	annotation := ""
	st := lipgloss.NewStyle()
	if nodeIsError(n) {
		st = faintRedFg
		annotation = Attention
	}

	if n.Item != nil && nodeIsCollapsible(n) {
		annotation = Expanded
		if hints.Is(tree.NodeCollapsed) {
			annotation = Collapsed
		}
		if len(n.c) == 0 {
			annotation = Unexpandable
			st = st.Faint(true)
		}
		if n.s.Is(NodeSyncing) {
			annotation = Attention
		}
	}

	return fmt.Sprintf("%-1s %s", annotation, st.Render(n.n))
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
	case pub.LinkTypes.Contains(typ):
		err = pub.OnLink(it, func(l *pub.Link) error {
			if nm := name(l); len(nm) > 0 {
				n = fmt.Sprintf("%s[%s]", nm, typ)
			}
			return nil
		})
	case pub.ActorTypes.Contains(typ):
		err = pub.OnActor(it, func(act *pub.Actor) error {
			if nm := name(act); len(nm) > 0 {
				n = fmt.Sprintf("%s[%s]", nm, typ)
			}
			return nil
		})
	case pub.ActivityTypes.Contains(typ), pub.IntransitiveActivityTypes.Contains(typ):
		n = string(typ)
		err = pub.OnActivity(it, func(act *pub.Activity) error {
			obType := ""
			pub.OnObject(act.Object, func(ob *pub.Object) error {
				obType = string(ob.GetType())
				return nil
			})
			if len(obType) > 0 {
				n = fmt.Sprintf("%s » %s", typ, obType)
			}
			return nil
		})
	case pub.ObjectTypes.Contains(typ):
		err = pub.OnObject(it, func(ob *pub.Object) error {
			if nm := name(ob); len(nm) > 0 {
				n = fmt.Sprintf("%s[%s]", nm, typ)
			} else {
				n = string(typ)
			}
			return nil
		})
	case typ == "":
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
	if pub.ActorTypes.Contains(it.GetType()) {
		_ = pub.OnActor(it, func(act *pub.Actor) error {
			result = append(result, getActorElements(*act, parent)...)
			return nil
		})
	}
	if pub.ActivityTypes.Contains(it.GetType()) || pub.ObjectTypes.Contains(it.GetType()) {
		_ = pub.OnObject(it, func(act *pub.Object) error {
			result = append(result, getObjectElements(*act, parent)...)
			return nil
		})
	}
	return result
}

func (m *model) loadDepsForNode(ctx context.Context, node *n) tea.Cmd {
	if nodeIsSynced(node) {
		m.logFn("Node already loaded: %s", node.n)
		return nil
	}
	node.startedSyncing()
	defer func() {
		node.s |= NodeSynced
		node.stoppedSyncing()
		m.logFn("Node loaded: %s", node.n)
	}()

	if err := dereferenceItemProperties(ctx, m.f, &node.Item); err != nil {
		m.logFn("error while loading attributes %s", err)
		node.s |= NodeError
	}

	if node.s.Is(tree.NodeCollapsible) && len(node.c) == 0 {
		count := filters.WithMaxCount(m.height)
		if err := m.loadChildrenForNode(ctx, node, count); err != nil {
			m.logFn("error while loading children %s", err)
			node.s |= NodeError
		}
	}

	return m.tree.stoppedLoading
}

func (m *model) loadChildrenForNode(ctx context.Context, nn *n, ff ...filters.Check) error {
	accum := func(children *[]*n) func(ctx context.Context, col pub.CollectionInterface) error {
		return func(ctx context.Context, col pub.CollectionInterface) error {
			for _, it := range col.Collection() {
				*children = append(*children, node(it, withState(tree.NodeCollapsed)))
			}
			return nil
		}
	}

	if len(nn.c) == 0 {
		children := make([]*n, 0)
		pub.OnCollectionIntf(nn.Item, func(col pub.CollectionInterface) error {
			return accum(&children)(ctx, col)
		})
		if len(children) == 0 {
			iri := nn.Item.GetLink()
			if err := accumFn(accum(&children)).LoadFromSearch(ctx, m.f, iri, ff...); err != nil {
				return err
			}
		}
		nn.setChildren(children...)
	}
	return nil
}

func dereferenceIRIs(ctx context.Context, f *fedbox, iris pub.ItemCollection) pub.ItemCollection {
	if len(iris) == 0 {
		return nil
	}
	items := make(pub.ItemCollection, 0, len(iris))
	for _, it := range iris {
		if deref := dereferenceIRI(ctx, f, it); pub.IsItemCollection(deref) {
			_ = pub.OnItemCollection(deref, func(col *pub.ItemCollection) error {
				items = append(items, pub.ItemCollectionDeduplication(col)...)
				return nil
			})
		} else {
			items = append(items, deref)
		}
	}
	return items
}

func dereferenceIRI(ctx context.Context, f *fedbox, it pub.Item) pub.Item {
	if pub.IsNil(it) {
		return nil
	}
	if !pub.IsIRI(it) {
		return it
	}
	if pub.PublicNS.Equals(it.GetLink(), false) {
		return it
	}
	loadFn := func(ctx context.Context, col pub.CollectionInterface) error {
		it = col
		return nil
	}
	accumFn(loadFn).LoadFromSearch(ctx, f, it.GetLink())

	return it
}

func dereferenceIntransitiveActivityProperties(ctx context.Context, f *fedbox) func(act *pub.IntransitiveActivity) error {
	if f == nil {
		return func(act *pub.IntransitiveActivity) error { return fmt.Errorf("invalid fedbox storage") }
	}
	return func(act *pub.IntransitiveActivity) error {
		g, gtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			pub.OnObject(act, dereferenceObjectProperties(gtx, f))
			act.Actor = dereferenceIRI(gtx, f, act.Actor)
			act.Target = dereferenceIRI(gtx, f, act.Target)
			act.Instrument = dereferenceIRI(gtx, f, act.Instrument)
			act.Result = dereferenceIRI(gtx, f, act.Result)
			return nil
		})
		return g.Wait()
	}
}

func dereferenceActivityProperties(ctx context.Context, f *fedbox) func(act *pub.Activity) error {
	if f == nil {
		return func(act *pub.Activity) error { return fmt.Errorf("invalid fedbox storage") }
	}
	return func(act *pub.Activity) error {
		g, gtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			pub.OnIntransitiveActivity(act, dereferenceIntransitiveActivityProperties(ctx, f))
			act.Actor = dereferenceIRI(gtx, f, act.Actor)
			return nil
		})
		return g.Wait()
	}
}

func dereferenceObjectProperties(ctx context.Context, f *fedbox) func(ob *pub.Object) error {
	if f == nil {
		return func(ob *pub.Object) error { return fmt.Errorf("invalid fedbox storage") }
	}
	return func(ob *pub.Object) error {
		g, gtx := errgroup.WithContext(ctx)
		g.Go(func() error {
			ob.AttributedTo = dereferenceIRI(gtx, f, ob.AttributedTo)
			ob.InReplyTo = dereferenceIRI(gtx, f, ob.InReplyTo)
			ob.Tag = dereferenceIRIs(gtx, f, ob.Tag)
			ob.To = dereferenceIRIs(ctx, f, ob.To)
			ob.CC = dereferenceIRIs(ctx, f, ob.CC)
			ob.Bto = dereferenceIRIs(ctx, f, ob.Bto)
			ob.BCC = dereferenceIRIs(ctx, f, ob.BCC)
			ob.Audience = dereferenceIRIs(ctx, f, ob.Audience)
			return nil
		})
		return g.Wait()
	}
}

type StopLoad struct{}

func (s StopLoad) Error() string {
	return "stop load"
}

func dereferenceItemProperties(ctx context.Context, f *fedbox, it *pub.Item) error {
	ob := *it
	if pub.IsIRI(ob) {
		*it = dereferenceIRI(ctx, f, ob.GetLink())
	}
	if pub.IsObject(ob) {
		typ := ob.GetType()
		switch {
		case pub.ObjectTypes.Contains(typ), pub.ActorTypes.Contains(typ), typ == "":
			return pub.OnObject(*it, dereferenceObjectProperties(ctx, f))
		case pub.IntransitiveActivityTypes.Contains(typ):
			return pub.OnIntransitiveActivity(*it, dereferenceIntransitiveActivityProperties(ctx, f))
		case pub.ActivityTypes.Contains(typ):
			return pub.OnActivity(*it, dereferenceActivityProperties(ctx, f))
		}
	}

	if pub.IsItemCollection(ob) {
		return pub.OnItemCollection(*it, func(col *pub.ItemCollection) error {
			*it = dereferenceIRIs(ctx, f, *col)
			return nil
		})
	}
	return nil
}

func getCollectionPrevNext(col pub.Item) (prev, next pub.IRI) {
	qFn := func(i pub.Item) url.Values {
		if i == nil {
			return url.Values{}
		}
		if u, err := i.GetLink().URL(); err == nil {
			return u.Query()
		}
		return url.Values{}
	}
	beforeFn := func(i pub.Item) pub.IRI {
		return pub.IRI(qFn(i).Get("before"))
	}
	afterFn := func(i pub.Item) pub.IRI {
		return pub.IRI(qFn(i).Get("after"))
	}
	nextFromLastFn := func(i pub.Item) pub.IRI {
		if u, err := i.GetLink().URL(); err == nil {
			_, next := path.Split(u.Path)
			return pub.IRI(next)
		}
		return ""
	}
	switch col.GetType() {
	case pub.OrderedCollectionPageType:
		if c, ok := col.(*pub.OrderedCollectionPage); ok {
			prev = beforeFn(c.Prev)
			if int(c.TotalItems) > len(c.OrderedItems) {
				next = afterFn(c.Next)
			}
		}
	case pub.OrderedCollectionType:
		if c, ok := col.(*pub.OrderedCollection); ok {
			if len(c.OrderedItems) > 0 && int(c.TotalItems) > len(c.OrderedItems) {
				next = nextFromLastFn(c.OrderedItems[len(c.OrderedItems)-1])
			}
		}
	case pub.CollectionPageType:
		if c, ok := col.(*pub.CollectionPage); ok {
			prev = beforeFn(c.Prev)
			if int(c.TotalItems) > len(c.Items) {
				next = afterFn(c.Next)
			}
		}
	case pub.CollectionType:
		if c, ok := col.(*pub.Collection); ok {
			if len(c.Items) > 0 && int(c.TotalItems) > len(c.Items) {
				next = nextFromLastFn(c.Items[len(c.Items)-1])
			}
		}
	}
	// NOTE(marius): we check if current Collection id contains a cursor, and if `next` points to the same URL
	//   we don't take it into consideration.
	if next != "" {
		f := struct {
			Next pub.IRI `qstring:"after"`
		}{}
		if err := qstring.Unmarshal(qFn(col.GetLink()), &f); err == nil && next.Equals(f.Next, false) {
			next = ""
		}
	}
	return prev, next
}

type accumFn func(context.Context, pub.CollectionInterface) error

func (f *fedbox) searchFn(ctx context.Context, g *errgroup.Group, loadIRI pub.IRI, accumFn accumFn, ff ...filters.Check) func() error {
	return func() error {
		col, err := f.Load(loadIRI, ff...)
		if err != nil {
			return errors.Annotatef(err, "failed to load search: %s", loadIRI)
		}

		if col.IsCollection() {
			maxItems := 0
			err = pub.OnCollectionIntf(col, func(c pub.CollectionInterface) error {
				maxItems = int(c.Count())
				return accumFn(ctx, c)
			})
			if err != nil {
				return err
			}

			count := filters.MaxCount(ff...)
			if count < 0 {
				count = 0
			}
			if maxItems <= count {
				return StopLoad{}
			}
			before, next := getCollectionPrevNext(col)
			if len(next) > 0 {
				ff = append(ff, filters.After(filters.SameID(next)))
			}
			if len(before) > 0 {
				ff = append(ff, filters.Before(filters.SameID(before)))
			}
			if len(next)+len(before) > 0 {
				g.Go(f.searchFn(ctx, g, loadIRI, accumFn, ff...))
			}
			return nil
		}
		return emptyAccum(ctx, nil) //accumFn(ctx, &pub.ItemCollection{col})
	}
}

func emptyAccum(_ context.Context, _ pub.CollectionInterface) error {
	return nil
}

func (a accumFn) LoadFromSearch(ctx context.Context, f *fedbox, iri pub.IRI, ff ...filters.Check) error {
	var cancel func()
	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	g.Go(f.searchFn(ctx, g, iri, a, ff...))

	if err := g.Wait(); err != nil {
		if errors.Is(err, StopLoad{}) {
			f.logFn("stopped loading search")
		} else {
			f.logFn("failed to load search %+s", err)
			return err
		}
	}
	return nil
}

func name(it pub.Item) string {
	n := ""
	pub.OnActor(it, func(a *pub.Actor) error {
		if a.PreferredUsername != nil {
			n = a.PreferredUsername.First().String()
		}
		return nil
	})
	if n != "" {
		return n
	}
	pub.OnObject(it, func(o *pub.Object) error {
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
	pub.OnLink(it, func(l *pub.Link) error {
		if l.Name != nil {
			n = l.Name.First().String()
		}
		return nil
	})
	return n
}
