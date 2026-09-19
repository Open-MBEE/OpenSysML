package export

import (
	"bytes"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// authoredSource is the parsed document's bytes as written, which is the
// notation the graph carries as sysx:sourceText, with its significant tokens.
type authoredSource struct {
	text  []byte
	kinds []lexer.Kind
	spans []source.Span
}

func newAuthoredSource(file *source.SourceFile) *authoredSource {
	s := &authoredSource{text: file.Bytes()}
	s.kinds, s.spans = significantTokens(file)
	return s
}

func significantTokens(file *source.SourceFile) ([]lexer.Kind, []source.Span) {
	lx := lexer.New(file)
	var kinds []lexer.Kind
	var spans []source.Span
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return kinds, spans
		}
		if tok.Kind != lexer.Whitespace {
			kinds = append(kinds, tok.Kind)
			spans = append(spans, tok.Span)
		}
	}
}

// index returns the position of the token holding an offset, or of the token
// before that offset when it falls between two.
func (s *authoredSource) index(offset int) int {
	return sort.Search(len(s.spans), func(i int) bool { return s.spans[i].Offset > offset }) - 1
}

// slice returns the text of a span as written.
func (s *authoredSource) slice(span source.Span) string {
	return string(s.text[span.Offset:span.End()])
}

// lastToken returns the offset of the last code token in a span spelled as
// word, or -1: a comment saying `from` is not the verb `from`.
func (s *authoredSource) lastToken(span source.Span, word string) int {
	for i := s.index(span.End() - 1); i >= 0 && s.spans[i].Offset >= span.Offset; i-- {
		if !isComment(s.kinds[i]) && s.slice(s.spans[i]) == word {
			return s.spans[i].Offset
		}
	}
	return -1
}

// code returns the text of a span up to its last code token: a node's span runs
// on over the notes and comments after it, which are not part of what it says.
func (s *authoredSource) code(span source.Span) string {
	last := s.index(span.End() - 1)
	for last >= 0 && s.spans[last].Offset >= span.Offset && isComment(s.kinds[last]) {
		last--
	}
	if last < 0 || s.spans[last].Offset < span.Offset {
		return ""
	}
	return string(s.text[span.Offset:min(span.End(), s.spans[last].End())])
}

func isComment(kind lexer.Kind) bool {
	return kind == lexer.SLNote || kind == lexer.MLNote || kind == lexer.RegularComment
}

// sameSpelling reports whether two texts read as the same tokens: layout and
// comments aside, which is what the graph does not state.
func sameSpelling(a, b string) bool {
	return strings.Join(words(a), " ") == strings.Join(words(b), " ")
}

// words is the text's code tokens in order, without whitespace or comments.
func words(text string) []string {
	var out []string
	lx := lexer.New(source.New("head.sysml", []byte(text)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind != lexer.Whitespace && !isComment(tok.Kind) {
			out = append(out, text[tok.Span.Offset:tok.Span.End()])
		}
	}
	return out
}

// region is a range of the text, from start up to end.
type region struct{ start, end int }

// tile returns the contiguous lines of each member of one body: from the end
// of the one before (the first, from the trivia ahead of it) to its last line.
func (s *authoredSource) tile(spans []source.Span) []region {
	out := make([]region, len(spans))
	for i, span := range spans {
		first := sort.Search(len(s.spans), func(k int) bool { return s.spans[k].Offset >= span.Offset })
		// A member's span runs on over the trivia after it, up to the next token.
		last := s.index(span.End() - 1)
		for last > first && s.trivia(last) {
			last--
		}
		next := last + 1
		for next < len(s.kinds) && s.trivia(next) && !s.newlineBefore(next) {
			next++
		}
		end := len(s.text)
		if next < len(s.kinds) {
			end = s.lineStart(s.spans[next].Offset)
		}
		// Blank lines ahead of the next member are its own, so they outlive this one.
		for {
			start, blank := s.blankLineBefore(end)
			if !blank {
				break
			}
			end = start
		}
		var start int
		if i > 0 {
			start = out[i-1].end
		} else {
			for first > 0 && s.trivia(first-1) && s.newlineBefore(first-1) {
				first--
			}
			start = s.lineStart(s.spans[first].Offset)
		}
		out[i] = region{start, max(start, end)}
	}
	return out
}

// shareLines moves a boundary two regions share a line at up to the second
// member's first token, so a member rebuilt there follows the trivia before it.
func (s *authoredSource) shareLines(regions []region, spans []source.Span) {
	for i := 1; i < len(regions); i++ {
		at := regions[i].start
		if at > 0 && s.text[at-1] != '\n' && spans[i].Offset >= at {
			regions[i-1].end = spans[i].Offset
			regions[i].start = spans[i].Offset
		}
	}
}

// wholeLines reports whether every region starts and ends on a line boundary,
// so each can be written or replaced on its own.
func (s *authoredSource) wholeLines(regions []region) bool {
	for _, r := range regions {
		if r.start > 0 && s.text[r.start-1] != '\n' {
			return false
		}
		if r.end < len(s.text) && r.end > 0 && s.text[r.end-1] != '\n' {
			return false
		}
	}
	return true
}

// split returns the text of an element apart from its members: the lines up to
// the first member, and those after the last.
func (s *authoredSource) split(own, members region) (head, tail string) {
	return string(s.text[own.start:members.start]), string(s.text[members.end:own.end])
}

func (s *authoredSource) region(r region) string { return string(s.text[r.start:r.end]) }

// newlineBefore reports whether token k starts on a later line than token k-1.
func (s *authoredSource) newlineBefore(k int) bool {
	if k == 0 {
		return true
	}
	prev := s.spans[k-1]
	return s.kinds[k-1] == lexer.SLNote || bytes.IndexByte(s.text[prev.End():s.spans[k].Offset], '\n') >= 0
}

// blankLineBefore returns where the line ending at end starts, if that line is
// empty: a newline, LF or CRLF, right after another.
func (s *authoredSource) blankLineBefore(end int) (int, bool) {
	if end == 0 || s.text[end-1] != '\n' {
		return 0, false
	}
	start := end - 1
	if start > 0 && s.text[start-1] == '\r' {
		start--
	}
	if start == 0 || s.text[start-1] != '\n' {
		return 0, false
	}
	return start, true
}

// lineStart returns the offset of the indentation before pos on its line, or
// pos itself when something other than blanks precedes it there.
func (s *authoredSource) lineStart(pos int) int {
	for pos > 0 && (s.text[pos-1] == ' ' || s.text[pos-1] == '\t') {
		pos--
	}
	return pos
}

// trivia reports whether token k is one the parser skips: a note, or a `/* */`
// comment not following a declaration head, which would make it its body.
func (s *authoredSource) trivia(k int) bool {
	switch s.kinds[k] {
	case lexer.SLNote, lexer.MLNote:
		return true
	case lexer.RegularComment:
		if k == 0 {
			return true
		}
		switch s.kinds[k-1] {
		case lexer.Semicolon, lexer.LBrace, lexer.RBrace, lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			return true
		}
	}
	return false
}
