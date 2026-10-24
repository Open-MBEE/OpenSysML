package docrender

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// AsciiDocOptions shape AsciiDoc output; each field means what it does for
// Markdown (see MarkdownOptions), the two backends writing the same document
// in two markups.
type AsciiDocOptions MarkdownOptions

// asciiDocForm names the backend in the errors it shares with the others.
const asciiDocForm = "AsciiDoc"

// AsciiDoc renders an evaluated document as AsciiDoc: the title as the
// document title, sections as section titles by depth, and every other
// content kind as the block AsciiDoc has for it, under its caption as a block
// title. Diagrams are drawn as Markdown draws them (see Markdown); a document
// with math declares `:stem: latexmath` in its header so stem macros and
// blocks read as LaTeX.
func AsciiDoc(document *docir.Document, opts AsciiDocOptions) (string, error) {
	if document == nil {
		return "", &Error{Kind: ErrorNilDocument}
	}
	if err := checkTableColumns(opts.TableColumns, asciiDocForm); err != nil {
		return "", err
	}
	diagrams := MarkdownOptions(opts).diagramOptions()
	if err := diagrams.check(); err != nil {
		return "", err
	}
	if err := drawAutomatic(document, &diagrams, opts.Drawer, &opts.DiagramSVG); err != nil {
		return "", err
	}
	w := &asciiDocWriter{
		opts: diagrams, files: opts.Files, svg: opts.DiagramSVG, outputDir: opts.OutputDir,
		numbers: captionNumbering{on: opts.NumberFigures}, tableColumns: opts.TableColumns, tableMeasure: opts.TableMeasure,
		names: document.ElementName, labels: refLabels(document, false, opts.NumberFigures),
	}
	var blocks []string
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
	header := "= " + asciiDocInline(document.Title())
	if w.math {
		header += "\n:stem: latexmath"
	}
	if opts.NumberFigures {
		header += "\n:table-caption!:\n:figure-caption!:"
	}
	return strings.Join(append([]string{header}, blocks...), "\n\n") + "\n", nil
}

// asciiDocWriter carries the state of one AsciiDoc rendering: the diagram
// options, the set's files for cross-document references, the SVGs drawn for
// the graph-shaped diagrams in order, caption numbering, and whether any
// stem macro or block was written.
type asciiDocWriter struct {
	opts                       DiagramOptions
	files                      map[string]string
	svg                        []string
	outputDir                  string
	numbers                    captionNumbering
	tableColumns, tableMeasure int
	names                      namer
	labels                     map[string]string
	diagrams                   int
	math                       bool
}

// renderContent renders one content node, with level the number of `=` a
// section at this depth writes. A node a reference targets carries its
// stable identifier as a block anchor on its first block; a node writing no
// block carries it as an inline anchor of its own.
func (w *asciiDocWriter) renderContent(node docir.Content, level int) ([]string, error) {
	blocks, err := w.renderNode(node, level)
	if err != nil {
		return nil, err
	}
	switch {
	case node.Anchor() == "":
	case len(blocks) == 0:
		blocks = []string{"anchor:" + node.Anchor() + "[]"}
	default:
		blocks[0] = "[[" + node.Anchor() + "]]\n" + blocks[0]
	}
	return blocks, nil
}

func (w *asciiDocWriter) renderNode(node docir.Content, level int) ([]string, error) {
	switch node.Kind() {
	case docir.ContentSection:
		blocks := []string{asciiDocHeading(level, node.Title())}
		for _, child := range node.Children() {
			rendered, err := w.renderContent(child, level+1)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, rendered...)
		}
		return blocks, nil
	case docir.ContentParagraph:
		return []string{asciiDocBlockStart(w.itemText(node.Runs()))}, nil
	case docir.ContentTable:
		return w.renderTable(node, w.numbers.caption(node).String()), nil
	case docir.ContentList:
		return w.renderList(node), nil
	case docir.ContentDefinitions:
		return w.renderDefinitions(node), nil
	case docir.ContentFormula:
		w.math = true
		return renderAsciiDocFormula(node), nil
	case docir.ContentDiagram:
		return w.diagramFigure(node.Name(), w.numbers.caption(node).String(), node.Rendering(), figureOptions(node, w.opts))
	case docir.ContentImage:
		return renderAsciiDocImage(node, w.numbers.caption(node).String(), w.outputDir), nil
	default:
		return nil, &Error{Kind: ErrorUnknownContent, Content: node.Name(), Actual: string(node.Kind())}
	}
}

// asciiDocHeading writes one section title, saturating at level 6: the
// document title holds level 1, so the top sections write `==`.
func asciiDocHeading(level int, title string) string {
	if level > 6 {
		level = 6
	}
	return strings.Repeat("=", level) + " " + asciiDocInline(title)
}

// renderTable writes a table as one `|===` block under its caption as block
// title, its header row the columns' labels and its column widths the stated
// ones; a table without columns writes the row elements under an "element"
// header, and one without rows still writes its header. A table wider than
// the limit or measure is written as continuation tables each repeating the
// first column under the caption with its continued suffix, as Markdown and
// HTML do. A grouped table writes its groups into one table, each opened by a
// row spanning every column that carries the group key in strong emphasis.
func (w *asciiDocWriter) renderTable(node docir.Content, caption string) []string {
	var blocks []string
	columns := node.Columns()
	for i, part := range tableParts(columns, w.tableColumns, w.tableMeasure).parts {
		title := caption
		if i > 0 && strings.TrimSpace(caption) != "" {
			title = caption + " " + continuedSuffix
		}
		blocks = append(blocks, asciiDocBlockTitle(title)+w.tablePart(node, columns, part))
	}
	return blocks
}

// tablePart writes the `|===` block over the columns at the given indexes.
func (w *asciiDocWriter) tablePart(node docir.Content, columns []queryexec.Column, indexes []int) string {
	labels := make([]string, 0, len(indexes))
	part := make([]queryexec.Column, 0, len(indexes))
	for _, i := range indexes {
		labels = append(labels, columns[i].Label())
		part = append(part, columns[i])
	}
	if len(labels) == 0 {
		labels = []string{elementColumn}
	}
	var b strings.Builder
	b.WriteString(asciiDocTableAttributes(part, w.tableMeasure) + "\n|===\n")
	writeAsciiDocRow(&b, labels, 0)
	if node.GroupBy() != "" {
		for _, group := range node.Groups() {
			key := asciiDocDelimited("*", node.GroupBy()+": "+group.Key())
			fmt.Fprintf(&b, "\n%d+| %s\n", len(labels), strings.ReplaceAll(key, "|", `\|`))
			w.writeRows(&b, group.Rows(), indexes)
		}
	} else {
		w.writeRows(&b, node.Rows(), indexes)
	}
	b.WriteString("|===")
	return b.String()
}

func (w *asciiDocWriter) writeRows(b *strings.Builder, rows []queryexec.Row, indexes []int) {
	for _, row := range rows {
		writeAsciiDocRow(b, tableCells(row, indexes, w.names), row.Depth())
	}
}

// asciiDocTableAttributes is the attribute line of a table: a header row and,
// when a column states a width, the columns' shares of the table as `cols`.
func asciiDocTableAttributes(columns []queryexec.Column, measure int) string {
	shares := columnShares(columns, measure)
	if shares == nil {
		return `[options="header"]`
	}
	percents := make([]string, len(shares))
	for i, share := range shares {
		percents[i] = strconv.Itoa(int(math.Max(1, math.Round(share*100))))
	}
	return `[cols="` + strings.Join(percents, ",") + `",options="header"]`
}

// writeAsciiDocRow writes one table row on one line, every cell opened by its
// own `|`; a nested row's depth is written into its first cell as an indent
// of non-breaking spaces ending in an arrow, as a table carries no row structure.
func writeAsciiDocRow(b *strings.Builder, cells []string, depth int64) {
	for i, cell := range cells {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString("| ")
		if i == 0 && depth > 0 {
			b.WriteString(strings.Repeat("{nbsp}{nbsp}{nbsp}{nbsp}", int(depth)-1) + "↳ ")
		}
		b.WriteString(asciiDocTableCell(cell))
	}
	b.WriteString("\n")
}

// renderList writes one bullet (`*`) or numbered (`.`) list, one item per
// query row; an empty list renders as nothing.
func (w *asciiDocWriter) renderList(node docir.Content) []string {
	items := node.Items()
	if len(items) == 0 {
		return nil
	}
	marker := "*"
	if node.Style() == docir.ListNumber {
		marker = "."
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, marker+" "+asciiDocNonEmpty(w.itemText(item.Runs())))
	}
	return []string{strings.Join(lines, "\n")}
}

// renderDefinitions writes one description list, `term:: description` per
// entry. An entry lacking its description writes the term alone, one lacking
// its term writes the description as a paragraph, and one lacking both writes
// nothing, as the Markdown backend does.
func (w *asciiDocWriter) renderDefinitions(node docir.Content) []string {
	var blocks []string
	var entries []string
	flush := func() {
		if len(entries) > 0 {
			blocks = append(blocks, strings.Join(entries, "\n"))
			entries = nil
		}
	}
	for _, entry := range node.Definitions() {
		term := strings.TrimSpace(asciiDocInline(joinRunText(entry.Term())))
		description := strings.TrimSpace(w.itemText(entry.Description()))
		switch {
		case term == "" && description == "":
		case term == "":
			flush()
			blocks = append(blocks, asciiDocBlockStart(description))
		case description == "":
			entries = append(entries, term+"::")
		default:
			entries = append(entries, term+":: "+description)
		}
	}
	flush()
	return blocks
}

// joinRunText joins runs' texts by single spaces, their kinds set aside.
func joinRunText(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = run.Text()
	}
	return strings.Join(parts, " ")
}

// renderAsciiDocFormula writes one stem block under its caption: the LaTeX
// source between passthrough delimiters longer than any run of pluses it holds.
func renderAsciiDocFormula(node docir.Content) []string {
	source := strings.TrimSpace(newlineNormalizer.Replace(node.Source()))
	return []string{asciiDocBlockTitle(node.Caption()) + "[stem]\n" + asciiDocDelimitedBlock('+', source)}
}

// asciiDocDelimitedBlock fences source between lines of the delimiter
// character, four long or one longer than the longest line of that character
// alone inside, so no source line can close the block.
func asciiDocDelimitedBlock(delimiter byte, source string) string {
	longest := 3
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimRight(line, " \t")
		if len(line) > longest && strings.Trim(line, string(delimiter)) == "" {
			longest = len(line)
		}
	}
	fence := strings.Repeat(string(delimiter), longest+1)
	return fence + "\n" + source + "\n" + fence
}

// renderAsciiDocImage writes one image block under its caption, its location
// as a document in outputDir refers to it (see MarkdownOptions.OutputDir);
// alt defaults to the stated caption.
func renderAsciiDocImage(node docir.Content, caption, outputDir string) []string {
	alt := node.Alt()
	if alt == "" {
		alt = node.Caption()
	}
	location := imageOf(node).Source(outputDir)
	return []string{asciiDocBlockTitle(caption) + "image::" + asciiDocTarget(location) + "[" + asciiDocAttribute(alt) + "]"}
}

// diagramFigure is the blocks of one diagram: a tabular view's table, or the
// fallback notice when the automatic choice did not draw the view as stated
// and the SVG drawn for it in a passthrough block or its source in a listing
// block named for the form; each under the caption as block title.
func (w *asciiDocWriter) diagramFigure(name, caption string, rendering *view.Rendering, options view.Options) ([]string, error) {
	if rendering == nil {
		return nil, &Error{Kind: ErrorMissingRendering, Content: name}
	}
	if rendering.Kind.Tabular() {
		return []string{asciiDocBlockTitle(caption) + asciiDocRenderingTable(rendering)}, nil
	}
	if !rendering.Kind.Supported() {
		return nil, &Error{Kind: ErrorUnrenderableDiagram, Content: name, Actual: string(rendering.Kind), Form: asciiDocForm}
	}
	form, fallback := w.opts.formFor(rendering)
	source, err := diagramSource(name, rendering, options, form)
	if err != nil {
		return nil, err
	}
	var blocks []string
	if fallback != "" {
		blocks = append(blocks, asciiDocDelimited("_", fallback))
	}
	figure := asciiDocBlockTitle(caption)
	if w.diagrams < len(w.svg) && w.svg[w.diagrams] != "" {
		figure += asciiDocDelimitedBlock('+', svgBlock(w.svg[w.diagrams]))
	} else {
		figure += "[" + asciiDocDiagramBlock(form, source) + "]\n" + asciiDocDelimitedBlock('.', source)
	}
	w.diagrams++
	return append(blocks, figure), nil
}

// asciiDocDiagramBlock names the Asciidoctor Diagram block that draws a
// diagram form: Graphviz draws DOT, under the engine the source's
// `// layout:` header names (its `-n` flags have no block attribute, so a
// positioned view is drawn exactly only from a pre-rendered SVG); the other
// forms go by their own names.
func asciiDocDiagramBlock(form view.Form, source string) string {
	if form != view.FormDot {
		return string(form)
	}
	first, _, _ := strings.Cut(source, "\n")
	if rest, ok := strings.CutPrefix(strings.TrimSpace(first), "// layout:"); ok {
		if fields := strings.Fields(rest); len(fields) > 0 {
			return "graphviz, layout=" + fields[0]
		}
	}
	return "graphviz"
}

// asciiDocRenderingTable writes a tabular rendering as a `|===` table under
// comments naming the rendering and what it could not represent; an empty
// rendering writes why it is empty instead of a table.
func asciiDocRenderingTable(rendering *view.Rendering) string {
	var b strings.Builder
	if rendering.View == "" {
		fmt.Fprintf(&b, "// %s rendering", rendering.Kind)
	} else {
		fmt.Fprintf(&b, "// %s — %s rendering", asciiDocComment(rendering.View), rendering.Kind)
	}
	if rendering.Stated != "" {
		fmt.Fprintf(&b, " (%s)", asciiDocComment(rendering.Stated))
	}
	b.WriteString("\n")
	for _, notice := range rendering.Notices {
		fmt.Fprintf(&b, "// not represented: %s\n", asciiDocComment(notice))
	}
	if rendering.Empty() {
		b.WriteString(asciiDocBlockStart(asciiDocInline(rendering.EmptyReason())))
		return b.String()
	}
	columns := rendering.Columns
	if len(columns) == 0 {
		columns = view.TableColumns()
	}
	b.WriteString("[options=\"header\"]\n|===\n")
	writeAsciiDocRow(&b, columns, 0)
	for _, row := range rendering.Rows {
		cells := append(append([]string(nil), row...), make([]string, max(0, len(columns)-len(row)))...)
		writeAsciiDocRow(&b, cells[:len(columns)], 0)
	}
	b.WriteString("|===")
	return b.String()
}

// asciiDocComment is text as a line comment holds it: on one line.
func asciiDocComment(text string) string {
	return strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " ")
}

// itemText joins text runs by single spaces (see joinRuns), rendering each by
// its kind: plain runs as escaped prose, styled runs in constrained emphasis,
// strong or monospace marks, math runs as stem macros, links as link macros
// and references as cross references.
func (w *asciiDocWriter) itemText(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = w.runText(run)
	}
	return joinRuns(runs, parts)
}

func (w *asciiDocWriter) runText(run docir.TextRun) string {
	switch run.Kind() {
	case docir.RunEmphasis:
		return asciiDocDelimited("_", run.Text())
	case docir.RunStrong:
		return asciiDocDelimited("*", run.Text())
	case docir.RunCode:
		return asciiDocDelimited("`", asciiDocNonEmpty(run.Text()))
	case docir.RunMath:
		w.math = true
		return "stem:[" + asciiDocMacroText(run.Text()) + "]"
	case docir.RunLink:
		return "link:" + asciiDocTarget(run.Target()) + "[" + asciiDocInline(run.Text()) + "]"
	case docir.RunRef:
		// An element outside every document has no destination; its name stands.
		if run.TargetElement() != "" {
			return asciiDocInline(run.Text())
		}
		return w.crossReference(run)
	default:
		return asciiDocInline(run.Text())
	}
}

// crossReference writes a reference run: `<<anchor,text>>` within the
// document, or an xref macro into another document's file, the set's or the
// default (see AsciiDocFileName), with its anchor.
func (w *asciiDocWriter) crossReference(run docir.TextRun) string {
	text := asciiDocInline(refText(run, w.labels))
	if run.TargetDocument() == "" {
		if text == "" {
			return "<<" + run.Target() + ">>"
		}
		return "<<" + run.Target() + "," + text + ">>"
	}
	return "xref:" + asciiDocTarget(refDestination(run, w.files, AsciiDocFileName)) + "[" + text + "]"
}

// AsciiDocFileName is the AsciiDoc file a document is written to on its own,
// DocumentFileStem of its qualified name plus `.adoc`.
func AsciiDocFileName(fqn string) string {
	return DocumentFileStem(fqn) + ".adoc"
}

// asciiDocBlockTitle writes a caption as the block title line of the block
// that follows, newline included; a blank caption writes nothing.
func asciiDocBlockTitle(caption string) string {
	caption = strings.TrimSpace(caption)
	if caption == "" {
		return ""
	}
	return "." + asciiDocInline(caption) + "\n"
}

// asciiDocDelimited wraps escaped text in constrained formatting marks,
// keeping leading and trailing whitespace outside them so the marks stay flanking.
func asciiDocDelimited(mark, text string) string {
	escaped := asciiDocInline(text)
	trimmed := strings.TrimSpace(escaped)
	if trimmed == "" {
		return escaped
	}
	left := escaped[:strings.Index(escaped, trimmed)]
	right := escaped[len(left)+len(trimmed):]
	return left + mark + trimmed + mark + right
}

// asciiDocNonEmpty stands the empty attribute in for text that would leave a
// list item or monospace span without content, which AsciiDoc would not read as one.
func asciiDocNonEmpty(text string) string {
	if strings.TrimSpace(text) == "" {
		return "{empty}"
	}
	return text
}

// asciiDocMacroText is text inside a macro's brackets: on one line, with the
// closing bracket escaped so it cannot end the macro early.
func asciiDocMacroText(text string) string {
	text = strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " ")
	return strings.ReplaceAll(text, "]", `\]`)
}

// asciiDocAttribute is escaped text as one positional attribute of a macro:
// commas and quotes, which would split or quote it, as character references.
func asciiDocAttribute(text string) string {
	return strings.NewReplacer(",", "&#44;", `"`, "&quot;", "'", "&#39;").Replace(asciiDocInline(text))
}

// asciiDocTarget percent-encodes the characters a macro target cannot carry:
// the space and brackets that would end it, and line breaks.
func asciiDocTarget(target string) string {
	return strings.NewReplacer(" ", "%20", "[", "%5B", "]", "%5D", "\n", "%0A", "\r", "%0D").Replace(target)
}

// asciiDocEscaper writes as character references the characters that open
// AsciiDoc formatting, macros, attribute references, passthroughs or cross
// references anywhere in a line, and breaks the digraphs AsciiDoc replaces
// (arrows, dashes, ellipses, marks) or reads as description-list terms, so
// prose keeps its text.
var asciiDocEscaper = strings.NewReplacer(
	"&", "&amp;",
	`\`, "&#92;",
	"*", "&#42;",
	"_", "&#95;",
	"`", "&#96;",
	"#", "&#35;",
	"^", "&#94;",
	"~", "&#126;",
	"+", "&#43;",
	"{", "&#123;",
	"[", "&#91;",
	"]", "&#93;",
	"<", "&lt;",
	">", "&gt;",
	"::", ":&#58;",
	";;", ";&#59;",
	":/", "&#58;/",
	"((", "(&#40;",
	"--", "-&#45;",
	"...", ".&#46;.",
	"(C)", "(&#67;)",
	"(R)", "(&#82;)",
	"(TM)", "(&#84;M)",
)

// asciiDocInline escapes prose for any position in a line, folding newlines
// to spaces since paragraph structure comes from the document, not from run content.
func asciiDocInline(text string) string {
	return asciiDocEscaper.Replace(strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " "))
}

// asciiDocTableCell escapes one table cell: a bar, which would open the next
// cell, with a backslash, and line breaks as hard line breaks so they cannot
// end the row.
func asciiDocTableCell(text string) string {
	escaped := strings.ReplaceAll(asciiDocEscaper.Replace(newlineNormalizer.Replace(text)), "|", `\|`)
	return strings.ReplaceAll(escaped, "\n", " +\n")
}

// asciiDocBlockOpeners match a line that would open block structure: block
// titles, list items, delimiters, comments, attribute lines, page and
// thematic breaks, table openers and admonitions.
var asciiDocBlockOpeners = regexp.MustCompile(`^(\.|[*\-=_][ *\-=_]|\+[+ ]?$|\+ |//|\[|:|<<<|\|===|'''|(\d+\.|[A-Za-z]\.|[IVXivx]+\))( |$)|(NOTE|TIP|IMPORTANT|WARNING|CAUTION): )`)

// asciiDocBlockStart keeps a paragraph a paragraph: a line that would open
// block structure is prefixed with the empty attribute, which renders as
// nothing. Leading spaces and tabs are dropped first, as they would open a
// literal paragraph.
func asciiDocBlockStart(text string) string {
	text = strings.TrimLeft(text, " \t")
	if asciiDocBlockOpeners.MatchString(text) {
		return "{empty}" + text
	}
	return text
}
