package motley

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	vocab "github.com/go-ap/activitypub"
	"github.com/muesli/reflow/wordwrap"
)

var _ tea.Model = NaturalLanguageValues{}

type NaturalLanguageValues struct {
	vocab.NaturalLanguageValues

	Label string

	view        viewport.Model
	selectedRef vocab.LangRef
}

func nameModel(val vocab.NaturalLanguageValues) NaturalLanguageValues {
	m := NewNaturalLanguageValues("Name", val)
	m.view.Height = 1
	return m
}

func summaryModel(val vocab.NaturalLanguageValues) NaturalLanguageValues {
	return NewNaturalLanguageValues("Summary", val)
}

func contentModel(val vocab.NaturalLanguageValues) NaturalLanguageValues {
	return NewNaturalLanguageValues("Content", val)
}

func NewNaturalLanguageValues(label string, val vocab.NaturalLanguageValues) NaturalLanguageValues {
	return NaturalLanguageValues{
		Label:                 label,
		NaturalLanguageValues: val,

		selectedRef: vocab.NilLangRef,
		view:        viewport.New(0, 0),
	}
}

func (n NaturalLanguageValues) Init() tea.Cmd {
	return nil
}

func (n NaturalLanguageValues) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return n, nil
}

func (n NaturalLanguageValues) renderLabel() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Width(9).MaxWidth(9).MarginRight(1)
	return lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render(n.Label))
}

func (n NaturalLanguageValues) renderContent() string {
	contentStyle := lipgloss.NewStyle().MaxHeight(n.view.Height).MaxWidth(n.view.Width)

	if n.selectedRef == vocab.NilLangRef {
		n.selectedRef = n.NaturalLanguageValues.First().Ref
	}
	for _, nlv := range n.NaturalLanguageValues {
		if nlv.Ref != n.selectedRef {
			continue
		}
		wrapped := wordwrap.String(nlv.Value.String(), n.view.Width-2)
		return contentStyle.Render(wrapped)
	}
	return ""
}

func (n NaturalLanguageValues) View() string {
	if len(n.NaturalLanguageValues) == 0 {
		return ""
	}

	label := n.renderLabel()
	content := n.renderContent()

	return lipgloss.JoinHorizontal(lipgloss.Top, label, content)
}
