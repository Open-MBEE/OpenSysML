package docpdf

import "strings"

// This file parses docrender's closed Markdown dialect back into blocks,
// keeping the PDF layer independent of the document IR.

// blockKind names one kind of parsed Markdown block.
type blockKind int

const (
	blockHeading blockKind = iota
	blockParagraph
	blockCaption
	blockTable
	blockList
	blockMermaid
	blockDOT
	blockAnchor
)

// block is one parsed Markdown block. Text fields hold Markdown-escaped
// prose; Source holds a diagram block's raw body.
type block struct {
	Kind    blockKind
	Level   int        // blockHeading: ATX level 1..6
	Text    string     // blockHeading, blockParagraph, blockCaption
	Anchor  string     // blockAnchor: the stable identifier
	Header  []string   // blockTable: column headers
	Rows    [][]string // blockTable: body rows
	Ordered bool       // blockList
	Items   []string   // blockList
	Source  string     // blockMermaid, blockDOT
}

// Fences docrender opens diagram blocks with.
const (
	mermaidFence = "```mermaid"
	dotFence     = "```dot"
)

// parseBlocks parses docrender's Markdown dialect into blocks.
func parseBlocks(markdown string) ([]block, error) {
	lines := strings.Split(markdown, "\n")
	var blocks []block
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case line == captionMarker:
			if i+1 >= len(lines) || !isCaption(lines[i+1]) {
				return nil, &Error{Kind: ErrorDanglingCaption}
			}
			blocks = append(blocks, block{Kind: blockCaption, Text: lines[i+1][1 : len(lines[i+1])-1]})
			i++
		case strings.HasPrefix(line, "<!--") && strings.HasSuffix(line, "-->"):
			// docrender's table-rendering provenance and notice comments are
			// metadata, not prose.
			continue
		case strings.HasPrefix(line, anchorOpen) && strings.HasSuffix(line, anchorClose):
			blocks = append(blocks, block{Kind: blockAnchor, Anchor: line[len(anchorOpen) : len(line)-len(anchorClose)]})
		case line == mermaidFence || line == dotFence:
			body, next, ok := fenceBody(lines, i+1)
			if !ok {
				return nil, &Error{Kind: ErrorUnclosedFence}
			}
			kind := blockMermaid
			if line == dotFence {
				kind = blockDOT
			}
			blocks = append(blocks, block{Kind: kind, Source: body})
			i = next
		case strings.HasPrefix(line, "#"):
			level, text := headingParts(line)
			blocks = append(blocks, block{Kind: blockHeading, Level: level, Text: text})
		case strings.HasPrefix(line, "| "):
			table, next := tableBlock(lines, i)
			blocks = append(blocks, table)
			i = next
		case isListItem(line):
			list, next := listBlock(lines, i)
			blocks = append(blocks, list)
			i = next
		default:
			blocks = append(blocks, block{Kind: blockParagraph, Text: line})
		}
	}
	return blocks, nil
}

// anchorOpen and anchorClose delimit the standalone HTML anchor line
// docrender writes before a content node a reference targets.
const (
	anchorOpen  = `<a id="`
	anchorClose = `"></a>`
)

// captionMarker is the comment line docrender writes before a caption,
// distinguishing it from a paragraph that is one emphasis run.
const captionMarker = "<!-- caption -->"

// markdownWithSpanCaptions rewrites each marked caption as a bracketed span
// carrying the caption class, so converters that read the Markdown
// themselves style captions the same way the prepared HTML does.
func markdownWithSpanCaptions(markdown string) string {
	lines := strings.Split(markdown, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if lines[i] == captionMarker && i+1 < len(lines) && isCaption(lines[i+1]) {
			out = append(out, "["+lines[i+1]+"]{.caption}")
			i++
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

// fenceBody collects the lines of a fenced block opened before start,
// returning the body, the index of the closing fence, and whether one closed.
func fenceBody(lines []string, start int) (string, int, bool) {
	for i := start; i < len(lines); i++ {
		if lines[i] == "```" {
			return strings.Join(lines[start:i], "\n"), i, true
		}
	}
	return "", 0, false
}

// headingParts splits an ATX heading into its level, saturating at 6, and text.
func headingParts(line string) (int, string) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level > 6 {
		level = 6
	}
	return level, strings.TrimPrefix(line[level:], " ")
}

// tableBlock parses a pipe table from index i, returning it and the index of
// its last line. Only the row right after the header is the delimiter; every
// later pipe line is data, hyphen-only cells included.
func tableBlock(lines []string, i int) (block, int) {
	table := block{Kind: blockTable}
	table.Header = tableCells(lines[i])
	last := i
	for j := i + 1; j < len(lines) && strings.HasPrefix(lines[j], "|"); j++ {
		last = j
		if j == i+1 && isDelimiterRow(lines[j]) {
			continue
		}
		table.Rows = append(table.Rows, tableCells(lines[j]))
	}
	return table, last
}

// isDelimiterRow reports whether a table line is the header delimiter.
func isDelimiterRow(line string) bool {
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r != '-' && r != ' ' && r != '|' {
			return false
		}
	}
	return true
}

// tableCells splits one pipe-table row into its cells: the row is written as
// "| a | b |" with every literal pipe escaped, so unescaped pipes delimit.
func tableCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cell.WriteRune('\\')
			cell.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}
	if escaped {
		cell.WriteRune('\\')
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	return cells
}

// isListItem reports whether a line opens a bullet or numbered list item.
func isListItem(line string) bool {
	if strings.HasPrefix(line, "- ") {
		return true
	}
	_, ok := numberedItemText(line)
	return ok
}

// numberedItemText returns the text of a "N. item" line and whether the line
// is a numbered item; an item's text may be empty.
func numberedItemText(line string) (string, bool) {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	if digits == 0 || !strings.HasPrefix(line[digits:], ". ") {
		return "", false
	}
	return line[digits+2:], true
}

// listBlock parses a list from index i, returning it and the index of its
// last line.
func listBlock(lines []string, i int) (block, int) {
	_, ordered := numberedItemText(lines[i])
	list := block{Kind: blockList, Ordered: ordered}
	last := i
	for j := i; j < len(lines) && isListItem(lines[j]); j++ {
		last = j
		if text, ok := numberedItemText(lines[j]); ok {
			list.Items = append(list.Items, text)
		} else {
			list.Items = append(list.Items, strings.TrimPrefix(lines[j], "- "))
		}
	}
	return list, last
}

// isCaption reports whether a line is one fully-emphasized span, which is
// how docrender writes a caption's text under its marker: the first
// unescaped asterisk after the opener must be the line's last byte.
func isCaption(line string) bool {
	if len(line) < 3 || line[0] != '*' {
		return false
	}
	inner := line[1:]
	escaped := false
	for i, r := range inner {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '*':
			return i == len(inner)-1
		}
	}
	return false
}

// unescape removes the backslash escapes docrender writes before ASCII
// punctuation, recovering the literal text.
func unescape(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	escaped := false
	for _, r := range text {
		switch {
		case escaped:
			if !isASCIIPunct(r) {
				b.WriteRune('\\')
			}
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	return b.String()
}

// isASCIIPunct reports whether a rune is ASCII punctuation, the class
// CommonMark backslash escapes cover.
func isASCIIPunct(r rune) bool {
	switch {
	case r >= '!' && r <= '/':
		return true
	case r >= ':' && r <= '@':
		return true
	case r >= '[' && r <= '`':
		return true
	case r >= '{' && r <= '~':
		return true
	}
	return false
}
