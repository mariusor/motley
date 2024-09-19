package motley

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
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

func (c CollectionModel) Init() (tea.Model, tea.Cmd) {
	return c, noop
}

func (c CollectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return c, noop
}

func (c CollectionModel) View() string {
	obView := c.ObjectModel.View()

	totalView := fmt.Sprintf("Total: %d", c.Total)
	return lipgloss.JoinVertical(lipgloss.Bottom, obView, totalView)
}

func (c *CollectionModel) updateCollection(col vocab.CollectionInterface) error {
	vocab.OnObject(col, func(object *vocab.Object) error {
		c.Object = *object
		return nil
	})

	c.Total = col.Count()
	return nil
}
