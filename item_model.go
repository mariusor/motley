package motley

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	vocab "github.com/go-ap/activitypub"
)

var _ tea.Model = pagerModel{}

type pagerModel struct {
	*commonModel

	item vocab.Item

	viewport viewport.Model
	model    tea.Model
}

func (p *pagerModel) setSize(w, h int) {
	p.viewport.Height = h
	p.viewport.Width = w
}

func (p pagerModel) View() string {
	h := p.viewport.Height
	w := p.viewport.Width
	s := lipgloss.NewStyle().Height(h).MaxHeight(h).MaxWidth(w).Width(w)
	p.viewport.SetContent(s.Render(p.model.View()))
	return p.viewport.View()
}

func (p *pagerModel) updateIntransitiveActivity(a *vocab.IntransitiveActivity) error {
	// TODO(marius): IntransitiveActivity stuff
	return nil
}

func (p *pagerModel) updateActivity(a *vocab.Activity) error {
	if err := vocab.OnIntransitiveActivity(a, p.updateIntransitiveActivity); err != nil {
		return err
	}
	// TODO(marius): Activity stuff
	return nil
}

func (p *pagerModel) updateActor(a *vocab.Actor) error {
	return nil
}

func (p *pagerModel) updateObject(o *vocab.Object) error {
	return nil
}
func (p *pagerModel) updateItems(items *vocab.ItemCollection) error {
	return nil
}

func (p *pagerModel) updateModel(it vocab.Item) error {
	if it == nil {
		return nil
	}

	if vocab.IsItemCollection(it) {
		return vocab.OnItemCollection(it, p.updateItems)
	}
	typ := it.GetType()
	if vocab.IntransitiveActivityTypes.Contains(typ) {
		return vocab.OnIntransitiveActivity(it, p.updateIntransitiveActivity)
	}
	if vocab.ActivityTypes.Contains(typ) {
		return vocab.OnActivity(it, p.updateActivity)
	}
	if vocab.ActorTypes.Contains(typ) {
		return vocab.OnActor(it, p.updateActor)
	}
	if vocab.ObjectTypes.Contains(typ) || typ == "" {
		return vocab.OnObject(it, p.updateObject)
	}
	return fmt.Errorf("unknown activitypub object of type %T", it)
}

func newItemModel(common *commonModel) pagerModel {
	// Init viewport
	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return pagerModel{
		commonModel: common,
		viewport:    vp,
		model:       M,
	}
}
func (p pagerModel) Init() tea.Cmd {
	p.logFn("Item View init")
	return noop
}

func (p pagerModel) updateAsModel(msg tea.Msg) (pagerModel, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	switch mm := msg.(type) {
	case tea.WindowSizeMsg:
		p.logFn("item resize: %+v", msg)
	case nodeUpdateMsg:
		var content tea.Model = M
		p.item = mm.Item
		if !(vocab.IsIRI(p.item) || vocab.IsItemCollection(p.item)) {
			ob := newObjectModel()
			if err := vocab.OnObject(p.item, ob.updateObject); err != nil {
				cmds = append(cmds, errCmd(err))
			}
			content = ob
		}
		p.model = content
	case tea.KeyMsg:
		switch mm.String() {
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

func (p pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p.updateAsModel(msg)
}

func ItemType(o vocab.Item) string {
	if typ := string(o.GetType()); typ != "" {
		return typ
	}
	return "Unknown"
}
