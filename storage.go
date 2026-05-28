package motley

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path"

	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	"github.com/mariusor/qstring"
)

func (f *fedbox) loadCollectionItems(ctx context.Context, nn *n, ff ...filters.Check) (pub.ItemCollection, error) {
	if len(nn.c) == 0 {
		items, err := LoadFromSearch(ctx, f, nn.GetLink(), ff...)
		if err != nil {
			return nil, err
		}
		return items, nil
	}
	f.l.With(slog.String("IRI", string(nn.GetLink()))).Debug("item already loaded")
	return nil, nil
}

func (f *fedbox) loadItemProperties(ctx context.Context, it *pub.Item) error {
	ob := *it
	if pub.IsIRI(ob) {
		*it = f.dereferenceIRI(ctx, ob.GetLink())
	}
	if pub.IsObject(ob) {
		typ := ob.GetType()
		switch {
		case pub.ObjectTypes.Match(typ), pub.ActorTypes.Match(typ), pub.NilType.Match(typ):
			return pub.OnObject(*it, f.dereferenceObjectProperties(ctx))
		case pub.IntransitiveActivityTypes.Match(typ):
			return pub.OnIntransitiveActivity(*it, f.dereferenceIntransitiveActivityProperties(ctx))
		case pub.ActivityTypes.Match(typ):
			return pub.OnActivity(*it, f.dereferenceActivityProperties(ctx))
		}
	}

	return nil
}

func (f *fedbox) dereferenceObjectProperties(ctx context.Context) func(ob *pub.Object) error {
	if f == nil {
		return func(ob *pub.Object) error { return errInvalidStorage }
	}
	return func(ob *pub.Object) error {
		ob.AttributedTo = f.dereferenceIRI(ctx, ob.AttributedTo)
		ob.InReplyTo = f.dereferenceIRI(ctx, ob.InReplyTo)
		ob.Tag = f.dereferenceIRIs(ctx, ob.Tag)
		ob.To = f.dereferenceIRIs(ctx, ob.To)
		ob.CC = f.dereferenceIRIs(ctx, ob.CC)
		ob.Bto = f.dereferenceIRIs(ctx, ob.Bto)
		ob.BCC = f.dereferenceIRIs(ctx, ob.BCC)
		ob.Audience = f.dereferenceIRIs(ctx, ob.Audience)
		return nil
	}
}

func (f *fedbox) dereferenceIRIs(ctx context.Context, iris pub.ItemCollection) pub.ItemCollection {
	if len(iris) == 0 {
		return nil
	}
	items := make(pub.ItemCollection, 0, len(iris))
	for _, it := range iris {
		if deref := f.dereferenceIRI(ctx, it); pub.IsItemCollection(deref) {
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

var errInvalidStorage = fmt.Errorf("invalid storage")

func (f *fedbox) dereferenceIntransitiveActivityProperties(ctx context.Context) func(act *pub.IntransitiveActivity) error {
	if f == nil {
		return func(act *pub.IntransitiveActivity) error { return errInvalidStorage }
	}
	return func(act *pub.IntransitiveActivity) error {
		err := pub.OnObject(act, f.dereferenceObjectProperties(ctx))
		if err != nil {
			return err
		}
		act.Actor = f.dereferenceIRI(ctx, act.Actor)
		act.Target = f.dereferenceIRI(ctx, act.Target)
		act.Instrument = f.dereferenceIRI(ctx, act.Instrument)
		act.Result = f.dereferenceIRI(ctx, act.Result)
		return nil
	}
}

func (f *fedbox) dereferenceActivityProperties(ctx context.Context) func(act *pub.Activity) error {
	if f == nil {
		return func(act *pub.Activity) error { return errInvalidStorage }
	}
	return func(act *pub.Activity) error {
		err := pub.OnIntransitiveActivity(act, f.dereferenceIntransitiveActivityProperties(ctx))
		if err != nil {
			return err
		}
		act.Actor = f.dereferenceIRI(ctx, act.Actor)
		return nil
	}
}

func (f *fedbox) dereferenceIRI(ctx context.Context, it pub.Item) pub.Item {
	if pub.IsNil(it) {
		return nil
	}
	if !pub.IsIRI(it) {
		return it
	}
	if pub.PublicNS.Equals(it.GetLink(), false) {
		return it
	}

	ob, err := f.Load(it.GetLink(), filters.WithMaxCount(0))
	if err == nil {
		it = ob
	}

	return it
}

func (f *fedbox) searchFn(ctx context.Context, loadIRI pub.IRI, ff ...filters.Check) func() (pub.ItemCollection, error) {
	return func() (pub.ItemCollection, error) {
		col, err := f.Load(loadIRI, ff...)
		if err != nil {
			return nil, errors.Annotatef(err, "failed to load search: %s", loadIRI)
		}

		var result pub.ItemCollection
		err = pub.OnCollectionIntf(col, func(c pub.CollectionInterface) error {
			result = c.Collection()
			return nil
		})
		if err != nil {
			return nil, errors.Annotatef(err, "invalid collection IRI: %s", loadIRI)
		}
		return result, nil
	}
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

type StopLoad struct{}

func (s StopLoad) Error() string {
	return "stop load"
}

func LoadFromSearch(ctx context.Context, f *fedbox, iri pub.IRI, ff ...filters.Check) (pub.ItemCollection, error) {
	return f.searchFn(ctx, iri, ff...)()
	/*
		if err := g.Wait(); err != nil {
			if errors.Is(err, StopLoad{}) {
				f.logFn("stopped loading search")
			} else {
				f.logFn("failed to load search %+s", err)
				return err
			}
		}
		return nil
	*/
}
