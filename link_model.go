package motley

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	vocab "github.com/go-ap/activitypub"
)

type LinkModel struct {
	vocab.Link

	Name NaturalLanguageValues
}

func newLinkModel() LinkModel {
	return LinkModel{}
}
func (l LinkModel) Init() tea.Cmd {
	return noop
}

func (l LinkModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return l, noop
}

func (l LinkModel) View() string {
	if l.ID == "" {
		return ""
	}
	pieces := make([]string, 0)

	// TODO(marius): move this to initialization, and move the setting of the nat values into the Update()
	if len(l.Link.Name) > 0 {
		l.Name = nameModel(l.Link.Name)
	}

	typeStyle := lipgloss.NewStyle().Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	title := typeStyle.Render(ItemType(l))
	if l.MediaType != "" {
		title = lipgloss.JoinHorizontal(lipgloss.Right, title, typeStyle.Bold(false).Render(" ("+string(l.MediaType)+")"))
	}
	pieces = append(pieces, title)
	if name := l.Name.View(); len(name) > 0 {
		pieces = append(pieces, name)
	}

	return lipgloss.JoinVertical(lipgloss.Top, pieces...)
}

func (l *LinkModel) updateLink(lnk *vocab.Link) error {
	l.Link = *lnk
	return nil
}
