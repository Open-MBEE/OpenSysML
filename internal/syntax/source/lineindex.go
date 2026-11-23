package source

import (
	"bytes"
	"slices"
	"sort"
)

// DominantNewline reports the line ending text written into content should use:
// CRLF only when content already uses it more than bare LF.
func DominantNewline(content []byte) string {
	crlf := bytes.Count(content, []byte("\r\n"))
	if crlf > bytes.Count(content, []byte("\n"))-crlf {
		return "\r\n"
	}
	return "\n"
}

// LineIndex maps byte offsets to 1-based line/column positions.
// lineStarts[i] is the byte offset where line (i+1) begins.
type LineIndex struct {
	content    []byte
	lineStarts []int
}

// NewLineIndex builds the index of content's line starts.
func NewLineIndex(content []byte) *LineIndex {
	starts := []int{0}
	for i, b := range content {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{content: content, lineStarts: starts}
}

// PosAt returns the 1-based line/col for a byte offset.
// Col counts bytes from the line start + 1.
func (li *LineIndex) PosAt(offset int) Pos {
	// binary search for the greatest lineStart <= offset
	lo, hi := 0, len(li.lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if li.lineStarts[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return Pos{Line: lo + 1, Col: offset - li.lineStarts[lo] + 1}
}

// OffsetAt returns the byte offset for a 1-based line/col, -1 for a line the
// content does not have or a column outside that line.
func (li *LineIndex) OffsetAt(p Pos) int {
	if li == nil || p.Line < 1 || p.Line > len(li.lineStarts) || p.Col < 1 {
		return -1
	}
	end := len(li.content)
	if p.Line < len(li.lineStarts) {
		end = li.lineStarts[p.Line] - 1
	}
	if p.Col-1 > end-li.lineStarts[p.Line-1] {
		return -1
	}
	return li.lineStarts[p.Line-1] + (p.Col - 1)
}

// Splice updates the index in place for an edit that replaced bytes [start, end) by
// inserted bytes; content is the text after it. Rebuild a shared index instead.
func (li *LineIndex) Splice(content []byte, start, end, inserted int) {
	// A line start s follows the newline at s-1, so the edit removes those in (start, end].
	first := sort.SearchInts(li.lineStarts, start+1)
	last := sort.SearchInts(li.lineStarts, end+1)
	var added []int
	for i, b := range content[start : start+inserted] {
		if b == '\n' {
			added = append(added, start+i+1)
		}
	}
	delta := inserted - (end - start)
	tail := li.lineStarts[last:]
	for i := range tail {
		tail[i] += delta
	}
	li.lineStarts = slices.Replace(li.lineStarts, first, last, added...)
	li.content = content
}
