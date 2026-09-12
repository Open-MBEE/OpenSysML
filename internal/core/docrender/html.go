package docrender

import (
	"embed"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// defaultCSS is the stylesheet a standalone document carries.
//
//go:embed document.css
var defaultCSS string

// DefaultStylesheet is the default document stylesheet: one cascade layer of
// declarations, every value taken from a --sysml-* token on .sysml-document.
func DefaultStylesheet() string { return defaultCSS }

// themeFS holds the bundled themes, one <name>.css each, written against the
// default sheet's tokens in its cascade layer.
//
//go:embed themes/*.css
var themeFS embed.FS

// DefaultTheme names the default stylesheet on its own.
const DefaultTheme = "default"

// Themes lists the bundled theme names, the default first and the rest sorted.
func Themes() []string {
	entries, err := themeFS.ReadDir("themes")
	if err != nil {
		panic("docrender: bundled themes unreadable: " + err.Error())
	}
	names := []string{DefaultTheme}
	for _, entry := range entries {
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

// StylesheetFileName is the file a rendered document set links its shared
// stylesheet from.
const StylesheetFileName = "sysml-document.css"

// Attributes and closers the generated markup writes more than once.
const (
	attrName   = "data-name"
	attrQuery  = "data-query"
	attrColumn = "data-column"
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

	// Lang is the page language, "en" when empty.
	Lang string

	// MermaidScript is the URL of a Mermaid script a standalone page loads to
	// draw its diagrams; empty loads none, leaving each as source.
	MermaidScript string
}

// MermaidScriptURL is the pinned Mermaid release a page loads from a public
// CDN when a script is asked for by name rather than URL.
const MermaidScriptURL = "https://cdn.jsdelivr.net/npm/mermaid@11.16.1/dist/mermaid.min.js"

// HTML renders an evaluated document as deterministic, semantic HTML: an
// <article> holding nested <section> elements, real tables with <caption> and
// <th scope>, <ul>/<ol> lists, <dl> definitions and <figure> diagrams
// carrying Mermaid source.
// Every node keeps its model facts in sysml- classes and data- attributes —
// content kind, declared name, query, group column, row element and its kind,
// projected column, value kind, reference target, diagram kind and direction —
// and every value is escaped so no content can corrupt the structure.
func HTML(document *docir.Document, opts HTMLOptions) (string, error) {
	if document == nil {
		return "", &Error{Kind: ErrorNilDocument}
	}
	for _, sheet := range opts.Stylesheets {
		if err := sheet.check(); err != nil {
			return "", err
		}
	}
	var base string
	if !opts.NoDefaultStylesheet {
		var err error
		if base, err = ThemeStylesheet(opts.Theme); err != nil {
			return "", err
		}
	}
	w := &htmlWriter{opts: opts, base: base, ids: contentIDs(document)}
	w.numbers = sectionNumbers(document.Content(), nil, "", map[string]string{})
	if err := w.writeDocument(document); err != nil {
		return "", err
	}
	return w.b.String(), nil
}

// check rejects a stylesheet that is neither content nor URL, or whose content
// would close the <style> element it is inlined in.
func (s Stylesheet) check() error {
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

// htmlWriter accumulates one rendered document. ids maps each content node's
// named path to the identifier it is addressed by.
type htmlWriter struct {
	b       strings.Builder
	opts    HTMLOptions
	base    string
	ids     map[string]string
	numbers map[string]string
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
	for _, sheet := range w.opts.Stylesheets {
		if sheet.Href != "" {
			w.b.WriteString("<link rel=\"stylesheet\"" + attr("href", sheet.Href) + ">\n")
			continue
		}
		w.b.WriteString("<style>\n" + strings.TrimRight(sheet.Content, "\n") + "\n</style>\n")
	}
	w.b.WriteString("</head>\n<body>\n")
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
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name())
	}
	if len(names) == 0 {
		names = []string{elementColumn}
	}
	w.b.WriteString("<table class=\"sysml-table\"" + attr("id", id) + " data-content=\"table\"" +
		attr(attrName, node.Name()) + attr(attrQuery, node.Query()) +
		attr("data-group-by", node.GroupBy()) + ">\n")
	if node.Caption() != "" {
		w.b.WriteString("<caption class=\"sysml-caption\">" + htmlText(node.Caption()) + "</caption>\n")
	}
	w.writeTableHead(names)
	if node.GroupBy() != "" {
		for _, group := range node.Groups() {
			w.b.WriteString("<tbody class=\"sysml-group\"" + attr("data-group", node.GroupBy()) +
				attr("data-group-key", group.Key()) + ">\n")
			w.b.WriteString("<tr class=\"sysml-group-heading\"><th scope=\"rowgroup\"" +
				attr("colspan", strconv.Itoa(len(names))) + ">" +
				"<span class=\"sysml-group-column\">" + htmlText(node.GroupBy()) + "</span>: " +
				"<span class=\"sysml-group-key\">" + htmlText(group.Key()) + "</span></th></tr>\n")
			w.writeRows(group.Rows(), names, len(columns))
			w.b.WriteString("</tbody>\n")
		}
		w.b.WriteString("</table>\n")
		return
	}
	w.b.WriteString("<tbody>\n")
	w.writeRows(node.Rows(), names, len(columns))
	w.b.WriteString("</tbody>\n</table>\n")
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
func (w *htmlWriter) writeRows(rows []queryexec.Row, names []string, columns int) {
	for _, row := range rows {
		w.b.WriteString("<tr class=\"sysml-row\"" + elementAttrs(row.Element()) + ">\n")
		if columns == 0 {
			w.writeCell([]queryexec.Value{row.Element()}, elementColumn)
			w.b.WriteString(rowEnd)
			continue
		}
		cells := row.Cells()
		for i := 0; i < columns; i++ {
			var values []queryexec.Value
			if i < len(cells) {
				values = cells[i].Values()
			}
			w.writeCell(values, names[i])
		}
		w.b.WriteString(rowEnd)
	}
}

// writeCell writes one projected cell: every value individually addressable,
// with the punctuation joining them an element of its own so a theme can hide
// or replace it.
func (w *htmlWriter) writeCell(values []queryexec.Value, column string) {
	w.b.WriteString("<td class=\"sysml-cell\"" + attr(attrColumn, column) +
		attr("data-value-kind", sharedValueKind(values)) + ">")
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
	w.b.WriteString("<span class=\"" + classes + "\"" + attr("data-value-kind", string(value.Kind())) +
		elementAttrs(value) + quantityAttrs(value) + ">" + htmlText(valueText(value)) + "</span>")
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

// writeDiagram writes one diagram as a figure: a table-kind view as a table,
// every other supported kind as its diagram source — Mermaid, which a loaded
// Mermaid script draws, or DOT for a Graphviz toolchain — shown as text.
func (w *htmlWriter) writeDiagram(node docir.Content, id string) error {
	return w.writeFigure(id, node.Name(), node.Caption(), node.Rendering(), node.Direction(), node.Form())
}

func (w *htmlWriter) writeFigure(id, name, caption string, rendering *view.Rendering, direction view.Direction, form view.Form) error {
	if rendering == nil {
		return &Error{Kind: ErrorMissingRendering, Content: name}
	}
	if rendering.Kind != view.KindTable && !rendering.Kind.Supported() {
		return &Error{Kind: ErrorUnrenderableDiagram, Content: name, Actual: string(rendering.Kind), Form: "HTML"}
	}
	var source string
	if rendering.Kind != view.KindTable {
		var err error
		if form, source, err = diagramSource(name, rendering, direction, form); err != nil {
			return err
		}
	}
	w.b.WriteString("<figure class=\"sysml-diagram\"" + attr("id", id) + " data-content=\"diagram\"" +
		attr(attrName, name) + attr("data-view", rendering.View) +
		attr("data-diagram-kind", string(rendering.Kind)) +
		attr("data-direction", string(direction)) + ">\n")
	if rendering.Kind == view.KindTable {
		w.writeRenderingTable(rendering)
	} else {
		w.b.WriteString("<pre" + attr("class", string(form)) + ">" + html.EscapeString(source) + "</pre>\n")
	}
	if caption != "" {
		w.b.WriteString("<figcaption class=\"sysml-caption\">" + htmlText(caption) + "</figcaption>\n")
	}
	w.b.WriteString("</figure>\n")
	return nil
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
// plain runs as prose, styled runs in <em>, <strong> or <code>, links and
// references as anchors, and element-valued runs carrying their element.
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
	case docir.RunLink:
		if target, ok := navigableURL(run.Target()); ok {
			return "<a class=\"sysml-link\"" + attr("href", target) + ">" + htmlText(run.Text()) + "</a>"
		}
		// A scheme a document must not navigate to is kept as data, not as a link.
		return "<a class=\"sysml-link\"" + attr("data-href", run.Target()) + ">" + htmlText(run.Text()) + "</a>"
	case docir.RunRef:
		return "<a class=\"sysml-ref\"" + attr("href", htmlRefDestination(run)) +
			attr("data-document", run.TargetDocument()) + ">" + htmlText(run.Text()) + "</a>"
	default:
		return htmlText(run.Text())
	}
}

// htmlRefDestination maps a reference run to its destination: an in-document
// anchor, or a relative link into another document's HTML file.
func htmlRefDestination(run docir.TextRun) string {
	if run.TargetDocument() == "" {
		return "#" + run.Target()
	}
	destination := DocumentHTMLFileName(run.TargetDocument())
	if run.Target() != "" {
		destination += "#" + run.Target()
	}
	return destination
}

// DocumentHTMLFileName derives the deterministic HTML file name of a rendered
// document from its fully-qualified name, escaped as anchors are so distinct
// documents never collide.
func DocumentHTMLFileName(fqn string) string {
	return documentFileName(fqn, ".html")
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

// elementAttrs writes the model element behind a row, list item or value: its
// qualified name and its element kind.
func elementAttrs(value queryexec.Value) string {
	element, ok := value.Element()
	if !ok || element == nil {
		return ""
	}
	return attr("data-element", elementID(element)) + attr("data-element-kind", element.Kind.String())
}

// elementID identifies an element by qualified name, falling back to its
// declared name.
func elementID(element *symbols.Symbol) string {
	if fqn := symbols.FQNOf(element); fqn != "" {
		return fqn
	}
	return element.Name
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
