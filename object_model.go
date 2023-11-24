package motley

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	vocab "github.com/go-ap/activitypub"
)

var _ tea.Model = ObjectModel{}

// ObjectModel
// We need to group different properties which serve similar purposes into the same controls
// Audience: To, CC, Bto, BCC
// Name: Name, PreferredName
// Content: Summary, Content, Source
type ObjectModel struct {
	vocab.Object

	Name    NaturalLanguageValues
	Summary NaturalLanguageValues
	Content NaturalLanguageValues
}

func newObjectModel() ObjectModel {
	return ObjectModel{}
}

func (o ObjectModel) Init() tea.Cmd {
	return nil
}

func (o ObjectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return o, nil
}

func (o ObjectModel) View() string {
	pieces := make([]string, 0)

	// TODO(marius): move this to initialization, and move the setting of the nat values into the Update()
	if len(o.Object.Name) > 0 {
		o.Name = nameModel(o.Object.Name)
	}
	if len(o.Object.Summary) > 0 {
		o.Summary = summaryModel(o.Object.Summary)
	}
	if len(o.Object.Content) > 0 {
		o.Content = contentModel(o.Object.Content)
	}

	typ := lipgloss.NewStyle().Bold(true).Render(string(o.GetType()))
	pieces = append(pieces, typ)
	if name := o.Name.View(); len(name) > 0 {
		pieces = append(pieces, name)
	}
	if summary := o.Summary.View(); len(summary) > 0 {
		pieces = append(pieces, summary)
	}
	if content := o.Content.View(); len(content) > 0 {
		pieces = append(pieces, content)
	}

	return lipgloss.JoinVertical(lipgloss.Top, pieces...)
}

func (o *ObjectModel) updateObject(ob *vocab.Object) error {
	o.Object = *ob
	return nil
}
