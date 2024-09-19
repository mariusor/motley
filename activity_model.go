package motley

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
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

func (l ActivityModel) Init() (tea.Model, tea.Cmd) {
	return l, noop
}

func (l ActivityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return l, noop
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
