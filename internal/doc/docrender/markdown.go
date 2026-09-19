// Package docrender renders evaluated documents into backend-specific
// artifacts, consuming only the document IR — never plans, symbols, or ASTs.
package docrender

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// elementColumn heads the single column of a table whose query projected no
// properties, so its rows are the elements themselves.
const elementColumn = "element"

// MarkdownOptions are the presentation choices of the Markdown backend. They
// are options of this backend, never document-model attributes.
type MarkdownOptions struct {
	// DiagramForm is the source every graph-shaped diagram is written as,
	// Mermaid when empty; a table-kind view is a pipe table whichever it is.
	DiagramForm view.Form
}

// Markdown renders an evaluated document as deterministic CommonMark: the
// title as a level-1 ATX heading, each section one level deeper (saturating
// at 6), paragraphs from space-joined text runs, GitHub-flavored pipe tables
// with projected column headers, bullet or numbered lists, definitions as one
// "**term** — description" paragraph per entry, formulas as $$-fenced
// display math with inline math in $…$, and diagrams as fenced blocks of
// their source in the chosen diagram form (table-kind views as pipe tables).
// Metacharacters in content are escaped so no value can corrupt the document
// structure; LaTeX is written verbatim, since math is not prose.
func Markdown(document *docir.Document, opts MarkdownOptions) (string, error) {
	if document == nil {
		return "", &Error{Kind: ErrorNilDocument}
	}
	form, err := diagramForm(opts.DiagramForm)
	if err != nil {
		return "", err
	}
	w := &markdownWriter{form: form}
	var blocks []string
	blocks = append(blocks, heading(1, document.Title()))
	for _, node := range document.Content() {
		rendered, err := w.renderContent(node, 2)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, rendered...)
	}
	return strings.Join(blocks, "\n\n") + "\n", nil
}

// diagramForm resolves the diagram form a render asks for: Mermaid when none
// is named, otherwise one a diagram is written as.
func diagramForm(form view.Form) (view.Form, error) {
	if form == "" {
		return view.FormMermaid, nil
	}
	if !slices.Contains(view.DiagramForms(), form) {
		return "", &Error{Kind: ErrorUnknownForm, DiagramForm: form}
	}
	return form, nil
}

// markdownWriter carries the choices one Markdown render applies to every
// node it writes.
type markdownWriter struct {
	form view.Form
}

// renderContent renders one content node, with level the ATX heading level a
// section at this depth writes. A node a reference targets is preceded by an
// HTML anchor carrying its stable identifier.
func (w *markdownWriter) renderContent(node docir.Content, level int) ([]string, error) {
	blocks, err := w.renderNode(node, level)
	if err != nil {
		return nil, err
	}
	if node.Anchor() != "" {
		blocks = append([]string{`<a id="` + node.Anchor() + `"></a>`}, blocks...)
	}
	return blocks, nil
}

func (w *markdownWriter) renderNode(node docir.Content, level int) ([]string, error) {
	switch node.Kind() {
	case docir.ContentSection:
		blocks := []string{heading(level, node.Title())}
		for _, child := range node.Children() {
			rendered, err := w.renderContent(child, level+1)
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
	case docir.ContentFormula:
		return renderFormula(node), nil
	case docir.ContentDiagram:
		return diagramBlocks(node.Name(), node.Caption(), node.Rendering(), node.Options(), w.form)
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

// renderTable writes one pipe table, preceded by its caption in emphasis. A
// query without projected columns gets a single "element" column, and a table
// without rows still writes its header and delimiter.
// A grouped table writes one subtable per group, each preceded by its group key in strong
// emphasis; the group column keeps its place in every subtable.
func renderTable(node docir.Content) []string {
	blocks := captionBlock(node.Caption())
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
			blocks = append(blocks, delimited("**", node.GroupBy()+": "+group.Key()))
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

// diagramBlocks writes one diagram under its caption in emphasis: a table-kind view
// as a pipe table, every other kind as a fence in the render's diagram form.
func diagramBlocks(name, caption string, rendering *view.Rendering, options view.Options, form view.Form) ([]string, error) {
	if rendering == nil {
		return nil, &Error{Kind: ErrorMissingRendering, Content: name}
	}
	blocks := captionBlock(caption)
	if rendering.Kind == view.KindTable {
		return append(blocks, strings.TrimRight(rendering.MarkdownCells(tableCell), "\n")), nil
	}
	if !rendering.Kind.Supported() {
		return nil, &Error{Kind: ErrorUnrenderableDiagram, Content: name, Actual: string(rendering.Kind)}
	}
	source, err := diagramSource(name, rendering, options, form)
	if err != nil {
		return nil, err
	}
	return append(blocks, "```"+string(form)+"\n"+source+"\n```"), nil
}

// diagramSource writes a graph-shaped rendering in the resolved diagram form
// with the diagram's direction and palette, without its trailing newline.
func diagramSource(name string, rendering *view.Rendering, options view.Options, form view.Form) (string, error) {
	if !rendering.Kind.SupportsForm(form) {
		return "", &Error{Kind: ErrorUnrenderableForm, Content: name, Actual: string(rendering.Kind), DiagramForm: form}
	}
	switch form {
	case view.FormMermaid:
		return strings.TrimRight(rendering.MermaidWith(options), "\n"), nil
	case view.FormDot:
		dot, err := rendering.DOTWith(options)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(dot, "\n"), nil
	case view.FormPlantUML:
		puml, err := rendering.PlantUMLWith(options)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(puml, "\n"), nil
	}
	return "", &Error{Kind: ErrorUnknownForm, DiagramForm: form}
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

// mathFence opens and closes a display-math block on lines of its own.
const mathFence = "$$"

// renderFormula writes one display-math block under its caption in emphasis:
// the LaTeX source between $$ fences, one source line per line.
func renderFormula(node docir.Content) []string {
	blocks := captionBlock(node.Caption())
	return append(blocks, mathFence+"\n"+displayMath(node.Source())+"\n"+mathFence)
}

// displayMath prepares LaTeX for a $$ block: lines keep their breaks, blank
// lines (which would end the block) are dropped, and bare dollars are
// escaped so none can close the block early.
func displayMath(source string) string {
	var lines []string
	for _, line := range strings.Split(newlineNormalizer.Replace(source), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, escapeDollars(line))
		}
	}
	return strings.Join(lines, "\n")
}

// mathSpan writes inline LaTeX between single dollars: whitespace trimmed and
// newlines folded so the delimiters hug non-space characters, as the
// dollar-math convention requires, and bare dollars escaped.
func mathSpan(source string) string {
	return "$" + inlineMath(source) + "$"
}

// inlineMath prepares LaTeX for a dollar span: trimmed, newlines folded, bare
// dollars escaped, and a trailing backslash doubled so it cannot escape the
// closing dollar.
func inlineMath(source string) string {
	source = strings.TrimSpace(strings.ReplaceAll(newlineNormalizer.Replace(source), "\n", " "))
	escaped := escapeDollars(source)
	if trailingBackslashes(escaped)%2 == 1 {
		escaped += `\`
	}
	return escaped
}

// trailingBackslashes counts the backslashes ending text; an odd count would
// escape whatever follows.
func trailingBackslashes(text string) int {
	n := 0
	for n < len(text) && text[len(text)-1-n] == '\\' {
		n++
	}
	return n
}

// escapeDollars backslash-escapes every dollar in LaTeX not already escaped,
// leaving other backslash sequences alone: \$ is LaTeX for a literal dollar,
// so the source keeps its meaning while no dollar can end the math.
func escapeDollars(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			b.WriteByte('\\')
			if i+1 < len(source) {
				i++
				b.WriteByte(source[i])
			}
		case '$':
			b.WriteString(`\$`)
		default:
			b.WriteByte(source[i])
		}
	}
	return b.String()
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
// or as code spans, math runs as dollar math, links and references as inline
// links.
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
	case docir.RunMath:
		return mathSpan(run.Text())
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

// captionBlock writes a caption as an emphasized paragraph without its surrounding
// blanks, which at block start would read as indentation; a blank caption writes nothing.
func captionBlock(caption string) []string {
	caption = strings.TrimSpace(caption)
	if caption == "" {
		return nil
	}
	return []string{"*" + inline(caption) + "*"}
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
// qualified name (falling back to declared name), objects by the label the
// session reaches them by (`car.wheels[2]`), verdicts, states and events by
// their summary, strings as their text, integers in base 10, reals in shortest
// 'g' form, booleans, infinity as "*", and quantities as their magnitude in
// the unit written: `2290000 [kg]`.
func valueText(value queryexec.Value) string {
	if element, ok := value.Element(); ok {
		if fqn := symbols.FQNOf(element); fqn != "" {
			return fqn
		}
		return element.Name
	}
	if _, label, ok := value.Object(); ok {
		return label
	}
	if verdict, ok := value.Verdict(); ok {
		return verdict.Summary()
	}
	if state, ok := value.State(); ok {
		return state.Label()
	}
	if event, ok := value.Event(); ok {
		return event.Summary()
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
// structure anywhere in a line, dollar math included.
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
	"$", `\$`,
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
