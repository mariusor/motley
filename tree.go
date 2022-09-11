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
	return treeModel{
		commonModel: common,
		list:        &ls,
	}
}

func (t *treeModel) Init() tea.Cmd {
	return nil
}

type percentageMsg float32

func percentageChanged(f float32) func() tea.Msg {
	return func() tea.Msg {
		return percentageMsg(f)
	}
}

func treeHeight(n tree.Nodes) int {
	visible := 0
	for _, nn := range n {
		st := nn.State()
		if st.Is(tree.NodeHidden) {
			continue
		}
		visible++
		if st.Is(tree.NodeCollapsible) && !st.Is(tree.NodeCollapsed) {
			visible = treeHeight(nn.Children())
		}
	}
	return visible
}

func (t *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := t.list.Update(msg)
	t.list = m.(*tree.Model)

	f := float32(t.list.YOffset()+t.list.Height()) / float32(treeHeight(t.list.Children())) * 100.0
	return t, tea.Batch(cmd, percentageChanged(f))
}

const treeWidth = 32

func (t *treeModel) View() string {
	return t.list.View()
}

func (t *treeModel) setSize(w, h int) {
	t.list.SetWidth(w)
	t.list.SetHeight(h)
}

func (t *treeModel) Back(previous *tree.Model) (tea.Model, tea.Cmd) {
	previous.SetWidth(t.list.Width())
	previous.SetHeight(t.list.Height())
	previous.Focus()
	t.list = previous
	return t, nil
}

func (t *treeModel) Advance(current *n) *tree.Model {
	current.p = nil

	newTree := tree.New(tree.Nodes{current})
	newTree.Symbols = t.list.Symbols
	newTree.KeyMap = t.list.KeyMap
	newTree.Styles = t.list.Styles
	newTree.SetWidth(t.list.Width())
	newTree.SetHeight(t.list.Height())
	newTree.Focus()

	oldTree := t.list
	t.list = &newTree
	return oldTree
}
