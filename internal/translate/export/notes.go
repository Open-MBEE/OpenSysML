package export

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A note is a `//` or `//* */` note, or a `/* */` comment the grammar admits no
// Comment element for, kept as a sysx:Note node of the element whose own lines hold it.
const (
	xNote        = "note"
	mNote        = "Note"
	xNoteText    = "noteText"
	xNoteToken   = "noteToken"
	xNoteOwnLine = "noteOwnLine"
	xNoteTail    = "noteTail"
	// xNoteTrailing marks a note no code token of its lines follows: it is
	// placed after the last token, whatever the canonical head's count.
	xNoteTrailing = "noteTrailing"
)

// note is one such comment: its text as written, how many of its element's own
// code tokens precede it — in the head, or in the tail closing the body — and
// whether it starts its line.
type note struct {
	text     string
	token    int
	ownLine  bool
	tail     bool
	trailing bool
}

// notes gives every element with lines of its own the notes those lines hold,
// each a node addressed by its position, so they outlive the source text when
// the element is rebuilt from the graph.
func (e *encoder) notes() {
	for _, subject := range e.graph.Subjects() {
		own, ok := e.noteRegions[subject]
		if !ok {
			continue
		}
		head, tail, hasTail := own, region{}, false
		if members, ok := e.noteBodies[subject]; ok {
			head, tail, hasTail = region{own.start, members.start}, region{members.end, own.end}, true
		}
		n := e.noteNodes(subject, head, false, 0)
		if hasTail {
			e.noteNodes(subject, tail, true, n)
		}
	}
}

// noteNodes emits the notes of one region of an element's lines, numbered from n.
func (e *encoder) noteNodes(subject rdf.Term, r region, tail bool, n int) int {
	first, end := e.src.tokensIn(r)
	total := 0
	for k := first; k < end; k++ {
		if !e.src.trivia(k) {
			total++
		}
	}
	tokens := 0
	for k := first; k < end; k++ {
		if !e.src.trivia(k) {
			tokens++
			continue
		}
		slot := fmt.Sprintf("note%d", n)
		node := e.ids.mintedNode(rdf.ExpressionIRI(subject, slot), subject, slot)
		n++
		e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
		e.graph.Add(subject, e.sysx(xNote), node)
		e.graph.Add(node, rdf.IRI(rdf.RDFType), e.sysx(mNote))
		e.graph.Add(node, e.sysx(xNoteText), rdf.String(noteText(e.src.slice(e.src.spans[k]))))
		if tokens == total {
			e.graph.Add(node, e.sysx(xNoteTrailing), rdf.Bool(true))
		} else {
			e.graph.Add(node, e.sysx(xNoteToken), rdf.Int(tokens))
		}
		e.graph.Add(node, e.sysx(xNoteOwnLine), rdf.Bool(e.src.newlineBefore(k)))
		if tail {
			e.graph.Add(node, e.sysx(xNoteTail), rdf.Bool(true))
		}
	}
	return n
}

// tokensIn returns the range of the tokens lying within a region of the text.
func (s *authoredSource) tokensIn(r region) (first, end int) {
	first = sort.Search(len(s.spans), func(k int) bool { return s.spans[k].Offset >= r.start })
	end = first
	for end < len(s.spans) && s.spans[end].End() <= r.end {
		end++
	}
	return first, end
}

// noteText is a note's text without the line terminator a `//` note's token runs on over.
func noteText(text string) string { return strings.TrimRight(text, "\r\n") }

// isLineNote reports whether a note's text is a `//` note, which ends its line.
func isLineNote(text string) bool {
	return strings.HasPrefix(text, "//") && !strings.HasPrefix(text, "//*")
}

// NoteClass is the class of a note node: part of an element's lines, no Element.
const NoteClass = rdf.OpenSysML + mNote

// IsNoteProperty reports whether iri is one of the properties stating a note, which
// say nothing of the model: layout, like the source text they are read from.
func IsNoteProperty(iri string) bool {
	if !strings.HasPrefix(iri, rdf.OpenSysML) {
		return false
	}
	switch rdf.LocalName(iri) {
	case xNote, xNoteText, xNoteToken, xNoteOwnLine, xNoteTail, xNoteTrailing:
		return true
	}
	return false
}

// isNote reports whether a subject is a sysx:Note node: part of an element's
// lines, written back with them, and no element or expression of its own.
func (d *decoder) isNote(subject rdf.Term) bool { return d.metaclass(subject) == mNote }

// notesOf lists an element's notes as the graph states them.
func (d *decoder) notesOf(el *element) []note { return notesIn(d.graph, rdf.IRI(el.iri)) }

// notesIn lists a subject's notes in a graph: those of the head before those of
// the tail, each in the order of the tokens they follow, notes at one token as stated.
func notesIn(g *rdf.Graph, subject rdf.Term) []note {
	var out []note
	for _, node := range g.Objects(subject, rdf.OpenSysML+xNote) {
		text, ok := g.Lexical(node, rdf.OpenSysML+xNoteText)
		if !ok {
			continue
		}
		out = append(out, note{
			text:     text,
			token:    intOf(g, node, rdf.OpenSysML+xNoteToken),
			ownLine:  g.BoolValue(node, rdf.OpenSysML+xNoteOwnLine),
			tail:     g.BoolValue(node, rdf.OpenSysML+xNoteTail),
			trailing: g.BoolValue(node, rdf.OpenSysML+xNoteTrailing),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].tail != out[j].tail {
			return out[j].tail
		}
		if out[i].trailing != out[j].trailing {
			return out[j].trailing
		}
		return out[i].token < out[j].token
	})
	return out
}

// notedSubjects lists the subjects the graph states notes of, in the graph's order.
func notedSubjects(g *rdf.Graph) []rdf.Term {
	var out []rdf.Term
	seen := map[string]bool{}
	for _, t := range g.Triples() {
		if t.Predicate.Value == rdf.OpenSysML+xNote && !seen[t.Subject.Value] {
			seen[t.Subject.Value] = true
			out = append(out, t.Subject)
		}
	}
	return out
}

// compareNotes matches the notes stated to those derived in order, each in every field
// (text, token, line, head or tail), and lists the stated and the derived left unmatched.
func compareNotes(stated, derived []note) (stale, unstated []note) {
	i := 0
	for _, m := range derived {
		if i < len(stated) && m == stated[i] {
			i++
		} else {
			unstated = append(unstated, m)
		}
	}
	return stated[i:], unstated
}

// notesWritten lists the notes written with an element: those of a succession
// folded into it as its `then` prefix, then its own.
func (d *decoder) notesWritten(el *element) []note {
	var out []note
	for edge, target := range d.folded {
		if target == el {
			out = append(out, d.notesOf(edge)...)
		}
	}
	return append(out, d.notesOf(el)...)
}

// commentTexts lists the notes and comments of a text in order, as written.
func commentTexts(text string) []string {
	var out []string
	lx := lexer.New(source.New("notes.sysml", []byte(text)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if isComment(tok.Kind) {
			out = append(out, noteText(text[tok.Span.Offset:tok.Span.End()]))
		}
	}
	return out
}

// insertion is a piece of text put into a rebuilt notation at an offset of it,
// in place of the skip bytes there: the blanks a note moves to its own line.
type insertion struct {
	at   int
	text string
	skip int
}

// withNotes puts an element's notes back into its rebuilt notation: each after
// the code token it followed in the head, or in the tail after the body members
// at body, and on its own line or inline as it was written. A note past the
// tokens the canonical head has is placed after its last. The insertions are
// returned so what was written after them can be relocated.
func withNotes(text string, notes []note, body region, hasBody bool, indent, nl string) (string, []insertion) {
	var head, tail []note
	for _, n := range notes {
		if n.tail && hasBody {
			tail = append(tail, n)
		} else {
			head = append(head, n)
		}
	}
	headEnd := len(text)
	if hasBody {
		headEnd = body.start
	}
	inner := indent + "    "
	head = unwritten(head, text[:headEnd])
	if hasBody {
		tail = unwritten(tail, text[body.end:])
	}
	ins := placeNotes(text[:headEnd], head, 0, func(n int, last bool) string {
		// A note ahead of the head, or after a bodiless declaration, lines up with it.
		if n == 0 || last && !hasBody {
			return indent
		}
		return inner
	}, nl)
	if hasBody {
		ins = append(ins, placeNotes(text[body.end:], tail, body.end, func(n int, _ bool) string {
			if n == 0 {
				return inner
			}
			return indent
		}, nl)...)
	}
	var b strings.Builder
	last := 0
	for _, in := range ins {
		b.WriteString(text[last:in.at])
		b.WriteString(in.text)
		last = in.at + in.skip
	}
	b.WriteString(text[last:])
	return b.String(), ins
}

// unwritten drops the notes a part of the rebuilt notation holds already, in
// order: those inside an expression kept as its text was written.
func unwritten(notes []note, part string) []note {
	var out []note
	comments := commentTexts(part)
	for _, n := range notes {
		if i := slices.Index(comments, n.text); i >= 0 {
			comments = comments[i+1:]
			continue
		}
		out = append(out, n)
	}
	return out
}

// placeNotes computes the insertions of notes into one part of the notation starting
// at base; indentOf indents a note on its own line after n tokens, last when none follows.
func placeNotes(part string, notes []note, base int, indentOf func(n int, last bool) string, nl string) []insertion {
	tokens := codeTokens(part)
	var out []insertion
	for _, n := range notes {
		count := min(n.token, len(tokens))
		if n.trailing {
			count = len(tokens)
		}
		at := len(part)
		if count == 0 && len(tokens) > 0 {
			at = tokens[0].Offset
		} else if count > 0 {
			at = tokens[count-1].End()
		}
		ind := indentOf(count, count == len(tokens))
		rest := part[at:]
		// Only indentation ahead of it on its line puts the note at the line's start.
		lineBegin := strings.LastIndexByte(part[:at], '\n') + 1
		lineStart := strings.TrimLeft(part[lineBegin:at], " \t") == ""
		restBreaks := strings.HasPrefix(rest, "\n") || strings.HasPrefix(rest, "\r\n")
		// A `//` note ahead of every token would run on over the first; it takes its line.
		ownLine := n.ownLine || isLineNote(n.text) && count == 0 && len(tokens) > 0
		switch {
		case ownLine && lineStart:
			out = append(out, insertion{base + lineBegin, ind + n.text + nl, 0})
		case ownLine:
			// The rest of the line moves below the note, less the blanks it began with.
			blanks := len(rest) - len(strings.TrimLeft(rest, " \t"))
			after := ""
			if !restBreaks {
				after = nl + ind
			}
			out = append(out, insertion{base + at, nl + ind + n.text + after, blanks})
		case count == 0 && len(tokens) > 0:
			out = append(out, insertion{base + at, n.text + " ", 0})
		case isLineNote(n.text) && !restBreaks && len(rest) > 0:
			blanks := len(rest) - len(strings.TrimLeft(rest, " \t"))
			out = append(out, insertion{base + at, " " + n.text + nl + ind, blanks})
		default:
			out = append(out, insertion{base + at, " " + n.text, 0})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at < out[j].at })
	return out
}

// codeTokens is the spans of a text's code tokens: a `/* */` here is a body,
// since a rebuilt notation holds no note before its notes are put back.
func codeTokens(text string) []source.Span {
	var out []source.Span
	lx := lexer.New(source.New("notes.sysml", []byte(text)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind != lexer.Whitespace && tok.Kind != lexer.SLNote && tok.Kind != lexer.MLNote {
			out = append(out, tok.Span)
		}
	}
	return out
}

// relocate maps an offset of a notation onto the same place once the insertions
// were made ahead of it; one made exactly there goes ahead when inclusive.
func relocate(offset int, ins []insertion, inclusive bool) int {
	moved := offset
	for _, in := range ins {
		if in.at < offset || inclusive && in.at == offset {
			moved += len(in.text) - min(in.skip, max(offset-in.at, 0))
		}
	}
	return moved
}
