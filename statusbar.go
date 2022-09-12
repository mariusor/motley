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

	statusNone  statusState = 0
	statusError statusState = 1 << iota
	statusHelp
	statusLoading
)

type statusModel struct {
	*commonModel

	logo  string
	state statusState

	spinner spinner.Model
	percent float32

	statusMessage      string
	statusMessageTimer *time.Timer
}

var glowLogoTextColor = Color("#ECFD65")

func newStatusModel(common *commonModel) statusModel {
	// Text input for search
	sp := spinner.New()
	sp.Style = lipgloss.Style{}.Foreground(statusBarNoteFg).Background(statusBarBg)
	sp.Spinner.FPS = time.Second / 10

	return statusModel{
		commonModel: common,
		spinner:     sp,
	}
}

func (s *statusModel) Init() tea.Cmd {
	s.logFn("status init")
	return nil
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
	percent := clamp(int(math.Round(float64(s.percent))), 0, 100)
	scrollPercent := statusBarMessageScrollPosStyle(fmt.Sprintf(" %d%% ", percent))

	haveErr := s.state.Is(statusError)

	var statusMessage string
	if haveErr {
		statusMessage = statusBarFailStyle(withPadding(s.statusMessage))
	} else {
		statusMessage = statusBarMessageStyle(withPadding(s.statusMessage))
	}

	// Empty space
	padding := max(0,
		s.width-
			ansi.PrintableRuneWidth(s.logo)-
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
		s.logo,
		statusMessage,
		emptySpace,
		scrollPercent,
	)
}

func (s *statusModel) unload() {
	if s.statusMessageTimer != nil {
		s.statusMessageTimer.Stop()
	}
	s.state ^= statusLoading
}

func (s *statusModel) updateState(state statusState) tea.Cmd {
	if s.state.Is(state) {
		s.state ^= state
	} else {
		s.state |= state
	}
	s.logFn("Updating status state: %d - new %d", state, s.state)
	return nil
}

func (s *statusModel) updatePercent(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case percentageMsg:
		s.percent = float32(m)
	}
	return nil
}

func (s *statusModel) updateTicker(msg tea.Msg) tea.Cmd {
	switch {
	case s.state.Is(statusLoading):
		// If the spinner's finished, and we're not doing any work, we have finished.
		s.state ^= statusLoading
		if s.state.Is(statusError) {
			return s.showStatusMessage("error!")
		}
		return s.showStatusMessage("success")
	default:
		// If we're still doing work, or if the spinner still needs to finish, spin it along.
		newSpinnerModel, cmd := s.spinner.Update(msg)
		s.spinner = newSpinnerModel
		return cmd
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
