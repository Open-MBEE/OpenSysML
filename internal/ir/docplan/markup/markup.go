// Package markup parses the body of a KerML textual representation written in
// a markup language into the content blocks a document plan is built from:
// paragraphs and lists of inline runs, each with its byte span in the body.
package markup

import (
	"errors"
	"fmt"
	"strings"
)

// Parser parses bodies written in one markup language.
type Parser interface {
	// Language is the language string the parser serves, in canonical spelling.
	Language() string
	// Parse maps a body onto content blocks, or fails with an
	// *UnsupportedConstructError for a construct no content block carries.
	Parse(body string) ([]Block, error)
	// External reports whether Parse runs a converter of the host rather than
	// code of this process, so that what it answers is the host's.
	External() bool
}

// For returns the parser for a textual representation's language, matched
// case-insensitively (KerML 1.0 §8.3.2.3.6); false for a language none serves.
func For(language string) (Parser, bool) {
	for _, parser := range parsers {
		if strings.EqualFold(parser.Language(), language) {
			return parser, true
		}
	}
	return nil, false
}

// Languages lists the supported languages, comma-separated, for diagnostics.
func Languages() string {
	names := make([]string, len(parsers))
	for i, parser := range parsers {
		names[i] = parser.Language()
	}
	return strings.Join(names, ", ")
}

// parsers are the supported markup languages.
var parsers = []Parser{Markdown{}, AsciiDoc{}}

// Span is a half-open byte range within a body.
type Span struct {
	Start int
	Stop  int
}

// BlockKind classifies a content block.
type BlockKind string

const (
	// BlockParagraph is a paragraph of inline runs.
	BlockParagraph BlockKind = "paragraph"
	// BlockList is a bullet or numbered list whose items are inline runs.
	BlockList BlockKind = "list"
	// BlockTable is a table of text cells under a header row.
	BlockTable BlockKind = "table"
	// BlockFormula is displayed mathematics: Source is LaTeX.
	BlockFormula BlockKind = "formula"
	// BlockImage is a figure read from a file: Location names it.
	BlockImage BlockKind = "image"
	// BlockHeading opens a section: Level is the markup's heading level and
	// Title its plain text, ID the explicit anchor the markup gave it.
	BlockHeading BlockKind = "heading"
)

// Block is one content block of a body.
type Block struct {
	Kind BlockKind
	// Runs are a paragraph's inline runs.
	Runs []Run
	// Items are a list's items.
	Items []Item
	// Ordered is true for a numbered list.
	Ordered bool
	// Caption titles a table, formula or image.
	Caption string
	// ColumnLabels head a table's columns and ColumnWidths size them relative
	// to one another (empty for automatic widths).
	ColumnLabels []string
	ColumnWidths []int
	// Rows are a table's body rows, one text cell per column.
	Rows []TableRow
	// Source is a formula's LaTeX.
	Source string
	// Location and Alt are an image's file and alternative text.
	Location string
	Alt      string
	// Level, Title and ID are a heading's level (1 the outermost), plain
	// text and explicit anchor (empty when the markup gives none).
	Level int
	Title string
	ID    string
	Span  Span
}

// TableRow is one body row of a table: one plain-text cell per column.
type TableRow struct {
	Cells []string
	Span  Span
}

// Item is one list item: a single paragraph of inline runs.
type Item struct {
	Runs []Run
	Span Span
}

// RunKind classifies an inline run.
type RunKind string

const (
	// RunText is styled prose.
	RunText RunKind = "text"
	// RunLink is a hyperlink: Text labels Destination, a URL.
	RunLink RunKind = "link"
	// RunRef is a model reference: Text labels Destination, the target's
	// notation after the `sysml:` scheme; an empty Text asks for the default label.
	RunRef RunKind = "ref"
	// RunValue is a model value: Destination is the feature's notation after
	// the `sysml:value:` scheme and Text the label the value follows, if any.
	RunValue RunKind = "value"
	// RunMath is inline mathematics: Text is LaTeX source.
	RunMath RunKind = "math"
)

// Style is the typographic style of a text run.
type Style string

const (
	StylePlain    Style = "plain"
	StyleEmphasis Style = "emphasis"
	StyleStrong   Style = "strong"
	StyleCode     Style = "code"
)

// Run is one inline run of a paragraph or list item.
type Run struct {
	Kind        RunKind
	Style       Style
	Text        string
	Destination string
	Span        Span
}

// RefScheme is the URI scheme under which a link destination names a model
// element rather than a URL.
const RefScheme = "sysml:"

// ValuePrefix follows RefScheme when a destination reads a feature's value
// rather than referencing the element.
const ValuePrefix = "value:"

// UnsupportedConstructError reports a markup construct no content block
// carries, with the line of the body it starts on (1-based).
type UnsupportedConstructError struct {
	Construct string
	Line      int
	Span      Span
}

func (e *UnsupportedConstructError) Error() string {
	return fmt.Sprintf("unsupported %s at body line %d", e.Construct, e.Line)
}

// lineOf is the 1-based line of a byte offset in body.
func lineOf(body string, offset int) int {
	if offset > len(body) {
		offset = len(body)
	}
	return 1 + strings.Count(body[:offset], "\n")
}

// headingTitle joins a heading's text runs into its title, whatever their
// style; a run of any other kind is the construct the error names.
func headingTitle(runs []Run) (string, error) {
	var b strings.Builder
	for _, run := range runs {
		if run.Kind != RunText {
			return "", errors.New(runConstruct(run.Kind) + " in a heading")
		}
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String()), nil
}

// runConstruct names a run kind as the manual does.
func runConstruct(kind RunKind) string {
	switch kind {
	case RunLink:
		return "link"
	case RunRef:
		return "reference"
	case RunMath:
		return "formula"
	}
	return string(kind)
}
