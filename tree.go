package motley

import (
	tea "github.com/charmbracelet/bubbletea"
	tree "github.com/mariusor/bubbles-tree"
)

type treeModel struct {
	*commonModel
	list *tree.Model
}

func newTreeModel(common *commonModel, t tree.Nodes) treeModel {
	ls := tree.New(t)
	ls.Symbols = tree.DefaultSymbols()
	ls.Symbols.UpAndRight = "╰─"
	ls.Focus()
	return treeModel{
		commonModel: common,
		list:        &ls,
	}
}

func (t *treeModel) Init() tea.Cmd {
	return nil
}

func (t *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := t.list.Update(msg)
	t.list = m.(*tree.Model)
	return t, cmd
}

func (t *treeModel) View() string {
	return t.list.View()
}

func (t *treeModel) setSize(w, h int) {
	t.list.SetWidth(w)
	t.list.SetHeight(h - statusBarHeight)
	t.logFn("tree size: %dx%d", t.list.Width(), t.list.Height())
}
