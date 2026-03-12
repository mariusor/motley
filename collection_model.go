package motley

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	vocab "github.com/go-ap/activitypub"
)

// CollectionModel
type CollectionModel struct {
	ObjectModel

	Total uint
}

func newCollectionModel() CollectionModel {
	return CollectionModel{}
}

func (c CollectionModel) Init() tea.Cmd {
	return noop
}

func (c CollectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return c, noop
}

func (c CollectionModel) View() tea.View {
	obView := c.ObjectModel.View()

	totalView := fmt.Sprintf("Total: %d", c.Total)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Bottom, obView.Content, totalView))
}

func (c *CollectionModel) updateCollection(col vocab.CollectionInterface) error {
	_ = vocab.OnObject(col, func(object *vocab.Object) error {
		c.Object = *object
		return nil
	})

	c.Total = col.Count()
	return nil
}
