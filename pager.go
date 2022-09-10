package motley

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pub "github.com/go-ap/activitypub"
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
	ti.CursorStyle = lipgloss.Style{}.Foreground(Fuchsia)
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

func (p *pagerModel) writePropertyWithLabel(s io.Writer, l string, it pub.Item) {
	if pub.IsNil(it) {
		return
	}
	if c, ok := it.(pub.ItemCollection); !ok || len(c) == 0 {
		return
	}
	fmt.Fprintf(s, "%s: ", l)
	p.writeItem(s, it)
	s.Write([]byte{'\n'})
}

func (p *pagerModel) writeObject(s io.Writer) func(ob *pub.Object) error {
	return func(ob *pub.Object) error {
		fmt.Fprintf(s, "Type: %s\n", ob.Type)
		fmt.Fprintf(s, "IRI: %s\n", ob.ID)
		if len(ob.MediaType) > 0 {
			fmt.Fprintf(s, "MediaType: %s\n", ob.MediaType)
		}

		p.writePropertyWithLabel(s, "AttributedTo", ob.AttributedTo)
		p.writePropertyWithLabel(s, "To", ob.To)
		p.writePropertyWithLabel(s, "CC", ob.CC)
		p.writePropertyWithLabel(s, "Bto", ob.Bto)
		p.writePropertyWithLabel(s, "BCC", ob.BCC)
		p.writePropertyWithLabel(s, "Audience", ob.Audience)

		p.writeNaturalLanguageValuesWithLabel(s, "Name", ob.Name)
		p.writeNaturalLanguageValuesWithLabel(s, "Summary", ob.Summary)
		p.writeNaturalLanguageValuesWithLabel(s, "Content", ob.Content)
		if ob.Source.Content != nil {
			if len(ob.MediaType) > 0 {
				fmt.Fprintf(s, "Source[%s]: %s\n", ob.Source.MediaType, ob.Source.Content)
			} else {
				fmt.Fprintf(s, "Source: %s\n", ob.Source.Content)
			}
		}

		p.writePropertyWithLabel(s, "Context", ob.Context)
		p.writePropertyWithLabel(s, "InReplyTo", ob.InReplyTo)

		p.writePropertyWithLabel(s, "Tag", ob.Tag)
		return nil
	}
}
func (p *pagerModel) writeActivity(s io.Writer) func(act *pub.Activity) error {
	return func(act *pub.Activity) error {
		if err := pub.OnIntransitiveActivity(act, p.writeIntransitiveActivity(s)); err != nil {
			return err
		}
		p.writePropertyWithLabel(s, "Object", act.Object)
		return nil
	}
}
func (p *pagerModel) writeIntransitiveActivity(s io.Writer) func(act *pub.IntransitiveActivity) error {
	return func(act *pub.IntransitiveActivity) error {
		if err := pub.OnObject(act, p.writeObject(s)); err != nil {
			return err
		}
		p.writePropertyWithLabel(s, "Actor", act.Actor)
		p.writePropertyWithLabel(s, "Target", act.Target)
		p.writePropertyWithLabel(s, "Result", act.Result)
		p.writePropertyWithLabel(s, "Origin", act.Origin)
		p.writePropertyWithLabel(s, "Instrument", act.Instrument)
		return nil
	}
}
func (p *pagerModel) writeActor(s io.Writer) func(act *pub.Actor) error {
	return func(act *pub.Actor) error {
		if err := pub.OnObject(act, p.writeObject(s)); err != nil {
			return err
		}
		p.writeNaturalLanguageValuesWithLabel(s, "PreferredUsername", act.PreferredUsername)
		p.writePropertyWithLabel(s, "Streams", act.Streams)
		if act.Endpoints != nil {
			p.writeItem(s, act.Endpoints.SharedInbox)
		}
		if len(act.PublicKey.ID) > 0 {
			fmt.Fprintf(s, "PublicKey: %s", act.PublicKey.PublicKeyPem)
		}
		p.writePropertyWithLabel(s, "Streams", act.Streams)
		return nil
	}
}
func (p *pagerModel) writeItemCollection(s io.Writer) func(col *pub.ItemCollection) error {
	return func(col *pub.ItemCollection) error {
		for _, it := range col.Collection() {
			if err := p.writeItem(s, it); err != nil {
				p.logFn("error: %s", err)
			}
		}
		return nil
	}
}
func (p *pagerModel) writeCollection(s io.Writer) func(col pub.CollectionInterface) error {
	return func(col pub.CollectionInterface) error {
		for _, it := range col.Collection() {
			if err := p.writeItem(s, it); err != nil {
				p.logFn("error: %s", err)
			}
		}
		return nil
	}
}

func (p *pagerModel) writeNaturalLanguageValuesWithLabel(s io.Writer, l string, values pub.NaturalLanguageValues) error {
	ll := len(values)
	if ll == 0 {
		return nil
	}
	if ll == 1 {
		fmt.Fprintf(s, "%s: %s\n", l, values[0])
		return nil
	}
	vals := make([]string, len(values))
	for i, v := range values {
		if v.Ref != "" || v.Ref != pub.NilLangRef {
			vals[i] = fmt.Sprintf("[%s]%s", v.Ref, v.Value)
		}
		vals[i] = fmt.Sprintf("%s", v.Value)
	}
	if ll > 1 {
		fmt.Fprintf(s, "%s: [ %s ]\n", l, strings.Join(vals, ", "))
	}
	return nil
}

func (p *pagerModel) writeItem(s io.Writer, it pub.Item) error {
	//m, _ := pub.MarshalJSON(it)
	//b := bytes.Buffer{}
	//json.Indent(&b, m, "", "  ")
	//s.Write(b.Bytes())
	//s.Write([]byte{'\n'})

	if pub.IsIRI(it) {
		fmt.Fprintf(s, "%s\n", it.GetLink())
		return nil
	}

	if pub.IsItemCollection(it) {
		return pub.OnItemCollection(it, p.writeItemCollection(s))
	}
	typ := it.GetType()
	if pub.IntransitiveActivityTypes.Contains(typ) {
		return pub.OnIntransitiveActivity(it, p.writeIntransitiveActivity(s))
	}
	if pub.ActivityTypes.Contains(typ) {
		return pub.OnActivity(it, p.writeActivity(s))
	}
	if pub.ActorTypes.Contains(typ) {
		return pub.OnActor(it, p.writeActor(s))
	}
	if pub.ObjectTypes.Contains(typ) || typ == "" {
		return pub.OnObject(it, p.writeObject(s))
	}
	return fmt.Errorf("unknown activitypub object of type %T", it)
}

func (p *pagerModel) showItem(it pub.Item) error {
	s := strings.Builder{}
	if err := p.writeItem(&s, it); err != nil {
		p.logFn("err: %s", err)
		return nil
	}
	p.setContent(s.String())
	return nil
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
	pagerStateError int = -1

	pagerStateBrowse int = iota
)

func (p *pagerModel) showError(err error) tea.Cmd {
	p.state = pagerStateError
	return p.showStatusMessage(err.Error())
}

// Perform stuff that needs to happen after a successful markdown stash. Note
// that the returned command should be sent back the through the pager
// update function.
func (p *pagerModel) showStatusMessage(statusMessage string) tea.Cmd {
	// Show a success message to the user
	p.state |= ^pagerStateError
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
		} else if p.state == pagerStateError {
			p.showStatusMessage("Error!")
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
	haveErr := p.state&pagerStateError == pagerStateError

	if !haveErr {
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
