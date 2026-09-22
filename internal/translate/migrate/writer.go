package migrate

import "strings"

// writer accumulates indented lines of notation. Each open block writes into a
// buffer of its own, so deciding whether a body is empty never copies the
// output written before it.
type writer struct {
	bufs   []*strings.Builder
	indent int
}

func (w *writer) buf() *strings.Builder {
	if len(w.bufs) == 0 {
		w.bufs = append(w.bufs, &strings.Builder{})
	}
	return w.bufs[len(w.bufs)-1]
}

func (w *writer) line(s string) {
	b := w.buf()
	if s == "" {
		b.WriteByte('\n')
		return
	}
	b.WriteString(strings.Repeat("    ", w.indent))
	b.WriteString(s)
	b.WriteByte('\n')
}

func (w *writer) lines(ls []string) {
	for _, l := range ls {
		w.line(l)
	}
}

// block writes header with a brace-delimited body, or as `header;` when the
// body writes nothing.
func (w *writer) block(header string, body func()) {
	w.enclose(header, ";", body)
}

// braced writes header with a brace-delimited body, `header { }` when the body
// writes nothing: for a clause the grammar continues after, which `;` would end.
func (w *writer) braced(header string, body func()) {
	w.enclose(header, " { }", body)
}

// enclose writes header with a brace-delimited body, or header followed by
// empty when the body writes nothing.
func (w *writer) enclose(header, empty string, body func()) {
	w.trailed(header, empty, "", body)
}

// trailed writes header followed by empty when the body writes nothing, else
// header with a brace-delimited body opened by the lead line, when there is one.
func (w *writer) trailed(header, empty, lead string, body func()) {
	inner := w.capture(body)
	if inner == "" {
		w.line(header + empty)
		return
	}
	w.line(header + " {")
	if lead != "" {
		w.indented(func() { w.line(lead) })
	}
	_, _ = w.buf().WriteString(inner)
	w.line("}")
}

// capture renders what body writes, one level deeper, without writing it.
func (w *writer) capture(body func()) string {
	w.buf()
	w.bufs = append(w.bufs, &strings.Builder{})
	w.indent++
	body()
	w.indent--
	inner := w.bufs[len(w.bufs)-1].String()
	w.bufs = w.bufs[:len(w.bufs)-1]
	return inner
}

// indented writes body one level deeper, for a clause continued on the next lines.
func (w *writer) indented(body func()) {
	w.indent++
	body()
	w.indent--
}

func (w *writer) String() string { return w.buf().String() }
