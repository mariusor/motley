package motley

import (
	tea "github.com/charmbracelet/bubbletea"
	tree "github.com/mariusor/bubbles-tree"
)

type treeModel struct {
	*commonModel

	state state
	list  *tree.Model
}

func newTreeModel(common *commonModel, t tree.Nodes) treeModel {
	ls := tree.New(t)
	ls.Symbols = tree.RoundedSymbols()

	return treeModel{
		commonModel: common,
		list:        &ls,
	}
}

var _ tea.Model = &treeModel{}

func (t *treeModel) Init() tea.Cmd {
	t.logFn("Tree Model init")
	return t.list.Init()
}

type percentageMsg float64

func percentageChanged(f float64) func() tea.Msg {
	return func() tea.Msg {
		return percentageMsg(f)
	}
}

func (t *treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch mm := msg.(type) {
	case state:
		t.state = mm
	}

	if m, cmd := t.list.Update(msg); cmd != nil {
		t.list = m.(*tree.Model)
		return t, tea.Batch(cmd, percentageChanged(t.list.ScrollPercent()))
	}

	return t, noop
}

const minTreeWidth = 32

func (t *treeModel) View() string {
	if t.list.Focused() {
		t.list.Styles.Selected = hintFg
	} else {
		t.list.Styles.Selected = hintDimFg
	}
	return t.list.View()
}

func (t *treeModel) setSize(w, h int) {
	t.logFn("Tree wxh: %dx%d", w, h)

	t.list.SetWidth(w)
	t.list.SetHeight(h)
}

func (t *treeModel) width() int {
	return t.list.Width()
}

func (t *treeModel) Back(previous *tree.Model) (tea.Model, tea.Cmd) {
	previous.Focus()
	t.list = previous
	return t, noop
}

func (t *treeModel) Advance(current *n) *tree.Model {
	current.p = nil

	current.s |= tree.NodeSelected
	newTree := tree.New(tree.Nodes{current})
	newTree.Symbols = t.list.Symbols
	newTree.KeyMap = t.list.KeyMap
	newTree.Styles = t.list.Styles
	newTree.SetWidth(t.list.Width())
	newTree.SetHeight(t.list.Height())

	oldTree := t.list
	t.list = &newTree
	return oldTree
}

func (t *treeModel) IsSyncing() bool {
	return t.state.Is(stateBusy)
}

type state uint8

func (s state) Is(st state) bool {
	return s&st == st
}

const stateBusy state = 1 << iota

func (t *treeModel) startedLoading() tea.Msg {
	t.state |= stateBusy
	return t.state
}

func (t *treeModel) stoppedLoading() tea.Msg {
	if t.state.Is(stateBusy) {
		t.state ^= stateBusy
	}
	return t.state
}
