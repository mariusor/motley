package motley

import (
	"fmt"
	"math"
	"strings"
	"time"

	"git.sr.ht/~marius/motley/internal/env"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	rw "github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/ansi"
	te "github.com/muesli/termenv"
)

type statusState int

func (s statusState) Is(st statusState) bool {
	return s&st == st
}

const (
	statusBarHeight = 1
	lockIcon        = "🔒"
)

const (
	statusError statusState = 1 << iota
	statusHelp
	statusLoading

	statusNone statusState = 0
)

type statusModel struct {
	*commonModel

	logo  string
	state statusState

	spinner spinner.Model
	percent float64

	statusMessage      string
	statusMessageTimer *time.Timer
}

var glowLogoTextColor = Color("#ECFD65")

func newStatusModel(common *commonModel) statusModel {
	// Text input for search
	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = lipgloss.NewStyle().Bold(true)
	//sp.Spinner.FPS = time.Second / 4
	//sp.Spinner.Frames = []string{"", "🞄", "•", "⚫", "•", "🞄"}
	//sp.Spinner.Frames = []string{"⨁ ", "⨂ "}
	//sp.Spinner.Frames = []string{"◤", "◥", "◢", "◣"}
	//sp.Spinner.Frames = []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"}
	//sp.Spinner.Frames = []string{"◒", "◐", "◓", "◑"}
	//sp.Spinner.Frames = []string{"🭶", "🭷", "🭸", "🭹", "🭺", "🭻"}
	//sp.Spinner.Frames = []string{"⠦", "⠖", "⠲", "⠴"}

	common.logFn("initializing status bar")
	return statusModel{
		commonModel: common,
		spinner:     sp,
	}
}

func (s *statusModel) Init() tea.Cmd {
	s.logFn("status init")
	return s.spinner.Tick
}

func (s *statusModel) showError(err error) tea.Cmd {
	s.state |= statusError
	return s.showStatusMessage(err.Error())
}

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the returned command should be sent back the through the pager
// update function.
func (s *statusModel) showStatusMessage(statusMessage string) tea.Cmd {
	s.statusMessage = statusMessage
	if s.statusMessageTimer != nil {
		s.statusMessageTimer.Stop()
	}
	s.statusMessageTimer = time.NewTimer(statusMessageTimeout)

	return waitForStatusMessageTimeout(1, s.statusMessageTimer)
}

func (s *statusModel) statusBarView(b *strings.Builder) {
	percent := clamp(int(math.Round(s.percent)), 0, 100)
	scrollPercent := fmt.Sprintf(" %d%% ", percent)

	haveErr := s.state.Is(statusError)

	spinner := s.spinner.View()
	// Empty space
	padding := max(0,
		s.width-
			ansi.PrintableRuneWidth(spinner)-
			ansi.PrintableRuneWidth(s.logo)-
			ansi.PrintableRuneWidth(s.statusMessage)-
			ansi.PrintableRuneWidth(scrollPercent),
	)

	emptySpace := strings.Repeat(" ", padding)
	render := statusBarMessageStyle
	if haveErr {
		render = statusBarFailStyle
	}
	fmt.Fprintf(b, "%s%s",
		s.logo,
		render(fmt.Sprintf("%s%s%s%s", s.statusMessage, emptySpace, scrollPercent, spinner)),
	)
}

func (s *statusModel) unload() {
	if s.statusMessageTimer != nil {
		s.statusMessageTimer.Stop()
	}
	s.state ^= statusLoading
}

func (s *statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case error:
		cmd = s.showError(msg)
	case spinner.TickMsg:
		cmd = s.updateTicker(msg)
	case statusState:
		cmd = s.updateState(msg)
	case percentageMsg:
		cmd = s.updatePercent(msg)
	}
	return s, cmd
}
func (s *statusModel) toggleState(state statusState) tea.Cmd {
	st := s.state
	if s.state.Is(state) {
		s.state ^= state
	} else {
		s.state |= state
	}
	s.logFn("toggled state %d: %d->%d", state, st, s.state)
	return nil
}

func (s *statusModel) updateState(state statusState) tea.Cmd {
	st := s.state
	s.state = state
	s.logFn("updated state: %d->%d", st, s.state)
	return nil
}

func (s *statusModel) updatePercent(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case percentageMsg:
		s.percent = float64(m) * 100.0
	}
	return nil
}

func stoppedLoading(st statusState) tea.Cmd {
	return func() tea.Msg {
		return st ^ statusLoading
	}
}

func (s *statusModel) updateTicker(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case spinner.TickMsg:
		// If we're still doing work, or if the spinner still needs to finish, spin it along.
		s.logFn("is loading: %t :: %d", s.state.Is(statusLoading), s.state)
		if s.state.Is(statusLoading) {
			newSpinnerModel, tick := s.spinner.Update(msg)
			s.spinner = newSpinnerModel
			return tick
		}
	case statusState:
		switch {
		case !s.state.Is(statusLoading):
			if s.state.Is(statusError) {
				return s.showStatusMessage("error!")
			}
			return s.showStatusMessage("success")
		}
	}
	return nil
}

func (s *statusModel) View() string {
	b := strings.Builder{}

	if s.state.Is(statusHelp) {
		s.statusHelpView(&b)
	}
	s.statusBarView(&b)

	return b.String()
}

func (s *statusModel) statusHelpView(b *strings.Builder) {
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

func (s *statusModel) helpView() (ss string) {

	// Fill up empty cells with spaces for background coloring
	if s.width > 0 {
		lines := strings.Split(ss, "\n")
		for i := 0; i < len(lines); i++ {
			l := rw.StringWidth(lines[i])
			n := max(s.width-l, 0)
			lines[i] += strings.Repeat(" ", n)
		}

		ss = strings.Join(lines, "\n")
	}

	return
}

func (s *statusModel) Height() int {
	height := statusBarHeight
	if s.state.Is(statusHelp) {
		if pagerHelpHeight == 0 {
			pagerHelpHeight = strings.Count(s.helpView(), "\n")
		}
		height += pagerHelpHeight
	}

	s.logFn("Statusbar height: %d", height)
	return height
}

func withPadding(s string) string {
	return " " + s + " "
}

func logoView(text string, e env.Type) string {
	var bg te.Color
	fg := Color(te.ANSIBrightWhite.String())
	if e.IsProd() {
		bg = Color(Red.Dark)
	}
	if !e.IsProd() {
		bg = Color(Green.Dark)
	}
	text = fmt.Sprintf("[%s] %s", e, text)
	return te.String(withPadding(text)).Bold().Foreground(fg).Background(bg).String()
}

// Lightweight version of reflow's indent function.
func indent(b *strings.Builder, s string, n int) {
	if n <= 0 || s == "" {
		return
	}
	l := strings.Split(s, "\n")

	i := strings.Repeat(" ", n)
	for _, v := range l {
		fmt.Fprintf(b, "%s%s\n", i, v)
	}
}

func showHelpCmd() tea.Cmd {
	return func() tea.Msg {
		return statusHelp
	}
}

func waitForStatusMessageTimeout(appCtx int, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C
		return appCtx
	}
}
