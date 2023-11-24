package motley

import (
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newPagerModel(common *commonModel) pagerModel {

	// Init viewport
	vp := viewport.New(0, 0)
	vp.YPosition = 0
	//vp.HighPerformanceRendering = false
	return pagerModel{
		commonModel: common,
		viewport:    vp,
		itemModel:   newItemModel(common),
	}
}

var _ tea.Model = pagerModel{}

type pagerModel struct {
	*commonModel

	viewport  viewport.Model
	itemModel itemModel
	textInput textinput.Model
}

func (p pagerModel) Init() tea.Cmd {
	p.logFn("Pager init")
	return nil
}

func (p *pagerModel) setSize(w, h int) {
	p.viewport.Width = w
	p.viewport.Height = h - 2 // padding
	p.itemModel.setSize(w, h-2)
	p.logFn("Pager wxh: %dx%d", w, h)
}

func (p *pagerModel) updateNode(n *n) tea.Cmd {
	p.itemModel.item = n.Item
	return nil
}

func (p pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var c tea.Cmd
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.setSize(m.Width, m.Height)
	}
	p.viewport, c = p.viewport.Update(msg)
	p.itemModel, c = p.itemModel.updateAsModel(msg)
	return p, c
}

func (p pagerModel) View() string {
	var b strings.Builder

	p.viewport.SetContent(p.itemModel.View())

	fmt.Fprint(&b,
		lipgloss.NewStyle().Padding(1).Height(p.viewport.Height-2).
			Render(p.viewport.View()),
	)
	return lipgloss.NewStyle().Width(p.viewport.Width).Render(b.String())
}
