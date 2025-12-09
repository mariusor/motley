package motley

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// renderScreen is a basic screen implementation for testing
type renderScreen struct {
	*uv.Buffer
	method ansi.Method
}

func newRenderer(width, height int) *renderScreen {
	return &renderScreen{
		Buffer: uv.NewBuffer(width, height),
		method: ansi.WcWidth,
	}
}

func (m *renderScreen) Bounds() uv.Rectangle {
	return m.Buffer.Bounds()
}

func (m *renderScreen) CellAt(x, y int) *uv.Cell {
	return m.Buffer.CellAt(x, y)
}

func (m *renderScreen) SetCell(x, y int, c *uv.Cell) {
	m.Buffer.SetCell(x, y, c)
}

func (m *renderScreen) WidthMethod() uv.WidthMethod {
	return m.method
}
