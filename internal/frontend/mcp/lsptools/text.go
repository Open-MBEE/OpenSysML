package lsptools

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// Range is a span of a document in 1-based lines and character columns; End is
// exclusive, as in the protocol.
type Range struct {
	Start toolkit.Position `json:"start" jsonschema:"first character of the span (1-based line and column)"`
	End   toolkit.Position `json:"end" jsonschema:"character after the span (1-based line and column)"`
}

// TextEdit replaces the text of Range with NewText.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText" jsonschema:"the text that replaces the range; empty deletes it"`
}

// lines cuts content into its lines without their line breaks.
func lines(content []byte) []string {
	out := strings.Split(string(content), "\n")
	for i, line := range out {
		out[i] = strings.TrimSuffix(line, "\r")
	}
	return out
}

// lineAt returns the text of the 0-based line, or "" past the end.
func lineAt(content []byte, line uint32) string {
	ls := lines(content)
	if int(line) >= len(ls) {
		return ""
	}
	return ls[line]
}

// toProtocol converts a 1-based position into the protocol's, using the line
// it names for the UTF-16 conversion. A line past the end is an error, so an
// agent learns its position was off rather than getting the last line's answer.
func toProtocol(content []byte, p toolkit.Position) (protocol.Position, error) {
	ls := lines(content)
	if p.Line < 1 || p.Line > len(ls) {
		return protocol.Position{}, fmt.Errorf("line %d is outside the document, which has %d lines", p.Line, len(ls))
	}
	line := ls[p.Line-1]
	if p.Column < 1 || p.Column > utf8.RuneCountInString(line)+1 {
		return protocol.Position{}, fmt.Errorf("column %d is outside line %d, which has %d characters", p.Column, p.Line, utf8.RuneCountInString(line))
	}
	return p.ToProtocol(line), nil
}

// fullRange is the protocol range covering all of content, for a change that
// replaces the whole text.
func fullRange(content []byte) protocol.Range {
	ls := lines(content)
	last := ls[len(ls)-1]
	return protocol.Range{End: toolkit.Position{Line: len(ls), Column: utf8.RuneCountInString(last) + 1}.ToProtocol(last)}
}

// fromProtocol converts a protocol position into the 1-based position tools
// report, over the content the position addresses.
func fromProtocol(content []byte, pos protocol.Position) toolkit.Position {
	return toolkit.FromProtocol(pos, lineAt(content, pos.Line))
}

// rangeFromProtocol converts a protocol range into a 1-based Range.
func rangeFromProtocol(content []byte, r protocol.Range) Range {
	return Range{Start: fromProtocol(content, r.Start), End: fromProtocol(content, r.End)}
}

// rangeToProtocol converts a 1-based Range into the protocol's.
func rangeToProtocol(content []byte, r Range) (protocol.Range, error) {
	start, err := toProtocol(content, r.Start)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("range start: %w", err)
	}
	end, err := toProtocol(content, r.End)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("range end: %w", err)
	}
	return protocol.Range{Start: start, End: end}, nil
}

// offsetRange converts a byte span into a 1-based Range.
func offsetRange(content []byte, offset, length int) Range {
	return Range{Start: offsetPosition(content, offset), End: offsetPosition(content, offset+length)}
}

// offsetPosition converts a byte offset into a 1-based line and character column.
func offsetPosition(content []byte, offset int) toolkit.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	before := content[:offset]
	line := strings.Count(string(before), "\n") + 1
	lineStart := strings.LastIndexByte(string(before), '\n') + 1
	return toolkit.Position{Line: line, Column: utf8.RuneCount(before[lineStart:]) + 1}
}

// byteOffset converts a protocol position into a byte offset in content: a
// line past the end is the end of the content, a character past the end of its
// line the end of that line.
func byteOffset(content []byte, pos protocol.Position) int {
	offset := 0
	for line := uint32(0); line < pos.Line; line++ {
		next := strings.IndexByte(string(content[offset:]), '\n')
		if next < 0 {
			return len(content)
		}
		offset += next + 1
	}
	end := len(content)
	if next := strings.IndexByte(string(content[offset:]), '\n'); next >= 0 {
		end = offset + next
	}
	units := uint32(0)
	for offset < end && units < pos.Character {
		r, size := utf8.DecodeRune(content[offset:end])
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
		offset += size
	}
	return offset
}

// applyTextEdits returns content with the protocol edits applied, which must
// not overlap.
func applyTextEdits(content []byte, edits []protocol.TextEdit) ([]byte, error) {
	type span struct {
		start, end int
		text       string
	}
	spans := make([]span, 0, len(edits))
	for _, e := range edits {
		start, end := byteOffset(content, e.Range.Start), byteOffset(content, e.Range.End)
		if end < start {
			return nil, errors.New("edit ends before it starts")
		}
		spans = append(spans, span{start, end, e.NewText})
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var out strings.Builder
	last := 0
	for _, s := range spans {
		if s.start < last {
			return nil, errors.New("edits overlap")
		}
		out.Write(content[last:s.start])
		out.WriteString(s.text)
		last = s.end
	}
	out.Write(content[last:])
	return []byte(out.String()), nil
}
