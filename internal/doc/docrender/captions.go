package docrender

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// captionNumbering counts the figures and tables written so far, so every
// backend captions the same block "Figure N." or "Table N."; off, captions are as stated.
type captionNumbering struct {
	on      bool
	figures int
	tables  int
}

// The labels a numbered caption opens with.
const (
	figureLabel = "Figure"
	tableLabel  = "Table"
)

// caption is a block's caption: its label when numbered, and its text as stated.
type caption struct {
	label string
	text  string
}

// String is the caption as one line: "Figure 1. text", or whichever part is present.
func (c caption) String() string {
	switch {
	case c.label == "":
		return c.text
	case c.text == "":
		return c.label
	}
	return c.label + ". " + c.text
}

// caption labels the next figure (a drawn diagram, an image) or table (a query
// table, a table-kind diagram) in document order; a formula is not numbered.
func (n *captionNumbering) caption(node docir.Content) caption {
	c := caption{text: node.Caption()}
	if !n.on {
		return c
	}
	switch {
	case node.Kind() == docir.ContentTable, node.Kind() == docir.ContentDiagram && isTableDiagram(node):
		n.tables++
		c.label = tableLabel + " " + strconv.Itoa(n.tables)
	case node.Kind() == docir.ContentDiagram, node.Kind() == docir.ContentImage:
		n.figures++
		c.label = figureLabel + " " + strconv.Itoa(n.figures)
	}
	if c.label != "" {
		c.text = strings.TrimSpace(c.text)
	}
	return c
}

// isTableDiagram reports a diagram rendered as a table rather than drawn.
func isTableDiagram(node docir.Content) bool {
	rendering := node.Rendering()
	return rendering != nil && rendering.Kind == view.KindTable
}
