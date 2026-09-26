// Package docrender renders evaluated documents into backend-specific
// artifacts, consuming only the document IR — never plans, symbols, or ASTs.
package docrender

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/filename"
)

// elementColumn heads the single column of a table whose query projected no
// properties, so its rows are the elements themselves.
const elementColumn = "element"

// MarkdownOptions are the presentation choices of the Markdown backend. They
// are options of this backend, never document-model attributes.
type MarkdownOptions struct {
	// DiagramForm is the source every graph-shaped diagram is written as; a
	// table-kind view is a pipe table whichever it is. Empty picks per diagram:
	// DOT for a rendering a Layout or Route positions, Mermaid otherwise.
	DiagramForm view.Form

	// WithoutGraphviz records that no Graphviz draws DOT for this render, so
	// the automatic choice writes Mermaid for positioned diagrams too, with an
	// emphasised notice stating so. An explicit DiagramForm is written regardless.
	WithoutGraphviz bool

	// Unplaced is where a diagram some Layout positions puts the nodes none
	// does: left undrawn when empty, or drawn too (a strip below a DOT drawing).
	Unplaced view.Unplaced

	// Style is the drawing style every DOT diagram is drawn in, the Pilot look
	// when empty; the other forms draw one look.
	Style view.DrawingStyle

	// Files is the file each document of the set this one is rendered in is
	// written to, by qualified name; a cross-document reference links to the
	// target's file here, or to DocumentFileName of its name when absent.
	Files map[string]string

	// DiagramSVG is SVG markup drawn ahead of the render, one per graph-shaped
	// diagram in the order Diagrams lists them, each written as an HTML block
	// in place of its fenced source; an empty entry, or none, keeps the fence.
	// More entries than diagrams is an error.
	DiagramSVG []string

	// Drawer draws the DOT diagrams the automatic choice picks, filling
	// DiagramSVG when that is empty; one that is not available settles the
	// choice on the Mermaid fallback. Nil leaves the choice to WithoutGraphviz.
	Drawer DiagramDrawer

	// OutputDir is the directory the document is written into, when known, so an
	// image's source-relative location is written relative to it; empty writes it as stated.
	OutputDir string

	// NumberFigures captions figures (drawn diagrams, images) "Figure N. …" and
	// tables (query tables, table-kind diagrams) "Table N. …" in document order.
	NumberFigures bool
}

// diagramOptions is the part of the options the diagrams are written by.
func (o MarkdownOptions) diagramOptions() DiagramOptions {
	return DiagramOptions{Form: o.DiagramForm, WithoutGraphviz: o.WithoutGraphviz, Unplaced: o.Unplaced, Style: o.Style}
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
	diagrams := opts.diagramOptions()
	if err := diagrams.check(); err != nil {
		return "", err
	}
	if err := drawAutomatic(document, &diagrams, opts.Drawer, &opts.DiagramSVG); err != nil {
		return "", err
	}
	w := &markdownWriter{
		opts: diagrams, files: opts.Files, svg: opts.DiagramSVG, outputDir: opts.OutputDir,
		numbers: captionNumbering{on: opts.NumberFigures},
	}
	var blocks []string
	blocks = append(blocks, heading(1, document.Title()))
	for _, node := range document.Content() {
		rendered, err := w.renderContent(node, 2)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, rendered...)
	}
	if w.diagrams < len(opts.DiagramSVG) {
		return "", &Error{Kind: ErrorSurplusDiagramImages, Actual: strconv.Itoa(len(opts.DiagramSVG)), Count: w.diagrams}
	}
	return strings.Join(blocks, "\n\n") + "\n", nil
}

// checkStyle rejects a drawing style there is none of; empty is the Pilot look.
func checkStyle(style view.DrawingStyle) error {
	if _, ok := view.ParseDrawingStyle(string(style)); !ok {
		return &view.UnknownDrawingStyleError{Name: string(style)}
	}
	return nil
}

// markdownWriter carries the choices one Markdown render applies to every
// node it writes.
type markdownWriter struct {
	opts      DiagramOptions
	files     map[string]string
	svg       []string
	diagrams  int
	outputDir string
	numbers   captionNumbering
}

// figureOptions is what a diagram's rendering is written with: its stated
// direction and palette, and the render's placement of unplaced nodes and
// drawing style.
func figureOptions(node docir.Content, opts DiagramOptions) view.Options {
	options := node.Options()
	options.Unplaced = opts.Unplaced
	options.Style = opts.Style
	return options
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
		return []string{w.blockText(node.Runs())}, nil
	case docir.ContentTable:
		return renderTable(node, w.numbers.caption(node).String()), nil
	case docir.ContentList:
		return w.renderList(node), nil
	case docir.ContentDefinitions:
		return w.renderDefinitions(node), nil
	case docir.ContentFormula:
		return renderFormula(node), nil
	case docir.ContentDiagram:
		return w.diagramFigure(node.Name(), w.numbers.caption(node).String(), node.Rendering(), figureOptions(node, w.opts))
	case docir.ContentImage:
		return renderImage(node, w.numbers.caption(node).String(), w.outputDir), nil
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
func renderTable(node docir.Content, caption string) []string {
	blocks := captionBlock(caption)
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

// diagramFigure is the blocks of one diagram: its caption, then a table-kind
// view's pipe table, or the fallback notice when the automatic choice did not
// draw the view as stated and the SVG drawn for it or its source fenced.
func (w *markdownWriter) diagramFigure(name, caption string, rendering *view.Rendering, options view.Options) ([]string, error) {
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
	form, fallback := w.opts.formFor(rendering)
	source, err := diagramSource(name, rendering, options, form)
	if err != nil {
		return nil, err
	}
	if fallback != "" {
		blocks = append(blocks, delimited("*", fallback))
	}
	if w.diagrams < len(w.svg) && w.svg[w.diagrams] != "" {
		blocks = append(blocks, "<figure class=\"sysml-diagram\">\n"+svgBlock(w.svg[w.diagrams])+"\n</figure>")
	} else {
		blocks = append(blocks, "```"+string(form)+"\n"+source+"\n```")
	}
	w.diagrams++
	return blocks, nil
}

// svgBlock is svg markup as one CommonMark HTML block holds it: without the
// XML prolog and doctype a drawing tool writes, and without blank lines,
// which would end the block.
func svgBlock(svg string) string {
	if i := strings.Index(svg, "<svg"); i > 0 {
		svg = svg[i:]
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(svg), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// diagramSource writes a graph-shaped rendering in the resolved diagram form
// with the diagram's direction, palette and placement of unplaced nodes,
// without its trailing newline. A Mermaid chart past the size a chart is drawn
// under is refused, not written.
func diagramSource(name string, rendering *view.Rendering, options view.Options, form view.Form) (string, error) {
	if !rendering.Kind.SupportsForm(form) {
		return "", &Error{Kind: ErrorUnrenderableForm, Content: name, Actual: string(rendering.Kind), DiagramForm: form}
	}
	switch form {
	case view.FormMermaid:
		source := strings.TrimRight(rendering.MermaidWith(options), "\n")
		if !view.MermaidFits(source) {
			textSize, edges := view.MermaidSize(source)
			return "", &Error{Kind: ErrorOversizedDiagram, Content: name, DiagramForm: form, TextSize: textSize, Edges: edges}
		}
		return source, nil
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
func (w *markdownWriter) renderList(node docir.Content) []string {
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
		lines = append(lines, marker+" "+w.itemText(item.Runs()))
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

// renderImage writes one image under its caption in emphasis, as Markdown's
// own image syntax, its location as a document in outputDir refers to it
// (see MarkdownOptions.OutputDir); alt defaults to the stated caption.
func renderImage(node docir.Content, caption, outputDir string) []string {
	blocks := captionBlock(caption)
	alt := node.Alt()
	if alt == "" {
		alt = node.Caption()
	}
	location := imageOf(node).Source(outputDir)
	location = strings.NewReplacer("<", "%3C", ">", "%3E", "(", "%28", ")", "%29", "\n", "%0A", "\r", "%0D").Replace(location)
	if strings.Contains(location, " ") {
		location = "<" + location + ">"
	}
	return append(blocks, "!["+inline(alt)+"]("+location+")")
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
func (w *markdownWriter) renderDefinitions(node docir.Content) []string {
	var blocks []string
	for _, entry := range node.Definitions() {
		term := strings.TrimSpace(strongText(entry.Term()))
		description := strings.TrimSpace(w.itemText(entry.Description()))
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
func (w *markdownWriter) blockText(runs []docir.TextRun) string {
	return blockStart(w.itemText(runs))
}

// itemText joins text runs by single spaces, rendering each by its kind:
// plain runs as escaped prose, styled runs in emphasis or strong delimiters
// or as code spans, math runs as dollar math, links and references as inline
// links.
func (w *markdownWriter) itemText(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = w.runText(run)
	}
	return strings.Join(parts, " ")
}

func (w *markdownWriter) runText(run docir.TextRun) string {
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
		return "[" + inline(run.Text()) + "](" + w.refDestination(run) + ")"
	default:
		return inline(run.Text())
	}
}

// refDestination maps a reference run to its Markdown destination: an
// in-document anchor, or a relative link into another document's file.
func (w *markdownWriter) refDestination(run docir.TextRun) string {
	return refDestination(run, w.files, DocumentFileName)
}

// refDestination is a reference run's destination: its anchor within the
// document, or the target document's file, the set's or the default, and anchor.
func refDestination(run docir.TextRun, files map[string]string, defaultFile func(string) string) string {
	if run.TargetDocument() == "" {
		return "#" + run.Target()
	}
	destination, ok := files[run.TargetDocument()]
	if !ok {
		destination = defaultFile(run.TargetDocument())
	}
	if run.Target() != "" {
		destination += "#" + run.Target()
	}
	return destination
}

// DocumentFileName is the Markdown file a document is written to on its own,
// DocumentFileStem of its qualified name plus `.md`.
func DocumentFileName(fqn string) string {
	return DocumentFileStem(fqn) + ".md"
}

// DocumentFileStem is the file name, extension aside, a document derives from
// its qualified name: `::` as `-` and every byte outside ASCII letters, digits
// and `_` as `.XX`, the encoding of anchors. A stem naming a Windows device
// has its first byte encoded too, and one opening with `.` is prefixed `_`, so
// the file is neither a device nor hidden. A set fits and tags the stems it writes.
func DocumentFileStem(fqn string) string {
	stem := docir.AnchorFor(strings.Split(fqn, "::"))
	if filename.DeviceStem(stem) {
		stem = fmt.Sprintf(".%02X", stem[0]) + stem[1:]
	}
	if strings.HasPrefix(stem, ".") {
		stem = "_" + stem
	}
	return stem
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
