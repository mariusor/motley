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
	"github.com/muesli/reflow/margin"
	"github.com/muesli/reflow/truncate"
	te "github.com/muesli/termenv"
)

type statusState uint8

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
	statusBusy

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

func initializeSpinner() spinner.Model {
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Bold(true)
	sp.Spinner = spinner.Line
	//sp.Spinner.Frames = []string{"", "🞄", "•", "⚫", "•", "🞄"}
	//sp.Spinner.Frames = []string{"⨁", "⨂"}
	//sp.Spinner.Frames = []string{"◤", "◥", "◢", "◣"}
	//sp.Spinner.Frames = []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"}
	//sp.Spinner.Frames = []string{"◒", "◐", "◓", "◑"}
	//sp.Spinner.Frames = []string{"🭶", "🭷", "🭸", "🭹", "🭺", "🭻"}
	//sp.Spinner.Frames = []string{"⠦", "⠖", "⠲", "⠴"},
	//sp.Spinner.Frames = []string{"+", "×"}
	//sp.Spinner.Frames = []string{"-", "︲"}
	sp.Spinner.FPS = time.Second / 4
	return sp
}

func newStatusModel(common *commonModel) statusModel {
	// Text input for search

	common.logFn("initializing status bar")
	return statusModel{
		commonModel: common,
		spinner:     initializeSpinner(),
	}
}

func (s *statusModel) Init() tea.Cmd {
	s.logFn("status init")
	return nil
}

func (s *statusModel) showError(err error) tea.Cmd {
	s.statusMessage = err.Error()
	return s.stateError
}

func (s *statusModel) showStatusMessage(statusMessage string) tea.Cmd {
	s.statusMessage = statusMessage
	return s.noError
}

func (s *statusModel) statusBarView(b *strings.Builder) {
	percent := clamp(int(math.Round(s.percent)), 0, 100)
	scrollPercent := fmt.Sprintf(" %d%% ", percent)

	spinner := s.spinner.View()
	// Empty space
	w := s.width - lipgloss.Width(spinner) - lipgloss.Width(s.logo) - lipgloss.Width(scrollPercent) - 1

	render := statusBarMessageStyle
	if s.state.Is(statusError) {
		render = statusBarFailStyle
	}
	b.WriteString(s.logo)
	b.WriteString(render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			margin.String(truncate.StringWithTail(s.statusMessage, uint(w), ellipsis), uint(w), 1),
			scrollPercent,
			margin.String(spinner, 3, 1),
		),
	))
}

func (s *statusModel) spin(msg tea.Msg) tea.Cmd {
	newSpinnerModel, tick := s.spinner.Update(msg)
	s.spinner = newSpinnerModel
	return tick
}

func (s *statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case error:
		cmd = s.showError(msg)
	case spinner.TickMsg:
		if s.state.Is(statusBusy) {
			cmd = s.spin(msg)
		}
	case statusState:
		if !msg.Is(statusError) && s.state.Is(statusError) {
			s.state ^= statusError
		}
		s.state = msg
		if msg.Is(statusBusy) {
			s.logFn("starting spinner")
			cmd = s.spinner.Tick
		} else {
			s.logFn("stopping spinner")
			initializeSpinner()
		}
	case percentageMsg:
		s.percent = float64(msg) * 100.0
	}

	return s, cmd
}

func (s *statusModel) noError() tea.Msg {
	return func() tea.Msg {
		return s.state ^ statusError
	}
}

func (s *statusModel) stateError() tea.Msg {
	return func() tea.Msg {
		return s.state | statusError
	}
}

func (s *statusModel) startedLoading() tea.Msg {
	return s.state | statusBusy
}

func (s *statusModel) stoppedLoading() tea.Msg {
	return s.state ^ statusBusy
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

func withPadding(s string, w int) string {
	return margin.String(s, uint(w), 1) + " "
}

func logoView(text string, e env.Type) string {
	var bg te.Color
	fg := Color(te.ANSIBrightWhite.String())
	bg = Color(Red.Dark)
	if e != "" {
		if !e.IsProd() {
			bg = Color(Green.Dark)
		}
		text = fmt.Sprintf("[%s] %s", e, text)
	} else {
		text = fmt.Sprintf("%s", text)
	}
	return te.String(withPadding(text, len(text))).Bold().Foreground(fg).Background(bg).String()
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
