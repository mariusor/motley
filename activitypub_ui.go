package motley

import (
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	vocab "github.com/go-ap/activitypub"
)

var _ tea.Model = itemModel{}

type itemModel struct {
	*commonModel

	viewport viewport.Model

	item vocab.Item
}

func (i *itemModel) setSize(w, h int) {
	i.viewport.Height = h
	i.viewport.Width = w
}

func (i itemModel) View() string {
	s := strings.Builder{}
	i.writeItem(&s, i.item)
	i.viewport.SetContent(s.String())
	return i.viewport.View()
}

func (i itemModel) writeActorItemIdentifier(s io.Writer, it vocab.Item) {
	i.writeActorIdentifier(s, it)
}

func (i itemModel) writeItemWithLabel(s io.Writer, l string, it vocab.Item) {
	if vocab.IsNil(it) {
		return
	}
	if c, ok := it.(vocab.ItemCollection); ok && len(c) == 0 {
		return
	}
	fmt.Fprintf(s, "%s: ", l)
	i.writeItem(s, it)
	s.Write([]byte{'\n'})
}

func (i itemModel) writeObject(s io.Writer) func(ob *vocab.Object) error {
	return func(ob *vocab.Object) error {
		fmt.Fprintf(s, "IRI: %s\n", ob.ID)
		if len(ob.Type) > 0 {
			m := textinput.New()
			m.Prompt = "Type: "
			m.SetValue(string(ob.Type))
			fmt.Fprintf(s, "%s\n", m.View())
		}
		if len(ob.MediaType) > 0 {
			fmt.Fprintf(s, "MediaType: %s\n", ob.MediaType)
		}

		//i.writeActorItemIdentifier(s, ob.AttributedTo)
		//fmt.Fprintf(s, "Recipients: ")
		//i.writeActorItemIdentifier(s, ob.To)
		//i.writeItemWithLabel(s, "CC", ob.CC)
		//i.writeItemWithLabel(s, "Bto", ob.Bto)
		//i.writeItemWithLabel(s, "BCC", ob.BCC)
		//i.writeItemWithLabel(s, "Audience", ob.Audience)

		i.writeNaturalLanguageValuesWithLabel(s, "Name", ob.Name)
		//i.writeNaturalLanguageValuesWithLabel(s, "Summary", ob.Summary)
		//i.writeNaturalLanguageValuesWithLabel(s, "Content", ob.Content)

		//if ob.Source.Content != nil {
		//	if len(ob.MediaType) > 0 {
		//		fmt.Fprintf(s, "Source[%s]: %s\n", ob.Source.MediaType, ob.Source.Content)
		//	} else {
		//		fmt.Fprintf(s, "Source: %s\n", ob.Source.Content)
		//	}
		//}

		//i.writeItemWithLabel(s, "URL", ob.URL)
		//
		//i.writeItemWithLabel(s, "Context", ob.Context)
		//i.writeItemWithLabel(s, "InReplyTo", ob.InReplyTo)
		//
		//i.writeItemWithLabel(s, "Tag", ob.Tag)
		return nil
	}
}
func (i itemModel) writeActivity(s io.Writer) func(act *vocab.Activity) error {
	return func(act *vocab.Activity) error {
		if err := vocab.OnIntransitiveActivity(act, i.writeIntransitiveActivity(s)); err != nil {
			return err
		}
		//i.writeItemWithLabel(s, "Object", act.Object)
		return nil
	}
}
func (i itemModel) writeIntransitiveActivity(s io.Writer) func(act *vocab.IntransitiveActivity) error {
	return func(act *vocab.IntransitiveActivity) error {
		if err := vocab.OnObject(act, i.writeObject(s)); err != nil {
			return err
		}
		//i.writeItemWithLabel(s, "Actor", act.Actor)
		//i.writeItemWithLabel(s, "Target", act.Target)
		//i.writeItemWithLabel(s, "Result", act.Result)
		//i.writeItemWithLabel(s, "Origin", act.Origin)
		//i.writeItemWithLabel(s, "Instrument", act.Instrument)
		return nil
	}
}
func (i itemModel) writeActor(s io.Writer) func(act *vocab.Actor) error {
	return func(act *vocab.Actor) error {
		return i.writeActorIdentifier(s, act)
	}
}
func (i itemModel) writeItemCollection(s io.Writer) func(col *vocab.ItemCollection) error {
	return func(col *vocab.ItemCollection) error {
		for _, it := range col.Collection() {
			if err := i.writeItem(s, it); err != nil {
				//p.logFn("error: %s", err)
			}
		}
		return nil
	}
}
func (i itemModel) writeCollection(s io.Writer) func(col vocab.CollectionInterface) error {
	return func(col vocab.CollectionInterface) error {
		for _, it := range col.Collection() {
			if err := i.writeItem(s, it); err != nil {
				//p.logFn("error: %s", err)
			}
		}
		return nil
	}
}

func (i itemModel) writeNaturalLanguageValuesWithLabel(s io.Writer, l string, values vocab.NaturalLanguageValues) error {
	ll := len(values)
	if ll == 0 {
		return nil
	}
	if ll == 1 {
		fmt.Fprintf(s, "%s: %s\n", l, values[0])
		return nil
	}
	vals := make([]string, len(values))
	for i, v := range values {
		if v.Ref == "" || v.Ref == vocab.NilLangRef {
			vals[i] = fmt.Sprintf("%s", v.Value)
		} else {
			vals[i] = fmt.Sprintf("[%s]%s", v.Ref, v.Value)
		}
	}
	if ll > 1 {
		fmt.Fprintf(s, "%s: [ %s ]\n", l, strings.Join(vals, ", "))
	}
	return nil
}

func (i itemModel) writeActorIdentifier(s io.Writer, it vocab.Item) error {
	if c, ok := it.(vocab.ItemCollection); ok && len(c) == 0 {
		return nil
	}
	return vocab.OnActor(it, func(act *vocab.Actor) error {
		if act.ID.Equals(vocab.PublicNS, true) {
			return nil
		}
		return vocab.OnObject(act, i.writeObject(s))
	})
}

func (i itemModel) writeItem(s io.Writer, it vocab.Item) error {
	if it == nil {
		return nil
	}

	if vocab.IsIRI(it) {
		fmt.Fprintf(s, "%s\n", it.GetLink())
		return nil
	}

	if vocab.IsItemCollection(it) {
		return vocab.OnItemCollection(it, i.writeItemCollection(s))
	}
	typ := it.GetType()
	if vocab.IntransitiveActivityTypes.Contains(typ) {
		return vocab.OnIntransitiveActivity(it, i.writeIntransitiveActivity(s))
	}
	if vocab.ActivityTypes.Contains(typ) {
		return vocab.OnActivity(it, i.writeActivity(s))
	}
	if vocab.ActorTypes.Contains(typ) {
		return vocab.OnActor(it, i.writeActor(s))
	}
	if vocab.ObjectTypes.Contains(typ) || typ == "" {
		return vocab.OnObject(it, i.writeObject(s))
	}
	return fmt.Errorf("unknown activitypub object of type %T", it)
}

func newItemModel(common *commonModel) itemModel {
	// Init viewport
	vp := viewport.New(0, 0)
	vp.YPosition = 0
	//vp.HighPerformanceRendering = false
	return itemModel{commonModel: common, viewport: vp}
}
func (i itemModel) Init() tea.Cmd {
	i.logFn("Item View init")
	return nil
}

func (i itemModel) updateAsModel(msg tea.Msg) (itemModel, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	switch msg := msg.(type) {
	case nodeUpdateMsg:
		i.item = msg.Item
	case tea.KeyMsg:
		switch msg.String() {
		case "home", "g":
			i.viewport.GotoTop()
			if i.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.Sync(i.viewport))
			}
		case "end", "G":
			i.viewport.GotoBottom()
			if i.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.Sync(i.viewport))
			}
		}
	}

	return i, tea.Batch(cmds...)
}

func (i itemModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return i.updateAsModel(msg)
}

type collectionModel struct{}
