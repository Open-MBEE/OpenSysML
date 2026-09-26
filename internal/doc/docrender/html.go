package docrender

import (
	"embed"
	"errors"
	"html"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// defaultCSS is the stylesheet a standalone document carries.
//
//go:embed document.css
var defaultCSS string

// DefaultStylesheet is the default document stylesheet: one cascade layer of
// declarations, every value taken from a --sysml-* token on .sysml-document.

// The note fragments the writer repeats.
const (
	spanClose = "</span>"
)

func DefaultStylesheet() string { return defaultCSS }

// themeFS holds the bundled themes, one <name>.css each written against the
// default sheet's tokens, and the <name>.print.css companion a theme may carry.
//
//go:embed themes/*.css
var themeFS embed.FS

// DefaultTheme names the default stylesheet on its own.
const DefaultTheme = "default"

// printCompanionSuffix ends the file of a theme's print companion.
const printCompanionSuffix = ".print.css"

// Themes lists the bundled theme names, the default first and the rest sorted.
func Themes() []string {
	entries, err := themeFS.ReadDir("themes")
	if err != nil {
		panic("docrender: bundled themes unreadable: " + err.Error())
	}
	names := []string{DefaultTheme}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), printCompanionSuffix) {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".css"))
	}
	sort.Strings(names[1:])
	return names
}

// ThemeStylesheet is the default stylesheet followed by the named theme's
// overrides in the same layer; empty or DefaultTheme is the default alone.
func ThemeStylesheet(name string) (string, error) {
	if name == "" || name == DefaultTheme {
		return defaultCSS, nil
	}
	if strings.ContainsAny(name, "/\\.") {
		return "", &Error{Kind: ErrorUnknownTheme, Actual: name}
	}
	overrides, err := themeFS.ReadFile("themes/" + name + ".css")
	if err != nil {
		return "", &Error{Kind: ErrorUnknownTheme, Actual: name}
	}
	return defaultCSS + "\n" + string(overrides), nil
}

// ThemePrintStylesheet is the named theme's print companion, the overrides a
// paged backend lays over its print stylesheet; empty for a theme without one.
func ThemePrintStylesheet(name string) (string, error) {
	if _, err := ThemeStylesheet(name); err != nil {
		return "", err
	}
	if name == "" || name == DefaultTheme {
		return "", nil
	}
	companion, err := themeFS.ReadFile("themes/" + name + printCompanionSuffix)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(companion), nil
}

// StylesheetFileName is the file a rendered document set links its shared
// stylesheet from.
const StylesheetFileName = "sysml-document.css"

// Attributes and closers the generated markup writes more than once.
const (
	attrName   = "data-name"
	attrQuery  = "data-query"
	attrColumn = "data-column"
	dataObject = "data-object"
	rowEnd     = "</tr>\n"
)

// Stylesheet is one stylesheet a standalone document carries: Content is
// inlined, Href is linked. Exactly one of the two is set; InlineStylesheet
// states a sheet is inlined when its content is empty.
type Stylesheet struct {
	// Content is stylesheet text to inline, keeping the document self-contained.
	Content string
	// Href is a stylesheet URL to link, for a site-hosted stylesheet.
	Href string
	// inline states the sheet is content, however empty that content is.
	inline bool
}

// InlineStylesheet is a stylesheet inlined in the document, whose content may
// be empty: a file the run named is a stylesheet even when it declares nothing.
func InlineStylesheet(content string) Stylesheet {
	return Stylesheet{Content: content, inline: true}
}

// LinkedStylesheet is a stylesheet the document links rather than carries.
func LinkedStylesheet(href string) Stylesheet { return Stylesheet{Href: href} }

// HTMLOptions are the presentation choices of the HTML backend. They are
// options of this backend, never document-model attributes.
type HTMLOptions struct {
	// Fragment writes the document element alone, without the page shell or
	// any stylesheet, for embedding in a host page.
	Fragment bool

	// NoDefaultStylesheet leaves the default stylesheet out.
	NoDefaultStylesheet bool

	// Theme names the bundled theme layered over the default stylesheet;
	// empty is the default alone. Ignored with NoDefaultStylesheet.
	Theme string

	// Stylesheets are attached after the default one, unlayered, so their
	// declarations win on cascade origin rather than on specificity.
	Stylesheets []Stylesheet

	// TitlePage puts the document title in a page of its own.
	TitlePage bool

	// TOC writes a table of contents ahead of the content.
	TOC bool

	// NumberSections numbers the section headings hierarchically.
	NumberSections bool

	// NumberFigures numbers figures and tables as MarkdownOptions.NumberFigures,
	// the label in a sysml-caption-number span.
	NumberFigures bool

	// Lang is the page language, "en" when empty.
	Lang string

	// MermaidScript is the URL of a Mermaid script a standalone page loads to
	// draw its diagrams; empty loads none, leaving each as source.
	MermaidScript string

	// MathScript is the URL of a MathJax-compatible script a standalone page
	// loads to typeset its formulas; empty loads none, leaving each as LaTeX
	// source in \(…\) or \[…\] delimiters.
	MathScript string

	// DiagramForm is the source every graph-shaped diagram is written as; a
	// table-kind view is a table whichever it is. Empty picks per diagram:
	// DOT for a rendering a Layout or Route positions, Mermaid otherwise.
	DiagramForm view.Form

	// WithoutGraphviz records that no Graphviz draws DOT for this render, so
	// the automatic choice writes Mermaid for positioned diagrams too, with a
	// notice stating so. An explicit DiagramForm is written regardless.
	WithoutGraphviz bool

	// Unplaced is where a diagram some Layout positions puts the nodes none
	// does: left undrawn when empty, or drawn too (a strip below a DOT drawing).
	Unplaced view.Unplaced

	// Style is the drawing style every DOT diagram is drawn in, the Pilot look
	// when empty; the other forms draw one look.
	Style view.DrawingStyle

	// Files is the file each document of the set this one is rendered in is
	// written to, by qualified name; a cross-document reference links to the
	// target's file here, or to DocumentHTMLFileName of its name when absent.
	Files map[string]string

	// DiagramImages are images drawn ahead of the render, one per graph-shaped
	// diagram in the order Diagrams lists them, each written as <img> in place
	// of its source; an empty entry, or none, keeps the source. More entries
	// than diagrams is an error.
	DiagramImages []string

	// DiagramSVG is SVG markup drawn ahead of the render, one per graph-shaped
	// diagram in the order Diagrams lists them, each written inline in place
	// of its source; an empty entry, or none, defers to DiagramImages, then
	// the source. More entries than diagrams is an error.
	DiagramSVG []string

	// Drawer draws the DOT diagrams the automatic choice picks, filling
	// DiagramSVG when that is empty; one that is not available settles the
	// choice on the Mermaid fallback. Nil leaves the choice to WithoutGraphviz.
	Drawer DiagramDrawer

	// Math is typeset HTML for formulas, keyed as Formulas lists them, written
	// in place of the delimited LaTeX; a formula with none keeps its LaTeX.
	Math map[Formula]string

	// OutputDir is the directory the page is written into, when known, so an
	// image's source-relative location is written relative to it; empty writes it as stated.
	OutputDir string

	// TableColumns is the most columns one table is written with: a table
	// projecting more is written as continuation tables, each repeating the
	// first column ahead of its share of the rest. 0 writes every table whole.
	TableColumns int
}

// MermaidScriptURL is the pinned Mermaid release a page loads from a public
// CDN when a script is asked for by name rather than URL.
const MermaidScriptURL = "https://cdn.jsdelivr.net/npm/mermaid@11.16.1/dist/mermaid.min.js"

// MathScriptURL is the pinned MathJax release a page loads from a public CDN
// when a math script is asked for by name rather than URL.
const MathScriptURL = "https://cdn.jsdelivr.net/npm/mathjax@3.2.2/es5/tex-mml-chtml.js"

// mathConfig configures MathJax ahead of its script: the delimiters the
// markup writes, and typesetting confined to .sysml-math so prose that
// happens to contain a delimiter is never read as math.
const mathConfig = `<script>window.MathJax = {tex: {inlineMath: [["\\(", "\\)"]], displayMath: [["\\[", "\\]"]]}, ` +
	`options: {ignoreHtmlClass: "sysml-document", processHtmlClass: "sysml-math"}};</script>
`

// HTML renders an evaluated document as deterministic, semantic HTML: an
// <article> holding nested <section> elements, real tables with <caption> and
// <th scope>, <ul>/<ol> lists, <dl> definitions, <figure> formulas holding
// their LaTeX in display delimiters and <figure> diagrams carrying their
// source in the chosen diagram form.
// Every node keeps its model facts in sysml- classes and data- attributes —
// content kind, declared name, query, group column, row element and its kind,
// projected column, value kind, reference target, diagram kind, direction and
// palette —
// and every value is escaped so no content can corrupt the structure.
func HTML(document *docir.Document, opts HTMLOptions) (string, error) {
	if document == nil {
		return "", &Error{Kind: ErrorNilDocument}
	}
	for _, sheet := range opts.Stylesheets {
		if err := sheet.Check(); err != nil {
			return "", err
		}
	}
	diagrams := opts.diagramOptions()
	if err := diagrams.check(); err != nil {
		return "", err
	}
	if err := drawAutomatic(document, &diagrams, opts.Drawer, &opts.DiagramSVG); err != nil {
		return "", err
	}
	var base string
	if !opts.NoDefaultStylesheet {
		var err error
		if base, err = ThemeStylesheet(opts.Theme); err != nil {
			return "", err
		}
	}
	w := &htmlWriter{opts: opts, base: base, forms: diagrams, ids: contentIDs(document), captions: captionNumbering{on: opts.NumberFigures}}
	w.numbers = sectionNumbers(document.Content(), nil, "", map[string]string{})
	if err := w.writeDocument(document); err != nil {
		return "", err
	}
	if w.diagrams < max(len(opts.DiagramImages), len(opts.DiagramSVG)) {
		return "", &Error{Kind: ErrorSurplusDiagramImages, Actual: strconv.Itoa(max(len(opts.DiagramImages), len(opts.DiagramSVG))), Count: w.diagrams}
	}
	return w.b.String(), nil
}

// Check rejects a stylesheet that is neither content nor URL, both at once, or
// whose content would close the <style> element it is inlined in.
func (s Stylesheet) Check() error {
	switch {
	case s.Content == "" && s.Href == "" && !s.inline:
		return &Error{Kind: ErrorEmptyStylesheet}
	case s.Content != "" && s.Href != "":
		return &Error{Kind: ErrorAmbiguousStylesheet, Actual: s.Href}
	case s.Content != "" && strings.Contains(strings.ToLower(s.Content), "</style"):
		return &Error{Kind: ErrorUnsafeStylesheet}
	}
	return nil
}

// diagramOptions is the part of the options the diagrams are written by.
func (o HTMLOptions) diagramOptions() DiagramOptions {
	return DiagramOptions{Form: o.DiagramForm, WithoutGraphviz: o.WithoutGraphviz, Unplaced: o.Unplaced, Style: o.Style}
}

// htmlWriter accumulates one rendered document, its diagrams in the forms
// forms picks; ids maps each content node's named path to the identifier
// addressing it, diagrams counts the graph-shaped diagrams written so far,
// and mermaid holds the Mermaid sources left for a loaded script to draw.
type htmlWriter struct {
	b        strings.Builder
	opts     HTMLOptions
	base     string
	forms    DiagramOptions
	ids      map[string]string
	numbers  map[string]string
	captions captionNumbering
	diagrams int
	mermaid  []string
}

// captionMarkup is a caption's inner HTML: its number, when it has one, in a
// span ahead of its text.
func captionMarkup(c caption) string {
	switch {
	case c.label == "":
		return htmlText(c.text)
	case c.text == "":
		return "<span class=\"sysml-caption-number\">" + htmlText(c.label) + "</span>"
	}
	return "<span class=\"sysml-caption-number\">" + htmlText(c.label+".") + "</span> " + htmlText(c.text)
}

func (w *htmlWriter) writeDocument(document *docir.Document) error {
	if !w.opts.Fragment {
		w.writeShellStart(document.Title())
	}
	w.b.WriteString("<article class=\"sysml-document\"" + attr("data-document", document.Name()) + ">\n")
	w.writeTitle(document.Title())
	if w.opts.TOC {
		w.writeTOC(w.outline(document.Content(), nil))
	}
	for i, node := range document.Content() {
		if err := w.writeContent(node, nil, i, 2); err != nil {
			return err
		}
	}
	w.b.WriteString("</article>\n")
	if !w.opts.Fragment {
		if w.opts.MermaidScript != "" {
			w.b.WriteString("<script" + attr("src", w.opts.MermaidScript) + "></script>\n")
			if len(w.mermaid) > 0 {
				textSize, edges := view.MermaidLimits(w.mermaid...)
				w.b.WriteString("<script>mermaid.initialize({maxTextSize: " + strconv.Itoa(textSize) +
					", maxEdges: " + strconv.Itoa(edges) + "});</script>\n")
			}
		}
		if w.opts.MathScript != "" {
			w.b.WriteString(mathConfig + "<script" + attr("src", w.opts.MathScript) + "></script>\n")
		}
		w.b.WriteString("</body>\n</html>\n")
	}
	return nil
}

// writeShellStart writes the page shell up to <body>, with the default
// stylesheet first and the supplied ones after it, unlayered.
func (w *htmlWriter) writeShellStart(title string) {
	lang := w.opts.Lang
	if lang == "" {
		lang = "en"
	}
	w.b.WriteString("<!DOCTYPE html>\n<html" + attr("lang", lang) + ">\n<head>\n")
	w.b.WriteString("<meta charset=\"utf-8\">\n")
	w.b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	w.b.WriteString("<title>" + htmlText(title) + "</title>\n")
	if !w.opts.NoDefaultStylesheet {
		w.b.WriteString("<style>\n" + w.base + "</style>\n")
	}
	w.b.WriteString(StylesheetMarkup(w.opts.Stylesheets))
	w.b.WriteString("</head>\n<body>\n")
}

// StylesheetMarkup is the head markup attaching sheets in order: a linked
// sheet as a <link> element, an inline one as a <style> element.
func StylesheetMarkup(sheets []Stylesheet) string {
	var b strings.Builder
	for _, sheet := range sheets {
		if sheet.Href != "" {
			b.WriteString("<link rel=\"stylesheet\"" + attr("href", sheet.Href) + ">\n")
			continue
		}
		b.WriteString("<style>\n" + strings.TrimRight(sheet.Content, "\n") + "\n</style>\n")
	}
	return b.String()
}

// writeTitle writes the document title: in a page of its own when a title page
// was asked for, and as the opening heading otherwise.
func (w *htmlWriter) writeTitle(title string) {
	heading := "<h1 class=\"sysml-title\">" + htmlText(title) + "</h1>\n"
	if w.opts.TitlePage {
		w.b.WriteString("<header class=\"sysml-title-page\">\n" + heading + "</header>\n")
		return
	}
	w.b.WriteString(heading)
}

// outlineEntry is one section of the table of contents.
type outlineEntry struct {
	id       string
	title    string
	number   string
	children []outlineEntry
}

// outline collects the section tree with each section's identifier and number.
func (w *htmlWriter) outline(nodes []docir.Content, path []step) []outlineEntry {
	var entries []outlineEntry
	for i, node := range nodes {
		if node.Kind() != docir.ContentSection {
			continue
		}
		nested := child(path, node.Name(), i)
		entries = append(entries, outlineEntry{
			id:       w.ids[pathKey(nested)],
			title:    node.Title(),
			number:   w.numbers[pathKey(nested)],
			children: w.outline(node.Children(), nested),
		})
	}
	return entries
}

// sectionNumbers numbers every section hierarchically, keyed by occurrence path.
func sectionNumbers(nodes []docir.Content, path []step, prefix string, into map[string]string) map[string]string {
	count := 0
	for i, node := range nodes {
		if node.Kind() != docir.ContentSection {
			continue
		}
		count++
		nested := child(path, node.Name(), i)
		number := prefix + strconv.Itoa(count)
		into[pathKey(nested)] = number
		sectionNumbers(node.Children(), nested, number+".", into)
	}
	return into
}

// writeTOC writes the table of contents as nested ordered lists, each entry
// linking to its section.
func (w *htmlWriter) writeTOC(entries []outlineEntry) {
	if len(entries) == 0 {
		return
	}
	w.b.WriteString("<nav class=\"sysml-toc\" aria-label=\"Contents\">\n")
	w.b.WriteString("<h2 class=\"sysml-toc-title\">Contents</h2>\n")
	w.writeTOCList(entries)
	w.b.WriteString("</nav>\n")
}

func (w *htmlWriter) writeTOCList(entries []outlineEntry) {
	w.b.WriteString("<ol>\n")
	for _, entry := range entries {
		w.b.WriteString("<li>")
		w.b.WriteString("<a" + attr("href", "#"+entry.id) + ">")
		if w.opts.NumberSections {
			w.b.WriteString("<span class=\"sysml-section-number\">" + htmlText(entry.number) + "</span> ")
		}
		w.b.WriteString(htmlText(entry.title) + "</a>")
		if len(entry.children) > 0 {
			w.b.WriteString("\n")
			w.writeTOCList(entry.children)
		}
		w.b.WriteString("</li>\n")
	}
	w.b.WriteString("</ol>\n")
}

// writeContent writes one content node, with level the heading level a section
// at this depth writes, path the occurrence path of its parent and index its
// place among its siblings.
func (w *htmlWriter) writeContent(node docir.Content, path []step, index, level int) error {
	nested := child(path, node.Name(), index)
	id := w.ids[pathKey(nested)]
	switch node.Kind() {
	case docir.ContentSection:
		return w.writeSection(node, nested, id, level)
	case docir.ContentParagraph:
		w.b.WriteString("<p class=\"sysml-paragraph\"" + attr("id", id) + " data-content=\"paragraph\"" +
			attr(attrName, node.Name()) + attr(attrQuery, node.Query()) + ">" +
			w.inlineRuns(node.Runs()) + "</p>\n")
		return nil
	case docir.ContentTable:
		w.writeTable(node, id)
		return nil
	case docir.ContentList:
		w.writeList(node, id)
		return nil
	case docir.ContentDefinitions:
		w.writeDefinitions(node, id)
		return nil
	case docir.ContentFormula:
		w.writeFormula(node, id)
		return nil
	case docir.ContentImage:
		w.writeImage(node, id)
		return nil
	case docir.ContentDiagram:
		return w.writeDiagram(node, id)
	default:
		return &Error{Kind: ErrorUnknownContent, Content: node.Name(), Actual: string(node.Kind())}
	}
}

// writeSection writes one section and its children, numbering the heading when
// numbering was asked for. Heading levels saturate at 6, as HTML has no more.
func (w *htmlWriter) writeSection(node docir.Content, path []step, id string, level int) error {
	w.b.WriteString("<section class=\"sysml-section\"" + attr("id", id) + " data-content=\"section\"" +
		attr(attrName, node.Name()) + ">\n")
	tag := "h" + strconv.Itoa(min(level, 6))
	w.b.WriteString("<" + tag + ">")
	if w.opts.NumberSections {
		w.b.WriteString("<span class=\"sysml-section-number\">" + htmlText(w.numbers[pathKey(path)]) + "</span> ")
	}
	w.b.WriteString(htmlText(node.Title()) + "</" + tag + ">\n")
	for i, child := range node.Children() {
		if err := w.writeContent(child, path, i, level+1); err != nil {
			return err
		}
	}
	w.b.WriteString("</section>\n")
	return nil
}

// writeTable writes one table: its caption, a header row of projected column
// names, and one row per query row. A query without projected columns gets a
// single "element" column, and a grouped table one <tbody> per group.
func (w *htmlWriter) writeTable(node docir.Content, id string) {
	columns := node.Columns()
	c := w.captions.caption(node)
	parts := tableParts(len(columns), w.opts.TableColumns)
	for i, part := range parts {
		partID, partCaption := id, c
		if i > 0 {
			if id != "" {
				partID = id + "-" + strconv.Itoa(i+1)
			}
			partCaption.text = strings.TrimSpace(c.text + " " + continuedSuffix)
		}
		w.writeTablePart(node, partID, partCaption, columns, part, i > 0)
	}
}

// continuedSuffix marks the caption of a continuation table.
const continuedSuffix = "(continued)"

// tableParts splits the indexes of a table's columns into the column sets its
// parts are written with: one part holding every column when at most limit
// (or when limit is 0), otherwise parts of the first column followed by
// consecutive slices of the rest, each part of at most limit columns.
func tableParts(columns, limit int) [][]int {
	all := make([]int, columns)
	for i := range all {
		all[i] = i
	}
	if limit <= 1 || columns <= limit {
		return [][]int{all}
	}
	var parts [][]int
	for start := 1; start < columns; start += limit - 1 {
		end := start + limit - 1
		if end > columns {
			end = columns
		}
		part := append([]int{0}, all[start:end]...)
		parts = append(parts, part)
	}
	return parts
}

// writeTablePart writes one table over the columns at the given indexes: its
// caption, the column group when a column states a width, the header, and the
// rows — grouped when the table groups.
func (w *htmlWriter) writeTablePart(
	node docir.Content,
	id string,
	c caption,
	columns []queryexec.Column,
	indexes []int,
	continued bool,
) {
	names := make([]string, 0, len(indexes))
	part := make([]queryexec.Column, 0, len(indexes))
	for _, i := range indexes {
		names = append(names, columns[i].Name())
		part = append(part, columns[i])
	}
	if len(names) == 0 {
		names = []string{elementColumn}
	}
	class := "sysml-table"
	if continued {
		class += " sysml-table-continued"
	}
	shares := columnShares(part)
	if shares != nil {
		class += " sysml-table-sized"
	}
	w.b.WriteString("<table" + attr("class", class) + attr("id", id) + " data-content=\"table\"" +
		attr(attrName, node.Name()) + attr(attrQuery, node.Query()) +
		attr("data-group-by", node.GroupBy()) + ">\n")
	if c.String() != "" {
		w.b.WriteString("<caption class=\"sysml-caption\">" + captionMarkup(c) + "</caption>\n")
	}
	w.writeColumnGroup(part, shares)
	w.writeTableHead(names)
	if node.GroupBy() != "" {
		for _, group := range node.Groups() {
			w.b.WriteString("<tbody class=\"sysml-group\"" + attr("data-group", node.GroupBy()) +
				attr("data-group-key", group.Key()) + ">\n")
			w.b.WriteString("<tr class=\"sysml-group-heading\"><th scope=\"rowgroup\"" +
				attr("colspan", strconv.Itoa(len(names))) + ">" +
				"<span class=\"sysml-group-column\">" + htmlText(node.GroupBy()) + "</span>: " +
				"<span class=\"sysml-group-key\">" + htmlText(group.Key()) + "</span></th></tr>\n")
			w.writeRows(group.Rows(), names, indexes, len(columns))
			w.b.WriteString("</tbody>\n")
		}
		w.b.WriteString("</table>\n")
		return
	}
	w.b.WriteString("<tbody>\n")
	w.writeRows(node.Rows(), names, indexes, len(columns))
	w.b.WriteString("</tbody>\n</table>\n")
}

// firstColumnMinShare is the least share of a sized table's width its first
// column takes, so the column naming each row stays readable however narrow
// its stated width is beside many value columns.
const firstColumnMinShare = 0.18

// columnShares is the share of the table's width each column takes from the
// stated widths: proportional to them, an automatic column counting as the
// mean stated width, the first column at least firstColumnMinShare. Nil when
// no column states a width.
func columnShares(columns []queryexec.Column) []float64 {
	total, stated := 0, 0
	for _, column := range columns {
		if column.Width() > 0 {
			total += column.Width()
			stated++
		}
	}
	if stated == 0 {
		return nil
	}
	mean := float64(total) / float64(stated)
	widths := make([]float64, len(columns))
	sum := 0.0
	for i, column := range columns {
		widths[i] = mean
		if column.Width() > 0 {
			widths[i] = float64(column.Width())
		}
		sum += widths[i]
	}
	shares := make([]float64, len(columns))
	for i, width := range widths {
		shares[i] = width / sum
	}
	if len(shares) > 1 && shares[0] < firstColumnMinShare {
		scale := (1 - firstColumnMinShare) / (1 - shares[0])
		for i := 1; i < len(shares); i++ {
			shares[i] *= scale
		}
		shares[0] = firstColumnMinShare
	}
	return shares
}

// writeColumnGroup writes one <col> per column of a sized table, its stated
// width as data and its share of the table's width as style, so a fixed
// layout honours the source's proportions.
func (w *htmlWriter) writeColumnGroup(columns []queryexec.Column, shares []float64) {
	if shares == nil {
		return
	}
	w.b.WriteString("<colgroup>\n")
	for i, column := range columns {
		width := ""
		if column.Width() > 0 {
			width = strconv.Itoa(column.Width())
		}
		w.b.WriteString("<col" + attr("data-width", width) +
			attr("style", "width: "+strconv.FormatFloat(shares[i]*100, 'f', 1, 64)+"%") + ">\n")
	}
	w.b.WriteString("</colgroup>\n")
}

func (w *htmlWriter) writeTableHead(names []string) {
	w.b.WriteString("<thead>\n<tr>\n")
	for _, name := range names {
		w.b.WriteString("<th scope=\"col\"" + attr(attrColumn, name) + ">" + htmlText(name) + "</th>\n")
	}
	w.b.WriteString("</tr>\n</thead>\n")
}

// writeRows writes one row per query row, each carrying the element it selected
// and its kind. A row of a table without projected columns is its element alone.
func (w *htmlWriter) writeRows(rows []queryexec.Row, names []string, indexes []int, columns int) {
	for _, row := range rows {
		w.b.WriteString("<tr class=\"sysml-row\"" + elementAttrs(row.Element()) + depthAttrs(row.Depth()) + ">\n")
		if columns == 0 {
			w.writeCell([]queryexec.Value{row.Element()}, elementColumn, row.Depth())
			w.b.WriteString(rowEnd)
			continue
		}
		cells := row.Cells()
		for n, i := range indexes {
			var values []queryexec.Value
			if i < len(cells) {
				values = cells[i].Values()
			}
			depth := int64(0)
			if n == 0 {
				depth = row.Depth()
			}
			w.writeCell(values, names[n], depth)
		}
		w.b.WriteString(rowEnd)
	}
}

// depthAttrs states a nested row's depth: as data, and as the --sysml-depth
// property the stylesheet indents the row's first cell by.
func depthAttrs(depth int64) string {
	if depth <= 0 {
		return ""
	}
	text := strconv.FormatInt(depth, 10)
	return attr("data-depth", text) + attr("style", "--sysml-depth: "+text)
}

// writeCell writes one projected cell, a nested row's first opening with the
// indent its depth sets: every value individually addressable,
// with the punctuation joining them an element of its own so a theme can hide
// or replace it.
func (w *htmlWriter) writeCell(values []queryexec.Value, column string, depth int64) {
	w.b.WriteString("<td class=\"sysml-cell\"" + attr(attrColumn, column) +
		attr("data-value-kind", sharedValueKind(values)) + ">")
	if depth > 0 {
		w.b.WriteString("<span class=\"sysml-indent\"></span>")
	}
	for i, value := range values {
		if i > 0 {
			w.b.WriteString("<span class=\"sysml-separator\">, </span>")
		}
		w.writeValue(value)
	}
	w.b.WriteString("</td>\n")
}

func (w *htmlWriter) writeValue(value queryexec.Value) {
	classes := "sysml-value"
	if _, ok := value.Element(); ok {
		classes += " sysml-element"
	}
	if _, _, ok := value.Object(); ok {
		classes += " sysml-object"
	}
	if _, ok := value.Verdict(); ok {
		classes += " sysml-verdict"
	}
	if _, ok := value.State(); ok {
		classes += " sysml-state"
	}
	if _, ok := value.Event(); ok {
		classes += " sysml-event"
	}
	w.b.WriteString("<span class=\"" + classes + "\"" + attr("data-value-kind", string(value.Kind())) +
		elementAttrs(value) + quantityAttrs(value) + ">" + htmlText(valueText(value)) + spanClose)
}

// quantityAttrs carries a quantity's magnitude and unit apart, so a theme or a
// script reads them without parsing the cell text.
func quantityAttrs(value queryexec.Value) string {
	quantity, ok := value.Quantity()
	if !ok {
		return ""
	}
	magnitude, _ := value.Magnitude()
	return attr("data-magnitude", valueText(magnitude)) + attr("data-unit", quantity.Unit.String())
}

// writeList writes one bullet or numbered list, one item per query row, each
// item carrying the element its row selected.
func (w *htmlWriter) writeList(node docir.Content, id string) {
	tag := "ul"
	if node.Style() == docir.ListNumber {
		tag = "ol"
	}
	w.b.WriteString("<" + tag + " class=\"sysml-list\"" + attr("id", id) + " data-content=\"list\"" +
		attr(attrName, node.Name()) + attr(attrQuery, node.Query()) + ">\n")
	for _, item := range node.Items() {
		w.b.WriteString("<li class=\"sysml-item\"" + elementAttrs(item.Element()) + ">" +
			w.inlineRuns(item.Runs()) + "</li>\n")
	}
	w.b.WriteString("</" + tag + ">\n")
}

// writeDefinitions writes one description list, one term/description group
// per query row, each group carrying the element its row selected.
func (w *htmlWriter) writeDefinitions(node docir.Content, id string) {
	w.b.WriteString("<dl class=\"sysml-definitions\"" + attr("id", id) + " data-content=\"definitions\"" +
		attr(attrName, node.Name()) + attr(attrQuery, node.Query()) + ">\n")
	for _, entry := range node.Definitions() {
		w.b.WriteString("<div class=\"sysml-entry\"" + elementAttrs(entry.Element()) + ">\n" +
			"<dt class=\"sysml-term\">" + w.inlineRuns(entry.Term()) + "</dt>\n" +
			"<dd class=\"sysml-description\">" + w.inlineRuns(entry.Description()) + "</dd>\n" +
			"</div>\n")
	}
	w.b.WriteString("</dl>\n")
}

// writeFormula writes one formula as a figure: its typeset HTML when the
// render carries it, else its LaTeX, escaped, in display delimiters for a math
// script to typeset; then its caption.
func (w *htmlWriter) writeFormula(node docir.Content, id string) {
	w.b.WriteString("<figure class=\"sysml-formula\"" + attr("id", id) + " data-content=\"formula\"" +
		attr(attrName, node.Name()) + ">\n")
	math := displayMathHTML(node.Source())
	if typeset, ok := w.opts.Math[displayFormula(node.Source())]; ok {
		math = typeset
	}
	w.b.WriteString("<div class=\"sysml-math\">" + math + "</div>\n")
	if node.Caption() != "" {
		w.b.WriteString("<figcaption class=\"sysml-caption\">" + htmlText(node.Caption()) + "</figcaption>\n")
	}
	w.b.WriteString("</figure>\n")
}

// writeImage writes one image as a figure: its location as the page refers
// to it (see HTMLOptions.OutputDir), and its caption.
func (w *htmlWriter) writeImage(node docir.Content, id string) {
	alt := node.Alt()
	if alt == "" {
		alt = node.Caption()
	}
	w.b.WriteString("<figure class=\"sysml-image\"" + attr("id", id) + " data-content=\"image\"" +
		attr(attrName, node.Name()) + ">\n")
	w.b.WriteString("<img" + attr("src", imageOf(node).Source(w.opts.OutputDir)) + attr("alt", alt) + ">\n")
	if c := w.captions.caption(node); c.String() != "" {
		w.b.WriteString("<figcaption class=\"sysml-caption\">" + captionMarkup(c) + "</figcaption>\n")
	}
	w.b.WriteString("</figure>\n")
}

// Delimiters a math script recognizes inline and display LaTeX by.
const (
	inlineMathOpen   = `\(`
	inlineMathClose  = `\)`
	displayMathOpen  = `\[`
	displayMathClose = `\]`
)

// inlineMathHTML writes inline LaTeX, escaped and trimmed with its newlines
// folded, in inline delimiters.
func inlineMathHTML(source string) string {
	return inlineMathOpen + htmlText(strings.TrimSpace(source)) + inlineMathClose
}

// displayMathHTML writes display LaTeX, escaped and trimmed with its line
// breaks kept, in display delimiters.
func displayMathHTML(source string) string {
	return displayMathOpen + html.EscapeString(strings.TrimSpace(newlineNormalizer.Replace(source))) + displayMathClose
}

// writeDiagram writes one diagram as a figure: a table-kind view as a table,
// every other supported kind as the image drawn for it ahead of the render,
// or else as its source in the render's diagram form — Mermaid, which a loaded
// Mermaid script draws, or DOT or PlantUML — shown as text.
func (w *htmlWriter) writeDiagram(node docir.Content, id string) error {
	return w.writeFigure(id, node.Name(), w.captions.caption(node), node.Rendering(), figureOptions(node, w.forms))
}

func (w *htmlWriter) writeFigure(id, name string, caption caption, rendering *view.Rendering, options view.Options) error {
	if rendering == nil {
		return &Error{Kind: ErrorMissingRendering, Content: name}
	}
	if rendering.Kind != view.KindTable && !rendering.Kind.Supported() {
		return &Error{Kind: ErrorUnrenderableDiagram, Content: name, Actual: string(rendering.Kind), Form: "HTML"}
	}
	var source, fallback string
	var form view.Form
	if rendering.Kind != view.KindTable {
		var err error
		form, fallback = w.forms.formFor(rendering)
		if source, err = diagramSource(name, rendering, options, form); err != nil {
			return err
		}
	}
	w.b.WriteString("<figure class=\"sysml-diagram\"" + attr("id", id) + " data-content=\"diagram\"" +
		attr(attrName, name) + attr("data-view", rendering.View) +
		attr("data-diagram-kind", string(rendering.Kind)) +
		attr("data-direction", string(options.Direction)) + attr("data-palette", string(options.Palette)) +
		attr("data-style", string(options.Style)) + ">\n")
	if fallback != "" {
		w.b.WriteString("<p class=\"sysml-diagram-notice\"><em>" + html.EscapeString(fallback) + "</em></p>\n")
	}
	switch {
	case rendering.Kind == view.KindTable:
		w.writeRenderingTable(rendering)
	case w.diagramSVG() != "":
		w.b.WriteString(svgBlock(w.diagramSVG()) + "\n")
		w.diagrams++
	case w.diagramImage() != "":
		alt := caption.text
		if alt == "" {
			alt = name
		}
		w.b.WriteString("<img" + attr("src", w.diagramImage()) + attr("alt", alt) + ">\n")
		w.diagrams++
	default:
		w.b.WriteString("<pre" + attr("class", string(form)) + ">" + html.EscapeString(source) + "</pre>\n")
		w.diagrams++
		if form == view.FormMermaid {
			w.mermaid = append(w.mermaid, source)
		}
	}
	if caption.String() != "" {
		w.b.WriteString("<figcaption class=\"sysml-caption\">" + captionMarkup(caption) + "</figcaption>\n")
	}
	w.b.WriteString("</figure>\n")
	return nil
}

// diagramImage is the image drawn for the graph-shaped diagram about to be
// written, empty when its source is to be shown instead.
func (w *htmlWriter) diagramImage() string {
	if w.diagrams < len(w.opts.DiagramImages) {
		return w.opts.DiagramImages[w.diagrams]
	}
	return ""
}

// diagramSVG is the SVG drawn for the graph-shaped diagram about to be
// written, empty when an image or its source is to be shown instead.
func (w *htmlWriter) diagramSVG() string {
	if w.diagrams < len(w.opts.DiagramSVG) {
		return w.opts.DiagramSVG[w.diagrams]
	}
	return ""
}

// writeRenderingTable writes a table-kind view's cells, keeping the notices the
// rendering could not represent as comments so none is lost.
func (w *htmlWriter) writeRenderingTable(rendering *view.Rendering) {
	for _, notice := range rendering.Notices {
		w.b.WriteString("<!-- not represented: " + htmlComment(notice) + " -->\n")
	}
	if rendering.Empty() {
		w.b.WriteString("<p class=\"sysml-paragraph\">" + htmlText(rendering.EmptyReason()) + "</p>\n")
		return
	}
	columns := rendering.Columns
	if len(columns) == 0 {
		columns = view.TableColumns()
	}
	w.b.WriteString("<table class=\"sysml-table\" data-content=\"table\">\n")
	w.writeTableHead(columns)
	w.b.WriteString("<tbody>\n")
	for _, row := range rendering.Rows {
		w.b.WriteString("<tr class=\"sysml-row\">\n")
		for i, name := range columns {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			w.b.WriteString("<td class=\"sysml-cell\"" + attr(attrColumn, name) + ">" + htmlText(cell) + "</td>\n")
		}
		w.b.WriteString(rowEnd)
	}
	w.b.WriteString("</tbody>\n</table>\n")
}

// inlineRuns renders text runs joined by single spaces, each by its kind:
// plain runs as prose, styled runs in <em>, <strong> or <code>, math runs as
// delimited LaTeX in a math span, links and references as anchors, and
// element-valued runs carrying their element.
func (w *htmlWriter) inlineRuns(runs []docir.TextRun) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = w.runHTML(run)
	}
	return strings.Join(parts, " ")
}

func (w *htmlWriter) runHTML(run docir.TextRun) string {
	switch run.Kind() {
	case docir.RunEmphasis:
		return "<em>" + htmlText(run.Text()) + "</em>"
	case docir.RunStrong:
		return "<strong>" + htmlText(run.Text()) + "</strong>"
	case docir.RunCode:
		return "<code>" + htmlText(run.Text()) + "</code>"
	case docir.RunMath:
		if typeset, ok := w.opts.Math[inlineFormula(run.Text())]; ok {
			return "<span class=\"sysml-math\">" + typeset + spanClose
		}
		return "<span class=\"sysml-math\">" + inlineMathHTML(run.Text()) + spanClose
	case docir.RunLink:
		if target, ok := navigableURL(run.Target()); ok {
			return "<a class=\"sysml-link\"" + attr("href", target) + ">" + htmlText(run.Text()) + "</a>"
		}
		// A scheme a document must not navigate to is kept as data, not as a link.
		return "<a class=\"sysml-link\"" + attr("data-href", run.Target()) + ">" + htmlText(run.Text()) + "</a>"
	case docir.RunRef:
		return "<a class=\"sysml-ref\"" + attr("href", refDestination(run, w.opts.Files, DocumentHTMLFileName)) +
			attr("data-document", run.TargetDocument()) + ">" + htmlText(run.Text()) + "</a>"
	default:
		return htmlText(run.Text())
	}
}

// DocumentHTMLFileName is the HTML file a document is written to on its own,
// DocumentFileStem of its qualified name plus `.html`.
func DocumentHTMLFileName(fqn string) string {
	return DocumentFileStem(fqn) + ".html"
}

// contentIDs maps every content node's occurrence path to the identifier the
// document addresses it by: the stable anchor the IR gave it, and for a
// section without one the anchor its named path derives, made unique so no two
// nodes share an identifier.
func contentIDs(document *docir.Document) map[string]string {
	reserved := reservedIDs(document.Content(), nil, map[string]bool{})
	anchors := anchorIDs(document.Content(), map[string]bool{})
	ids, used := map[string]string{}, map[string]bool{}
	var assign func(nodes []docir.Content, path []step)
	assign = func(nodes []docir.Content, path []step) {
		for i, node := range nodes {
			nested := child(path, node.Name(), i)
			id := node.Anchor()
			if id == "" && node.Kind() == docir.ContentSection {
				id = uniqueID(derivedID(nested), reserved, used, anchors)
			}
			if id != "" {
				ids[pathKey(nested)] = id
				used[id] = true
			}
			assign(node.Children(), nested)
		}
	}
	assign(document.Content(), nil)
	return ids
}

// reservedIDs collects every identifier the document may address a node by, so
// a derived one is never made to collide with an anchor emitted later.
func reservedIDs(nodes []docir.Content, path []step, into map[string]bool) map[string]bool {
	for i, node := range nodes {
		nested := child(path, node.Name(), i)
		into[derivedID(nested)] = true
		if node.Anchor() != "" {
			into[node.Anchor()] = true
		}
		reservedIDs(node.Children(), nested, into)
	}
	return into
}

// anchorIDs collects the anchors the IR gave nodes, which a derived identifier
// must leave to them however early it is allocated.
func anchorIDs(nodes []docir.Content, into map[string]bool) map[string]bool {
	for _, node := range nodes {
		if node.Anchor() != "" {
			into[node.Anchor()] = true
		}
		anchorIDs(node.Children(), into)
	}
	return into
}

// derivedID is the identifier a path derives, which is the anchor of its
// names; a path of anonymous nodes alone derives none, and is numbered from
// the fallback instead.
func derivedID(path []step) string {
	names := make([]string, 0, len(path))
	for _, s := range path {
		names = append(names, s.name)
	}
	if anchor := docir.AnchorFor(names); strings.Trim(anchor, "-") != "" {
		return anchor
	}
	return anonymousID
}

// anonymousID is the identifier a section with no named ancestry is numbered
// from, since it has no name to derive one.
const anonymousID = "section"

// uniqueID is the derived identifier itself when no other node has taken it or
// owns it as an anchor, and the first free numbered variant otherwise: two
// anonymous siblings derive the same path.
func uniqueID(derived string, reserved, used, anchors map[string]bool) string {
	if !used[derived] && !anchors[derived] {
		return derived
	}
	for n := 2; ; n++ {
		candidate := derived + "-" + strconv.Itoa(n)
		if !reserved[candidate] && !used[candidate] {
			return candidate
		}
	}
}

// step is one node of an occurrence path: the node's declared name, empty when
// anonymous, and its place among its siblings, which tells two anonymous
// siblings apart.
type step struct {
	name  string
	index int
}

// pathKey keys an occurrence path by a separator no name can contain.
func pathKey(path []step) string {
	var b strings.Builder
	for _, s := range path {
		b.WriteString(strconv.Itoa(s.index))
		b.WriteByte(0)
		b.WriteString(s.name)
		b.WriteByte(0)
	}
	return b.String()
}

// child extends an occurrence path without sharing its backing array.
func child(path []step, name string, index int) []step {
	return append(append(make([]step, 0, len(path)+1), path...), step{name: name, index: index})
}

// attr writes one attribute, escaped, and nothing for an empty value.
func attr(name, value string) string {
	if value == "" {
		return ""
	}
	return " " + name + "=\"" + html.EscapeString(value) + "\""
}

// elementAttrs writes the element behind a row, item or value (an object's declaration,
// a verdict's assertion, a state's declaration), plus an object's identity, a verdict's
// status and carrier path, a state's machine and path, or an event's kind and instant.
func elementAttrs(value queryexec.Value) string {
	objectAttrs := ""
	if inst, _, ok := value.Object(); ok {
		objectAttrs = attr(dataObject, "#"+strconv.FormatInt(inst.ID, 10))
	}
	if verdict, ok := value.Verdict(); ok {
		if carrier, held := verdict.Carrier(); held && carrier != nil {
			objectAttrs = attr(dataObject, "#"+strconv.FormatInt(carrier.ID, 10))
		}
		objectAttrs += attr("data-verdict", verdict.Status().String()) + attr("data-path", verdict.Path())
	}
	if state, ok := value.State(); ok {
		if inst, _ := state.Object(); inst != nil {
			objectAttrs = attr(dataObject, "#"+strconv.FormatInt(inst.ID, 10))
		}
		objectAttrs += attr("data-machine", state.Machine()) + attr("data-state", state.Path()) + attr("data-region", state.Region())
	}
	if event, ok := value.Event(); ok {
		if inst, _ := event.Object(); inst != nil {
			objectAttrs = attr(dataObject, "#"+strconv.FormatInt(inst.ID, 10))
		}
		objectAttrs += attr("data-event-kind", event.Kind()) + attr("data-time", strconv.FormatFloat(event.At(), 'g', -1, 64))
	}
	element := value.Declaration()
	if element == nil {
		return objectAttrs
	}
	return objectAttrs + attr("data-element", symbols.FQNOf(element)) + attr("data-element-kind", element.Kind.String())
}

// navigableSchemes are the URL schemes a rendered document links to; every
// other scheme, javascript: above all, is not navigated to from a document.
var navigableSchemes = map[string]bool{
	"http": true, "https": true, "mailto": true, "ftp": true, "ftps": true, "tel": true,
}

// navigableURL reports whether a link target may be an href: a relative URL,
// a fragment, or an absolute URL in a navigable scheme.
func navigableURL(target string) (string, bool) {
	clean := strings.ReplaceAll(newlineNormalizer.Replace(target), "\n", "")
	colon := strings.IndexByte(clean, ':')
	if colon < 0 {
		return clean, true
	}
	if stop := strings.IndexAny(clean, "/?#"); stop >= 0 && stop < colon {
		return clean, true
	}
	return clean, navigableSchemes[strings.ToLower(clean[:colon])]
}

// sharedValueKind is the value kind of a cell whose values all have one,
// empty for an empty cell or a cell of mixed kinds.
func sharedValueKind(values []queryexec.Value) string {
	if len(values) == 0 {
		return ""
	}
	kind := values[0].Kind()
	for _, value := range values[1:] {
		if value.Kind() != kind {
			return ""
		}
	}
	return string(kind)
}

// htmlText escapes prose for any position in the document, folding newlines to
// spaces since paragraph structure comes from the document, not run content.
func htmlText(text string) string {
	return html.EscapeString(strings.ReplaceAll(newlineNormalizer.Replace(text), "\n", " "))
}

// htmlComment escapes a comment, where no escaping mechanism exists: the
// sequence that would close it early is broken up and newlines fold to spaces.
func htmlComment(text string) string {
	return strings.NewReplacer("--", "- -", "\r\n", " ", "\r", " ", "\n", " ").Replace(text)
}
