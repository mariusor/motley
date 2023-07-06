package motley

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
)

type pagerModel struct {
	*commonModel

	viewport  viewport.Model
	textInput textinput.Model
}

func newPagerModel(common *commonModel) pagerModel {
	// Init viewport
	vp := viewport.Model{}
	vp.YPosition = 0
	vp.HighPerformanceRendering = false

	return pagerModel{
		commonModel: common,
		viewport:    vp,
	}
}

func (p *pagerModel) Init() tea.Cmd {
	p.logFn("pager init")
	return nil
}

func (p *pagerModel) setSize(w, h int) {
	p.viewport.Width = w
	p.viewport.Height = h - 2 // padding
	p.logFn("Pager wxh: %dx%d", w, h)
}

type pagerNode struct {
	*n
}

func (p pagerNode) View() string {
	s := strings.Builder{}
	p.writeItem(&s, p.n.Item)
	return lipgloss.NewStyle().Render(s.String())
}

func (p pagerNode) writeActorItemIdentifier(s io.Writer, it pub.Item) {
	p.writeActorIdentifier(s, it)
}

func (p pagerNode) writeItemWithLabel(s io.Writer, l string, it pub.Item) {
	if pub.IsNil(it) {
		return
	}
	if c, ok := it.(pub.ItemCollection); ok && len(c) == 0 {
		return
	}
	fmt.Fprintf(s, "%s: ", l)
	p.writeItem(s, it)
	s.Write([]byte{'\n'})
}

func (p pagerNode) writeObject(s io.Writer) func(ob *pub.Object) error {
	return func(ob *pub.Object) error {
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

		p.writeActorItemIdentifier(s, ob.AttributedTo)
		fmt.Fprintf(s, "Recipients: ")
		p.writeActorItemIdentifier(s, ob.To)
		p.writeItemWithLabel(s, "CC", ob.CC)
		p.writeItemWithLabel(s, "Bto", ob.Bto)
		p.writeItemWithLabel(s, "BCC", ob.BCC)
		p.writeItemWithLabel(s, "Audience", ob.Audience)

		p.writeNaturalLanguageValuesWithLabel(s, "Name", ob.Name)
		p.writeNaturalLanguageValuesWithLabel(s, "Summary", ob.Summary)
		p.writeNaturalLanguageValuesWithLabel(s, "Content", ob.Content)

		if ob.Source.Content != nil {
			if len(ob.MediaType) > 0 {
				fmt.Fprintf(s, "Source[%s]: %s\n", ob.Source.MediaType, ob.Source.Content)
			} else {
				fmt.Fprintf(s, "Source: %s\n", ob.Source.Content)
			}
		}

		p.writeItemWithLabel(s, "URL", ob.URL)

		p.writeItemWithLabel(s, "Context", ob.Context)
		p.writeItemWithLabel(s, "InReplyTo", ob.InReplyTo)

		p.writeItemWithLabel(s, "Tag", ob.Tag)
		return nil
	}
}
func (p pagerNode) writeActivity(s io.Writer) func(act *pub.Activity) error {
	return func(act *pub.Activity) error {
		if err := pub.OnIntransitiveActivity(act, p.writeIntransitiveActivity(s)); err != nil {
			return err
		}
		p.writeItemWithLabel(s, "Object", act.Object)
		return nil
	}
}
func (p pagerNode) writeIntransitiveActivity(s io.Writer) func(act *pub.IntransitiveActivity) error {
	return func(act *pub.IntransitiveActivity) error {
		if err := pub.OnObject(act, p.writeObject(s)); err != nil {
			return err
		}
		p.writeItemWithLabel(s, "Actor", act.Actor)
		p.writeItemWithLabel(s, "Target", act.Target)
		p.writeItemWithLabel(s, "Result", act.Result)
		p.writeItemWithLabel(s, "Origin", act.Origin)
		p.writeItemWithLabel(s, "Instrument", act.Instrument)
		return nil
	}
}
func (p pagerNode) writeActor(s io.Writer) func(act *pub.Actor) error {
	return func(act *pub.Actor) error {
		return p.writeActorIdentifier(s, act)
	}
}
func (p pagerNode) writeItemCollection(s io.Writer) func(col *pub.ItemCollection) error {
	return func(col *pub.ItemCollection) error {
		for _, it := range col.Collection() {
			if err := p.writeItem(s, it); err != nil {
				//p.logFn("error: %s", err)
			}
		}
		return nil
	}
}
func (p pagerNode) writeCollection(s io.Writer) func(col pub.CollectionInterface) error {
	return func(col pub.CollectionInterface) error {
		for _, it := range col.Collection() {
			if err := p.writeItem(s, it); err != nil {
				//p.logFn("error: %s", err)
			}
		}
		return nil
	}
}

func (p pagerNode) writeNaturalLanguageValuesWithLabel(s io.Writer, l string, values pub.NaturalLanguageValues) error {
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
		if v.Ref == "" || v.Ref == pub.NilLangRef {
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

func (p pagerNode) writeActorIdentifier(s io.Writer, it pub.Item) error {
	if c, ok := it.(pub.ItemCollection); ok && len(c) == 0 {
		return nil
	}
	return pub.OnActor(it, func(act *pub.Actor) error {
		if act.ID.Equals(pub.PublicNS, true) {
			return nil
		}
		return pub.OnObject(act, p.writeObject(s))
	})
}

func (p pagerNode) writeItem(s io.Writer, it pub.Item) error {
	if it == nil {
		return nil
	}

	if pub.IsIRI(it) {
		fmt.Fprintf(s, "%s\n", it.GetLink())
		return nil
	}

	if pub.IsItemCollection(it) {
		return pub.OnItemCollection(it, p.writeItemCollection(s))
	}
	typ := it.GetType()
	if pub.IntransitiveActivityTypes.Contains(typ) {
		return pub.OnIntransitiveActivity(it, p.writeIntransitiveActivity(s))
	}
	if pub.ActivityTypes.Contains(typ) {
		return pub.OnActivity(it, p.writeActivity(s))
	}
	if pub.ActorTypes.Contains(typ) {
		return pub.OnActor(it, p.writeActor(s))
	}
	if pub.ObjectTypes.Contains(typ) || typ == "" {
		return pub.OnObject(it, p.writeObject(s))
	}
	return fmt.Errorf("unknown activitypub object of type %T", it)
}

func (p *pagerModel) updateNode(n *n) tea.Cmd {
	nn := pagerNode{n}
	p.setContent(nn.View())
	return nil
}

func (p *pagerModel) setContent(s string) {
	p.viewport.SetContent(s)
}

func (p *pagerModel) unload() {
	p.viewport.SetContent("")
	p.viewport.YOffset = 0
}

func (p *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case paintMsg:
		cmds = append(cmds, p.updateNode(msg.n))
	case tea.KeyMsg:
		switch msg.String() {
		case "home", "g":
			p.viewport.GotoTop()
			if p.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.Sync(p.viewport))
			}
		case "end", "G":
			p.viewport.GotoBottom()
			if p.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.Sync(p.viewport))
			}
		}
	}

	return p, tea.Batch(cmds...)
}

func (p *pagerModel) View() string {
	var b strings.Builder

	fmt.Fprint(&b,
		lipgloss.NewStyle().Padding(1).Height(p.viewport.Height-2).
			Render(p.viewport.View()),
	)
	return lipgloss.NewStyle().Width(p.viewport.Width).Render(b.String())
}
