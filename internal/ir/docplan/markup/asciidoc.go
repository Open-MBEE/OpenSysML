package markup

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// AsciiDocEnv names the environment variable that points at the AsciiDoc
// converter, scripts/asciidoc/asciidoctor-json.js run by node with Asciidoctor.js
// installed beside it, as scripts/download-doc-pdf-toolchain.sh provisions it.
const AsciiDocEnv = "OPENSYSML_ASCIIDOCTOR"

// asciiDocTool is the converter's name on PATH when the variable is unset.
const asciiDocTool = "asciidoctor-json"

// asciiDocName is the converter to look for: what the variable names, and
// whether it named one, else the default to look up on PATH.
func asciiDocName() (name string, named bool) {
	if name = strings.TrimSpace(os.Getenv(AsciiDocEnv)); name != "" {
		return name, true
	}
	return asciiDocTool, false
}

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

// External reports that the converter runs outside the process.
func (AsciiDoc) External() bool { return true }

// Parse implements Parser.
func (AsciiDoc) Parse(body string) ([]Block, error) {
	output, err := runAsciiDoc(body)
	if err != nil {
		return nil, err
	}
	return decodeAsciiDoc(body, output)
}

// ToolError reports the converter missing or failing: Tool is the converter
// looked for, Named whether EnvVar named it (else it is the default looked up
// on PATH), Missing whether it was not found, Detail what else was observed.
type ToolError struct {
	Tool    string
	EnvVar  string
	Named   bool
	Missing bool
	Detail  string
}

func (e *ToolError) Error() string {
	if e.Missing {
		msg := fmt.Sprintf("the AsciiDoc converter %s was not found", e.Tool)
		switch {
		case e.Detail != "":
			msg += " (" + e.Detail + ")"
		case e.Named:
			msg += fmt.Sprintf(" (%s names it)", e.EnvVar)
		default:
			msg += fmt.Sprintf(" on PATH (%s is unset)", e.EnvVar)
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
	Start       int              `json:"start"`
	Items       []asciiDocItem   `json:"items"`
	Blocks      []asciiDocBlock  `json:"blocks"`
	Cols        []int            `json:"cols"`
	Header      [][]asciiDocCell `json:"header"`
	Rows        [][]asciiDocCell `json:"rows"`
	Footer      [][]asciiDocCell `json:"footer"`
	Source      string           `json:"source"`
	Target      string           `json:"target"`
	Alt         string           `json:"alt"`
	Level       int              `json:"level"`
	ID          string           `json:"id"`
}

type asciiDocItem struct {
	Line   int             `json:"line"`
	Runs   []asciiDocRun   `json:"runs"`
	Blocks []asciiDocBlock `json:"blocks"`
}

type asciiDocCell struct {
	Line int           `json:"line"`
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
	nodes := flattenAsciiDoc(nil, tree.Blocks)
	blocks := make([]Block, 0, len(nodes))
	for i, node := range nodes {
		stop := len(body)
		if i+1 < len(nodes) {
			stop = m.blockStart(nodes[i+1].Line)
		}
		block, err := m.block(node, stop)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// flattenAsciiDoc lays the converter's tree out in body order: a section is
// its heading followed by its blocks, subsections among them.
func flattenAsciiDoc(flat []asciiDocBlock, nodes []asciiDocBlock) []asciiDocBlock {
	for _, node := range nodes {
		if node.Kind != "section" {
			flat = append(flat, node)
			continue
		}
		children := node.Blocks
		node.Blocks = nil
		flat = flattenAsciiDoc(append(flat, node), children)
	}
	return flat
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
var asciiDocHandled = map[string]bool{"paragraph": true, "admonition": true, "list": true, "table": true, "stem": true, "image": true, "section": true}

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
	case "section":
		return m.section(node, span)
	case "admonition":
		runs, err := m.admonitionRuns(node, span, stop)
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

// section maps a section title onto a heading: the title's text, styles
// flattened, with the id an anchor gave it; the span is the title line and
// the anchor or attribute lines ahead of it.
func (m *asciiDocMapper) section(node asciiDocBlock, span Span) (Block, error) {
	if node.Level < 1 {
		return Block{}, m.refuse(fmt.Sprintf("level %d section", node.Level), span)
	}
	runs, err := m.runs(node.Runs, span, StylePlain)
	if err != nil {
		return Block{}, err
	}
	title, err := headingTitle(runs)
	if err != nil {
		return Block{}, m.refuse(err.Error(), span)
	}
	return Block{Kind: BlockHeading, Level: node.Level, Title: title, ID: node.ID, Span: span}, nil
}

// admonitionRuns reads an admonition's prose: the inline text of a one-line
// admonition, or the one paragraph a delimited admonition holds.
func (m *asciiDocMapper) admonitionRuns(node asciiDocBlock, span Span, stop int) ([]Run, error) {
	if node.Blocks == nil {
		return m.runs(node.Runs, span, StylePlain)
	}
	if len(node.Blocks) == 0 {
		return nil, m.refuse("empty admonition", span)
	}
	if len(node.Blocks) > 1 {
		return nil, m.refuse("admonition of several blocks", span)
	}
	child := node.Blocks[0]
	if child.Kind != "paragraph" {
		return nil, m.refuse(asciiDocConstruct(child.Kind)+" in an admonition", m.blockSpan(child.Line, stop))
	}
	if child.Title != "" || child.StyledTitle {
		return nil, m.refuse("title on a paragraph", m.blockSpan(child.Line, stop))
	}
	return m.runs(child.Runs, m.blockSpan(child.Line, stop), StylePlain)
}

func (m *asciiDocMapper) list(node asciiDocBlock, span Span, stop int) (Block, error) {
	if node.Ordered && node.Start != 1 {
		return Block{}, m.refuse(fmt.Sprintf("ordered list starting at %d", node.Start), span)
	}
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
	for i, row := range node.Rows {
		rowSpan := m.rowSpan(node.Rows, i, span)
		cells := make([]string, 0, len(row))
		for _, cell := range row {
			text, err := m.cellText(cell, rowSpan)
			if err != nil {
				return Block{}, err
			}
			cells = append(cells, text)
		}
		for len(cells) < len(block.ColumnLabels) {
			cells = append(cells, "")
		}
		block.Rows = append(block.Rows, TableRow{Cells: cells, Span: rowSpan})
	}
	return block, nil
}

// rowSpan covers row i's lines up to the first later row on a further line or
// the closing delimiter — rows sharing a line share the span; no line, the table's.
func (m *asciiDocMapper) rowSpan(rows [][]asciiDocCell, i int, table Span) Span {
	line := rowLine(rows[i])
	if line < 1 {
		return table
	}
	start := m.lineStart(line)
	stop := m.lineStart(lineOf(m.body, table.Stop-1))
	for j := i + 1; j < len(rows); j++ {
		if next := rowLine(rows[j]); next > line {
			stop = m.lineStart(next)
			break
		}
	}
	if start < table.Start || stop > table.Stop || stop < start {
		return table
	}
	return Span{Start: start, Stop: start + len(strings.TrimRight(m.body[start:stop], " \t\r\n"))}
}

// rowLine is the line a row's first cell starts on; 0 when unknown.
func rowLine(row []asciiDocCell) int {
	if len(row) == 0 {
		return 0
	}
	return row[0].Line
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

// locate finds where a run was written: a macro's target (math's text) at its
// first occurrence after the cursor within the block, text word by word, else
// the block itself.
func (m *asciiDocMapper) locate(run asciiDocRun, span Span, cursor *int) Span {
	if run.Kind == "text" {
		return m.locateText(run.Text, span, cursor)
	}
	needle := run.Target
	if run.Kind == "math" {
		needle = run.Text
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

// locateText places a text run over its words found in order after the cursor,
// skipping words a substitution rewrote; whitespace alone is found as written.
func (m *asciiDocMapper) locateText(text string, span Span, cursor *int) Span {
	words := strings.Fields(text)
	if len(words) == 0 {
		words = []string{text}
	}
	start, stop := -1, -1
	for _, word := range words {
		if *cursor >= span.Stop {
			break
		}
		index := strings.Index(m.body[*cursor:span.Stop], word)
		if index < 0 {
			continue
		}
		at := *cursor + index
		if start < 0 {
			start = at
		}
		*cursor = at + len(word)
		stop = *cursor
	}
	if start < 0 {
		return span
	}
	return Span{Start: start, Stop: stop}
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
