package motley

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	vocab "github.com/go-ap/activitypub"
)

var _ tea.Model = itemModel{}

type itemModel struct {
	*commonModel

	viewport viewport.Model

	item vocab.Item

	model tea.Model
}

func (i *itemModel) setSize(w, h int) {
	i.viewport.Height = h
	i.viewport.Width = w
}

func (i itemModel) View() string {
	h := i.viewport.Height
	w := i.viewport.Width
	s := lipgloss.NewStyle().Height(h).MaxHeight(h).MaxWidth(w).Width(w)
	i.viewport.SetContent(s.Render(i.model.View()))
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

func (i *itemModel) updateIntransitiveActivity(a *vocab.IntransitiveActivity) error {
	// TODO(marius): IntransitiveActivity stuff
	return nil
}

func (i *itemModel) updateActivity(a *vocab.Activity) error {
	if err := vocab.OnIntransitiveActivity(a, i.updateIntransitiveActivity); err != nil {
		return err
	}
	// TODO(marius): Activity stuff
	return nil
}

func (i *itemModel) updateActor(a *vocab.Actor) error {
	return nil
}

func (i *itemModel) updateItems(items *vocab.ItemCollection) error {
	return nil
}

func (i *itemModel) updateModel(it vocab.Item) error {
	if it == nil {
		return nil
	}

	if vocab.IsItemCollection(it) {
		return vocab.OnItemCollection(it, i.updateItems)
	}
	typ := it.GetType()
	if vocab.IntransitiveActivityTypes.Contains(typ) {
		return vocab.OnIntransitiveActivity(it, i.updateIntransitiveActivity)
	}
	if vocab.ActivityTypes.Contains(typ) {
		return vocab.OnActivity(it, i.updateActivity)
	}
	if vocab.ActorTypes.Contains(typ) {
		return vocab.OnActor(it, i.updateActor)
	}
	//if vocab.ObjectTypes.Contains(typ) || typ == "" {
	//	return vocab.OnObject(it, i.updateObject)
	//}
	return fmt.Errorf("unknown activitypub object of type %T", it)
}

func newItemModel(common *commonModel) itemModel {
	// Init viewport
	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return itemModel{
		commonModel: common,
		viewport:    vp,
		model:       newObjectModel(),
	}
}
func (i itemModel) Init() tea.Cmd {
	i.logFn("Item View init")
	return noop
}

func (i itemModel) updateAsModel(msg tea.Msg) (itemModel, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	switch mm := msg.(type) {
	case tea.WindowSizeMsg:
		i.logFn("item resize: %+v", msg)
	case nodeUpdateMsg:
		if mm.n != nil {
			i.item = mm.n.Item
			ob := newObjectModel()
			err := vocab.OnObject(i.item, ob.updateObject)
			if err != nil {
				cmds = append(cmds, errCmd(err))
			} else {
				i.model = ob
			}
		}
	case tea.KeyMsg:
		switch mm.String() {
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

func ItemType(o vocab.Item) string {
	if typ := string(o.GetType()); typ != "" {
		return typ
	}
	return "Unknown"
}
