package motley

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	f "github.com/go-ap/fedbox"
	"github.com/go-ap/filters"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/mariusor/qstring"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	//HasChanges   = "⧆"
	//Locked       = "⚿"
	Collapsed    = "⊞"
	Expanded     = "⊟"
	Unexpandable = "⬚"
	//Unexpandable = "⊠"
	Attention = "⊡"
)

const (
	NodeSyncing = tree.NodeLastChild << (iota + 1)
	NodeSynced
	NodeError
)

type loggerFn func(string, ...interface{})

var logFn = func(string, ...interface{}) {}

type fedbox struct {
	tree  map[pub.IRI]pub.Item
	iri   pub.IRI
	s     f.FullStorage
	logFn loggerFn
}

func FedBOX(base pub.IRI, r f.FullStorage, l *logrus.Logger) *fedbox {
	logFn = l.Infof
	return &fedbox{tree: make(map[pub.IRI]pub.Item), iri: base, s: r, logFn: l.Infof}
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

func nodeIsError(n *n) bool {
	return n.s.Is(NodeError)
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
	return n, nil
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
	name := filepath.Base(it.GetLink().String())
	var err error
	typ := it.GetType()
	switch {
	case pub.ActorTypes.Contains(typ):
		err = pub.OnActor(it, func(act *pub.Actor) error {
			if len(act.PreferredUsername) > 0 {
				name = fmt.Sprintf("%s", act.PreferredUsername.First())
			} else if len(act.Name) > 0 {
				name = fmt.Sprintf("%s", act.Name.First())
			}
			return nil
		})
	case pub.ActivityTypes.Contains(typ), pub.IntransitiveActivityTypes.Contains(typ):
		err = pub.OnIntransitiveActivity(it, func(act *pub.IntransitiveActivity) error {
			name = fmt.Sprintf("%s", typ)
			return nil
		})
	case pub.ObjectTypes.Contains(typ):
		err = pub.OnObject(it, func(ob *pub.Object) error {
			if len(ob.Name) > 0 {
				name = fmt.Sprintf("[%s]%s", typ, ob.Name.First())
			} else {
				name = fmt.Sprintf("%s", typ)
			}
			return nil
		})
	case typ == "":
		err = pub.OnObject(it, func(ob *pub.Object) error {
			if len(ob.Name) > 0 {
				name = fmt.Sprintf("%s", ob.Name.First())
			}
			return nil
		})
	}
	if err != nil && len(name) == 0 {
		return err.Error()
	}
	return name
}

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
	return result
}

func (m *model) loadDepsForNode(ctx context.Context, node *n) tea.Cmd {
	g, gtx := errgroup.WithContext(ctx)
	node.s |= NodeSyncing
	g.Go(func() error {
		if err := dereferenceItemProperties(gtx, m.f, node.Item); err != nil {
			m.logFn("error while loading attributes %s", err)
			node.s |= NodeError
			return err
		}
		return nil
	})

	g.Go(func() error {
		if node.s.Is(tree.NodeCollapsible) && len(node.c) == 0 {
			if err := m.loadChildrenForNode(ctx, node); err != nil {
				node.s |= NodeError
				m.logFn("error while loading children %s", err)
				return err
			}
		}
		return nil
	})
	go func() {
		g.Wait()
		node.s ^= NodeSyncing
	}()
	return m.status.spinner.Tick
}

func (m *model) loadChildrenForNode(ctx context.Context, nn *n) error {
	iri := nn.Item.GetLink()
	accum := func(children *[]*n) func(ctx context.Context, col pub.CollectionInterface) error {
		return func(ctx context.Context, col pub.CollectionInterface) error {
			for _, it := range col.Collection() {
				*children = append(*children, node(it, withState(tree.NodeCollapsed)))
			}
			return nil
		}
	}

	children := make([]*n, 0)
	if err := accumFn(accum(&children)).LoadFromSearch(ctx, m.f, iri); err != nil {
		return err
	}
	nn.setChildren(children...)
	return nil
}

func dereferenceIRIs(ctx context.Context, f *fedbox, iris pub.ItemCollection) pub.ItemCollection {
	if len(iris) == 0 {
		return nil
	}
	items := make(pub.ItemCollection, 0, len(iris))
	for _, it := range iris {
		if deref := dereferenceIRI(ctx, f, it); pub.IsItemCollection(deref) {
			pub.OnItemCollection(deref, func(col *pub.ItemCollection) error {
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

func dereferenceItemProperties(ctx context.Context, f *fedbox, it pub.Item) error {
	if pub.IsObject(it) {
		typ := it.GetType()
		switch {
		case pub.ObjectTypes.Contains(typ), pub.ActorTypes.Contains(typ), typ == "":
			return pub.OnObject(it, dereferenceObjectProperties(ctx, f))
		case pub.IntransitiveActivityTypes.Contains(typ):
			return pub.OnIntransitiveActivity(it, dereferenceIntransitiveActivityProperties(ctx, f))
		case pub.ActivityTypes.Contains(typ):
			return pub.OnActivity(it, dereferenceActivityProperties(ctx, f))
		}
	}

	if pub.IsItemCollection(it) {
		return pub.OnItemCollection(it, func(col *pub.ItemCollection) error {
			it = dereferenceIRIs(ctx, f, *col)
			return nil
		})
	}
	return nil
}

func getCollectionPrevNext(col pub.Item) (prev, next string) {
	qFn := func(i pub.Item) url.Values {
		if i == nil {
			return url.Values{}
		}
		if u, err := i.GetLink().URL(); err == nil {
			return u.Query()
		}
		return url.Values{}
	}
	beforeFn := func(i pub.Item) string {
		return qFn(i).Get("before")
	}
	afterFn := func(i pub.Item) string {
		return qFn(i).Get("after")
	}
	nextFromLastFn := func(i pub.Item) string {
		if u, err := i.GetLink().URL(); err == nil {
			_, next = path.Split(u.Path)
			return next
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
			Next string `qstring:"after"`
		}{}
		if err := qstring.Unmarshal(qFn(col.GetLink()), &f); err == nil && next == f.Next {
			next = ""
		}
	}
	return prev, next
}

const MaxItems = 100

type accumFn func(context.Context, pub.CollectionInterface) error

func (f *fedbox) searchFn(ctx context.Context, g *errgroup.Group, loadIRI pub.IRI, accumFn accumFn) func() error {
	return func() error {
		col, err := f.s.Load(loadIRI)
		if err != nil {
			return errors.Annotatef(err, "failed to load search: %s", loadIRI)
		}

		if pub.IsItemCollection(col) {
			maxItems := 0
			err = pub.OnCollectionIntf(col, func(c pub.CollectionInterface) error {
				maxItems = int(c.Count())
				return accumFn(ctx, c)
			})
			if err != nil {
				return err
			}

			var next string

			if maxItems-MaxItems < 5 {
				if _, next = getCollectionPrevNext(col); len(next) > 0 {
					g.Go(f.searchFn(ctx, g, loadIRI, accumFn))
				}
				return nil
			} else {
				return StopLoad{}
			}
		}
		return accumFn(ctx, &pub.ItemCollection{col})
	}
}

func emptyAccum(ctx context.Context, c pub.CollectionInterface) error {
	return nil
}

func (a accumFn) LoadFromSearch(ctx context.Context, f *fedbox, iris ...pub.IRI) error {
	var cancelFn func()

	ctx, cancelFn = context.WithCancel(ctx)
	g, gtx := errgroup.WithContext(ctx)

	for _, iri := range iris {
		g.Go(f.searchFn(gtx, g, iri, a))
	}
	if err := g.Wait(); err != nil {
		if errors.Is(err, StopLoad{}) {
			f.logFn("stopped loading search")
			cancelFn()
		} else {
			f.logFn("%s", err)
		}
	}
	return nil
}

func pubUrl(it pub.Item) string {
	name := ""
	pub.OnObject(it, func(o *pub.Object) error {
		u, _ := o.URL.GetLink().URL()
		name = u.Hostname()
		return nil
	})
	return name
}
