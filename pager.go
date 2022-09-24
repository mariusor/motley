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
	p.viewport.Height = h
	p.logFn("Pager wxh: %dx%d", w, h)
}

func (p *pagerModel) writePropertyWithLabel(s io.Writer, l string, it pub.Item) {
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

func (p *pagerModel) writeObject(s io.Writer) func(ob *pub.Object) error {
	return func(ob *pub.Object) error {
		fmt.Fprintf(s, "Type: %s\n", ob.Type)
		fmt.Fprintf(s, "IRI: %s\n", ob.ID)
		if len(ob.MediaType) > 0 {
			fmt.Fprintf(s, "MediaType: %s\n", ob.MediaType)
		}

		p.writePropertyWithLabel(s, "AttributedTo", ob.AttributedTo)
		p.writePropertyWithLabel(s, "To", ob.To)
		p.writePropertyWithLabel(s, "CC", ob.CC)
		p.writePropertyWithLabel(s, "Bto", ob.Bto)
		p.writePropertyWithLabel(s, "BCC", ob.BCC)
		p.writePropertyWithLabel(s, "Audience", ob.Audience)

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

		p.writePropertyWithLabel(s, "URL", ob.URL)

		p.writePropertyWithLabel(s, "Context", ob.Context)
		p.writePropertyWithLabel(s, "InReplyTo", ob.InReplyTo)

		p.writePropertyWithLabel(s, "Tag", ob.Tag)
		return nil
	}
}
func (p *pagerModel) writeActivity(s io.Writer) func(act *pub.Activity) error {
	return func(act *pub.Activity) error {
		if err := pub.OnIntransitiveActivity(act, p.writeIntransitiveActivity(s)); err != nil {
			return err
		}
		p.writePropertyWithLabel(s, "Object", act.Object)
		return nil
	}
}
func (p *pagerModel) writeIntransitiveActivity(s io.Writer) func(act *pub.IntransitiveActivity) error {
	return func(act *pub.IntransitiveActivity) error {
		if err := pub.OnObject(act, p.writeObject(s)); err != nil {
			return err
		}
		p.writePropertyWithLabel(s, "Actor", act.Actor)
		p.writePropertyWithLabel(s, "Target", act.Target)
		p.writePropertyWithLabel(s, "Result", act.Result)
		p.writePropertyWithLabel(s, "Origin", act.Origin)
		p.writePropertyWithLabel(s, "Instrument", act.Instrument)
		return nil
	}
}
func (p *pagerModel) writeActor(s io.Writer) func(act *pub.Actor) error {
	return func(act *pub.Actor) error {
		if err := pub.OnObject(act, p.writeObject(s)); err != nil {
			return err
		}
		p.writeNaturalLanguageValuesWithLabel(s, "PreferredUsername", act.PreferredUsername)
		p.writePropertyWithLabel(s, "Streams", act.Streams)
		if act.Endpoints != nil {
			p.writeItem(s, act.Endpoints.SharedInbox)
		}
		if len(act.PublicKey.ID) > 0 {
			fmt.Fprintf(s, "PublicKey: %s", act.PublicKey.PublicKeyPem)
		}
		return nil
	}
}
func (p *pagerModel) writeItemCollection(s io.Writer) func(col *pub.ItemCollection) error {
	return func(col *pub.ItemCollection) error {
		for _, it := range col.Collection() {
			if err := p.writeItem(s, it); err != nil {
				p.logFn("error: %s", err)
			}
		}
		return nil
	}
}
func (p *pagerModel) writeCollection(s io.Writer) func(col pub.CollectionInterface) error {
	return func(col pub.CollectionInterface) error {
		for _, it := range col.Collection() {
			if err := p.writeItem(s, it); err != nil {
				p.logFn("error: %s", err)
			}
		}
		return nil
	}
}

func (p *pagerModel) writeNaturalLanguageValuesWithLabel(s io.Writer, l string, values pub.NaturalLanguageValues) error {
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

func (p *pagerModel) writeItem(s io.Writer, it pub.Item) error {
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

func (p *pagerModel) showItem(it pub.Item) tea.Cmd {
	s := strings.Builder{}
	if err := p.writeItem(&s, it); err != nil {
		p.logFn("err: %s", err)
		return nil
	}
	p.setContent(s.String())
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
		if n := msg.n; n != nil {
			cmds = append(cmds, p.showItem(n.Item))
		}
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
	fmt.Fprint(&b, p.viewport.View())
	return lipgloss.NewStyle().Width(p.viewport.Width).Render(b.String())
}
