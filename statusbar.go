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

const (
	lockIcon = "🔒"

	statusError  statusState = -1
	statusBrowse statusState = iota
)

type statusModel struct {
	*commonModel

	logo string

	spinner            spinner.Model
	state              statusState
	showHelp           bool
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

func (s *statusModel) showError(err error) tea.Cmd {
	s.state = statusError
	return s.showStatusMessage(err.Error())
}

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the returned command should be sent back the through the pager
// update function.
func (s *statusModel) showStatusMessage(statusMessage string) tea.Cmd {
	// Show a success message to the user
	s.state |= ^statusError
	s.statusMessage = statusMessage
	if s.statusMessageTimer != nil {
		s.statusMessageTimer.Stop()
	}
	s.statusMessageTimer = time.NewTimer(statusMessageTimeout)

	return waitForStatusMessageTimeout(1, s.statusMessageTimer)
}

func (s *statusModel) statusBarView(b *strings.Builder) {
	const (
		minPercent               float64 = 0.0
		maxPercent               float64 = 1.0
		percentToStringMagnitude float64 = 100.0
	)

	haveErr := s.state&statusError == statusError

	if !haveErr {
	}

	// Scroll percent
	// TODO(marius): get percent from treeModel
	percent := math.Max(minPercent, math.Min(maxPercent, 10))
	scrollPercent := statusBarMessageScrollPosStyle(fmt.Sprintf(" %3.f%% ", percent*percentToStringMagnitude))

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
	s.state = statusBrowse
}

func (s *statusModel) updateTicket(msg tea.Msg) tea.Cmd {
	if s.state > statusBrowse {
		// If we're still stashing, or if the spinner still needs to
		// finish, spin it along.
		newSpinnerModel, cmd := s.spinner.Update(msg)
		s.spinner = newSpinnerModel
		return cmd
	} else if s.state == statusBrowse {
		// If the spinner's finished and we haven't told the user the
		// stash was successful, do that.
		s.state = statusBrowse
		return s.showStatusMessage("Stashed!")
	} else if s.state == statusError {
		return s.showStatusMessage("Error!")
	}
	return nil
}
func (s *statusModel) View() string {
	b := strings.Builder{}

	// Footer
	switch s.state {
	default:
		s.statusBarView(&b)
	}

	if s.showHelp {
		fmt.Fprint(&b, s.helpView())
	}

	return b.String()
}

func (s *statusModel) helpView() (ss string) {
	memoOrStash := "s       set memo"

	col1 := []string{
		"g/home  go to top",
		"G/end   go to bottom",
		"",
		memoOrStash,
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

	ss = indent(ss, 2)

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

	return helpViewStyle(ss)
}

func (s *statusModel) Height() int {
	height := statusBarHeight
	// TODO(marius): replace status.showHelp for stateShowHelp
	if s.showHelp {
		if pagerHelpHeight == 0 {
			pagerHelpHeight = strings.Count(s.helpView(), "\n")
		}
		height -= pagerHelpHeight
	}
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

func waitForStatusMessageTimeout(appCtx int, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C
		return appCtx
	}
}
