package docpdf

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

// Options are the deliverable-document choices the PDF backend offers. They
// are presentation options of this backend, never document-model attributes.
type Options struct {
	// TitlePage puts the document title on a page of its own.
	TitlePage bool

	// TOC writes a table of contents ahead of the content.
	TOC bool

	// NumberSections numbers the section headings hierarchically.
	NumberSections bool
}

// documentHTML writes the parsed document as one standalone, deterministic
// HTML page for a converter to lay out. Mermaid blocks reference the files
// named in images, in block order; DOT blocks are kept as source.
func documentHTML(blocks []block, images []string, opts Options) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<title>" + html.EscapeString(documentTitle(blocks)) + "</title>\n")
	b.WriteString("<style>\n" + styleSheet + "</style>\n</head>\n<body>\n")
	writeTitle(&b, blocks, opts)
	if opts.TOC {
		writeTOC(&b, blocks, opts)
	}
	writeContent(&b, blocks, images, opts)
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// documentTitle is the text of the document's title heading: the level-1
// heading docrender writes first.
func documentTitle(blocks []block) string {
	return unescape(titleHeading(blocks))
}

// titleHeading is the still-escaped text of the document's title heading.
func titleHeading(blocks []block) string {
	for _, blk := range blocks {
		if blk.Kind == blockHeading && blk.Level == 1 {
			return blk.Text
		}
	}
	return ""
}

// writeTitle writes the document title: on a page of its own when the title
// page was asked for, and as the opening heading otherwise.
func writeTitle(b *strings.Builder, blocks []block, opts Options) {
	title := inlineHTML(titleHeading(blocks))
	if opts.TitlePage {
		b.WriteString("<div class=\"title-page\"><h1>" + title + "</h1></div>\n")
		return
	}
	b.WriteString("<h1>" + title + "</h1>\n")
}

// writeTOC writes the table of contents: every section heading, numbered as
// the headings themselves are, linked to its anchor.
func writeTOC(b *strings.Builder, blocks []block, opts Options) {
	entries := false
	var counters []int
	for _, blk := range blocks {
		if blk.Kind != blockHeading || blk.Level < 2 {
			continue
		}
		if !entries {
			b.WriteString("<nav class=\"toc\">\n<h2>Contents</h2>\n<ul>\n")
			entries = true
		}
		number, anchor := headingNumber(&counters, blk.Level)
		label := inlineHTML(blk.Text)
		if opts.NumberSections {
			label = number + " " + label
		}
		b.WriteString(fmt.Sprintf("<li class=\"toc-%d\"><a href=\"#%s\">%s</a></li>\n", blk.Level-1, anchor, label))
	}
	if entries {
		b.WriteString("</ul>\n</nav>\n")
	}
}

// writeContent writes every block after the title heading.
func writeContent(b *strings.Builder, blocks []block, images []string, opts Options) {
	var counters []int
	image := 0
	seenTitle := false
	for _, blk := range blocks {
		switch blk.Kind {
		case blockHeading:
			if blk.Level == 1 && !seenTitle {
				seenTitle = true
				continue
			}
			number, anchor := headingNumber(&counters, blk.Level)
			label := inlineHTML(blk.Text)
			if opts.NumberSections {
				label = "<span class=\"section-number\">" + number + "</span> " + label
			}
			level := blk.Level
			b.WriteString(fmt.Sprintf("<h%d id=\"%s\">%s</h%d>\n", level, anchor, label, level))
		case blockParagraph:
			b.WriteString("<p>" + inlineHTML(blk.Text) + "</p>\n")
		case blockCaption:
			b.WriteString("<p class=\"caption\"><em>" + inlineHTML(blk.Text) + "</em></p>\n")
		case blockAnchor:
			b.WriteString(`<a id="` + html.EscapeString(blk.Anchor) + `"></a>` + "\n")
		case blockTable:
			writeTable(b, blk)
		case blockList:
			writeList(b, blk)
		case blockMermaid:
			if image < len(images) {
				b.WriteString("<figure><img src=\"" + html.EscapeString(images[image]) + "\" alt=\"diagram\"></figure>\n")
				image++
			}
		case blockDOT:
			b.WriteString("<figure class=\"dot\"><p class=\"notice\"><em>" + html.EscapeString(dotNotice) + "</em></p>\n" +
				"<pre>" + html.EscapeString(blk.Source) + "</pre></figure>\n")
		}
	}
}

// headingNumber advances the hierarchical counters for a heading at the given
// level and returns its dotted number and anchor. Level 2 is the outermost
// numbered level, matching how docrender nests sections under the title.
func headingNumber(counters *[]int, level int) (string, string) {
	depth := level - 1
	for len(*counters) < depth {
		*counters = append(*counters, 0)
	}
	*counters = (*counters)[:depth]
	(*counters)[depth-1]++
	parts := make([]string, depth)
	for i, c := range *counters {
		parts[i] = strconv.Itoa(c)
	}
	return strings.Join(parts, "."), "sec-" + strings.Join(parts, "-")
}

// writeTable writes one table, headers and body rows in document order.
func writeTable(b *strings.Builder, blk block) {
	b.WriteString("<table>\n<thead>\n<tr>\n")
	for _, cell := range blk.Header {
		b.WriteString("<th>" + cellHTML(cell) + "</th>\n")
	}
	b.WriteString("</tr>\n</thead>\n<tbody>\n")
	for _, row := range blk.Rows {
		b.WriteString("<tr>\n")
		for _, cell := range row {
			b.WriteString("<td>" + cellHTML(cell) + "</td>\n")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n")
}

// writeList writes one bullet or numbered list.
func writeList(b *strings.Builder, blk block) {
	tag := "ul"
	if blk.Ordered {
		tag = "ol"
	}
	b.WriteString("<" + tag + ">\n")
	for _, item := range blk.Items {
		b.WriteString("<li>" + inlineHTML(item) + "</li>\n")
	}
	b.WriteString("</" + tag + ">\n")
}

// cellHTML writes one table cell. Every literal metacharacter in a cell is
// Markdown-escaped, so an unescaped <br> is docrender's fold of a newline and
// is kept as the line break it stands for.
func cellHTML(cell string) string {
	parts := strings.Split(cell, "<br>")
	for i, part := range parts {
		parts[i] = inlineHTML(part)
	}
	return strings.Join(parts, "<br>")
}

// styleSheet lays the document out for print: pages numbered in the footer,
// bordered tables, captions in small type, diagrams scaled to the text width,
// and a title page and table of contents on pages of their own when written.
const styleSheet = `@page {
  margin: 2.2cm 2.2cm;
  @bottom-center { content: counter(page); font-size: 9pt; color: #444444; }
}
body { font-family: serif; font-size: 11pt; line-height: 1.45; }
h1, h2, h3, h4, h5, h6 { font-family: sans-serif; line-height: 1.2; }
h1 { font-size: 20pt; }
h2 { font-size: 15pt; margin-top: 1.4em; }
h3 { font-size: 13pt; margin-top: 1.2em; }
h4, h5, h6 { font-size: 11pt; margin-top: 1em; }
.title-page { page-break-after: always; text-align: center; padding-top: 35%; }
.title-page h1 { font-size: 26pt; }
nav.toc { page-break-after: always; }
nav.toc ul { list-style: none; padding-left: 0; }
nav.toc li { margin: 0.25em 0; }
nav.toc li.toc-2 { padding-left: 1.5em; }
nav.toc li.toc-3 { padding-left: 3em; }
nav.toc li.toc-4 { padding-left: 4.5em; }
nav.toc li.toc-5 { padding-left: 6em; }
nav.toc a { text-decoration: none; color: inherit; }
table { border-collapse: collapse; margin: 0.8em 0; }
th, td { border: 0.5pt solid #666666; padding: 0.3em 0.6em; text-align: left; }
th { background: #eeeeee; }
p.caption, span.caption { font-size: 9.5pt; color: #444444; }
figure { margin: 0.8em 0; }
figure img { max-width: 100%; }
figure.dot pre { font-size: 9pt; white-space: pre-wrap; }
p.notice { font-size: 9.5pt; color: #444444; }
`
