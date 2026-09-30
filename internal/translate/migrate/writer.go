package migrate

import (
	"strconv"
	"strings"
)

// writer accumulates indented lines of notation. Each open block writes into a
// buffer of its own, so deciding whether a body is empty never copies the
// output written before it.
type writer struct {
	bufs   []*buffer
	indent int
	// free holds the buffers of closed blocks, for the blocks opened next.
	free []*buffer
	// holes are the gaps left for text written once the rest of the document is.
	holes []hole
	// marker writes the line closing a body whose members it lists were declared
	// under made-up names, if any body is; nil writes none.
	marker func(names []string) string
	// standInMarker writes the line closing a body whose members it lists stand
	// for no source element; nil writes none.
	standInMarker func(names []string) string
}

// buffer is the text of one open block and the names it declared that were
// made up, and of those standing for no source element, to be marked before
// the block closes.
type buffer struct {
	strings.Builder
	madeUp   []string
	standIns []string
}

// hole is a gap in the output: what fills it, at what indent, and the text once filled.
type hole struct {
	indent int
	fill   func()
	text   string
}

// holeMark delimits a hole's marker line in the output until it is filled.
const holeMark = "\x00"

// hole leaves a gap at the current position that fill writes into, at this
// indent, when the document is otherwise complete and fill is called.
func (w *writer) hole(fill func()) {
	_, _ = w.buf().WriteString(holeMark + strconv.Itoa(len(w.holes)) + holeMark + "\n")
	w.holes = append(w.holes, hole{indent: w.indent, fill: fill})
}

// fill writes every hole's text, each as its fill writes at the hole's indent.
func (w *writer) fill() {
	for i := range w.holes {
		h := &w.holes[i]
		w.open()
		indent := w.indent
		w.indent = h.indent
		h.fill()
		h.text = w.close()
		w.indent = indent
	}
}

// filled is the output with every hole's marker line replaced by its text.
func (w *writer) filled(out string) string {
	if len(w.holes) == 0 {
		return out
	}
	var b strings.Builder
	for {
		start := strings.Index(out, holeMark)
		if start < 0 {
			b.WriteString(out)
			return b.String()
		}
		end := start + 1 + strings.Index(out[start+1:], holeMark)
		i, _ := strconv.Atoi(out[start+1 : end])
		b.WriteString(out[:start])
		b.WriteString(w.holes[i].text)
		out = out[end+2:]
	}
}

func (w *writer) buf() *buffer {
	if len(w.bufs) == 0 {
		w.bufs = append(w.bufs, &buffer{})
	}
	return w.bufs[len(w.bufs)-1]
}

// madeUp records that the block being written declared name for an element
// its source left anonymous; the block is marked as it closes.
func (w *writer) madeUp(name string) {
	b := w.buf()
	b.madeUp = append(b.madeUp, name)
}

// standIn records that the block being written declared name for a member
// standing for no source element; the block is marked as it closes.
func (w *writer) standIn(name string) {
	b := w.buf()
	b.standIns = append(b.standIns, name)
}

// markMadeUp writes the markers for the names the open block made up so far,
// if any, so that the block need not: for a body whose writer knows what the
// markers may refer to as. The block's stand-ins are marked after its made-up names.
func (w *writer) markMadeUp() {
	b := w.buf()
	if len(b.madeUp) > 0 && w.marker != nil {
		names := b.madeUp
		b.madeUp = nil
		w.line(w.marker(names))
	}
	if len(b.standIns) > 0 && w.standInMarker != nil {
		names := b.standIns
		b.standIns = nil
		w.line(w.standInMarker(names))
	}
}

// open pushes a block over the current one, reusing the buffer of a closed block.
func (w *writer) open() {
	var b *buffer
	if n := len(w.free); n > 0 {
		b, w.free = w.free[n-1], w.free[:n-1]
	} else {
		b = &buffer{}
	}
	w.bufs = append(w.bufs, b)
}

// close pops the innermost block, marking the names it made up that no body
// marked, and returns its text.
func (w *writer) close() string {
	w.markMadeUp()
	b := w.bufs[len(w.bufs)-1]
	w.bufs = w.bufs[:len(w.bufs)-1]
	text := b.String()
	*b = buffer{}
	w.free = append(w.free, b)
	return text
}

func (w *writer) line(s string) {
	if s == "" {
		_ = w.buf().WriteByte('\n')
		return
	}
	w.lineOf(s, "")
}

// lineOf writes a line of two parts at the current indent.
func (w *writer) lineOf(head, tail string) {
	b := &w.buf().Builder
	indent := indentOf(w.indent)
	b.Grow(len(indent) + len(head) + len(tail) + 1)
	b.WriteString(indent)
	b.WriteString(head)
	b.WriteString(tail)
	b.WriteByte('\n')
}

// indents are the indentations of the first levels, spelled once.
var indents = func() (out [16]string) {
	for i := range out {
		out[i] = strings.Repeat("    ", i)
	}
	return out
}()

func indentOf(level int) string {
	if level < len(indents) {
		return indents[level]
	}
	return strings.Repeat("    ", level)
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
		w.lineOf(header, empty)
		return
	}
	w.lineOf(header, " {")
	if lead != "" {
		w.indented(func() { w.line(lead) })
	}
	_, _ = w.buf().WriteString(inner)
	w.line("}")
}

// capture renders what body writes, one level deeper, without writing it.
func (w *writer) capture(body func()) string {
	w.indent++
	inner := w.aside(body)
	w.indent--
	return inner
}

// captureAt renders what body writes at the current indent, without writing
// it: for a body continued on lines already inside the braces.
func (w *writer) captureAt(body func()) string {
	w.buf()
	w.open()
	body()
	return w.close()
}

// aside renders what body writes, at the current level, without writing it.
func (w *writer) aside(body func()) string {
	w.buf()
	w.open()
	body()
	return w.close()
}

// indented writes body one level deeper, for a clause continued on the next lines.
func (w *writer) indented(body func()) {
	w.indent++
	body()
	w.indent--
}

// String is the document: the root block, marked for the names it made up
// like any block, with every hole filled.
func (w *writer) String() string {
	w.markMadeUp()
	return w.filled(w.buf().String())
}
