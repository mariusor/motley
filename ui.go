package motley

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
	st "github.com/go-ap/storage"
	tree "github.com/mariusor/bubbles-tree"
	rw "github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/wordwrap"
	te "github.com/muesli/termenv"
	"github.com/openshift/osin"
)

const (
	noteCharacterLimit   = 256             // should match server
	statusMessageTimeout = time.Second * 2 // how long to show status messages like "stashed!"
	ellipsis             = "…"

	darkGray        = "#333333"
	wrapAt          = 60
	statusBarHeight = 1
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

	fuchsiaFg    = newFgStyle(Fuschia)
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

	// HasDarkBackground stores whether or not the terminal has a dark
	// background.
	HasDarkBackground = te.HasDarkBackground()
)

// Colors for dark and light backgrounds.
var (
	Indigo       = NewColorPair("#7571F9", "#5A56E0")
	SubtleIndigo = NewColorPair("#514DC1", "#7D79F6")
	Cream        = NewColorPair("#FFFDF5", "#FFFDF5")
	YellowGreen  = NewColorPair("#ECFD65", "#04B575")
	Fuschia      = NewColorPair("#EE6FF8", "#EE6FF8")
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
	GlamourEnabled  = true
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
	statusBarFailStyle             = newStyle(statusBarNoteFg, FaintRed, false)
	statusBarStashDotStyle         = newStyle(Green, statusBarBg, false)
	statusBarMessageStyle          = newStyle(mintGreen, darkGreen, false)
	statusBarMessageStashIconStyle = newStyle(mintGreen, darkGreen, false)
	statusBarMessageScrollPosStyle = newStyle(mintGreen, darkGreen, false)
	statusBarMessageHelpStyle      = newStyle(NewColorPair("#B6FFE4", "#B6FFE4"), Green, false)
	helpViewStyle                  = newStyle(statusBarNoteFg, NewColorPair("#1B1B1B", "#f2f2f2"), false)
)

//
//func Launch(base pub.IRI, r st.Store, o osin.Storage) error {
//	return tea.NewProgram(newModel(base, r, o)).Start()
//}

func Launch(base pub.IRI, r st.Store, o osin.Storage) error {
	t := tree.New(FedBOX(base, r, o))
	t.Debug = true
	return tea.NewProgram(t).Start()
}

func newModel(base pub.IRI, r st.Store, o osin.Storage) *model {
	if te.HasDarkBackground() {
		GlamourStyle = "dark"
	} else {
		GlamourStyle = "light"
	}
	m := new(model)
	m.commonModel = new(commonModel)

	m.f = FedBOX(base, r, o)
	m.tree = tree.New(m.f)
	m.pager = newPagerModel(m.commonModel)
	return m
}

func newPagerModel(common *commonModel) pagerModel {
	// Init viewport
	vp := viewport.Model{}
	vp.YPosition = 0
	vp.HighPerformanceRendering = false

	// Text input for notes/memos
	ti := textinput.NewModel()
	ti.Prompt = te.String(" > ").
		Foreground(Color(darkGray)).
		Background(Color(YellowGreen.Dark)).
		String()

	/*
		ti.TextColor = darkGray
		ti.BackgroundColor = YellowGreen.String()
		ti.CursorColor = Fuschia.String()
		ti.CharLimit = noteCharacterLimit
		ti.Focus()
	*/

	// Text input for search
	sp := spinner.New()
	/*
		sp.ForegroundColor = statusBarNoteFg.String()
		sp.BackgroundColor = statusBarBg.String()
	*/
	sp.HideFor = time.Millisecond * 50
	sp.MinimumLifetime = time.Millisecond * 180

	return pagerModel{
		commonModel: common,
		textInput:   ti,
		viewport:    vp,
		spinner:     sp,
	}
}

type commonModel struct {
	f      *fedbox
	width  int
	height int
}

type pagerModel struct {
	*commonModel
	state    int
	showHelp bool

	viewport  viewport.Model
	textInput textinput.Model
	spinner   spinner.Model

	statusMessage      string
	statusMessageTimer *time.Timer
}

type model struct {
	*commonModel
	fatalErr error
	// Inbox/Outbox tree model
	tree  tree.Model
	pager pagerModel
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.tree.Init())
	return tea.Batch(cmds...)
}

func (m *model) setSize(w, h int) {
	m.width = w
	m.height = h

	tw := int(float32(w) * 0.33)
	t, _ := m.tree.Update(tea.WindowSizeMsg{
		Width:  tw,
		Height: m.height,
	})
	m.tree = t.(tree.Model)
	m.pager.setSize(w-tw, h)
}

func (m *model) updatePager(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.pager, cmd = m.pager.update(msg)
	return cmd
}

func (m *model) updateTree(msg tea.Msg) tea.Cmd {
	t, cmd := m.tree.Update(msg)
	m.tree = t.(tree.Model)
	return cmd
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If there's been an error, any key exits
	if m.fatalErr != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "left", "h", "delete":
		// Ctrl+C always quits no matter where in the application you are.
		case "ctrl+c":
			return m, tea.Quit
		}
	// Window size is received when starting up and on every resize
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
	}
	cmd = m.updateTree(msg)
	cmds = append(cmds, cmd)

	cmd = m.updatePager(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var b strings.Builder
	fmt.Fprint(&b, m.tree.View()+"\n")
	fmt.Fprintf(&b, m.pager.View())

	return b.String()
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

func (m *pagerModel) setSize(w, h int) {
	m.viewport.Width = w
	m.viewport.Height = h - statusBarHeight
	m.textInput.Width = w - ansi.PrintableRuneWidth(m.textInput.Prompt) - 1

	if m.showHelp {
		if pagerHelpHeight == 0 {
			pagerHelpHeight = strings.Count(m.helpView(), "\n")
		}
		m.viewport.Height -= statusBarHeight + pagerHelpHeight
	}
}

func (m *pagerModel) setContent(s string) {
	m.viewport.SetContent(s)
}

func (m *pagerModel) toggleHelp() {
	m.showHelp = !m.showHelp
	m.setSize(m.width, m.height)
	if m.viewport.PastBottom() {
		m.viewport.GotoBottom()
	}
}

const (
	pagerStateBrowse int = iota
)

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the the returned command should be sent back the through the pager
// update function.
func (m *pagerModel) showStatusMessage(statusMessage string) tea.Cmd {
	// Show a success message to the user
	m.statusMessage = statusMessage
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.statusMessageTimer = time.NewTimer(statusMessageTimeout)

	return waitForStatusMessageTimeout(1, m.statusMessageTimer)
}

func waitForStatusMessageTimeout(appCtx int, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C
		return appCtx
	}
}
func (m *pagerModel) unload() {
	if m.showHelp {
		m.toggleHelp()
	}
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
	m.state = pagerStateBrowse
	m.viewport.SetContent("")
	m.viewport.YOffset = 0
	m.textInput.Reset()
}

func (m pagerModel) update(msg tea.Msg) (pagerModel, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		default:
			switch msg.String() {
			case "q", "esc":
				if m.state != pagerStateBrowse {
					m.state = pagerStateBrowse
					return m, nil
				}
			case "home", "g":
				m.viewport.GotoTop()
				if m.viewport.HighPerformanceRendering {
					cmds = append(cmds, viewport.Sync(m.viewport))
				}
			case "end", "G":
				m.viewport.GotoBottom()
				if m.viewport.HighPerformanceRendering {
					cmds = append(cmds, viewport.Sync(m.viewport))
				}
			case "m":
			case "?":
				m.toggleHelp()
				if m.viewport.HighPerformanceRendering {
					cmds = append(cmds, viewport.Sync(m.viewport))
				}
			}
		}
	case spinner.TickMsg:
		if m.state > pagerStateBrowse || m.spinner.Visible() {
			// If we're still stashing, or if the spinner still needs to
			// finish, spin it along.
			newSpinnerModel, cmd := m.spinner.Update(msg)
			m.spinner = newSpinnerModel
			cmds = append(cmds, cmd)
		} else if m.state == pagerStateBrowse && !m.spinner.Visible() {
			// If the spinner's finished and we haven't told the user the
			// stash was successful, do that.
			m.state = pagerStateBrowse
			cmds = append(cmds, m.showStatusMessage("Stashed!"))
		}

	case tea.WindowSizeMsg:
		return m, renderWithGlamour(m, "")
	default:
		m.state = pagerStateBrowse
	}

	switch m.state {
	default:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m pagerModel) View() string {
	var b strings.Builder
	fmt.Fprint(&b, m.viewport.View()+"\n")

	// Footer
	switch m.state {
	default:
		m.statusBarView(&b)
	}

	if m.showHelp {
		fmt.Fprint(&b, m.helpView())
	}

	return b.String()
}

const (
	pagerStashIcon = "🔒"
)

var glowLogoTextColor = Color("#ECFD65")

func withPadding(s string) string {
	return " " + s + " "
}

func logoView(text string) string {
	return te.String(withPadding(text)).
		Bold().
		Foreground(glowLogoTextColor).
		Background(Color(Fuschia.Dark)).
		String()
}

func (m pagerModel) statusBarView(b *strings.Builder) {
	const (
		minPercent               float64 = 0.0
		maxPercent               float64 = 1.0
		percentToStringMagnitude float64 = 100.0
	)

	// Logo
	name := "FedBOX Admin TUI"
	haveErr := false

	s, err := m.f.getService()
	if err != nil {
		haveErr = true
		m.statusMessage = "Error: invalid connection"
	}
	if s != nil {
		m.statusMessage = fmt.Sprintf("Connected to %s", s.GetLink())
	}
	logo := logoView(name)

	// Scroll percent
	percent := math.Max(minPercent, math.Min(maxPercent, m.viewport.ScrollPercent()))
	scrollPercent := statusBarMessageScrollPosStyle(fmt.Sprintf(" %3.f%% ", percent*percentToStringMagnitude))

	var statusMessage string
	if haveErr {
		statusMessage = statusBarFailStyle(withPadding(m.statusMessage))
	} else {
		statusMessage = statusBarMessageStyle(withPadding(m.statusMessage))
	}

	// Empty space
	padding := max(0,
		m.width-
			ansi.PrintableRuneWidth(logo)-
			ansi.PrintableRuneWidth(statusMessage)-
			ansi.PrintableRuneWidth(scrollPercent),
	)

	emptySpace := strings.Repeat(" ", padding)
	if haveErr {
		emptySpace = statusBarFailStyle(emptySpace)
	} else {
		emptySpace = statusBarMessageStyle(emptySpace)
	}

	fmt.Fprintf(b, "%s%s%s%s",
		logo,
		statusMessage,
		emptySpace,
		scrollPercent,
	)
}

func (m pagerModel) setNoteView(b *strings.Builder) {
	fmt.Fprint(b, m.textInput.View())
}

func (m pagerModel) helpView() (s string) {
	memoOrStash := "m       set memo"

	col1 := []string{
		"g/home  go to top",
		"G/end   go to bottom",
		"",
		memoOrStash,
		"esc     back to files",
		"q       quit",
	}

	s += "\n"
	s += "k/↑      up                  " + col1[0] + "\n"
	s += "j/↓      down                " + col1[1] + "\n"
	s += "b/pgup   page up             " + col1[2] + "\n"
	s += "f/pgdn   page down           " + col1[3] + "\n"
	s += "u        ½ page up           " + col1[4] + "\n"
	s += "d        ½ page down         "

	if len(col1) > 5 {
		s += col1[5]
	}

	s = indent(s, 2)

	// Fill up empty cells with spaces for background coloring
	if m.width > 0 {
		lines := strings.Split(s, "\n")
		for i := 0; i < len(lines); i++ {
			l := rw.StringWidth(lines[i])
			n := max(m.width-l, 0)
			lines[i] += strings.Repeat(" ", n)
		}

		s = strings.Join(lines, "\n")
	}

	return helpViewStyle(s)
}

// COMMANDS

func renderWithGlamour(m pagerModel, md string) tea.Cmd {
	return func() tea.Msg {
		s, err := glamourRender(m, md)
		if err != nil {
			if true {
				log.Println("error rendering with Glamour:", err)
			}
			return err
		}
		return s
	}
}

// This is where the magic happens.
func glamourRender(m pagerModel, markdown string) (string, error) {
	if !GlamourEnabled {
		return markdown, nil
	}

	// initialize glamour
	var gs glamour.TermRendererOption
	if GlamourStyle == "auto" {
		gs = glamour.WithAutoStyle()
	} else {
		gs = glamour.WithStylePath(GlamourStyle)
	}

	width := max(0, min(int(GlamourMaxWidth), m.viewport.Width))
	r, err := glamour.NewTermRenderer(
		gs,
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}

	out, err := r.Render(markdown)
	if err != nil {
		return "", err
	}

	// trim lines
	lines := strings.Split(out, "\n")

	var content string
	for i, s := range lines {
		content += strings.TrimSpace(s)

		// don't add an artificial newline after the last split
		if i+1 < len(lines) {
			content += "\n"
		}
	}

	return content, nil
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
