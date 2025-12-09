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

func (l ActivityModel) Update(msg tea.Msg) tea.Cmd {
	return noop
}

func (l ActivityModel) View() string {
	selfView := l.ObjectModel.View()

	obView := l.Object.View()
	actView := l.Actor.View()

	return lipgloss.JoinVertical(lipgloss.Top, selfView, actView, obView)
}

func (l *ActivityModel) updateActivity(act *vocab.Activity) error {
	vocab.OnObject(act, func(ob *vocab.Object) error {
		return l.ObjectModel.updateObject(ob)
	})
	vocab.OnObject(act.Object, func(ob *vocab.Object) error {
		return l.Object.updateObject(ob)
	})
	vocab.OnObject(act.Actor, func(ob *vocab.Object) error {
		return l.Actor.updateObject(ob)
	})
	return nil
}
