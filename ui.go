package motley

import (
	"fmt"
	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
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

	darkGray        = "#333333"
	wrapAt          = 60
	statusBarHeight = 1

	treeWidth = 32
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

	m.f = ff

	m.tree = newTreeModel(m.commonModel, initNodes(m.f))
	m.pager = newPagerModel(m.commonModel)
	m.status = newStatusModel(m.commonModel, env)
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

	tree   treeModel
	pager  pagerModel
	status statusModel
}

func (m *model) Init() tea.Cmd {
	m.breadCrumbs = make([]*tree.Model, 0)
	return tea.Batch(
		m.tree.list.Init(),
	)
}

func (m *model) setSize(w, h int) {
	m.width = w
	m.height = h

	height := h - m.status.Height()
	tw := max(treeWidth, int(0.33*float32(w)))
	m.tree.setSize(tw, height)
	m.pager.setSize(w-tw, height)
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

func (m *model) update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	if cmd := m.updateTree(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.updatePager(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
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
)

func (m *model) Back(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.breadCrumbs) == 0 {
		m.logFn("No previous tree to go back to.")
		return m, nil
	}
	if oldTree := m.breadCrumbs[len(m.breadCrumbs)-1]; oldTree != nil {
		m.tree.Back(oldTree)
		m.breadCrumbs = m.breadCrumbs[:len(m.breadCrumbs)-1]
	}
	return m, nil
}

func advanceCmd(n *n) tea.Cmd {
	return func() tea.Msg {
		return advanceMsg{n}
	}
}

func (m *model) Advance(msg tea.Msg) (tea.Model, tea.Cmd) {
	return nil, nil
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

func (m *model) loadChildrenForNode(nn *n) error {
	iri := nn.Item.GetLink()
	col, err := m.f.s.Load(iri)
	if err != nil {
		return err
	}
	if pub.IsItemCollection(col) {
		children := make([]*n, 0)
		err = pub.OnItemCollection(col, func(col *pub.ItemCollection) error {
			for _, it := range *col {
				children = append(children, node(it, withState(tree.NodeCollapsed)))
			}
			return nil
		})
		if err != nil {
			return err
		}
		nn.setChildren(children...)
	}

	return nil
}

func (m *model) toggleHelp() {
	m.status.showHelp = !m.status.showHelp
	m.setSize(m.width, m.height)
	if m.pager.viewport.PastBottom() {
		m.pager.viewport.GotoBottom()
	}
	//if m.tree.list.PastBottom() {
	//	m.tree.list.GotoBottom()
	//}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		m.status.showError(msg)
	case *n:
		if msg.State().Is(NodeError) {
			return m, errCmd(fmt.Errorf("%s", msg.n))
		}
		if len(msg.c) == 0 && msg.s.Is(tree.NodeCollapsible) {
			if err := m.loadChildrenForNode(msg); err != nil {
				return m, errCmd(err)
			}
		}
		m.currentNode = msg
		m.displayItem(msg)
	case advanceMsg:
		//if m.breadCrumbs[len(m.breadCrumbs)-1].Children()[0].Name() == m.currentNode.Name() {
		//	// skip if trying to advance to same element
		//	return m, nil
		//}
		//if msg.State().Is(NodeError) {
		//	return m, errCmd(fmt.Errorf("%s", msg.n.n))
		//}
		iri := msg.GetLink().String()
		newNode := node(msg.Item, withParent(msg.n), withName(iri))
		if err := m.loadChildrenForNode(newNode); err != nil {
			return m, errCmd(fmt.Errorf("%s", msg.n.n))
		}
		if newNode.s.Is(tree.NodeCollapsible) && len(newNode.c) == 0 {
			return m, errCmd(fmt.Errorf("no items in collection %s", iri))
		}
		oldTree := m.tree.Advance(newNode)
		m.breadCrumbs = append(m.breadCrumbs, oldTree)
		return m, nodeCmd(newNode)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, quitKey):
			if m.status.state != statusBrowse {
				m.status.state = statusBrowse
			}
			return m, tea.Quit
		case key.Matches(msg, helpKey):
			m.toggleHelp()
		case key.Matches(msg, advanceKey):
			return m, advanceCmd(m.currentNode)
		case key.Matches(msg, backKey):
			return m.Back(msg)
		default:
			return m, tea.Batch(m.update(msg))
		}
	// Window size is received when starting up and on every resize
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
	case spinner.TickMsg:
		return m, m.status.updateTicket(msg)
	default:
		return m, tea.Batch(m.update(msg))
	}

	return m, nil
}

type advanceMsg struct {
	*n
}

func (m *model) displayItem(n *n) {
	it := n.Item
	switch it.(type) {
	case pub.ItemCollection:
		m.status.showStatusMessage(fmt.Sprintf("Collection: %s %d items", n.n, len(n.c)))
	case pub.Item:
		err := m.pager.showItem(it)
		if err != nil {
			m.status.showError(err)
			return
		}
		m.status.showStatusMessage(fmt.Sprintf("%s: %s", it.GetType(), it.GetLink()))
	}
}

func (m *model) View() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			lipgloss.NewStyle().
				Border(lipgloss.ThickBorder(), false, true, false, false).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
				Padding(0, 0, 0, 1).Render(m.tree.list.View()),
			lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, false).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
				Padding(0, 0, 0, 0).Render(m.pager.View()),
		),
		m.status.View(),
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

func waitForStatusMessageTimeout(appCtx int, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C
		return appCtx
	}
}

var glowLogoTextColor = Color("#ECFD65")

func withPadding(s string) string {
	return " " + s + " "
}

func logoView(text string, e env.Type) string {
	var (
		fg te.Color
		bg te.Color
	)
	if e.IsProd() {
		fg = Color(FaintRed.Dark)
		bg = Color(Red.Dark)
	}
	if !e.IsProd() {
		fg = Color(Green.Dark)
		bg = Color(darkGreen.Dark)
	}
	return te.String(withPadding(text)).Bold().Foreground(fg).Background(bg).String()
}

// Lightweight version of reflow's indent function.
func indent(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	l := strings.Split(s, "\n")
	b := strings.Builder{}
	i := strings.Repeat(" ", n)
	for _, v := range l {
		fmt.Fprintf(&b, "%s%s\n", i, v)
	}
	return b.String()
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
