package motley

import (
	pub "github.com/go-ap/activitypub"
	st "github.com/go-ap/storage"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/openshift/osin"
)

type fedbox struct {
	iri pub.IRI
	s    st.Store
	o    osin.Storage
}

func (f fedbox) getService() (pub.Item, error) {
	return f.s.Load(f.iri)
}

func (f fedbox) Advance(to string) (tree.Treeish, error) {
	f.iri = pub.IRI(to)
	return f, nil
}

func (f fedbox) State(what string) (tree.NodeState, error) {
	return 0, nil
}

func (f fedbox) Walk(depth int) ([]string, error) {
	result := []string{
		"ana",
		"are",
		"mere",
	}
	return result, nil
}
