package motley

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	vocab "github.com/go-ap/activitypub"
)

type ActivityModel struct {
	ObjectModel

	Actor  ObjectModel
	Object ObjectModel
}

func newActivityModel() ActivityModel {
	return ActivityModel{}
}

func (l ActivityModel) Init() tea.Cmd {
	return noop
}

func (l ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return l, noop
}

func (l ActivityModel) View() tea.View {
	selfView := l.ObjectModel.View()

	obView := l.Object.View()
	actView := l.Actor.View()

	return tea.NewView(lipgloss.JoinVertical(lipgloss.Top, selfView.Content, actView.Content, obView.Content))
}

func (l *ActivityModel) updateActivity(act *vocab.Activity) error {
	_ = vocab.OnObject(act, func(ob *vocab.Object) error {
		return l.ObjectModel.updateObject(ob)
	})
	_ = vocab.OnObject(act.Object, func(ob *vocab.Object) error {
		return l.Object.updateObject(ob)
	})
	_ = vocab.OnObject(act.Actor, func(ob *vocab.Object) error {
		return l.Actor.updateObject(ob)
	})
	return nil
}
