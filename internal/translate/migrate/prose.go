package migrate

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

// A tool stores documentation as HTML; a comment body's runs are its text and
// the cross-references in it: a View Editor <mms-cf> element, whose type says
// what of the element it shows (its name, a value or its documentation) and
// whose body is the editor's cached text, the editor's <mms-view-link> to a
// view, and a MagicDraw element hyperlink, <a href="mdel://id">, whose anchor
// text is the name at the time of writing. The editor also writes a view
// link as a hyperlink to its own page for the view, named by id in the URL.
const (
	cfName   = "name"
	cfValue  = "val"
	cfDoc    = "com"
	cfView   = "vlink"
	mdelLink = "mdel://"
)

// proseRun is one run of a comment body: text, or a cross-reference holding
// the text the tool cached for it.
type proseRun struct {
	text string
	// id is the xmi:id the reference names; "" for text.
	id string
	// cf is the <mms-cf> type, name, val or com, or vlink for a view link;
	// "" for an element hyperlink.
	cf string
	// link marks an <a href="mdel://…"> element hyperlink.
	link bool
}

// isRef reports whether the run is a cross-reference.
func (r proseRun) isRef() bool { return r.id != "" }

var (
	cfPlaceholder = regexp.MustCompile(`^\[cf:(.*)\.(\w*)\]$`)
	spaceRuns     = regexp.MustCompile(`[ \t\f\r\x{00A0}]+`)
	breakSpaces   = regexp.MustCompile(` ?\n ?`)
)

// cfFallback is the text a dangling <mms-cf> prints: the label of its
// `[cf:label.type]` placeholder, or its cached text when the body is of
// another form.
func cfFallback(text string) string {
	if match := cfPlaceholder.FindStringSubmatch(text); match != nil {
		return strings.TrimSpace(match[1])
	}
	return text
}

// isHTML reports whether a body is marked up, by the tags tools write.
func isHTML(body string) bool {
	lower := strings.ToLower(body)
	for _, tag := range []string{"<html", "<p>", "<p ", "<br", "<style", "<script", "<img", "<div", "<span", "<mms-", "<a "} {
		if strings.Contains(lower, tag) {
			return true
		}
	}
	return false
}

// parseProse splits a comment body into runs: text, with a paragraph end or
// line break as a newline, and the cross-references it holds. Every other
// tag is dropped, with a style or script element's content; entities are
// decoded. A plain-text body is one run as written.
func parseProse(body string) []proseRun {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !isHTML(body) {
		return tidyRuns([]proseRun{{text: body}}, false)
	}
	p := &proseParser{src: body}
	p.parse()
	return tidyRuns(p.runs, true)
}

// proseParser scans HTML once, left to right.
type proseParser struct {
	src  string
	pos  int
	runs []proseRun
	// ref is the reference whose body is being read, nil outside one.
	ref *proseRun
	// refEnd is the tag closing ref.
	refEnd string
	text   strings.Builder
}

// parse reads the whole source.
func (p *proseParser) parse() {
	for p.pos < len(p.src) {
		i := strings.IndexByte(p.src[p.pos:], '<')
		if i < 0 {
			p.write(p.src[p.pos:])
			p.pos = len(p.src)
			break
		}
		p.write(p.src[p.pos : p.pos+i])
		p.pos += i
		p.tag()
	}
	p.flush()
}

// write adds decoded text to the run being built.
func (p *proseParser) write(raw string) {
	p.text.WriteString(html.UnescapeString(raw))
}

// flush ends the text run being built, if any.
func (p *proseParser) flush() {
	if p.text.Len() == 0 {
		return
	}
	p.runs = append(p.runs, proseRun{text: p.text.String()})
	p.text.Reset()
}

// tag reads the tag at pos and acts on it.
func (p *proseParser) tag() {
	if strings.HasPrefix(p.src[p.pos:], "<!--") {
		end := strings.Index(p.src[p.pos+4:], "-->")
		if end < 0 {
			p.pos = len(p.src)
			return
		}
		p.pos += 4 + end + 3
		return
	}
	raw, next := tagEnd(p.src, p.pos)
	p.pos = next
	name, closing, attrs := tagParts(raw)
	switch {
	case p.ref != nil && closing && name == p.refEnd:
		p.ref.text = p.text.String()
		p.text.Reset()
		p.runs = append(p.runs, *p.ref)
		p.ref = nil
	case name == "style", name == "script":
		if !closing {
			p.skipElement(name)
		}
	case name == "br", closing && name == "p":
		p.text.WriteByte('\n')
	case p.ref != nil:
	case name == "mms-cf" && !closing:
		if id := attrs["mms-element-id"]; id != "" {
			p.open(proseRun{id: id, cf: strings.ToLower(attrs["mms-cf-type"])}, name)
		}
	case name == "mms-view-link" && !closing:
		if id := attrs["data-mms-element-id"]; id != "" {
			p.open(proseRun{id: id, cf: cfView}, name)
		}
	case name == "a" && !closing:
		if id := linkedID(attrs["href"]); id != "" {
			p.open(proseRun{id: id, link: true}, name)
		}
	}
}

// linkedID is the id of the element a hyperlink names: a MagicDraw element
// link, mdel://id, or a View Editor view page, whose URL carries the view's id
// as viewId=id (in the query, or in the query of its #/… route) or as the
// path step after views/.
func linkedID(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(strings.ToLower(href), mdelLink) {
		return strings.TrimSpace(href[len(mdelLink):])
	}
	u, err := url.Parse(href)
	if err != nil || u.Host == "" {
		return ""
	}
	if id := u.Query().Get("viewId"); id != "" {
		return id
	}
	route := u.Fragment
	if i := strings.IndexByte(route, '?'); i >= 0 {
		if q, err := url.ParseQuery(route[i+1:]); err == nil && q.Get("viewId") != "" {
			return q.Get("viewId")
		}
		route = route[:i]
	}
	for _, path := range []string{route, u.Path} {
		if i := strings.Index(path, "/views/"); i >= 0 {
			if id, _, _ := strings.Cut(path[i+len("/views/"):], "/"); id != "" {
				return id
			}
		}
	}
	return ""
}

// open starts reading a reference's body, closed by the tag named end.
func (p *proseParser) open(ref proseRun, end string) {
	p.flush()
	p.ref = &ref
	p.refEnd = end
}

// skipElement drops everything up to the closing tag of name.
func (p *proseParser) skipElement(name string) {
	i := strings.Index(strings.ToLower(p.src[p.pos:]), "</"+name)
	if i < 0 {
		p.pos = len(p.src)
		return
	}
	_, p.pos = tagEnd(p.src, p.pos+i)
}

// tagEnd returns the tag starting at pos, which may hold a quoted '>', and
// the position after it; an unterminated tag runs to the end.
func tagEnd(src string, pos int) (string, int) {
	quote := byte(0)
	for i := pos + 1; i < len(src); i++ {
		switch ch := src[i]; {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '"', ch == '\'':
			quote = ch
		case ch == '>':
			return src[pos : i+1], i + 1
		}
	}
	return src[pos:], len(src)
}

var attrRe = regexp.MustCompile(`([^\s=/<>"']+)\s*(?:=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+)))?`)

// tagParts reads a tag's lowercased name, whether it closes an element, and
// its attributes by lowercased name, entities decoded.
func tagParts(tag string) (name string, closing bool, attrs map[string]string) {
	inner := strings.TrimSuffix(strings.TrimPrefix(tag, "<"), ">")
	inner = strings.TrimSuffix(strings.TrimSpace(inner), "/")
	if strings.HasPrefix(inner, "/") {
		closing = true
		inner = inner[1:]
	}
	matches := attrRe.FindAllStringSubmatch(inner, -1)
	attrs = map[string]string{}
	for i, match := range matches {
		if i == 0 {
			name = strings.ToLower(match[1])
			continue
		}
		value := match[2] + match[3] + match[4]
		attrs[strings.ToLower(match[1])] = html.UnescapeString(value)
	}
	return name, closing, attrs
}

// tidyRuns merges adjacent text runs and normalizes the whitespace of a
// body: runs of spaces, which dropped tags leave, collapse to one in marked
// up text; a line break drops the spaces around it; more than one blank
// line is one; and the text starts and ends on a non-blank. A reference's
// cached text is one line, trimmed.
func tidyRuns(runs []proseRun, markedUp bool) []proseRun {
	var out []proseRun
	for _, r := range runs {
		if r.isRef() {
			r.text = strings.Join(strings.Fields(r.text), " ")
			out = append(out, r)
			continue
		}
		if n := len(out); n > 0 && !out[n-1].isRef() {
			out[n-1].text += r.text
			continue
		}
		out = append(out, r)
	}
	for i := range out {
		if out[i].isRef() {
			continue
		}
		out[i].text = tidyText(out[i].text, markedUp)
	}
	if n := len(out); n > 0 {
		if !out[0].isRef() {
			out[0].text = strings.TrimLeft(out[0].text, " \t\n")
		}
		if !out[n-1].isRef() {
			out[n-1].text = strings.TrimRight(out[n-1].text, " \t\n")
		}
	}
	kept := out[:0]
	for _, r := range out {
		if r.isRef() || r.text != "" {
			kept = append(kept, r)
		}
	}
	return kept
}

// tidyText normalizes one text run's whitespace, see tidyRuns.
func tidyText(text string, markedUp bool) string {
	if markedUp {
		text = spaceRuns.ReplaceAllString(text, " ")
		text = breakSpaces.ReplaceAllString(text, "\n")
	}
	return blankLines.ReplaceAllString(text, "\n\n")
}

// joinText appends b to a, closing up the double space left where a
// reference between them rendered as nothing.
func joinText(a, b string) string {
	if strings.HasSuffix(a, " ") && strings.HasPrefix(b, " ") {
		b = strings.TrimLeft(b, " ")
	}
	return a + b
}

// proseText flattens runs to text, each reference rendered by render. A
// plain-text body, one run as written, keeps its spacing; marked-up runs
// have theirs normalized as they are joined, as the dropped tags left it.
func proseText(runs []proseRun, render func(proseRun) string) string {
	if len(runs) == 1 && !runs[0].isRef() {
		return runs[0].text
	}
	var b strings.Builder
	for _, r := range runs {
		if r.isRef() {
			b.WriteString(render(r))
			continue
		}
		b.WriteString(r.text)
	}
	return strings.TrimSpace(tidyText(b.String(), true))
}

// cachedText is the text a reference prints with no model to resolve it in:
// the hyperlink's anchor text, or the <mms-cf>'s fallback.
func cachedText(r proseRun) string {
	if r.link {
		return r.text
	}
	return cfFallback(r.text)
}
