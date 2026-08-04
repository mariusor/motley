package motley

import (
	"context"
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"git.sr.ht/~mariusor/motley/internal/config"
	"github.com/common-nighthawk/go-figure"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/filters"
	tree "github.com/mariusor/bubbles-tree"
)

const (
	noteCharacterLimit   = 256
	statusMessageTimeout = time.Second * 2
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
	Color = lipgloss.Color

	// HasDarkBackground stores whether the terminal has a dark background.
	HasDarkBackground = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
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

func Launch(conf config.Options, l *slog.Logger) error {
	_, err := tea.NewProgram(newModel(conf, l)).Run()
	return err
}

var _ tea.Model = new(model)

// Model is a way for the motley main model to be initialized from calling code.
func Model(l *slog.Logger, st ...Store) *model {
	if HasDarkBackground {
		GlamourStyle = "dark"
	} else {
		GlamourStyle = "light"
	}

	m := new(model)
	m.commonModel = new(commonModel)
	m.commonModel.l = l

	m.pager = newItemModel(m.commonModel)
	m.status = newStatusModel(m.commonModel)

	m.f = new(fedbox)
	m.f.stores = st
	m.f.l = l
	m.tree = newTreeModel(m.commonModel, initNodes(m.f))
	return m
}

func newModel(conf config.Options, l *slog.Logger) *model {
	f, err := fedBOX(conf.URLs, conf.Storage)
	if err != nil {
		l.With(slog.Any("err", err)).Error("Not all storage paths could be used")
	}
	return Model(l, f.stores...)
}

type commonModel struct {
	f    *fedbox
	root vocab.Item

	timer *time.Timer

	l *slog.Logger
}

type model struct {
	*commonModel

	width  int
	height int

	currentNode         *n
	currentNodePosition int

	breadCrumbs []*tree.Model

	tree   treeModel
	pager  pagerModel
	status statusModel
}

func (m *model) Init() tea.Cmd {
	m.l.Debug("ui init")
	m.breadCrumbs = make([]*tree.Model, 0)

	return tea.Batch(m.tree.Init(), m.pager.Init(), m.status.Init())
}

func (m *model) setSize(w, h int) {
	m.width = w
	m.height = h

	m.l.With(slog.Int("w", w), slog.Int("h", h)).Debug("UI size change")

	h = h - m.status.Height() - 2 // 2 for border
	m.status.width = w

	w = w - 2 - 2 // 1 for padding, 1 for border

	tw := max(minTreeWidth, int(0.28*float32(w)))
	m.tree.setSize(tw-1-1, h)
	m.pager.setSize(w-tw-1-1, h)

	m.l.With(slog.Int("w", m.status.width), slog.Int("h", m.status.Height())).Debug("new statusbar size")

	if m.pager.viewport.PastBottom() {
		m.l.Debug("Pager is past bottom")
		m.pager.viewport.GotoBottom()
	}
	if m.tree.list.PastBottom() {
		m.l.Debug("Tree is past bottom")
		m.tree.list.GotoBottom()
	}
}

type nodeUpdateMsg n

func nodeUpdateCmd(n n) tea.Cmd {
	return func() tea.Msg {
		return nodeUpdateMsg(n)
	}
}

type timedNodeMsg struct {
	node   *n
	loaded bool
	timer  *time.Timer
}

func waitCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(defaultFrameDuration)
		return msg
	}
}

const (
	// defaultDurationBeforeLoad is the wait duration before loading an item after the user has stopped moving the cursor.
	defaultDurationBeforeLoad = 600 * time.Millisecond

	// defaultFrameDuration is the wait time before triggering another wait event while the UI is waiting
	// for the defaultDurationBeforeLoad to expire.
	defaultFrameDuration = defaultDurationBeforeLoad / 20
)

var startTime = time.Now().UTC().Truncate(time.Microsecond)

func delayedNodeLoad(n *n, timer *time.Timer) tea.Cmd {
	timer = time.NewTimer(defaultDurationBeforeLoad)
	m := timedNodeMsg{node: n, timer: timer}
	return func() tea.Msg {
		return m
	}
}

func (m *model) update(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	defer func() {
		startTime = time.Now().UTC().Truncate(time.Microsecond)
	}()

	m.l.With(slog.Duration("dur", time.Since(startTime)), slog.String("msg", fmt.Sprintf("%T", msg))).Debug("received msg")
	ctx := context.Background()
	switch mm := msg.(type) {
	case timedNodeMsg:
		// NOTE(marius): the cursor was moved to a new element in the node tree, we wait out the timer,
		// and then we load the ActivityPub item corresponding to it.
		select {
		case <-mm.timer.C:
			m.l.With(slog.Int("pos", m.currentNodePosition), slog.String("URL", string(mm.node.GetLink()))).Debug("Loading node")
			cmd := m.loadNodeProperties(ctx, m.currentNode)
			if vocab.IsCollection(m.currentNode.Item) {
				cmd = m.loadNodeCollection(ctx, m.currentNode)
			}
			cmds = append(cmds, cmd)
		default:
			if m.tree.IsSyncing() {
				cmds = append(cmds, waitCmd(mm))
			}
		}
	case *n:
		// NOTE(marius): the cursor has moved to the new entry, we return a delayed node command
		if !vocab.IsNil(mm.Item) {
			m.currentNodePosition = m.tree.list.Cursor()
			m.currentNode = mm
			for _, st := range m.f.stores {
				if mm.GetLink().Contains(st.root.GetLink(), true) {
					m.root = st.root
					m.status.env = st.env
					break
				}
			}
			m.l.With(slog.Int("pos", m.currentNodePosition), slog.String("name", mm.n), slog.Bool("collection?", vocab.IsCollection(mm))).Debug("moved to new node")
		}
		cmds = append(cmds, delayedNodeLoad(mm, m.timer), m.tree.startedLoading)
	case tree.ExpandedMsg:
		cmds = append(cmds, nodeUpdateCmd(*m.currentNode))
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

		if m.currentNodePosition < m.height-3 && m.currentNode != nil {
			parent := m.currentNode.p
			if parent != nil && vocab.IsCollection(parent) {
				count := filters.WithMaxCount(m.height)
				after := filters.After(filters.SameID(m.currentNode.GetLink()))
				_, _ = m.f.loadCollectionItems(ctx, parent, after, count)
			}
		}
	case tea.WindowSizeMsg:
		m.setSize(mm.Width, mm.Height)
		return m.tree.list.SetCursor(m.currentNodePosition)
	case quitMsg:
		return tea.Quit
	}

	cmds = append(cmds, m.updateTree(msg))
	cmds = append(cmds, m.updatePager(msg))
	cmds = append(cmds, m.updateStatusBar(msg))
	return tea.Batch(cmds...)
}

func (m *model) updatePager(msg tea.Msg) tea.Cmd {
	mp, cmd := m.pager.Update(msg)
	m.pager, _ = mp.(pagerModel)
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
	return m.status.Update(msg)
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
		m.l.Debug("no previous tree to go back to.")
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
		m.l.Debug("will not advance to top of the tree")
		return noop
	}

	nn := n(msg)
	if msg.s.Is(NodeError) {
		return errCmd(fmt.Errorf("error: %s", nn.n))
	}

	rootName := getRootNodeName(&nn)
	newNode := node(msg.Item, withParent(&nn), withName(rootName))

	count := filters.WithMaxCount(m.height)
	_, err := m.f.loadCollectionItems(context.Background(), newNode, count)
	if err != nil {
		return errCmd(fmt.Errorf("unable to advance to %q: %w", nn.n, err))
	}
	if newNode.s.Is(tree.NodeCollapsible) && len(newNode.c) == 0 {
		return errCmd(fmt.Errorf("no items in collection %s", rootName))
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
	cmd := m.update(msg)
	return m, cmd
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

func renderTree(t treeModel) string {
	if t.width() <= 0 || t.height() <= 0 {
		return ""
	}
	return t.View().Content
}

func (m *model) View() tea.View {
	renderedTree := renderWithBorder(renderTree(m.tree), m.tree.list.Focused())
	return tea.NewView(lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			renderedTree,
			renderWithBorder(m.pager.View().Content, !m.tree.list.Focused()),
		),
		lipgloss.NewStyle().Render(m.status.View()),
	))
}

func (m *model) IsBusy() bool {
	return m.status.state.Is(statusBusy)
}

// ColorPair is a pair of colors, one intended for a dark background and the
// other intended for a light background. We'll automatically determine which
// of these colors to use.
type ColorPair = color.Color

// NewColorPair is a helper function for creating a ColorPair.
func NewColorPair(dark, light string) color.Color {
	lightDark := lipgloss.LightDark(HasDarkBackground)
	return lightDark(Color(dark), Color(light))
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

func clamp(v, low, high int) int {
	return min(high, max(low, v))
}

type motelyPager struct {
	Title string
}

func (m motelyPager) Init() tea.Cmd {
	return noop
}

func (m motelyPager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, noop
}

func (m motelyPager) View() tea.View {
	tit := figure.NewFigure(m.Title, "", true)
	return tea.NewView(tit.String())
}

var M = motelyPager{Title: "Motley"}

func (s *motelyPager) statusHelpView(b *strings.Builder) {
	// TODO(marius): this help message can be probably generated from the default key bindings.
	ss := ""
	col1 := []string{
		"g/home  go to top",
		"G/end   go to bottom",
		"",
		"esc     back to files",
		"q       quit",
	}
	ss += "\n"
	ss += "k/↑      up                  " + col1[0] + "\n"
	ss += "j/↓      down                " + col1[1] + "\n"
	ss += "b/pgup   page up             " + col1[2] + "\n"
	ss += "f/pgdn   page down           " + col1[3] + "\n"
	ss += "u        ½ page up           " + col1[4] + "\n"
	ss += "d        ½ page down         "
	if len(col1) > 5 {
		ss += col1[5]
	}

	indent(b, helpViewStyle(ss), 2)
}
