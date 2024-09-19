package motley

import (
	"context"
	"fmt"
	"time"

	"git.sr.ht/~mariusor/lw"
	"git.sr.ht/~mariusor/motley/internal/config"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/filters"
	tree "github.com/mariusor/bubbles-tree"
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

func Launch(conf config.Options, l lw.Logger) error {
	_, err := tea.NewProgram(newModel(conf, l), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func newModel(conf config.Options, l lw.Logger) *model {
	if lipgloss.HasDarkBackground() {
		GlamourStyle = "dark"
	} else {
		GlamourStyle = "light"
	}

	m := new(model)
	m.commonModel = new(commonModel)
	m.commonModel.logFn = l.Infof

	m.pager = newItemModel(m.commonModel)
	m.status = newStatusModel(m.commonModel)

	var err error
	var nodes tree.Nodes

	m.f, err = FedBOX(conf.URLs, conf.Storage, l)
	if err != nil {
		m.status.showError(err)
	} else {
		nodes = initNodes(m.f)
	}
	m.tree = newTreeModel(m.commonModel, nodes)
	return m
}

type commonModel struct {
	f    *fedbox
	root pub.Item

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
	pager  pagerModel
	status statusModel
}

func (m *model) Init() (tea.Model, tea.Cmd) {
	m.logFn("UI init")
	m.breadCrumbs = make([]*tree.Model, 0)

	tm, cmdt := m.tree.Init()
	if mm, ok := tm.(*treeModel); ok {
		m.tree = *mm
	}
	pm, cmdp := m.pager.Init()
	if mm, ok := pm.(pagerModel); ok {
		m.pager = mm
	}
	sm, cmds := m.status.Init()
	if mm, ok := sm.(*statusModel); ok {
		m.status = *mm
	}
	return m, tea.Batch(cmdt, cmdp, cmds)
}

func (m *model) setSize(w, h int) {
	m.width = w
	m.height = h

	m.logFn("UI wxh: %dx%d", w, h)

	h = h - m.status.Height() - 2 // 2 for border
	m.status.width = w

	w = w - 2 - 2 // 1 for padding, 1 for border

	tw := max(minTreeWidth, int(0.28*float32(w)))
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

type nodeUpdateMsg n

func nodeUpdateCmd(n n) tea.Cmd {
	return func() tea.Msg {
		return nodeUpdateMsg(n)
	}
}

func skipMessageFromLogs(msg tea.Msg) bool {
	if _, ok := msg.(*n); ok {
		return true
	}
	if _, ok := msg.(n); ok {
		return true
	}
	if _, ok := msg.(advanceMsg); ok {
		return true
	}
	if _, ok := msg.(nodeUpdateMsg); ok {
		return true
	}
	return false
}

func (m *model) logMessage(msg tea.Msg) {
	if !skipMessageFromLogs(msg) {
		m.logFn("update: %T: %s", msg, msg)
	}
}

func (m *model) update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*300)
	defer cancel()

	m.logMessage(msg)
	switch mm := msg.(type) {
	case *n:
		if mm != nil {
			m.currentNodePosition = m.tree.list.Cursor()
			m.currentNode = mm
			m.tree.state |= stateBusy
			cmd := m.loadDepsForNode(ctx, m.currentNode)
			for _, st := range m.f.stores {
				if mm.GetLink().Contains(st.root.GetLink(), true) {
					m.root = st.root
					m.status.env = st.env
					break
				}
			}
			m.logFn("Moved to node[%d]: %s:%s, is collection: %t", m.currentNodePosition, mm.n, mm.s, mm.IsCollection())
			cmds = append(cmds, nodeUpdateCmd(*m.currentNode), cmd)
		}
	case advanceMsg:
		cmds = append(cmds, m.Advance(mm))
	case tea.KeyMsg:
		switch {
		case key.Matches(mm, movePane):
			if m.tree.list.Focused() {
				m.tree.list.Blur()
			} else {
				m.tree.list.Focus()
				// the model.Tree sets cursor to -1 when bluring, so we need to add an extra +1
				cmds = append(cmds, m.tree.list.SetCursor(m.currentNodePosition))
			}
		case key.Matches(mm, quitKey):
			return quitCmd
		case key.Matches(mm, helpKey):
			return tea.Batch(showHelpCmd(), resizeCmd(m.width, m.height))
		case key.Matches(mm, advanceKey):
			return advanceCmd(*m.currentNode)
		case key.Matches(mm, backKey):
			return m.Back(mm)
		}

		if m.currentNodePosition < m.height-3 {
			parent := m.currentNode.p
			if parent != nil && parent.IsCollection() {
				count := filters.WithMaxCount(m.height)
				after := filters.After(filters.SameID(m.currentNode.GetLink()))
				m.loadNode(ctx, parent, after, count)
			}
		}
	case tea.WindowSizeMsg:
		m.setSize(mm.Width, mm.Height)
		return m.tree.list.SetCursor(m.currentNodePosition)
	case quitMsg:
		return tea.Quit
	}

	cmds = append(cmds, m.updateTree(msg))
	if !m.tree.IsSyncing() {
		cmds = append(cmds, m.updatePager(msg))
	}
	cmds = append(cmds, m.updateStatusBar(msg))
	return tea.Batch(cmds...)
}

func (m *model) updatePager(msg tea.Msg) tea.Cmd {
	p, cmd := m.pager.Update(msg)
	if pp, ok := p.(pagerModel); ok {
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
		return noop
	}
	if oldTree := m.breadCrumbs[len(m.breadCrumbs)-1]; oldTree != nil {
		m.tree.Back(oldTree)
		m.breadCrumbs = m.breadCrumbs[:len(m.breadCrumbs)-1]
	}
	return noop
}

var noop tea.Cmd = nil

func advanceCmd(n n) tea.Cmd {
	return func() tea.Msg {
		return advanceMsg(n)
	}
}

func getRootNodeName(n *n) string {
	name := n.n
	if len(name) == 0 || name == "." {
		name = n.Item.GetLink().String()
	}
	return name
}

func (m *model) shouldAdvance() bool {
	children := m.tree.list.Children()
	return len(children) >= 1 && children[0] != m.currentNode
}

func (m *model) Advance(msg advanceMsg) tea.Cmd {
	if !m.shouldAdvance() {
		m.logFn("will not advance to top of the tree")
		return noop
	}

	nn := n(msg)
	if msg.s.Is(NodeError) {
		return errCmd(fmt.Errorf("error: %s", nn.n))
	}

	name := getRootNodeName(&nn)
	newNode := node(msg.Item, withParent(&nn), withName(name))

	count := filters.WithMaxCount(m.height)
	if err := m.loadNode(context.Background(), newNode, count); err != nil {
		return errCmd(fmt.Errorf("unable to advance to %q: %w", nn.n, err))
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
	if node == nil {
		return noop
	}
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

type advanceMsg n

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

func (m *model) IsBusy() bool {
	return m.status.state.Is(statusBusy)
}

// ColorPair is a pair of colors, one intended for a dark background and the
// other intended for a light background. We'll automatically determine which
// of these colors to use.
type ColorPair = lipgloss.AdaptiveColor

// NewColorPair is a helper function for creating a ColorPair.
func NewColorPair(dark, light string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Dark: dark, Light: light}
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

type motelyPager struct {
	Title string
}

func (m motelyPager) Init() (tea.Model, tea.Cmd) {
	return m, nil
}

func (m motelyPager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m motelyPager) View() string {
	tit := figure.NewFigure(m.Title, "", true)
	return tit.String()
}

var M = motelyPager{Title: "Motley"}
