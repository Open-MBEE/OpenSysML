// Package docrender renders evaluated documents into backend-specific
// artifacts, consuming only the document IR — never plans, symbols, or ASTs.
package docrender

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// elementColumn heads the single column of a table whose query projected no
// properties, so its rows are the elements themselves.
const elementColumn = "element"

// captionMarker precedes every caption line, distinguishing a caption from a
// paragraph that is one emphasis run; both are written as *text*. The marker
// is an HTML comment, so rendered output is unaffected.
const captionMarker = "<!-- caption -->"

// Markdown renders an evaluated document as deterministic CommonMark: the
// title as a level-1 ATX heading, each section one level deeper (saturating
// at 6), paragraphs from space-joined text runs, GitHub-flavored pipe tables
// with projected column headers, bullet or numbered lists, definitions as one
// "**term** — description" paragraph per entry, and diagrams as fenced
// Mermaid blocks (table-kind views as pipe tables). Metacharacters
// in content are escaped so no value can corrupt the document structure.
func Markdown(document *docir.Document) (string, error) {
	if document == nil {
		return "", &Error{Kind: ErrorNilDocument}
	}
	var blocks []string
	blocks = append(blocks, heading(1, document.Title()))
	for _, node := range document.Content() {
		rendered, err := renderContent(node, 2)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, rendered...)
	}
	return strings.Join(blocks, "\n\n") + "\n", nil
}

// renderContent renders one content node, with level the ATX heading level a
// section at this depth writes. A node a reference targets is preceded by an
// HTML anchor carrying its stable identifier.
func renderContent(node docir.Content, level int) ([]string, error) {
	blocks, err := renderNode(node, level)
	if err != nil {
		return nil, err
	}
	if node.Anchor() != "" {
		blocks = append([]string{`<a id="` + node.Anchor() + `"></a>`}, blocks...)
	}
	return blocks, nil
}

func renderNode(node docir.Content, level int) ([]string, error) {
	switch node.Kind() {
	case docir.ContentSection:
		blocks := []string{heading(level, node.Title())}
		for _, child := range node.Children() {
			rendered, err := renderContent(child, level+1)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, rendered...)
		}
		return blocks, nil
	case docir.ContentParagraph:
		return []string{blockText(node.Runs())}, nil
	case docir.ContentTable:
		return renderTable(node), nil
	case docir.ContentList:
		return renderList(node), nil
	case docir.ContentDefinitions:
		return renderDefinitions(node), nil
	case docir.ContentDiagram:
		return renderDiagram(node)
	default:
		return nil, &Error{Kind: ErrorUnknownContent, Content: node.Name(), Actual: string(node.Kind())}
	}
}

// heading writes one ATX heading, saturating at level 6.
func heading(level int, title string) string {
	if level > 6 {
		level = 6
	}
	return strings.Repeat("#", level) + " " + inline(title)
}

// renderTable writes one pipe table, preceded by its marked caption in
// emphasis. A query without projected columns gets a single "element"
// column, and a table without rows still writes its header and delimiter.
// A grouped table writes one subtable per group, each preceded by its group key in strong
// emphasis; the group column keeps its place in every subtable.
func renderTable(node docir.Content) []string {
	var blocks []string
	if node.Caption() != "" {
		blocks = append(blocks, captionMarker+"\n*"+inline(node.Caption())+"*")
	}
	columns := node.Columns()
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name())
	}
	if len(names) == 0 {
		names = []string{elementColumn}
	}
	if node.GroupBy() != "" {
		for _, group := range node.Groups() {
			blocks = append(blocks, "**"+inline(node.GroupBy()+": "+group.Key())+"**")
			blocks = append(blocks, pipeTable(names, group.Rows(), len(columns)))
		}
		if len(node.Groups()) == 0 {
			blocks = append(blocks, pipeTable(names, nil, len(columns)))
		}
		return blocks
	}
	return append(blocks, pipeTable(names, node.Rows(), len(columns)))
}

// pipeTable writes one pipe table: header, delimiter, and one line per row.
func pipeTable(names []string, rows []queryexec.Row, columns int) string {
	var b strings.Builder
	writeTableRow(&b, names)
	b.WriteString("|" + strings.Repeat(" --- |", len(names)) + "\n")
	for _, row := range rows {
		writeTableRow(&b, tableCells(row, columns))
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderDiagram writes one diagram, preceded by its marked caption in
// emphasis: a table-kind view as a pipe table, every other supported kind
// as a fenced block of its diagram source drawn in the diagram's direction.
func renderDiagram(node docir.Content) ([]string, error) {
	return diagramBlocks(node.Name(), node.Caption(), node.Rendering(), node.Direction(), node.Form())
}

// diagramBlocks writes a marked caption in emphasis, then the rendering
// itself: a table-kind view as a pipe table, every other kind as a fence
// whose info string names the form (Mermaid unless another is stated).
func diagramBlocks(name, caption string, rendering *view.Rendering, direction view.Direction, form view.Form) ([]string, error) {
	if rendering == nil {
		return nil, &Error{Kind: ErrorMissingRendering, Content: name}
	}
	var blocks []string
	if caption != "" {
		blocks = append(blocks, captionMarker+"\n*"+inline(caption)+"*")
	}
	if rendering.Kind == view.KindTable {
		return append(blocks, strings.TrimRight(rendering.MarkdownCells(tableCell), "\n")), nil
	}
	if !rendering.Kind.Supported() {
		return nil, &Error{Kind: ErrorUnrenderableDiagram, Content: name, Actual: string(rendering.Kind)}
	}
	form, source, err := diagramSource(name, rendering, direction, form)
	if err != nil {
		return nil, err
	}
	return append(blocks, "```"+string(form)+"\n"+source+"\n```"), nil
}

// diagramSource writes a graph-shaped rendering in the stated diagram form,
// Mermaid when none is stated, returning the form written and its source.
func diagramSource(name string, rendering *view.Rendering, direction view.Direction, form view.Form) (view.Form, string, error) {
	switch form {
	case "", view.FormMermaid:
		return view.FormMermaid, strings.TrimRight(rendering.MermaidDirected(direction), "\n"), nil
	case view.FormDot:
		dot, err := rendering.DOTDirected(direction)
		if err != nil {
			return "", "", &Error{Kind: ErrorUnrenderableForm, Content: name, Actual: string(form), Expected: string(rendering.Kind)}
		}
		return view.FormDot, strings.TrimRight(dot, "\n"), nil
	}
	return "", "", &Error{Kind: ErrorUnknownForm, Content: name, Actual: string(form)}
}

// tableCells renders one row's cells, padded or truncated to the column count.
// A row of a table without projected columns is its element alone.
func tableCells(row queryexec.Row, columns int) []string {
	if columns == 0 {
		return []string{valueText(row.Element())}
	}
	cells := row.Cells()
	out := make([]string, columns)
	for i := 0; i < columns; i++ {
		if i < len(cells) {
			out[i] = cellText(cells[i])
		}
	}
	return out
}

// cellText renders one projected cell: its values joined by ", ".
func cellText(cell queryexec.Cell) string {
	values := cell.Values()
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = valueText(value)
	}
	return strings.Join(parts, ", ")
}

func writeTableRow(b *strings.Builder, cells []string) {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		escaped[i] = tableCell(cell)
	}
	b.WriteString("| " + strings.Join(escaped, " | ") + " |\n")
}

// renderList writes one bullet or numbered list, one item per query row. An
// empty list renders as nothing, which is the valid Markdown for no items.
func renderList(node docir.Content) []string {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(items))
	for i, item := range items {
		marker := "-"
		if node.Style() == docir.ListNumber {
			marker = strconv.Itoa(i+1) + "."
		}
		lines = append(lines, marker+" "+itemText(item.Runs()))
	}
	return []string{strings.Join(lines, "\n")}
}

// definitionSeparator joins an entry's term to its description.
const definitionSeparator = " — "

// renderDefinitions writes one paragraph per entry: the term in strong
// emphasis, an em dash, then the description. An entry lacking one side
// writes the other alone; one lacking both, like an empty block, writes nothing.
func renderDefinitions(node docir.Content) []string {
	var blocks []string
	for _, entry := range node.Definitions() {
		term := strings.TrimSpace(strongText(entry.Term()))
		description := strings.TrimSpace(itemText(entry.Description()))
		switch {
		case term == "" && description == "":
			continue
		case term == "":
			blocks = append(blocks, blockStart(description))
		case description == "":
			blocks = append(blocks, term)
		default:
			blocks = append(blocks, term+definitionSeparator+description)
		}
	}
	return blocks
}

// strongText renders runs joined by single spaces inside one strong span.
func strongText(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = run.Text()
	}
	return delimited("**", strings.Join(parts, " "))
}

// blockText renders a paragraph's runs joined by single spaces, escaped so the
// first character cannot open a heading, list, or quote.
func blockText(runs []docir.TextRun) string {
	return blockStart(itemText(runs))
}

// itemText joins text runs by single spaces, rendering each by its kind:
// plain runs as escaped prose, styled runs in emphasis or strong delimiters
// or as code spans, links and references as inline links.
func itemText(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = runText(run)
	}
	return strings.Join(parts, " ")
}

func runText(run docir.TextRun) string {
	switch run.Kind() {
	case docir.RunEmphasis:
		return delimited("*", run.Text())
	case docir.RunStrong:
		return delimited("**", run.Text())
	case docir.RunCode:
		return codeSpan(run.Text())
	case docir.RunLink:
		return "[" + inline(run.Text()) + "](<" + destination(run.Target()) + ">)"
	case docir.RunRef:
		return "[" + inline(run.Text()) + "](" + refDestination(run) + ")"
	default:
		return inline(run.Text())
	}
}

// refDestination maps a reference run to its Markdown destination: an
// in-document anchor, or a relative link into another document's file.
func refDestination(run docir.TextRun) string {
	if run.TargetDocument() == "" {
		return "#" + run.Target()
	}
	destination := DocumentFileName(run.TargetDocument())
	if run.Target() != "" {
		destination += "#" + run.Target()
	}
	return destination
}

// DocumentFileName derives the deterministic Markdown file name of a rendered
// document from its fully-qualified name, using the same escaping as anchors
// so distinct documents never collide.
func DocumentFileName(fqn string) string {
	return documentFileName(fqn, ".md")
}

// documentFileName derives a rendered document's file name in one backend's
// extension, escaped as anchors are.
func documentFileName(fqn, extension string) string {
	return docir.AnchorFor(strings.Split(fqn, "::")) + extension
}

// delimited wraps escaped text in emphasis delimiters, keeping leading and
// trailing whitespace outside them so the delimiters stay flanking.
func delimited(marker, text string) string {
	escaped := inline(text)
	trimmed := strings.TrimSpace(escaped)
	if trimmed == "" {
		return escaped
	}
	left := escaped[:strings.Index(escaped, trimmed)]
	right := escaped[len(left)+len(trimmed):]
	return left + marker + trimmed + marker + right
}

// codeSpan wraps text in a backtick fence longer than any backtick sequence
// inside it, padding with spaces when the content starts or ends with a
// backtick or a space. Newlines fold to spaces as everywhere in prose.
func codeSpan(text string) string {
	text = strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " ")
	longest, current := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	if text == "" || strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") ||
		strings.HasPrefix(text, " ") || strings.HasSuffix(text, " ") {
		return fence + " " + text + " " + fence
	}
	return fence + text + fence
}

// destinationEscaper escapes the characters that would end or corrupt a
// pointy-bracket link destination.
var destinationEscaper = strings.NewReplacer(
	`\`, `\\`,
	"<", `\<`,
	">", `\>`,
)

// destination escapes a link destination for the pointy-bracket form, which
// admits any character except unescaped angle brackets and newlines; newlines
// percent-encode since a backslash cannot escape them.
func destination(target string) string {
	return strings.ReplaceAll(destinationEscaper.Replace(newlineNormalizer.Replace(target)), "\n", "%0A")
}

// valueText renders one typed value as plain, unescaped text: elements by
// qualified name (falling back to declared name), strings as their text,
// integers in base 10, reals in shortest 'g' form, booleans, infinity as "*",
// and quantities as their magnitude in the unit written: `2290000 [kg]`.
func valueText(value queryexec.Value) string {
	if element, ok := value.Element(); ok {
		if fqn := symbols.FQNOf(element); fqn != "" {
			return fqn
		}
		return element.Name
	}
	if text, ok := value.String(); ok {
		return text
	}
	if integer, ok := value.Integer(); ok {
		return strconv.FormatInt(integer, 10)
	}
	if realVal, ok := value.Real(); ok {
		return strconv.FormatFloat(realVal, 'g', -1, 64)
	}
	if boolean, ok := value.Boolean(); ok {
		return strconv.FormatBool(boolean)
	}
	if value.Kind() == queryexec.ValueInfinity {
		return "*"
	}
	if quantity, ok := value.Quantity(); ok {
		magnitude, _ := value.Magnitude()
		return quantity.TextWithMagnitude(valueText(magnitude))
	}
	return ""
}

// inlineEscaper backslash-escapes the characters that open Markdown or HTML
// structure anywhere in a line.
var inlineEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"*", `\*`,
	"_", `\_`,
	"[", `\[`,
	"]", `\]`,
	"<", `\<`,
	"&", `\&`,
	"|", `\|`,
	"#", `\#`,
)

// newlineNormalizer folds CRLF and lone CR to LF, so a carriage return cannot
// end a Markdown line either.
var newlineNormalizer = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// inline escapes prose for any position in a line, folding newlines to spaces
// since paragraph structure comes from the document, not from run content.
func inline(text string) string {
	return inlineEscaper.Replace(strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " "))
}

// tableCell escapes one table cell, folding newlines to <br> so they cannot
// end the row.
func tableCell(text string) string {
	return strings.ReplaceAll(inlineEscaper.Replace(newlineNormalizer.Replace(text)), "\n", "<br>")
}

// blockStart escapes a leading quote, bullet, or ordered-list marker that
// would open block structure; inline escaping already covered "#". Leading
// spaces and tabs are dropped first: they would open an indented code block
// or shelter a marker, and CommonMark collapses them in a paragraph anyway.
func blockStart(text string) string {
	text = strings.TrimLeft(text, " \t")
	if text == "" {
		return text
	}
	switch text[0] {
	case '>', '-', '+':
		return `\` + text
	}
	digits := 0
	for digits < len(text) && text[digits] >= '0' && text[digits] <= '9' {
		digits++
	}
	if digits > 0 && digits < len(text) && (text[digits] == '.' || text[digits] == ')') {
		return text[:digits] + `\` + text[digits:]
	}
	return text
}
