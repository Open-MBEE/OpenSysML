package markup

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AsciiDocEnv names the environment variable that points at the AsciiDoc
// converter, scripts/asciidoc/asciidoctor-json.js run by node with Asciidoctor.js
// installed beside it, as scripts/download-doc-pdf-toolchain.sh provisions it.
const AsciiDocEnv = "OPENSYSML_ASCIIDOCTOR"

// asciiDocTool is the converter's name on PATH when the variable is unset.
const asciiDocTool = "asciidoctor-json"

// asciiDocTimeout bounds one converter run, as every document converter is bounded.
const asciiDocTimeout = 5 * time.Minute

// AsciiDoc parses AsciiDoc bodies through Asciidoctor.js, run as an external
// converter that answers the document tree as JSON: paragraphs, admonitions,
// bullet and numbered lists of single paragraphs, tables of text cells under a
// header row, stem blocks and block images, with emphasis, strong, monospaced,
// `stem:[…]`, links, `<<xref>>`, `sysml:ref[…]` and `sysml:value[…]` inline.
type AsciiDoc struct{}

// Language implements Parser.
func (AsciiDoc) Language() string { return "asciidoc" }

// Parse implements Parser.
func (AsciiDoc) Parse(body string) ([]Block, error) {
	output, err := runAsciiDoc(body)
	if err != nil {
		return nil, err
	}
	return decodeAsciiDoc(body, output)
}

// ToolError reports the converter missing or failing.
type ToolError struct {
	Tool    string
	EnvVar  string
	Missing bool
	Detail  string
}

func (e *ToolError) Error() string {
	if e.Missing {
		msg := fmt.Sprintf("the AsciiDoc converter %s was not found", e.Tool)
		if e.Detail != "" {
			msg += " (" + e.Detail + ")"
		}
		return msg + fmt.Sprintf("; run ./scripts/download-doc-pdf-toolchain.sh and point %s at build/doc-pdf/asciidoctor/asciidoctor-json.js", e.EnvVar)
	}
	msg := fmt.Sprintf("the AsciiDoc converter %s failed", e.Tool)
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg
}

// The converter's JSON tree.
type asciiDocTree struct {
	Blocks []asciiDocBlock `json:"blocks"`
}

type asciiDocBlock struct {
	Kind        string           `json:"kind"`
	Line        int              `json:"line"`
	Title       string           `json:"title"`
	StyledTitle bool             `json:"styledTitle"`
	Style       string           `json:"style"`
	Label       string           `json:"label"`
	Runs        []asciiDocRun    `json:"runs"`
	Ordered     bool             `json:"ordered"`
	Items       []asciiDocItem   `json:"items"`
	Cols        []int            `json:"cols"`
	Header      [][]asciiDocCell `json:"header"`
	Rows        [][]asciiDocCell `json:"rows"`
	Footer      [][]asciiDocCell `json:"footer"`
	Source      string           `json:"source"`
	Target      string           `json:"target"`
	Alt         string           `json:"alt"`
}

type asciiDocItem struct {
	Line   int             `json:"line"`
	Runs   []asciiDocRun   `json:"runs"`
	Blocks []asciiDocBlock `json:"blocks"`
}

type asciiDocCell struct {
	Runs []asciiDocRun `json:"runs"`
}

type asciiDocRun struct {
	Kind      string `json:"kind"`
	Style     string `json:"style"`
	Text      string `json:"text"`
	Target    string `json:"target"`
	Construct string `json:"construct"`
}

// decodeAsciiDoc maps the converter's tree onto blocks of the body.
func decodeAsciiDoc(body string, output []byte) ([]Block, error) {
	var tree asciiDocTree
	if err := json.Unmarshal(output, &tree); err != nil {
		return nil, &ToolError{Tool: asciiDocTool, EnvVar: AsciiDocEnv, Detail: "unreadable answer: " + err.Error()}
	}
	m := &asciiDocMapper{body: body, lines: lineStarts(body)}
	blocks := make([]Block, 0, len(tree.Blocks))
	for i, node := range tree.Blocks {
		stop := len(body)
		if i+1 < len(tree.Blocks) {
			stop = m.blockStart(tree.Blocks[i+1].Line)
		}
		block, err := m.block(node, stop)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// lineStarts indexes the byte offset each 1-based line of body starts at.
func lineStarts(body string) []int {
	starts := []int{0}
	for i := 0; i < len(body); i++ {
		if body[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// asciiDocMapper maps one body's converter tree onto spans within the body.
type asciiDocMapper struct {
	body  string
	lines []int
}

// lineStart is the offset line (1-based) starts at; past the end, the body's end.
func (m *asciiDocMapper) lineStart(line int) int {
	if line < 1 {
		return 0
	}
	if line > len(m.lines) {
		return len(m.body)
	}
	return m.lines[line-1]
}

// blockSpan covers a block from its first line — a block title or attribute
// line ahead of the one the converter reports included — to the end of its
// last non-blank line before stop.
func (m *asciiDocMapper) blockSpan(line, stop int) Span {
	start := m.blockStart(line)
	if stop < start {
		stop = start
	}
	end := start + len(strings.TrimRight(m.body[start:stop], " \t\r\n"))
	return Span{Start: start, Stop: end}
}

// blockStart is the offset a block reported at line starts at: the first of
// the block title (`.Title`) and attribute (`[cols=…]`) lines written ahead
// of it, else that line.
func (m *asciiDocMapper) blockStart(line int) int {
	for line > 1 && asciiDocMetadataLine(m.lineText(line-1)) {
		line--
	}
	return m.lineStart(line)
}

// lineText is the text of line (1-based) without its line break.
func (m *asciiDocMapper) lineText(line int) string {
	start, stop := m.lineStart(line), m.lineStart(line+1)
	return strings.TrimRight(m.body[start:stop], "\r\n")
}

// asciiDocMetadataLine reports whether text is a block title or attribute
// line, which asciidoctor attaches to the block that follows it.
func asciiDocMetadataLine(text string) bool {
	switch {
	case len(text) > 1 && text[0] == '.' && text[1] != '.' && text[1] != ' ':
		return true
	case len(text) > 1 && text[0] == '[' && strings.HasSuffix(text, "]"):
		return true
	}
	return false
}

// asciiDocCaptioned lists the block kinds a block title captions.
var asciiDocCaptioned = map[string]bool{"table": true, "stem": true, "image": true}

// asciiDocHandled lists the block kinds a content block carries.
var asciiDocHandled = map[string]bool{"paragraph": true, "admonition": true, "list": true, "table": true, "stem": true, "image": true}

func (m *asciiDocMapper) block(node asciiDocBlock, stop int) (Block, error) {
	span := m.blockSpan(node.Line, stop)
	if !asciiDocHandled[node.Kind] {
		return Block{}, m.refuse(asciiDocConstruct(node.Kind), span)
	}
	if node.StyledTitle {
		return Block{}, m.refuse("styled block title", span)
	}
	if node.Title != "" && !asciiDocCaptioned[node.Kind] {
		return Block{}, m.refuse("title on a "+asciiDocConstruct(node.Kind), span)
	}
	switch node.Kind {
	case "paragraph":
		runs, err := m.runs(node.Runs, span, StylePlain)
		if err != nil {
			return Block{}, err
		}
		return Block{Kind: BlockParagraph, Runs: runs, Span: span}, nil
	case "admonition":
		runs, err := m.runs(node.Runs, span, StylePlain)
		if err != nil {
			return Block{}, err
		}
		label := node.Label
		if label == "" {
			label = node.Style
		}
		at := Span{Start: span.Start, Stop: span.Start}
		lead := []Run{{Kind: RunText, Style: StyleStrong, Text: label + ":", Span: at}}
		lead = appendText(lead, Run{Kind: RunText, Style: StylePlain, Text: " ", Span: at})
		for _, run := range runs {
			if run.Kind == RunText {
				lead = appendText(lead, run)
			} else {
				lead = append(lead, run)
			}
		}
		return Block{Kind: BlockParagraph, Runs: lead, Span: span}, nil
	case "list":
		return m.list(node, span, stop)
	case "table":
		return m.table(node, span)
	case "stem":
		if node.Style != "latexmath" {
			return Block{}, m.refuse(node.Style+" stem block", span)
		}
		return Block{Kind: BlockFormula, Source: strings.TrimSpace(node.Source), Caption: node.Title, Span: span}, nil
	}
	return Block{Kind: BlockImage, Location: node.Target, Alt: node.Alt, Caption: node.Title, Span: span}, nil
}

func (m *asciiDocMapper) list(node asciiDocBlock, span Span, stop int) (Block, error) {
	block := Block{Kind: BlockList, Ordered: node.Ordered, Span: span}
	for i, item := range node.Items {
		itemStop := stop
		if i+1 < len(node.Items) {
			itemStop = m.blockStart(node.Items[i+1].Line)
		}
		itemSpan := m.blockSpan(item.Line, itemStop)
		if len(item.Blocks) > 0 {
			construct := "nested list"
			if item.Blocks[0].Kind != "list" {
				construct = asciiDocConstruct(item.Blocks[0].Kind) + " attached to a list item"
			}
			return Block{}, m.refuse(construct, m.blockSpan(item.Blocks[0].Line, itemStop))
		}
		runs, err := m.runs(item.Runs, itemSpan, StylePlain)
		if err != nil {
			return Block{}, err
		}
		block.Items = append(block.Items, Item{Runs: runs, Span: itemSpan})
	}
	return block, nil
}

func (m *asciiDocMapper) table(node asciiDocBlock, span Span) (Block, error) {
	if len(node.Header) == 0 {
		return Block{}, m.refuse("table without a header row", span)
	}
	if len(node.Footer) > 0 {
		return Block{}, m.refuse("table footer", span)
	}
	block := Block{Kind: BlockTable, Caption: node.Title, Span: span}
	for _, cell := range node.Header[0] {
		text, err := m.cellText(cell, span)
		if err != nil {
			return Block{}, err
		}
		block.ColumnLabels = append(block.ColumnLabels, text)
	}
	if len(node.Cols) == len(block.ColumnLabels) {
		block.ColumnWidths = append([]int(nil), node.Cols...)
	}
	for _, row := range node.Rows {
		cells := make([]string, 0, len(row))
		for _, cell := range row {
			text, err := m.cellText(cell, span)
			if err != nil {
				return Block{}, err
			}
			cells = append(cells, text)
		}
		for len(cells) < len(block.ColumnLabels) {
			cells = append(cells, "")
		}
		block.Rows = append(block.Rows, TableRow{Cells: cells, Span: span})
	}
	return block, nil
}

// cellText reads a cell that is plain text alone; a table cell is one value.
func (m *asciiDocMapper) cellText(cell asciiDocCell, span Span) (string, error) {
	var b strings.Builder
	for _, run := range cell.Runs {
		switch {
		case run.Kind == "unsupported":
			return "", m.refuse(run.Construct, span)
		case run.Kind != "text":
			return "", m.refuse(asciiDocRunConstruct(run)+" in a table cell", span)
		case run.Style != "plain":
			return "", m.refuse(run.Style+" text in a table cell", span)
		}
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String()), nil
}

// runs maps a block's inline runs, placing each at its first occurrence in
// the block's source after the one before, else over the whole block.
func (m *asciiDocMapper) runs(runs []asciiDocRun, span Span, style Style) ([]Run, error) {
	out := make([]Run, 0, len(runs))
	cursor := span.Start
	for _, run := range runs {
		at := m.locate(run, span, &cursor)
		switch run.Kind {
		case "text":
			s, ok := asciiDocStyle(run.Style)
			if !ok {
				return nil, m.refuse(run.Style+" text", at)
			}
			if style != StylePlain && s != StylePlain {
				return nil, m.refuse("nested quoted text", at)
			}
			if s == StylePlain {
				s = style
			}
			out = appendText(out, Run{Kind: RunText, Style: s, Text: run.Text, Span: at})
		case "math":
			out = append(out, Run{Kind: RunMath, Style: StylePlain, Text: run.Text, Span: at})
		case "link":
			out = append(out, Run{Kind: RunLink, Style: StylePlain, Text: run.Text, Destination: run.Target, Span: at})
		case "xref", "ref":
			if strings.TrimSpace(run.Target) == "" {
				return nil, m.refuse(asciiDocRunConstruct(run)+" without a target", at)
			}
			out = append(out, Run{Kind: RunRef, Style: StylePlain, Text: run.Text, Destination: run.Target, Span: at})
		case "value":
			if strings.TrimSpace(run.Target) == "" {
				return nil, m.refuse(asciiDocRunConstruct(run)+" without a target", at)
			}
			out = append(out, Run{Kind: RunValue, Style: StylePlain, Text: run.Text, Destination: run.Target, Span: at})
		case "unsupported":
			return nil, m.refuse(run.Construct, at)
		default:
			return nil, m.refuse(asciiDocRunConstruct(run), at)
		}
	}
	return out, nil
}

// appendText adds a text run, merging it into a preceding run of one style.
func appendText(runs []Run, run Run) []Run {
	if n := len(runs); n > 0 && runs[n-1].Kind == RunText && runs[n-1].Style == run.Style {
		runs[n-1].Text += run.Text
		runs[n-1].Span.Stop = run.Span.Stop
		return runs
	}
	return append(runs, run)
}

// locate finds where a run was written: its text (a macro's target) at its
// first occurrence after the cursor within the block, else the block itself.
func (m *asciiDocMapper) locate(run asciiDocRun, span Span, cursor *int) Span {
	needle := run.Text
	if run.Kind != "text" && run.Kind != "math" {
		needle = run.Target
	}
	needle = strings.TrimSpace(needle)
	if needle == "" || *cursor >= span.Stop {
		return span
	}
	index := strings.Index(m.body[*cursor:span.Stop], needle)
	if index < 0 {
		return span
	}
	start := *cursor + index
	*cursor = start + len(needle)
	return Span{Start: start, Stop: *cursor}
}

func asciiDocStyle(style string) (Style, bool) {
	switch style {
	case "plain", "":
		return StylePlain, true
	case "emphasis":
		return StyleEmphasis, true
	case "strong":
		return StyleStrong, true
	case "monospaced":
		return StyleCode, true
	}
	return "", false
}

// asciiDocConstruct names a block context the way the manual does.
func asciiDocConstruct(kind string) string {
	switch kind {
	case "header":
		return "document header"
	case "section":
		return "section heading"
	case "listing", "literal":
		return kind + " block"
	case "dlist":
		return "description list"
	case "colist":
		return "callout list"
	case "thematic_break":
		return "thematic break"
	case "page_break":
		return "page break"
	case "open", "sidebar", "example", "quote", "verse", "pass":
		return kind + " block"
	case "list":
		return "list"
	}
	return strings.ReplaceAll(kind, "_", " ")
}

// asciiDocRunConstruct names an inline node the way the manual does.
func asciiDocRunConstruct(run asciiDocRun) string {
	switch run.Kind {
	case "value":
		return "sysml:value macro"
	case "ref":
		return "sysml:ref macro"
	case "image":
		return "inline image"
	case "xref":
		return "cross reference"
	case "link", "math":
		return run.Kind
	}
	return strings.ReplaceAll(run.Kind, "_", " ")
}

func (m *asciiDocMapper) refuse(construct string, span Span) error {
	return &UnsupportedConstructError{Construct: construct, Line: lineOf(m.body, span.Start), Span: span}
}
