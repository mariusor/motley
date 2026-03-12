package motley

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	vocab "github.com/go-ap/activitypub"
)

type viewModel interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
}

var _ tea.Model = pagerModel{}

type pagerModel struct {
	*commonModel

	item vocab.Item

	viewport viewport.Model
	model    tea.Model
}

func (p *pagerModel) setSize(w, h int) {
	p.viewport.SetHeight(h)
	p.viewport.SetWidth(w)
}

func (p pagerModel) View() tea.View {
	h := p.viewport.Height()
	w := p.viewport.Width()
	s := lipgloss.NewStyle().Height(h).MaxHeight(h).MaxWidth(w).Width(w)
	p.viewport.SetContent(s.Render(p.model.View().Content))
	return tea.NewView(p.viewport.View())
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
	if vocab.IntransitiveActivityTypes.Match(typ) {
		return vocab.OnIntransitiveActivity(it, p.updateIntransitiveActivity)
	}
	if vocab.ActivityTypes.Match(typ) {
		return vocab.OnActivity(it, p.updateActivity)
	}
	if vocab.ActorTypes.Match(typ) {
		return vocab.OnActor(it, p.updateActor)
	}
	if vocab.ObjectTypes.Match(typ) || vocab.NilType.Match(typ) {
		return vocab.OnObject(it, p.updateObject)
	}
	return fmt.Errorf("unknown activitypub object of type %T", it)
}

func newItemModel(common *commonModel) pagerModel {
	// Init viewport
	vp := viewport.New()
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

func (p *pagerModel) updateAsModel(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	switch mm := msg.(type) {
	case tea.WindowSizeMsg:
		p.logFn("item resize: %+v", msg)
	case nodeUpdateMsg:
		var content tea.Model = M
		p.item = mm.Item
		if vocab.IsIRI(p.item) {
		}
		if vocab.IsItemCollection(p.item) {
		}
		if vocab.IsObject(p.item) {
			t := p.item.GetType()
			switch {
			case vocab.ActivityVocabularyTypes{vocab.AcceptType, vocab.AddType, vocab.AnnounceType, vocab.BlockType, vocab.CreateType,
				vocab.DeleteType, vocab.DislikeType, vocab.FlagType, vocab.FollowType, vocab.IgnoreType,
				vocab.InviteType, vocab.JoinType, vocab.LeaveType, vocab.LikeType, vocab.ListenType,
				vocab.MoveType, vocab.OfferType, vocab.RejectType, vocab.ReadType, vocab.RemoveType,
				vocab.TentativeRejectType, vocab.TentativeAcceptType, vocab.UndoType, vocab.UpdateType, vocab.ViewType}.Match(t):
				ob := newActivityModel()
				if err := vocab.OnActivity(p.item, ob.updateActivity); err != nil {
					cmds = append(cmds, errCmd(err))
				}
				content = ob
			case vocab.ActivityVocabularyTypes{vocab.OrderedCollectionPageType, vocab.CollectionPageType, vocab.OrderedCollectionType, vocab.CollectionType}.Match(t):
				ob := newCollectionModel()
				if err := vocab.OnCollectionIntf(p.item, ob.updateCollection); err != nil {
					cmds = append(cmds, errCmd(err))
				}
				content = ob
			default:
				ob := newObjectModel()
				if err := vocab.OnObject(p.item, ob.updateObject); err != nil {
					cmds = append(cmds, errCmd(err))
				}
				content = ob
			}
		}
		if vocab.IsLink(p.item) {
			ob := newLinkModel()
			if err := vocab.OnLink(p.item, ob.updateLink); err != nil {
				cmds = append(cmds, errCmd(err))
			}
			content = ob
		}
		p.model = content
	case tea.KeyMsg:
		switch mm.String() {
		case "home", "g":
			p.viewport.GotoTop()
		case "end", "G":
			p.viewport.GotoBottom()
		}
	}

	return tea.Batch(cmds...)
}

func (p pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := p.updateAsModel(msg)
	return p, cmd
}

func ItemType(o vocab.Item) string {
	typ := o.GetType()
	if typ != nil {
		return typ.AsTypes().String()
	}
	return "Unknown"
}
