package motley

import (
	"mime"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	vocab "github.com/go-ap/activitypub"
)

var _ tea.Model = ObjectModel{}

// ObjectModel
// We need to group different properties which serve similar purposes into the same controls
// Audience: To, CC, Bto, BCC
// Name: Name, PreferredName
// Content: Summary, Content, Source
type ObjectModel struct {
	vocab.Object

	Name    NaturalLanguageValues
	Summary NaturalLanguageValues
	Content NaturalLanguageValues
}

func newObjectModel() ObjectModel {
	return ObjectModel{}
}

func (o ObjectModel) Init() (tea.Model, tea.Cmd) {
	return o, noop
}

func (o ObjectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return o, noop
}

func mimeIsBinary(mimeType vocab.MimeType) bool {
	var binContentMimeTypes = []string{"image", "audio", "video", "application"}
	justType, _, _ := mime.ParseMediaType(string(mimeType))
	if pieces := strings.Split(justType, "/"); len(pieces) >= 1 {
		m := pieces[0]
		for _, check := range binContentMimeTypes {
			if strings.EqualFold(check, m) {
				return true
			}
		}
	}
	return false
}

var binContentTypes = vocab.ActivityVocabularyTypes{
	vocab.ImageType,
	vocab.AudioType,
	vocab.VideoType,
}

func (o ObjectModel) View() string {
	if o.ID == "" {
		return ""
	}
	pieces := make([]string, 0)

	// TODO(marius): move this to initialization, and move the setting of the nat values into the Update()
	if len(o.Object.Name) > 0 {
		o.Name = nameModel(o.Object.Name)
	}
	if len(o.Object.Summary) > 0 {
		o.Summary = summaryModel(o.Object.Summary)
	}
	if ll := len(o.Object.Content); ll > 0 {
		o.Content = contentModel(o.Object.Content)
	}

	typeStyle := lipgloss.NewStyle().Bold(true).BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	title := typeStyle.Render(ItemType(o))
	if o.MediaType != "" {
		title = lipgloss.JoinHorizontal(lipgloss.Right, title, typeStyle.Bold(false).Render(" ("+string(o.MediaType)+")"))
	}
	pieces = append(pieces, title)
	if name := o.Name.View(); len(name) > 0 {
		pieces = append(pieces, name)
	}
	if summary := o.Summary.View(); len(summary) > 0 {
		pieces = append(pieces, summary)
	}

	if !binContentTypes.Contains(o.GetType()) && !mimeIsBinary(o.MediaType) {
		if content := o.Content.View(); len(content) > 0 {
			pieces = append(pieces, content)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Top, pieces...)
}

func (o *ObjectModel) updateObject(ob *vocab.Object) error {
	o.Object = *ob
	return nil
}
