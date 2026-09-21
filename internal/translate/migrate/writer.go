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
	w.blockOr(header, header+";", body)
}

// blockOr writes header with a brace-delimited body, or the line alone when the
// body writes nothing.
func (w *writer) blockOr(header, alone string, body func()) {
	w.buf()
	w.bufs = append(w.bufs, &strings.Builder{})
	w.indent++
	body()
	w.indent--
	inner := w.bufs[len(w.bufs)-1].String()
	w.bufs = w.bufs[:len(w.bufs)-1]
	if inner == "" {
		w.line(alone)
		return
	}
	w.line(header + " {")
	_, _ = w.buf().WriteString(inner)
	w.line("}")
}

// indented writes body one level deeper, for a clause continued on the next lines.
func (w *writer) indented(body func()) {
	w.indent++
	body()
	w.indent--
}

func (w *writer) String() string { return w.buf().String() }
