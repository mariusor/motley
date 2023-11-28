package motley

import (
	"context"
	"fmt"
	f "github.com/go-ap/fedbox"
	"time"

	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/muesli/reflow/wordwrap"
	"github.com/sirupsen/logrus"
)

const (
	noteCharacterLimit   = 256             // should match server
	statusMessageTimeout = time.Second * 2 // how long to show status messages like "stashed!"
	ellipsis             = "…"

	wrapAt = 60
)

var (
	faintRedFg = newFgStyle(FaintRed)

	hintFg    = lipgloss.NewStyle().Background(hintColor)
	hintDimFg = lipgloss.NewStyle().Background(hintDimColor)
)

var (
	// Color wraps lipgloss.ColorProfile.Color, which produces a color for use in termenv styling.
	Color = lipgloss.ColorProfile().Color

	// HasDarkBackground stores whether the terminal has a dark background.
	HasDarkBackground = lipgloss.HasDarkBackground()
)

// Colors for dark and light backgrounds.
var (
	normalFgColor    = NewColorPair("#dddddd", "#1a1a1a")
	dimNormalFgColor = NewColorPair("#777777", "#A49FA5")

	hintColor    = Indigo       //NewColorPair("#F793FF", "#AD58B4")
	hintDimColor = SubtleIndigo //NewColorPair("#6B3A6F", "#F6C9FF")

	brightGrayColor    = NewColorPair("#979797", "#847A85")
	dimBrightGrayColor = NewColorPair("#4D4D4D", "#C2B8C2")

	grayFgColor     = NewColorPair("#626262", "#909090")
	midGrayFgColor  = NewColorPair("#4A4A4A", "#B2B2B2")
	darkGrayFgColor = NewColorPair("#3C3C3C", "#DDDADA")

	Indigo       = NewColorPair("#7571F9", "#5A56E0")
	SubtleIndigo = NewColorPair("#514DC1", "#7D79F6")
	Cream        = NewColorPair("#FFFDF5", "#FFFDF5")
	YellowGreen  = NewColorPair("#ECFD65", "#04B575")
	Fuchsia      = NewColorPair("#EE6FF8", "#EE6FF8")
	DimFuchsia   = NewColorPair("#99519E", "#F1A8FF")
	Green        = NewColorPair("#04B575", "#04B575")
	Red          = NewColorPair("#ED567A", "#FF4672")
	FaintRed     = NewColorPair("#C74665", "#FF6F91")
	SpinnerColor = NewColorPair("#747373", "#8E8E8E")
	NoColor      = NewColorPair("", "")
)

// Functions for styling strings.
var (
	IndigoFg       = lipgloss.Style{}.Foreground(Indigo).Render
	SubtleIndigoFg = lipgloss.Style{}.Foreground(SubtleIndigo).Render
	RedFg          = lipgloss.Style{}.Foreground(Red).Render
	FaintRedFg     = lipgloss.Style{}.Foreground(FaintRed).Render
)

var (
	GlamourStyle    = "auto"
	GlamourMaxWidth = 800
)

var (
	mintGreen = NewColorPair("#89F0CB", "#89F0CB")
	darkGreen = NewColorPair("#1C8760", "#1C8760")

	statusBarNoteFg       = NewColorPair("#7D7D7D", "#656565")
	pagerHelpHeight       int
	statusBarFailStyle    = newStyle(NewColorPair("#1B1B1B", "#f2f2f2"), FaintRed, false)
	statusBarMessageStyle = newStyle(mintGreen, darkGreen, false)
	helpViewStyle         = newStyle(statusBarNoteFg, NewColorPair("#1B1B1B", "#f2f2f2"), false)
)

func Launch(conf config.Options, r f.FullStorage, l *logrus.Logger) error {
	base := pub.IRI(conf.BaseURL)
	mm := newModel(FedBOX(base, r, l), conf.Env, l)
	_, err := tea.NewProgram(mm, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func newModel(ff *fedbox, env env.Type, l *logrus.Logger) *model {
	if lipgloss.HasDarkBackground() {
		GlamourStyle = "dark"
	} else {
		GlamourStyle = "light"
	}

	m := new(model)
	m.commonModel = new(commonModel)
	m.commonModel.logFn = l.Infof

	m.f = ff

	m.tree = newTreeModel(m.commonModel, initNodes(m.f))
	m.pager = newItemModel(m.commonModel)
	m.status = newStatusModel(m.commonModel)
	m.status.logo = logoView(pubUrl(ff.getService()), env)
	return m
}

type commonModel struct {
	f     *fedbox
	logFn func(string, ...interface{})
}

type model struct {
	*commonModel

	width  int
	height int

	currentNode         *n
	currentNodePosition int
	breadCrumbs         []*tree.Model

	tree   treeModel
	pager  itemModel
	status statusModel
}

func (m *model) Init() tea.Cmd {
	m.logFn("UI init")
	m.breadCrumbs = make([]*tree.Model, 0)
	return tea.Batch(
		m.tree.Init(),
		m.pager.Init(),
		m.status.Init(),
	)
}

func (m *model) setSize(w, h int) {
	m.width = w
	m.height = h

	m.logFn("UI wxh: %dx%d", w, h)

	h = h - m.status.Height() - 2 // 2 for border
	m.status.width = w

	w = w - 2 - 2 // 1 for padding, 1 for border

	tw := max(treeWidth, int(0.28*float32(w)))
	m.tree.setSize(tw-1-1, h)
	m.pager.setSize(w-tw-1-1, h)

	m.logFn("Statusbar wxh: %dx%d", m.status.width, m.status.Height())

	if m.pager.viewport.PastBottom() {
		m.logFn("Pager is past bottom")
		m.pager.viewport.GotoBottom()
	}
	if m.tree.list.PastBottom() {
		m.logFn("Tree is past bottom")
		m.tree.list.GotoBottom()
	}
}

type nodeUpdateMsg struct {
	*n
}

func nodeUpdateCmd(n *n) tea.Cmd {
	return func() tea.Msg {
		return nodeUpdateMsg{n}
	}
}

func (m *model) update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
	case *n:
		m.currentNodePosition = m.tree.list.Cursor()
		m.currentNode = msg
		ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*300)
		cmd := m.loadDepsForNode(ctx, m.currentNode)
		cmds = append(cmds, nodeUpdateCmd(m.currentNode), cmd)
	case advanceMsg:
		cmds = append(cmds, m.Advance(msg))
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, movePane):
			if m.tree.list.Focused() {
				m.tree.list.Blur()
			} else {
				m.tree.list.Focus()
				// the model.Tree sets cursor to -1 when bluring, so we need to add an extra +1
				cmds = append(cmds, m.tree.list.SetCursor(m.currentNodePosition))
			}
		case key.Matches(msg, quitKey):
			return quitCmd
		case key.Matches(msg, helpKey):
			return tea.Batch(showHelpCmd(), resizeCmd(m.width, m.height))
		case key.Matches(msg, advanceKey):
			return advanceCmd(m.currentNode)
		case key.Matches(msg, backKey):
			return m.Back(msg)
		}

	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m.tree.list.SetCursor(m.currentNodePosition)
	case quitMsg:
		return tea.Quit
	}

	if cmd := m.updateTree(msg); cmd != nil {
		cmds = append(cmds, cmd)
		if m.tree.IsSyncing() {
			cmds = append(cmds, m.status.startedLoading)
		}
	}
	if m.status.state.Is(statusBusy) && !m.tree.IsSyncing() {
		cmds = append(cmds, m.status.stoppedLoading, nodeUpdateCmd(m.currentNode))
	}
	cmds = append(cmds, m.updatePager(msg))
	cmds = append(cmds, m.updateStatusBar(msg))
	return tea.Batch(cmds...)
}

func (m *model) updatePager(msg tea.Msg) tea.Cmd {
	p, cmd := m.pager.Update(msg)
	if pp, ok := p.(itemModel); ok {
		m.pager = pp
	} else {
		return errCmd(fmt.Errorf("invalid pager: %T", p))
	}
	return cmd
}

func (m *model) updateTree(msg tea.Msg) tea.Cmd {
	t, cmd := m.tree.Update(msg)
	if tt, ok := t.(*treeModel); ok {
		m.tree = *tt
	}
	return cmd
}

func (m *model) updateStatusBar(msg tea.Msg) tea.Cmd {
	s, cmd := m.status.Update(msg)
	m.status = *(s.(*statusModel))
	return cmd
}

var (
	advanceKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "move to current element in tree"),
	)
	backKey = key.NewBinding(
		key.WithKeys("backspace"),
		key.WithHelp("backspace", "move to the previous element in tree"),
	)
	helpKey = key.NewBinding(
		key.WithKeys("m", "?"),
		key.WithHelp("?", "move to current element in tree"),
	)
	quitKey = key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("esc", "exit"),
	)
	movePane = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "change current pane"),
	)
)

func (m *model) Back(msg tea.Msg) tea.Cmd {
	if len(m.breadCrumbs) == 0 {
		m.logFn("No previous tree to go back to.")
		return nil
	}
	if oldTree := m.breadCrumbs[len(m.breadCrumbs)-1]; oldTree != nil {
		m.tree.Back(oldTree)
		m.breadCrumbs = m.breadCrumbs[:len(m.breadCrumbs)-1]
	}
	return nil
}

func advanceCmd(n *n) tea.Cmd {
	return func() tea.Msg {
		return advanceMsg{n}
	}
}

func getRootNodeName(n *n) string {
	name := n.n
	if len(name) == 0 || name == "." {
		name = n.Item.GetLink().String()
	}
	return name
}

func (m *model) Advance(msg advanceMsg) tea.Cmd {
	if top := m.tree.list.Children()[0]; top == m.currentNode {
		m.logFn("will not advance to top of the tree")
		return nil
	}

	if msg.n == nil {
		m.logFn("invalid node to advance to")
		return errCmd(fmt.Errorf("trying to advance to an invalid node"))
	}
	if msg.n.s.Is(NodeError) {
		return errCmd(fmt.Errorf("error: %s", msg.n.n))
	}
	name := getRootNodeName(msg.n)
	newNode := node(msg.Item, withParent(msg.n), withName(name))
	if err := m.loadChildrenForNode(context.Background(), newNode); err != nil {
		return errCmd(fmt.Errorf("unable to advance to %q: %w", msg.n.n, err))
	}
	if newNode.s.Is(tree.NodeCollapsible) && len(newNode.c) == 0 {
		return errCmd(fmt.Errorf("no items in collection %s", name))
	}
	oldTree := m.tree.Advance(newNode)
	m.breadCrumbs = append(m.breadCrumbs, oldTree)
	return nodeCmd(newNode)
}

func errCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return err
	}
}

func nodeCmd(node *n) tea.Cmd {
	return func() tea.Msg {
		return node
	}
}

func resizeCmd(w, h int) tea.Cmd {
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: w, Height: h}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.update(msg)
}

type quitMsg struct{}

func quitCmd() tea.Msg {
	return quitMsg{}
}

type advanceMsg struct {
	*n
}

func renderWithBorder(s string, focused bool) string {
	borderColour := hintColor
	if !focused {
		borderColour = hintDimColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, true, true, true).
		BorderForeground(borderColour).
		Padding(0, 1, 0, 1).Render(s)
}

func (m *model) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			renderWithBorder(m.tree.View(), m.tree.list.Focused()),
			renderWithBorder(m.pager.View(), !m.tree.list.Focused()),
		),
		lipgloss.NewStyle().Render(m.status.View()),
	)
}

// ColorPair is a pair of colors, one intended for a dark background and the
// other intended for a light background. We'll automatically determine which
// of these colors to use.
type ColorPair = lipgloss.AdaptiveColor

// NewColorPair is a helper function for creating a ColorPair.
func NewColorPair(dark, light string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Dark: dark, Light: light}
}

// Wrap wraps lines at a predefined width via package muesli/reflow.
func Wrap(s string) string {
	return wordwrap.String(s, wrapAt)
}

// Returns a termenv style with foreground and background options.
func newStyle(fg, bg ColorPair, bold bool) func(...string) string {
	s := lipgloss.Style{}.Foreground(fg).Background(bg)
	s = s.Bold(bold)
	return s.Render
}

// Returns a new termenv style with background options only.
func newFgStyle(c ColorPair) lipgloss.Style {
	return lipgloss.Style{}.Foreground(c)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, low, high int) int {
	return min(high, max(low, v))
}
