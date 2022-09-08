package motley

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	rw "github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/ansi"
	te "github.com/muesli/termenv"
)

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

func newPagerModel(common *commonModel) pagerModel {
	// Init viewport
	vp := viewport.Model{}
	vp.YPosition = 0
	vp.HighPerformanceRendering = false

	// Text input for notes/memos
	ti := textinput.New()
	ti.CursorStyle = lipgloss.Style{}.Foreground(Fuschia)
	ti.CharLimit = noteCharacterLimit
	ti.Prompt = te.String(" > ").
		Foreground(Color(darkGray)).
		Background(Color(YellowGreen.Dark)).
		String()
	ti.Focus()

	// Text input for search
	sp := spinner.New()
	sp.Style = lipgloss.Style{}.Foreground(statusBarNoteFg).Background(statusBarBg)
	sp.Spinner.FPS = time.Second / 10

	return pagerModel{
		commonModel: common,
		textInput:   ti,
		viewport:    vp,
		spinner:     sp,
	}
}

func (p *pagerModel) Init() tea.Cmd {
	return nil
}

func (p *pagerModel) setSize(w, h int) {
	p.viewport.Width = w
	p.viewport.Height = h - statusBarHeight
	p.textInput.Width = w - ansi.PrintableRuneWidth(p.textInput.Prompt) - 1
	p.logFn("tree size: %dx%d", p.viewport.Width, p.viewport.Height)

	if p.showHelp {
		if pagerHelpHeight == 0 {
			pagerHelpHeight = strings.Count(p.helpView(), "\n")
		}
		p.viewport.Height -= statusBarHeight + pagerHelpHeight
	}
}

func (p *pagerModel) setContent(s string) {
	p.viewport.SetContent(s)
}

func (p *pagerModel) toggleHelp() {
	p.showHelp = !p.showHelp
	p.setSize(p.width, p.height)
	if p.viewport.PastBottom() {
		p.viewport.GotoBottom()
	}
}

const (
	pagerStateBrowse int = iota
)

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the returned command should be sent back the through the pager
// update function.
func (p *pagerModel) showStatusMessage(statusMessage string) tea.Cmd {
	// Show a success message to the user
	p.statusMessage = statusMessage
	if p.statusMessageTimer != nil {
		p.statusMessageTimer.Stop()
	}
	p.statusMessageTimer = time.NewTimer(statusMessageTimeout)

	return waitForStatusMessageTimeout(1, p.statusMessageTimer)
}

func (p *pagerModel) unload() {
	if p.showHelp {
		p.toggleHelp()
	}
	if p.statusMessageTimer != nil {
		p.statusMessageTimer.Stop()
	}
	p.state = pagerStateBrowse
	p.viewport.SetContent("")
	p.viewport.YOffset = 0
	p.textInput.Reset()
}

func (p *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch p.state {
		default:
			switch msg.String() {
			case "q", "esc":
				if p.state != pagerStateBrowse {
					p.state = pagerStateBrowse
					return p, nil
				}
			case "home", "g":
				p.viewport.GotoTop()
				if p.viewport.HighPerformanceRendering {
					cmds = append(cmds, viewport.Sync(p.viewport))
				}
			case "end", "G":
				p.viewport.GotoBottom()
				if p.viewport.HighPerformanceRendering {
					cmds = append(cmds, viewport.Sync(p.viewport))
				}
			case "m":
			case "?":
				p.toggleHelp()
				if p.viewport.HighPerformanceRendering {
					cmds = append(cmds, viewport.Sync(p.viewport))
				}
			}
		}
	case spinner.TickMsg:
		if p.state > pagerStateBrowse {
			// If we're still stashing, or if the spinner still needs to
			// finish, spin it along.
			newSpinnerModel, cmd := p.spinner.Update(msg)
			p.spinner = newSpinnerModel
			cmds = append(cmds, cmd)
		} else if p.state == pagerStateBrowse {
			// If the spinner's finished and we haven't told the user the
			// stash was successful, do that.
			p.state = pagerStateBrowse
			cmds = append(cmds, p.showStatusMessage("Stashed!"))
		}

	case tea.WindowSizeMsg:
		return p, renderWithGlamour(*p, "")
	default:
		p.state = pagerStateBrowse
	}

	switch p.state {
	default:
		p.viewport, cmd = p.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

func (p *pagerModel) View() string {
	var b strings.Builder
	fmt.Fprint(&b, p.viewport.View()+"\n")

	// Footer
	switch p.state {
	default:
		p.statusBarView(&b)
	}

	if p.showHelp {
		fmt.Fprint(&b, p.helpView())
	}

	return b.String()
}

const (
	pagerStashIcon = "🔒"
)

func (p *pagerModel) statusBarView(b *strings.Builder) {
	const (
		minPercent               float64 = 0.0
		maxPercent               float64 = 1.0
		percentToStringMagnitude float64 = 100.0
	)

	// Logo
	name := "FedBOX Admin TUI"
	haveErr := false

	s := p.f.getService()
	if s != nil {
		p.statusMessage = fmt.Sprintf("Connected to %s", s.GetLink())
	} else {
		haveErr = true
		p.statusMessage = "Error: invalid connection"
	}
	logo := logoView(name)

	// Scroll percent
	percent := math.Max(minPercent, math.Min(maxPercent, p.viewport.ScrollPercent()))
	scrollPercent := statusBarMessageScrollPosStyle(fmt.Sprintf(" %3.f%% ", percent*percentToStringMagnitude))

	var statusMessage string
	if haveErr {
		statusMessage = statusBarFailStyle(withPadding(p.statusMessage))
	} else {
		statusMessage = statusBarMessageStyle(withPadding(p.statusMessage))
	}

	// Empty space
	padding := max(0,
		p.width-
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

func (p *pagerModel) setNoteView(b *strings.Builder) {
	b.WriteString(p.textInput.View())
}

func (p *pagerModel) helpView() (s string) {
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
	if p.width > 0 {
		lines := strings.Split(s, "\n")
		for i := 0; i < len(lines); i++ {
			l := rw.StringWidth(lines[i])
			n := max(p.width-l, 0)
			lines[i] += strings.Repeat(" ", n)
		}

		s = strings.Join(lines, "\n")
	}

	return helpViewStyle(s)
}
