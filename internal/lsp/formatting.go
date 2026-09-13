package lsp

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/core/format"
)

// formatSource is the formatter behind both formatting requests; tests swap it
// to exercise the error paths the real one only takes on a formatter bug.
var formatSource = format.Source

// Formatting re-indents the whole document as one edit per changed region, so
// the editor keeps its undo history, cursor and folds.
func (s *Server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return s.formattingEdits(params.TextDocument.URI, params.Options, allLines)
}

// RangeFormatting formats the whole document as Formatting does and returns only
// the edits on the requested lines, so nothing outside them moves.
func (s *Server) RangeFormatting(ctx context.Context, params *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	first, last := lineSpan(params.Range)
	return s.formattingEdits(params.TextDocument.URI, params.Options, lineWindow{int(first), int(last) + 1})
}

// lineWindow is the half-open range of document lines an edit may touch.
type lineWindow struct{ from, to int }

var allLines = lineWindow{0, math.MaxInt}

func (w lineWindow) has(line int) bool { return w.from <= line && line < w.to }

// formattingEdits returns the edits within w that turn the named document into
// its formatted form, or none when there is nothing to do. A document that does
// not parse is left alone: its brace structure is unreliable, and reformatting a
// file the author is midway through editing is worse than doing nothing.
func (s *Server) formattingEdits(uri protocol.DocumentURI, opts protocol.FormattingOptions, w lineWindow) ([]protocol.TextEdit, error) {
	name := uriToName(uri)
	doc := s.ws.Document(name)
	if doc == nil || len(doc.ParseDiagnostics) > 0 {
		return nil, nil
	}
	out, err := formatSource(name, doc.Content, formatOptions(opts))
	if errors.Is(err, format.ErrNotIdempotent) {
		// The formatter declined to rewrite the file. That is its bug, not the
		// author's, so leave the document alone rather than raise it in the editor.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if bytes.Equal(out, doc.Content) {
		return nil, nil
	}
	return formatEdits(doc.Content, out, w), nil
}

// lineSpan returns the first and last line a range touches; a range ending at
// column 0 of a later line stops before that line.
func lineSpan(r protocol.Range) (first, last uint32) {
	first, last = r.Start.Line, r.End.Line
	if last > first && r.End.Character == 0 {
		last--
	}
	if last < first {
		last = first
	}
	return first, last
}

// formatOptions maps the client's editor settings onto the formatter's.
func formatOptions(opts protocol.FormattingOptions) format.Options {
	out := format.Options{IndentWidth: int(opts.TabSize), UseTabs: !opts.InsertSpaces}
	if out.IndentWidth <= 0 {
		out.IndentWidth = format.DefaultOptions.IndentWidth
	}
	return out
}

// formatEdits returns the ordered, non-overlapping edits on the lines of w that
// turn content into formatted. Lines are matched on their non-whitespace text,
// since that is all the formatter preserves; each changed line becomes one edit
// and each run of dropped lines one deletion.
func formatEdits(content, formatted []byte, w lineWindow) []protocol.TextEdit {
	oldLines, oldOffsets := cutLines(content)
	newLines, _ := cutLines(formatted)
	table := newLineTable(content, oldOffsets)
	w.from, w.to = max(w.from, 0), min(w.to, len(oldLines))

	var edits []protocol.TextEdit
	replace := func(start, end int, text string) {
		edits = append(edits, protocol.TextEdit{
			Range:   protocol.Range{Start: table.position(start), End: table.position(end)},
			NewText: text,
		})
	}
	// editLine rewrites old line i into new line j where their whitespace differs.
	editLine := func(i, j int) {
		if !w.has(i) {
			return
		}
		if start, end, text, changed := trimCommon(oldLines[i], newLines[j]); changed {
			replace(oldOffsets[i]+start, oldOffsets[i]+end, text)
		}
	}
	pairLines := func(oldFrom, oldTo, newFrom int) {
		for i, j := oldFrom, newFrom; i < oldTo; i, j = i+1, j+1 {
			editLine(i, j)
		}
	}
	oldNext, newNext := 0, 0
	keys := lineKeys{ids: make(map[string]int, len(oldLines))}
	oldIDs, newIDs := keys.of(oldLines, len(content)), keys.of(newLines, len(formatted))
	hunks := append(diffLines(oldIDs, newIDs), hunk{len(oldLines), len(oldLines), len(newLines), len(newLines)})
	for k, h := range hunks {
		surplus := (h.oldEnd - h.oldStart) - (h.newEnd - h.newStart)
		if surplus > 0 && uniform(oldIDs[h.oldStart:h.oldEnd], oldIDs[h.oldStart]) && uniform(newIDs[h.newStart:h.newEnd], oldIDs[h.oldStart]) {
			// Dropping from a run of like lines (blanks) has no natural place; widen
			// the hunk over the run so any selected line of it may be the one to go.
			id := oldIDs[h.oldStart]
			for h.oldStart > oldNext && oldIDs[h.oldStart-1] == id {
				h.oldStart, h.newStart = h.oldStart-1, h.newStart-1
			}
			for h.oldEnd < hunks[k+1].oldStart && oldIDs[h.oldEnd] == id {
				h.oldEnd, h.newEnd = h.oldEnd+1, h.newEnd+1
			}
		}
		pairLines(oldNext, h.oldStart, newNext)
		oldNext, newNext = h.oldEnd, h.newEnd
		if surplus == 0 {
			pairLines(h.oldStart, h.oldEnd, h.newStart)
			continue
		}
		if surplus < 0 {
			// More new lines than old: pair in order, insert the rest after the hunk.
			pairLines(h.oldStart, h.oldEnd, h.newStart)
			if w.has(max(h.oldEnd-1, 0)) {
				replace(oldOffsets[h.oldEnd], oldOffsets[h.oldEnd], strings.Join(newLines[h.newStart+h.oldEnd-h.oldStart:h.newEnd], ""))
			}
			continue
		}
		// The hunk drops lines (the formatter only ever removes blank ones). Take
		// them from the end of the window's part first, so a selection never has
		// to change a line outside it, and pair the survivors with the new lines.
		from, to := max(h.oldStart, w.from), min(h.oldEnd, w.to)
		if from >= to {
			continue
		}
		dropFrom := max(to-surplus, from)
		outside := surplus - (to - dropFrom)
		dropped := func(i int) bool {
			switch {
			case i >= to:
				return h.oldEnd-i <= outside
			case i < from:
				return i-h.oldStart < outside-(h.oldEnd-to)
			default:
				return i >= dropFrom
			}
		}
		j := h.newStart
		for i := h.oldStart; i < h.oldEnd; i++ {
			if dropped(i) {
				continue
			}
			editLine(i, j)
			j++
		}
		replace(oldOffsets[dropFrom], oldOffsets[to], "")
	}
	return edits
}

// uniform reports whether every id is the given one.
func uniform(ids []int, id int) bool {
	for _, other := range ids {
		if other != id {
			return false
		}
	}
	return true
}

// cutLines splits text after each newline, keeping terminators so the lines
// concatenate back to text; offsets[i] is where lines[i] starts, offsets[len(lines)] is len(text).
func cutLines(text []byte) (lines []string, offsets []int) {
	s := string(text)
	offsets = append(offsets, 0)
	for start := 0; start < len(s); {
		end := strings.IndexByte(s[start:], '\n')
		if end < 0 {
			end = len(s)
		} else {
			end += start + 1
		}
		lines = append(lines, s[start:end])
		offsets = append(offsets, end)
		start = end
	}
	return lines, offsets
}

// lineKeys numbers lines by their text with whitespace removed: two lines get
// the same id exactly when they differ only in whitespace.
type lineKeys struct {
	ids map[string]int
}

// of returns the ids of lines, whose total length is size.
func (k *lineKeys) of(lines []string, size int) []int {
	// Strip every line into one buffer, then intern substrings of it.
	var stripped strings.Builder
	stripped.Grow(size)
	ends := make([]int, len(lines))
	for i, line := range lines {
		for j := 0; j < len(line); j++ {
			switch c := line[j]; c {
			case ' ', '\t', '\r', '\n', '\f', '\v':
			default:
				stripped.WriteByte(c)
			}
		}
		ends[i] = stripped.Len()
	}
	all := stripped.String()
	out := make([]int, len(lines))
	start := 0
	for i, end := range ends {
		key := all[start:end]
		start = end
		id, ok := k.ids[key]
		if !ok {
			id = len(k.ids)
			k.ids[key] = id
		}
		out[i] = id
	}
	return out
}

// trimCommon strips the shared prefix and suffix and reports the byte range of
// old that remains and the text replacing it. The range never splits a rune or
// a CRLF: a position between its CR and LF is not addressable in the protocol.
func trimCommon(old, updated string) (start, end int, text string, changed bool) {
	if old == updated {
		return 0, 0, "", false
	}
	start = 0
	for start < len(old) && start < len(updated) && old[start] == updated[start] {
		start++
	}
	for start > 0 && (splits(old, start) || splits(updated, start)) {
		start--
	}
	end, newEnd := len(old), len(updated)
	for end > start && newEnd > start && old[end-1] == updated[newEnd-1] {
		end--
		newEnd--
	}
	for splits(old, end) || splits(updated, newEnd) {
		end++
		newEnd++
	}
	return start, end, updated[start:newEnd], true
}

// splits reports whether cutting s at i lands inside a rune or between a CR and its LF.
func splits(s string, i int) bool {
	if i <= 0 || i >= len(s) {
		return false
	}
	return !utf8.RuneStart(s[i]) || (s[i] == '\n' && s[i-1] == '\r')
}

// lineTable converts byte offsets to protocol positions in O(log lines) each.
type lineTable struct {
	content []byte
	starts  []int // byte offset of each line start
}

func newLineTable(content []byte, offsets []int) lineTable {
	starts := offsets
	if n := len(content); n > 0 && content[n-1] != '\n' {
		starts = offsets[:len(offsets)-1]
	}
	return lineTable{content: content, starts: starts}
}

// position converts a byte offset to a position exactly as offsetToPosition does.
func (t lineTable) position(offset int) protocol.Position {
	line := sort.SearchInts(t.starts, offset+1) - 1
	if line < 0 {
		line = 0
	}
	return protocol.Position{
		Line:      uint32Clamp(line),
		Character: uint32Clamp(utf16Len(t.content[t.starts[line]:offset])),
	}
}
