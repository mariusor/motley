package motley

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/handlers"
	st "github.com/go-ap/storage"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/openshift/osin"
)

type fedbox struct {
	tree map[pub.IRI]pub.Item
	iri  pub.IRI
	s    st.Store
	o    osin.Storage
}

func FedBOX(base pub.IRI, r st.Store, o osin.Storage) *fedbox {
	return &fedbox{tree: make(map[pub.IRI]pub.Item), iri: base, s: r, o: o}
}

func (f fedbox) getService() (pub.Item, error) {
	col, err := f.s.Load(f.iri)
	if err != nil {
		return nil, err
	}
	var service pub.Item
	pub.OnObject(col, func(o *pub.Object) error {
		service = o
		return nil
	})
	return service, nil
}

func (f *fedbox) Advance(to string) (tree.Treeish, error) {
	f.iri = pub.IRI(to)
	return f, nil
}

func (f *fedbox) State(what string) (tree.NodeState, error) {
	var curNode pub.Item
	for iri, it := range f.tree {
		if iri.Equals(pub.IRI(what), false) {
			curNode = it
			break
		}
	}
	if curNode == nil {
		return 0, nil
	}
	var st tree.NodeState = tree.NodeVisible
	if pub.IsItemCollection(curNode) {
		st |= tree.NodeCollapsible
	}
	if _, col := handlers.Split(curNode.GetLink()); col != "" {
		st |= tree.NodeCollapsible
	}
	//fmt.Fprintf(os.Stderr, "%s state %d\n", what, st)
	return st, nil
}

func getObjectElements(ob pub.Object) []string {
	result := make([]string, 0)
	if ob.Likes != nil {
		result = append(result, ob.Likes.GetLink().String())
	}
	if ob.Shares != nil {
		result = append(result, ob.Shares.GetLink().String())
	}
	if ob.Replies != nil {
		result = append(result, ob.Replies.GetLink().String())
	}
	return result
}

func getActorElements(act pub.Actor) []string {
	result := make([]string, 0)
	pub.OnObject(&act, func(o *pub.Object) error {
		result = append(result, getObjectElements(*o)...)
		return nil
	})
	result = append(result, act.Inbox.GetLink().String())
	result = append(result, act.Outbox.GetLink().String())
	if act.Liked != nil {
		result = append(result, act.Liked.GetLink().String())
	}
	return result
}
func getItemElements(it pub.Item) []string {
	result := make([]string, 0)
	if pub.IsItemCollection(it) {
		pub.OnItemCollection(it, func(c *pub.ItemCollection) error {
			for _, it := range c.Collection() {
				result = append(getItemElements(it))
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

func (f *fedbox) Walk(depth int) ([]string, error) {
	//fmt.Fprintf(os.Stderr, "Walking %#v\n", *f)
	it, err := f.s.Load(f.iri)
	if err != nil {
		return nil, err
	}

	if it.IsCollection() {
		err = pub.OnCollectionIntf(it, func(col pub.CollectionInterface) error {
			for _, it := range col.Collection() {
				f.tree[it.GetLink()] = it
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		f.tree[it.GetLink()] = it
	}
	//fmt.Fprintf(os.Stderr, "%#v\n", f.tree)
	var result []string
	for iri := range f.tree {
		result = append(result, iri.String())
		result = append(result, getItemElements(f.tree[iri])...)
	}

	//fmt.Fprintf(os.Stderr, "results %#v\n", result)
	return result, err
}
