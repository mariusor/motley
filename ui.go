package motley

import (
	"fmt"
	"time"

	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/processing"
	tree "github.com/mariusor/bubbles-tree"
	"github.com/muesli/reflow/wordwrap"
	te "github.com/muesli/termenv"
	"github.com/openshift/osin"
	"github.com/sirupsen/logrus"
)

const (
	noteCharacterLimit   = 256             // should match server
	statusMessageTimeout = time.Second * 2 // how long to show status messages like "stashed!"
	ellipsis             = "…"

	darkGray = "#333333"
	wrapAt   = 60
)

var (
	normalFg    = newFgStyle(NewColorPair("#dddddd", "#1a1a1a"))
	dimNormalFg = newFgStyle(NewColorPair("#777777", "#A49FA5"))

	brightGrayFg    = newFgStyle(NewColorPair("#979797", "#847A85"))
	dimBrightGrayFg = newFgStyle(NewColorPair("#4D4D4D", "#C2B8C2"))

	grayFg     = newFgStyle(NewColorPair("#626262", "#909090"))
	midGrayFg  = newFgStyle(NewColorPair("#4A4A4A", "#B2B2B2"))
	darkGrayFg = newFgStyle(NewColorPair("#3C3C3C", "#DDDADA"))

	greenFg        = newFgStyle(NewColorPair("#04B575", "#04B575"))
	semiDimGreenFg = newFgStyle(NewColorPair("#036B46", "#35D79C"))
	dimGreenFg     = newFgStyle(NewColorPair("#0B5137", "#72D2B0"))

	fuchsiaFg    = newFgStyle(Fuchsia)
	dimFuchsiaFg = newFgStyle(NewColorPair("#99519E", "#F1A8FF"))

	dullFuchsiaFg    = newFgStyle(NewColorPair("#AD58B4", "#F793FF"))
	dimDullFuchsiaFg = newFgStyle(NewColorPair("#6B3A6F", "#F6C9FF"))

	indigoFg    = newFgStyle(Indigo)
	dimIndigoFg = newFgStyle(NewColorPair("#494690", "#9498FF"))

	subtleIndigoFg    = newFgStyle(NewColorPair("#514DC1", "#7D79F6"))
	dimSubtleIndigoFg = newFgStyle(NewColorPair("#383584", "#BBBDFF"))

	yellowFg     = newFgStyle(YellowGreen)                        // renders light green on light backgrounds
	dullYellowFg = newFgStyle(NewColorPair("#9BA92F", "#6BCB94")) // renders light green on light backgrounds
	redFg        = newFgStyle(Red)
	faintRedFg   = newFgStyle(FaintRed)
)

var (
	// Color wraps termenv.ColorProfile.Color, which produces a termenv color
	// for use in termenv styling.
	Color = lipgloss.ColorProfile().Color

	// HasDarkBackground stores whether the terminal has a dark background.
	HasDarkBackground = te.HasDarkBackground()
)

// Colors for dark and light backgrounds.
var (
	Indigo       = NewColorPair("#7571F9", "#5A56E0")
	SubtleIndigo = NewColorPair("#514DC1", "#7D79F6")
	Cream        = NewColorPair("#FFFDF5", "#FFFDF5")
	YellowGreen  = NewColorPair("#ECFD65", "#04B575")
	Fuchsia      = NewColorPair("#EE6FF8", "#EE6FF8")
	Green        = NewColorPair("#04B575", "#04B575")
	Red          = NewColorPair("#ED567A", "#FF4672")
	FaintRed     = NewColorPair("#C74665", "#FF6F91")
	SpinnerColor = NewColorPair("#747373", "#8E8E8E")
	NoColor      = NewColorPair("", "")
)

// Functions for styling strings.
var (
	IndigoFg       func(string) string = lipgloss.Style{}.Foreground(Indigo).Render
	SubtleIndigoFg                     = lipgloss.Style{}.Foreground(SubtleIndigo).Render
	RedFg                              = lipgloss.Style{}.Foreground(Red).Render
	FaintRedFg                         = lipgloss.Style{}.Foreground(FaintRed).Render
)

var (
	GlamourStyle    = "auto"
	GlamourMaxWidth = 800
)

var (
	pagerHelpHeight int

	mintGreen = NewColorPair("#89F0CB", "#89F0CB")
	darkGreen = NewColorPair("#1C8760", "#1C8760")

	statusBarNoteFg = NewColorPair("#7D7D7D", "#656565")
	statusBarBg     = NewColorPair("#242424", "#E6E6E6")

	// Styling funcs.
	statusBarScrollPosStyle        = newStyle(NewColorPair("#5A5A5A", "#949494"), statusBarBg, false)
	statusBarOKStyle               = newStyle(statusBarNoteFg, statusBarBg, false)
	statusBarFailStyle             = newStyle(NewColorPair("#1B1B1B", "#f2f2f2"), FaintRed, false)
	statusBarStashDotStyle         = newStyle(Green, statusBarBg, false)
	statusBarMessageStyle          = newStyle(mintGreen, darkGreen, false)
	statusBarMessageStashIconStyle = newStyle(mintGreen, darkGreen, false)
	statusBarMessageScrollPosStyle = newStyle(mintGreen, darkGreen, false)
	statusBarMessageHelpStyle      = newStyle(NewColorPair("#B6FFE4", "#B6FFE4"), Green, false)
	helpViewStyle                  = newStyle(statusBarNoteFg, NewColorPair("#1B1B1B", "#f2f2f2"), false)
)

func Launch(conf config.Options, r processing.Store, o osin.Storage, l *logrus.Logger) error {
	base := pub.IRI(conf.BaseURL)
	return tea.NewProgram(newModel(FedBOX(base, r, o, l), conf.Env, l)).Start()
}

func newModel(ff *fedbox, env env.Type, l *logrus.Logger) *model {
	if te.HasDarkBackground() {
		GlamourStyle = "dark"
	} else {
		GlamourStyle = "light"
	}

	m := new(model)
	m.commonModel = new(commonModel)
	m.commonModel.logFn = l.Infof

	m.sub = make(chan pub.ItemCollection, 10)

	m.f = ff

	m.tree = newTreeModel(m.commonModel, initNodes(m.f))
	m.pager = newPagerModel(m.commonModel)
	m.status = newStatusModel(m.commonModel)
	m.status.logo = logoView(pubUrl(ff.getService()), env)
	return m
}

type commonModel struct {
	f      *fedbox
	logFn  func(string, ...interface{})
	width  int
	height int
}

type model struct {
	*commonModel

	currentNode *n
	breadCrumbs []*tree.Model
	sub         chan pub.ItemCollection

	tree   treeModel
	pager  pagerModel
	status statusModel
}

func (m *model) Init() tea.Cmd {
	m.logFn("ui init")
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
	w = w - 2 - 2                 // 2 for padding, 2 for border

	tw := max(treeWidth, int(0.30*float32(w)))
	m.tree.setSize(tw-1-1, h)    // 1 for padding, 1 for border
	m.pager.setSize(w-tw-1-1, h) // 1 for padding, 1 for border

	if m.pager.viewport.PastBottom() {
		m.logFn("Pager is past bottom")
		m.pager.viewport.GotoBottom()
	}
	if m.tree.list.PastBottom() {
		m.logFn("Tree is past bottom")
		m.tree.list.GotoBottom()
	}
}

type loadingFromStorage struct {
	parent *n
	pipe   chan pub.ItemCollection
}

func loadingFromStorageCmd(node *n, sub chan pub.ItemCollection) tea.Cmd {
	return func() tea.Msg {
		return loadingFromStorage{parent: node, pipe: sub}
	}
}

func (m *model) update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
	case *n:
		m.currentNode = msg
		m.displayItem(msg)
		m.loadDepsForNode(msg)
		if msg.s.Is(tree.NodeCollapsible) && len(msg.c) == 0 {
			if err := m.dispatchLoadedItemCollection(msg.GetLink(), m.sub); err != nil {
				m.logFn("error while loading children %s", err)
				msg.s |= NodeError
				cmds = append(cmds, errCmd(err))
			} else {
				cmds = append(cmds, loadingFromStorageCmd(msg, m.sub))
			}
		}
	case loadingFromStorage:
		select {
		case items := <-m.sub:
			if len(items) > 0 {
				children := make([]*n, len(items))
				for i, it := range items.Collection() {
					children[i] = node(it, withState(tree.NodeCollapsed))
				}
				m.currentNode.setChildren(children...)
			}
			cmds = append(cmds, m.status.stoppedLoading)
		default:
		}
	case advanceMsg:
		cmds = append(cmds, m.Advance(msg))
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, movePane):
			if m.tree.list.Focused() {
				m.tree.list.Styles = tree.Styles{}
				m.tree.list.Blur()
			} else {
				m.tree.list.Styles = tree.DefaultStyles()
				m.tree.list.Focus()
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
		return nil
	case quitMsg:
		return tea.Quit
	}

	if cmd := m.updateTree(msg); cmd != nil {
		cmds = append(cmds, cmd, m.status.startedLoading)
	}
	cmds = append(cmds, m.updatePager(msg))
	cmds = append(cmds, m.updateStatusBar(msg))
	return tea.Batch(cmds...)
}

func (m *model) updatePager(msg tea.Msg) tea.Cmd {
	p, cmd := m.pager.Update(msg)
	m.pager = *(p.(*pagerModel))
	return cmd
}

func (m *model) updateTree(msg tea.Msg) tea.Cmd {
	t, cmd := m.tree.Update(msg)
	m.tree = *(t.(*treeModel))
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
	if err := m.loadChildrenForNode(newNode); err != nil {
		return errCmd(fmt.Errorf("%s", msg.n.n))
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

func (m *model) displayItem(n *n) tea.Cmd {
	it := n.Item
	switch it.(type) {
	case pub.ItemCollection, pub.IRI:
		m.pager.showItem(it)
		return m.status.showStatusMessage(fmt.Sprintf("Collection %s: %d items", n.n, len(n.c)))
	case pub.Item:
		m.pager.showItem(it)
		return m.status.showStatusMessage(fmt.Sprintf("%s: %s", it.GetType(), it.GetLink()))
	}
	return nil
}

func (m *model) View() string {
	renderWithBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, true, true, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Padding(0, 1, 0, 1).Render

	return lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			renderWithBorder(m.tree.View()),
			renderWithBorder(m.pager.View()),
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

// Keyword applies special formatting to imporant words or phrases.
func Keyword(s string) string {
	return te.String(s).Foreground(Color(Green.Dark)).String()
}

type styleFunc func(string) string

// Returns a termenv style with foreground and background options.
func newStyle(fg, bg ColorPair, bold bool) func(string) string {
	s := lipgloss.Style{}.Foreground(fg).Background(bg)
	s = s.Bold(bold)
	return s.Render
}

// Returns a new termenv style with background options only.
func newFgStyle(c ColorPair) styleFunc {
	return te.Style{}.Foreground(Color(c.Dark)).Styled
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
