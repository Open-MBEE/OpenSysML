package markup

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Markdown parses CommonMark bodies: paragraphs and tight or loose lists of
// single paragraphs, with emphasis, strong emphasis, code spans, links,
// autolinks, `sysml:` references and `$…$` mathematics inline. No GFM
// extension is enabled, so tables, footnotes, strikethrough and task lists
// read as the paragraphs CommonMark makes of them.
type Markdown struct{}

// Language implements Parser.
func (Markdown) Language() string { return "markdown" }

// markdown is the configured CommonMark parser: the default block and inline
// parsers plus inline mathematics.
var markdown = goldmark.New(goldmark.WithParserOptions(
	parser.WithInlineParsers(util.Prioritized(mathParser{}, 150)),
))

// Parse implements Parser.
func (Markdown) Parse(body string) ([]Block, error) {
	source := []byte(body)
	root := markdown.Parser().Parse(text.NewReader(source))
	m := &markdownMapper{body: body, source: source}
	blocks := make([]Block, 0)
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		block, err := m.block(node)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

// markdownMapper maps goldmark nodes onto blocks and runs of one body.
type markdownMapper struct {
	body   string
	source []byte
}

// block maps a top-level block node.
func (m *markdownMapper) block(node gmast.Node) (Block, error) {
	switch n := node.(type) {
	case *gmast.Paragraph:
		runs, err := m.runs(n, StylePlain)
		if err != nil {
			return Block{}, err
		}
		return Block{Kind: BlockParagraph, Runs: runs, Span: m.blockSpan(n)}, nil
	case *gmast.List:
		return m.list(n)
	case *gmast.Heading:
		return m.heading(n)
	default:
		return Block{}, m.unsupported(node)
	}
}

// heading maps an ATX or setext heading: its text, styles flattened, is the
// section title; a link, reference or formula in it has no title form.
func (m *markdownMapper) heading(heading *gmast.Heading) (Block, error) {
	span := m.headingSpan(heading)
	runs, err := m.runs(heading, StylePlain)
	if err != nil {
		return Block{}, err
	}
	title, err := headingTitle(runs)
	if err != nil {
		return Block{}, m.refuse(err.Error(), span)
	}
	return Block{Kind: BlockHeading, Level: heading.Level, Title: title, Span: span}, nil
}

// headingSpan covers a heading's whole lines: goldmark records its text only,
// so the span widens to the line ends and, for a setext heading, its underline.
func (m *markdownMapper) headingSpan(heading *gmast.Heading) Span {
	span := m.blockSpan(heading)
	span.Start = strings.LastIndexByte(m.body[:span.Start], '\n') + 1
	if end := strings.IndexByte(m.body[span.Stop:], '\n'); end >= 0 {
		span.Stop += end
	} else {
		span.Stop = len(m.body)
	}
	if !strings.HasPrefix(strings.TrimLeft(m.body[span.Start:span.Stop], " "), "#") && span.Stop < len(m.body) {
		if end := strings.IndexByte(m.body[span.Stop+1:], '\n'); end >= 0 {
			span.Stop += 1 + end
		} else {
			span.Stop = len(m.body)
		}
	}
	return span
}

// list maps a list whose every item is one paragraph.
func (m *markdownMapper) list(list *gmast.List) (Block, error) {
	if list.IsOrdered() && list.Start != 1 {
		return Block{}, m.refuse(fmt.Sprintf("ordered list starting at %d", list.Start), m.blockSpan(list))
	}
	block := Block{Kind: BlockList, Ordered: list.IsOrdered(), Span: m.blockSpan(list), Items: make([]Item, 0, list.ChildCount())}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		for child := item.FirstChild(); child != nil; child = child.NextSibling() {
			if child.Kind() == gmast.KindList {
				return Block{}, m.refuse("nested list", m.blockSpan(child))
			}
		}
		if item.ChildCount() != 1 {
			return Block{}, m.refuse("list item of more than one block", m.blockSpan(item))
		}
		content := item.FirstChild()
		if content.Kind() != gmast.KindTextBlock && content.Kind() != gmast.KindParagraph {
			return Block{}, m.unsupported(content)
		}
		runs, err := m.runs(content, StylePlain)
		if err != nil {
			return Block{}, err
		}
		block.Items = append(block.Items, Item{Runs: runs, Span: m.blockSpan(content)})
	}
	return block, nil
}

// runs maps the inline children of a block or emphasis node under style,
// merging adjacent text of one style into one run.
func (m *markdownMapper) runs(parent gmast.Node, style Style) ([]Run, error) {
	var runs []Run
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		mapped, err := m.inline(node, style)
		if err != nil {
			return nil, err
		}
		for _, run := range mapped {
			if last := len(runs) - 1; last >= 0 && run.Kind == RunText && runs[last].Kind == RunText && runs[last].Style == run.Style {
				runs[last].Text += run.Text
				runs[last].Span.Stop = run.Span.Stop
				continue
			}
			runs = append(runs, run)
		}
	}
	return runs, nil
}

// inline maps one inline node.
func (m *markdownMapper) inline(node gmast.Node, style Style) ([]Run, error) {
	switch n := node.(type) {
	case *gmast.Text:
		return []Run{m.textRun(n, style)}, nil
	case *gmast.String:
		return []Run{{Kind: RunText, Style: style, Text: string(n.Value), Span: m.inlineSpan(n)}}, nil
	case *gmast.Emphasis:
		if style != StylePlain {
			return nil, m.refuse("nested emphasis", m.inlineSpan(n))
		}
		inner := StyleEmphasis
		if n.Level >= 2 {
			inner = StyleStrong
		}
		return m.runs(n, inner)
	case *gmast.CodeSpan:
		if style != StylePlain {
			return nil, m.refuse("code span inside emphasis", m.inlineSpan(n))
		}
		var b strings.Builder
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			b.Write(child.(*gmast.Text).Segment.Value(m.source))
		}
		return []Run{{Kind: RunText, Style: StyleCode, Text: b.String(), Span: m.inlineSpan(n)}}, nil
	case *mathNode:
		if style != StylePlain {
			return nil, m.refuse("mathematics inside emphasis", Span{n.Segment.Start, n.Segment.Stop})
		}
		return []Run{{Kind: RunMath, Text: string(n.Segment.Value(m.source)), Span: Span{n.Segment.Start - 1, n.Segment.Stop + 1}}}, nil
	case *gmast.Link:
		if style != StylePlain {
			return nil, m.refuse("link inside emphasis", m.inlineSpan(n))
		}
		if len(n.Title) > 0 {
			return nil, m.refuse("link title", m.inlineSpan(n))
		}
		label, err := m.label(n)
		if err != nil {
			return nil, err
		}
		destination := string(n.Destination)
		span := m.inlineSpan(n)
		if target, ok := strings.CutPrefix(destination, RefScheme); ok {
			return []Run{{Kind: RunRef, Text: label, Destination: target, Span: span}}, nil
		}
		return []Run{{Kind: RunLink, Text: label, Destination: destination, Span: span}}, nil
	case *gmast.AutoLink:
		if style != StylePlain {
			return nil, m.refuse("link inside emphasis", m.inlineSpan(n))
		}
		url := string(n.URL(m.source))
		return []Run{{Kind: RunLink, Text: string(n.Label(m.source)), Destination: url, Span: m.inlineSpan(n)}}, nil
	case *gmast.Image:
		return nil, m.refuse("image", m.inlineSpan(n))
	case *gmast.RawHTML:
		return nil, m.refuse("inline HTML", m.inlineSpan(n))
	default:
		return nil, m.unsupported(node)
	}
}

// label is a link's text: its plain inline content, since a link run carries
// no style of its own.
func (m *markdownMapper) label(link gmast.Node) (string, error) {
	var b strings.Builder
	for child := link.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *gmast.Text:
			b.WriteString(m.textRun(n, StylePlain).Text)
		case *gmast.String:
			b.Write(n.Value)
		default:
			return "", m.refuse("styled link text", m.inlineSpan(child))
		}
	}
	return b.String(), nil
}

// textRun is a text node as prose: backslash escapes and entities resolved,
// a line break read as one space.
func (m *markdownMapper) textRun(n *gmast.Text, style Style) Run {
	value := n.Segment.Value(m.source)
	if !n.IsRaw() {
		value = util.ResolveEntityNames(util.UnescapePunctuations(value))
	}
	textValue := string(value)
	span := Span{n.Segment.Start, n.Segment.Stop}
	if n.SoftLineBreak() || n.HardLineBreak() {
		textValue += " "
	}
	return Run{Kind: RunText, Style: style, Text: textValue, Span: span}
}

// blockSpan is the byte range a block node's lines cover.
func (m *markdownMapper) blockSpan(node gmast.Node) Span {
	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		if node.HasChildren() {
			return m.descendantSpan(node)
		}
		return m.lineAfter(m.precedingStop(node))
	}
	span := Span{lines.At(0).Start, lines.At(lines.Len() - 1).Stop}
	if node.Kind() == gmast.KindFencedCodeBlock {
		span.Start = m.lineAfter(m.precedingStop(node)).Start
	}
	return span
}

// precedingStop is the offset the body text of node begins after: the end of
// its previous sibling, else the start of its parent.
func (m *markdownMapper) precedingStop(node gmast.Node) int {
	if prev := node.PreviousSibling(); prev != nil {
		return m.blockSpan(prev).Stop
	}
	if parent := node.Parent(); parent != nil && parent.Kind() != gmast.KindDocument {
		return m.blockSpan(parent).Start
	}
	return 0
}

// lineAfter is the first non-blank line at or after offset, for a block that
// records no lines of its own (a thematic break, an empty fenced code block).
func (m *markdownMapper) lineAfter(offset int) Span {
	start := offset
	for start < len(m.body) && (m.body[start] == ' ' || m.body[start] == '\t' || m.body[start] == '\n') {
		start++
	}
	stop := strings.IndexByte(m.body[start:], '\n')
	if stop < 0 {
		stop = len(m.body)
	} else {
		stop += start
	}
	return Span{start, stop}
}

// descendantSpan is the byte range from a node's first to last descendant
// with a position of its own, for block nodes that carry no lines (lists).
func (m *markdownMapper) descendantSpan(node gmast.Node) Span {
	span := Span{Start: -1}
	_ = gmast.Walk(node, func(n gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}
		var at Span
		switch n := n.(type) {
		case *gmast.Text:
			at = Span{n.Segment.Start, n.Segment.Stop}
		case *mathNode:
			at = Span{n.Segment.Start - 1, n.Segment.Stop + 1}
		default:
			if n == node || n.Type() != gmast.TypeBlock {
				return gmast.WalkContinue, nil
			}
			lines := n.Lines()
			if lines == nil || lines.Len() == 0 {
				return gmast.WalkContinue, nil
			}
			at = Span{lines.At(0).Start, lines.At(lines.Len() - 1).Stop}
		}
		if span.Start < 0 || at.Start < span.Start {
			span.Start = at.Start
		}
		if at.Stop > span.Stop {
			span.Stop = at.Stop
		}
		return gmast.WalkContinue, nil
	})
	if span.Start < 0 {
		span.Start = 0
	}
	return span
}

// inlineSpan is the byte range an inline node's text descendants cover.
func (m *markdownMapper) inlineSpan(node gmast.Node) Span {
	if n, ok := node.(*gmast.Text); ok {
		return Span{n.Segment.Start, n.Segment.Stop}
	}
	if n, ok := node.(*gmast.RawHTML); ok && n.Segments != nil && n.Segments.Len() > 0 {
		return Span{n.Segments.At(0).Start, n.Segments.At(n.Segments.Len() - 1).Stop}
	}
	switch node.Kind() {
	case gmast.KindLink, gmast.KindImage:
		return m.linkSpan(node)
	case gmast.KindAutoLink:
		span := m.descendantSpan(node)
		if span.Start > 0 && m.body[span.Start-1] == '<' && span.Stop < len(m.body) && m.body[span.Stop] == '>' {
			span.Start--
			span.Stop++
		}
		return span
	}
	return m.descendantSpan(node)
}

// linkSpan is the byte range of a link or image from its opening bracket to
// the parenthesis closing its destination.
func (m *markdownMapper) linkSpan(node gmast.Node) Span {
	from := 0
	if prev := node.PreviousSibling(); prev != nil {
		from = m.inlineSpan(prev).Stop
	} else if parent := node.Parent(); parent != nil && parent.Type() == gmast.TypeBlock {
		from = m.blockSpan(parent).Start
	} else if parent != nil {
		from = m.inlineSpan(parent).Start
	}
	start := strings.IndexByte(m.body[from:], '[')
	if start < 0 {
		return Span{from, from}
	}
	start += from
	if node.Kind() == gmast.KindImage && start > 0 && m.body[start-1] == '!' {
		start--
	}
	stop := start + 1
	if label := m.descendantSpan(node); label.Stop > stop {
		stop = label.Stop
	}
	depth := 0
	for stop < len(m.body) {
		switch m.body[stop] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return Span{start, stop + 1}
			}
		case '\\':
			stop++
		}
		stop++
	}
	return Span{start, len(m.body)}
}

// unsupported refuses a node by the construct it is.
func (m *markdownMapper) unsupported(node gmast.Node) error {
	var span Span
	if node.Type() == gmast.TypeBlock {
		span = m.blockSpan(node)
	} else {
		span = m.inlineSpan(node)
	}
	return m.refuse(constructName(node), span)
}

// refuse is the error for a construct at span.
func (m *markdownMapper) refuse(construct string, span Span) error {
	return &UnsupportedConstructError{Construct: construct, Line: lineOf(m.body, span.Start), Span: span}
}

// constructName names a goldmark node kind as the manual does.
func constructName(node gmast.Node) string {
	switch n := node.(type) {
	case *gmast.Heading:
		return fmt.Sprintf("level %d heading", n.Level)
	case *gmast.Blockquote:
		return "block quote"
	case *gmast.CodeBlock:
		return "code block"
	case *gmast.FencedCodeBlock:
		return "fenced code block"
	case *gmast.HTMLBlock:
		return "HTML block"
	case *gmast.ThematicBreak:
		return "thematic break"
	case *gmast.Image:
		return "image"
	case *gmast.RawHTML:
		return "inline HTML"
	case *gmast.List:
		return "nested list"
	}
	return strings.ToLower(node.Kind().String())
}
