package motley

import (
	"fmt"
	"math"
	"strings"
	"time"

	"git.sr.ht/~mariusor/motley/internal/env"
	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
	rw "github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/margin"
	"github.com/muesli/reflow/truncate"
	te "github.com/muesli/termenv"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	statusHelp statusState = 1 << iota
	statusBusy

	statusNone statusState = 0
)

type statusModel struct {
	*commonModel

	width int
	state statusState
	env   env.Type

	spinner spinner.Model
	percent float64

	error   error
	message string

	timer *time.Timer
}

func initializeSpinner() spinner.Model {
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Bold(true)
	sp.Spinner = spinner.Ellipsis
	sp.Spinner.FPS = time.Second / 4
	return sp
}

func newStatusModel(common *commonModel) statusModel {
	return statusModel{
		commonModel: common,
		spinner:     initializeSpinner(),
	}
}

func (s *statusModel) Init() (tea.Model, tea.Cmd) {
	s.logFn("Status Bar init")
	return s, noop
}

func (s *statusModel) showError(err error) tea.Cmd {
	s.error = err
	return noop
}

func (s *statusModel) showStatusMessage(statusMessage string) tea.Cmd {
	if lipgloss.Height(statusMessage) > 1 {
		statusMessage = strings.ReplaceAll(strings.ReplaceAll(statusMessage, "\r", ""), "\n", " ")
	}
	s.message = statusMessage
	s.error = nil
	return noop
}

func ucfirst(s string) string {
	pieces := strings.SplitN(s, " ", 2)
	if len(pieces) > 0 {
		pieces[0] = cases.Title(language.English).String(pieces[0])
	}
	return strings.Join(pieces, " ")
}

func (s *statusModel) statusBarView(b *strings.Builder) {
	percent := clamp(int(math.Round(s.percent)), 0, 100)
	scrollPercent := fmt.Sprintf(" %d%% ", percent)

	spinner := s.spinner.View()
	logo := logoView(name(s.root), s.env)

	// Empty space
	w := max(0, s.width-lipgloss.Width(spinner)-lipgloss.Width(logo)-lipgloss.Width(scrollPercent)-1)

	render := statusBarMessageStyle
	message := truncate.StringWithTail(s.message, uint(w), ellipsis)
	if s.error != nil {
		render = statusBarFailStyle
		message = ucfirst(s.error.Error())
	}

	b.WriteString(logo)
	b.WriteString(render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			margin.String(message, uint(w), 1),
			scrollPercent,
			margin.String(spinner, 3, 0),
		),
	))
}

func (s *statusModel) spin(msg tea.Msg) tea.Cmd {
	newSpinnerModel, tick := s.spinner.Update(msg)
	s.spinner = newSpinnerModel
	return tick
}

type statusNode n

func (a statusNode) View() string {
	s := strings.Builder{}
	switch it := a.Item; it.(type) {
	case pub.IRI, *pub.IRI:
		fmt.Fprintf(&s, "%s", it.GetID())
	case pub.ItemCollection:
		fmt.Fprintf(&s, "%s %s: %d items", ItemType(it), a.n, len(a.c))
	case pub.Item:
		fmt.Fprintf(&s, "%s: %s", ItemType(it), it.GetID())
	}
	return s.String()
}

func (s *statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch mm := msg.(type) {
	case error:
		cmd = s.showError(mm)
	case spinner.TickMsg:
		if s.state.Is(statusBusy) {
			cmd = s.spin(msg)
		}
	case nodeUpdateMsg:
		cmd = s.showStatusMessage(statusNode(mm).View())
	case statusState:
		s.state |= mm
		if !s.state.Is(statusBusy) {
			s.logFn("resetting spinner")
			s.spinner = initializeSpinner()
		}
	case percentageMsg:
		s.percent = float64(mm) * 100.0
	}

	return s, cmd
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
