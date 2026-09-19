package source

// LineIndex maps byte offsets to 1-based line/column positions.
// lineStarts[i] is the byte offset where line (i+1) begins.
type LineIndex struct {
	content    []byte
	lineStarts []int
}

func newLineIndex(content []byte) *LineIndex {
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

// OffsetAt returns the byte offset for a 1-based line/col.
func (li *LineIndex) OffsetAt(p Pos) int {
	if p.Line < 1 || p.Line > len(li.lineStarts) {
		return -1
	}
	return li.lineStarts[p.Line-1] + (p.Col - 1)
}
