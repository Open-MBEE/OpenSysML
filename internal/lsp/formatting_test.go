package lsp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/format"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

var spaces4 = protocol.FormattingOptions{TabSize: 4, InsertSpaces: true}

func openFormatDoc(t *testing.T, src string) (*Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	name := uri.File("/tmp/fmt.sysml").Filename()
	ws.Open(name, []byte(src), 1)
	return NewServer(ws), name
}

// formatEditsFor runs Formatting on src and returns its edits.
func formatEditsFor(t *testing.T, src string, opts protocol.FormattingOptions) []protocol.TextEdit {
	t.Helper()
	s, name := openFormatDoc(t, src)
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Options:      opts,
	})
	if err != nil {
		t.Fatalf("Formatting err = %v", err)
	}
	return edits
}

// rangeEditsFor runs RangeFormatting over r on src and returns its edits.
func rangeEditsFor(t *testing.T, src string, r protocol.Range) []protocol.TextEdit {
	t.Helper()
	s, name := openFormatDoc(t, src)
	edits, err := s.RangeFormatting(context.Background(), &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Range:        r,
		Options:      spaces4,
	})
	if err != nil {
		t.Fatalf("RangeFormatting err = %v", err)
	}
	return edits
}

func formatDoc(t *testing.T, src string, opts protocol.FormattingOptions) ([]protocol.TextEdit, string) {
	t.Helper()
	edits := formatEditsFor(t, src, opts)
	return edits, applyOrderedEdits(t, src, edits)
}

func pos(line, char int) protocol.Position {
	return protocol.Position{Line: uint32(line), Character: uint32(char)}
}

func lines(from, to int) protocol.Range {
	return protocol.Range{Start: pos(from, 0), End: pos(to, 0)}
}

// clientOffset resolves a position the way an editor does: the line's text ends
// before its CRLF or LF terminator, and the column must land on a UTF-16 code
// unit boundary within that text. Anything else is not a valid position.
func clientOffset(t *testing.T, content string, p protocol.Position) int {
	t.Helper()
	lines, offsets := cutLines([]byte(content))
	if int(p.Line) == len(lines) && (len(content) == 0 || strings.HasSuffix(content, "\n")) {
		if p.Character != 0 {
			t.Fatalf("position %v: the empty last line has no column %d", p, p.Character)
		}
		return len(content)
	}
	if int(p.Line) >= len(lines) {
		t.Fatalf("position %v: line beyond the document's %d lines", p, len(lines))
	}
	text := strings.TrimSuffix(strings.TrimSuffix(lines[p.Line], "\n"), "\r")
	units := 0
	for i, r := range text {
		if units == int(p.Character) {
			return offsets[p.Line] + i
		}
		if units > int(p.Character) {
			t.Fatalf("position %v splits the surrogate pair of %q", p, r)
		}
		units += utf16RuneLen(r)
	}
	if units != int(p.Character) {
		t.Fatalf("position %v: line %q is only %d UTF-16 units long", p, text, units)
	}
	return offsets[p.Line] + len(text)
}

// applyOrderedEdits checks edits are in document order, never overlap and use
// positions a client can address, then applies them back to front as a client does.
func applyOrderedEdits(t *testing.T, content string, edits []protocol.TextEdit) string {
	t.Helper()
	src := []byte(content)
	type span struct{ start, end int }
	spans := make([]span, len(edits))
	for i, e := range edits {
		start, end := clientOffset(t, content, e.Range.Start), clientOffset(t, content, e.Range.End)
		if offsetToPosition(src, start) != e.Range.Start || offsetToPosition(src, end) != e.Range.End {
			t.Fatalf("edit %d range %v disagrees with offsetToPosition", i, e.Range)
		}
		if end < start {
			t.Fatalf("edit %d %v ends before it starts", i, e.Range)
		}
		if i > 0 && start < spans[i-1].end {
			t.Fatalf("edit %d %v overlaps or precedes edit %d %v", i, e.Range, i-1, edits[i-1].Range)
		}
		spans[i] = span{start, end}
	}
	for i := len(edits) - 1; i >= 0; i-- {
		src = append(src[:spans[i].start], append([]byte(edits[i].NewText), src[spans[i].end:]...)...)
	}
	return string(src)
}

func TestFormattingReindentsDocument(t *testing.T) {
	edits, got := formatDoc(t, "package P {\npart def Car;\n}\n", spaces4)
	if want := "package P {\n    part def Car;\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	want := []protocol.TextEdit{{Range: protocol.Range{Start: pos(1, 0), End: pos(1, 0)}, NewText: "    "}}
	if len(edits) != 1 || edits[0] != want[0] {
		t.Fatalf("edits = %+v, want %+v", edits, want)
	}
}

func TestFormattingHonoursClientIndentSettings(t *testing.T) {
	if _, got := formatDoc(t, "package P {\npart def Car;\n}\n", protocol.FormattingOptions{TabSize: 2, InsertSpaces: true}); got != "package P {\n  part def Car;\n}\n" {
		t.Errorf("two-space indent: got %q", got)
	}
	if _, got := formatDoc(t, "package P {\npart def Car;\n}\n", protocol.FormattingOptions{TabSize: 4}); got != "package P {\n\tpart def Car;\n}\n" {
		t.Errorf("tab indent: got %q", got)
	}
}

func TestFormattingReturnsNoEditsForFormattedDocument(t *testing.T) {
	if edits, _ := formatDoc(t, "package P {\n    part def Car;\n}\n", spaces4); len(edits) != 0 {
		t.Fatalf("edits = %v, want none", edits)
	}
}

// A file whose last line is a comment with no newline after it still formats.
func TestFormattingDocumentEndingInUnterminatedComment(t *testing.T) {
	_, got := formatDoc(t, "package P {\npart def Car;\n}\n// note", spaces4)
	if want := "package P {\n    part def Car;\n}\n// note\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormattingSkipsDocumentThatDoesNotParse(t *testing.T) {
	// Brace depth is meaningless here, so the file must be left untouched.
	if edits, _ := formatDoc(t, "package P { part def\n", spaces4); len(edits) != 0 {
		t.Fatalf("edits = %v, want none for an unparseable document", edits)
	}
}

func TestFormattingUnknownDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/tmp/missing.sysml")},
		Options:      spaces4,
	})
	if err != nil || edits != nil {
		t.Fatalf("Formatting = %v, %v; want nil, nil", edits, err)
	}
}

// Only the lines the formatter changes are edited, and only the characters
// that differ, so the editor can keep its undo history, cursor and folds.
func TestFormattingEditsOnlyChangedLines(t *testing.T) {
	src := "package P {\n    part def Car;\n  part def Bus;\n    part def Van;   \n}\n"
	edits, got := formatDoc(t, src, spaces4)
	if want := "package P {\n    part def Car;\n    part def Bus;\n    part def Van;\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	want := []protocol.TextEdit{
		{Range: protocol.Range{Start: pos(2, 2), End: pos(2, 2)}, NewText: "  "},
		{Range: protocol.Range{Start: pos(3, 17), End: pos(3, 20)}, NewText: ""},
	}
	if len(edits) != len(want) || edits[0] != want[0] || edits[1] != want[1] {
		t.Fatalf("edits = %+v, want %+v", edits, want)
	}
}

// Blank lines the formatter collapses become one deletion, not a rewrite of
// the surrounding lines.
func TestFormattingDeletesCollapsedBlankLines(t *testing.T) {
	src := "package P {\n    part def Car;\n\n\n\n    part def Bus;\n}\n"
	edits, got := formatDoc(t, src, spaces4)
	want, err := format.Source("fmt.sysml", []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("got %q, want %q", got, string(want))
	}
	if len(edits) != 1 || edits[0].NewText != "" || edits[0].Range.Start.Line < 2 || edits[0].Range.End.Line > 5 {
		t.Fatalf("edits = %+v, want one deletion inside the blank run", edits)
	}
}

// Columns are UTF-16 code units: an astral-plane character counts as two.
func TestFormattingPositionsAreUTF16(t *testing.T) {
	src := "package P {\n    /* 😀 */ part def Car;   \n  attribute s = \"🚗\";\n}\n"
	edits, got := formatDoc(t, src, spaces4)
	want, err := format.Source("fmt.sysml", []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("got %q, want %q", got, string(want))
	}
	// "    /* 😀 */ part def Car;" is 26 UTF-16 units (28 bytes): the emoji
	// is two units, four bytes.
	wantEdits := []protocol.TextEdit{
		{Range: protocol.Range{Start: pos(1, 26), End: pos(1, 29)}, NewText: ""},
		{Range: protocol.Range{Start: pos(2, 2), End: pos(2, 2)}, NewText: "  "},
	}
	if len(edits) != len(wantEdits) || edits[0] != wantEdits[0] || edits[1] != wantEdits[1] {
		t.Fatalf("edits = %+v, want %+v", edits, wantEdits)
	}
}

// A CRLF document keeps its line endings, and each edit stays within its line.
func TestFormattingKeepsCRLF(t *testing.T) {
	src := "package P {\r\npart def Car;\r\n}\r\n"
	edits, got := formatDoc(t, src, spaces4)
	if want := "package P {\r\n    part def Car;\r\n}\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(edits) != 1 || edits[0].NewText != "    " {
		t.Fatalf("edits = %+v, want one indentation insert", edits)
	}
}

// Mixed line endings are normalised to the dominant one. A protocol position
// cannot sit between a CR and its LF, so the edit that drops or adds the CR
// replaces the whole terminator, running to the start of the next line.
func TestFormattingNormalisesMixedLineEndings(t *testing.T) {
	cases := []struct {
		name, src, want string
		edits           []protocol.TextEdit
	}{
		{
			name: "one CRLF line in an LF file",
			src:  "package P {\r\n    part def Car;\n    part def Bus;\n}\n",
			want: "package P {\n    part def Car;\n    part def Bus;\n}\n",
			edits: []protocol.TextEdit{
				{Range: protocol.Range{Start: pos(0, 11), End: pos(1, 0)}, NewText: "\n"},
			},
		},
		{
			name: "one LF line in a CRLF file",
			src:  "package P {\r\n    part def Car;\n    part def Bus;\r\n}\r\n",
			want: "package P {\r\n    part def Car;\r\n    part def Bus;\r\n}\r\n",
			edits: []protocol.TextEdit{
				{Range: protocol.Range{Start: pos(1, 17), End: pos(2, 0)}, NewText: "\r\n"},
			},
		},
		{
			name: "CRLF line that also needs re-indenting",
			src:  "package P {\npart def Car;\r\n    part def Bus;\n}\n",
			want: "package P {\n    part def Car;\n    part def Bus;\n}\n",
			edits: []protocol.TextEdit{
				{Range: protocol.Range{Start: pos(1, 0), End: pos(2, 0)}, NewText: "    part def Car;\n"},
			},
		},
		{
			name: "blank CRLF line in an LF file",
			src:  "package P {\n    part def Car;\n\r\n    part def Bus;\n}\n",
			want: "package P {\n    part def Car;\n\n    part def Bus;\n}\n",
			edits: []protocol.TextEdit{
				{Range: protocol.Range{Start: pos(2, 0), End: pos(3, 0)}, NewText: "\n"},
			},
		},
		{
			name: "CRLF inside a multi-line comment in an LF file",
			src:  "package P {\n    /* one\r\n       two */\n    part def Car;\n}\n",
			want: "package P {\n    /* one\n       two */\n    part def Car;\n}\n",
			edits: []protocol.TextEdit{
				{Range: protocol.Range{Start: pos(1, 10), End: pos(2, 0)}, NewText: "\n"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edits, got := formatDoc(t, tc.src, spaces4)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if len(edits) != len(tc.edits) {
				t.Fatalf("edits = %+v, want %+v", edits, tc.edits)
			}
			for i := range edits {
				if edits[i] != tc.edits[i] {
					t.Fatalf("edit %d = %+v, want %+v", i, edits[i], tc.edits[i])
				}
			}
		})
	}
}

// trimCommon never cuts between a CR and its LF, whichever side has the CR.
func TestTrimCommonKeepsCRLFWhole(t *testing.T) {
	cases := []struct {
		old, new   string
		start, end int
		text       string
	}{
		{"foo\r\n", "foo\n", 3, 5, "\n"},
		{"foo\n", "foo\r\n", 3, 4, "\r\n"},
		{"\r\n", "\n", 0, 2, "\n"},
		{"\n", "\r\n", 0, 1, "\r\n"},
		{"  foo\r\n", "foo\n", 0, 7, "foo\n"},
		{"foo  \r\n", "foo\r\n", 3, 5, ""},
	}
	for _, tc := range cases {
		start, end, text, changed := trimCommon(tc.old, tc.new)
		if !changed || start != tc.start || end != tc.end || text != tc.text {
			t.Errorf("trimCommon(%q, %q) = %d, %d, %q, %v; want %d, %d, %q, true", tc.old, tc.new, start, end, text, changed, tc.start, tc.end, tc.text)
		}
		if got := tc.old[:start] + text + tc.old[end:]; got != tc.new {
			t.Errorf("trimCommon(%q, %q): applying the edit gives %q", tc.old, tc.new, got)
		}
	}
}

// Every corpus model formats to exactly what format.Source produces when the
// edits are applied, including files that need no edits or do not parse.
func TestFormattingEditsReproduceFormatterOutput(t *testing.T) {
	for _, path := range formattingCorpus(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			checkFormattingEdits(t, path, string(content))
			// A mangled copy changes many lines at once.
			checkFormattingEdits(t, path, mangleIndentation(string(content)))
		})
	}
}

func formattingCorpus(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, pattern := range []string{
		"../core/format/testdata/*.sysml",
		"../../examples/*.sysml",
		"../repl/testdata/*.sysml",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 0 {
			t.Fatalf("no fixtures match %s", pattern)
		}
		paths = append(paths, matches...)
	}
	return paths
}

// mangleIndentation shifts every line's indentation and pads a few lines with
// trailing whitespace and extra blank lines.
func mangleIndentation(src string) string {
	var b strings.Builder
	for i, line := range strings.SplitAfter(src, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		switch i % 4 {
		case 0:
			b.WriteString(trimmed)
		case 1:
			b.WriteString("\t" + line)
		case 2:
			b.WriteString(strings.TrimSuffix(line, "\n") + "  \n")
		default:
			b.WriteString(line + "\n\n")
		}
	}
	return b.String()
}

func checkFormattingEdits(t *testing.T, path, src string) {
	t.Helper()
	s, name := openFormatDoc(t, src)
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Options:      spaces4,
	})
	if err != nil {
		t.Fatalf("%s: Formatting err = %v", path, err)
	}
	if doc := s.ws.Document(name); len(doc.ParseDiagnostics) > 0 {
		if len(edits) != 0 {
			t.Fatalf("%s: edits = %d for a document with parse errors, want none", path, len(edits))
		}
		return
	}
	want, err := format.Source(name, []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatalf("%s: format.Source: %v", path, err)
	}
	if string(want) == src && len(edits) != 0 {
		t.Fatalf("%s: edits = %d for an already formatted document, want none", path, len(edits))
	}
	if got := applyOrderedEdits(t, src, edits); got != string(want) {
		t.Fatalf("%s: applying %d edits gives\n%s\nwant\n%s", path, len(edits), got, want)
	}
}

// Computing the edits costs far less than the formatter's own pass, even on the
// largest library file with every line changed.
func TestFormattingDiffIsCheaperThanFormatting(t *testing.T) {
	src := libs.DefaultSource()
	var largest []byte
	for _, name := range src.List() {
		content, err := src.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(content) > len(largest) {
			largest = content
		}
	}
	mangled := []byte(mangleIndentation(string(largest)))
	formatted, err := format.Source("largest.sysml", mangled, format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	fastest := func(f func()) time.Duration {
		best := time.Duration(1<<63 - 1)
		for i := 0; i < 5; i++ {
			start := time.Now()
			f()
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	formatTime := fastest(func() { _, _ = format.Source("largest.sysml", mangled, format.DefaultOptions) })
	diffTime := fastest(func() { formatEdits(mangled, formatted, allLines) })
	t.Logf("%d bytes: format %v, diff %v", len(mangled), formatTime, diffTime)
	// Coverage counters instrument this package but not the formatter's, so
	// the two timings are only comparable in an uninstrumented build.
	if mode := testing.CoverMode(); mode != "" {
		t.Logf("coverage mode %q: timings not compared", mode)
	} else if diffTime > formatTime {
		t.Fatalf("diff took %v, formatter %v; the diff must not dominate", diffTime, formatTime)
	}
	edits := formatEdits(mangled, formatted, allLines)
	if applyOrderedEdits(t, string(mangled), edits) != string(formatted) {
		t.Fatal("edits do not reproduce the formatter output")
	}
}

func BenchmarkFormatEdits(b *testing.B) {
	content, err := os.ReadFile("../repl/testdata/compile_calcs.sysml")
	if err != nil {
		b.Fatal(err)
	}
	mangled := []byte(mangleIndentation(string(content)))
	formatted, err := format.Source("b.sysml", mangled, format.DefaultOptions)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatEdits(mangled, formatted, allLines)
	}
}

const rangeSource = "package P {\n    part def Car;\n  part def Bus;\n    part def Van;\n    part def Bike;\n}\n"

// A range over one mis-indented line yields exactly that line's edit.
func TestRangeFormattingSingleBadLine(t *testing.T) {
	edits := rangeEditsFor(t, rangeSource, lines(2, 3))
	want := protocol.TextEdit{Range: protocol.Range{Start: pos(2, 2), End: pos(2, 2)}, NewText: "  "}
	if len(edits) != 1 || edits[0] != want {
		t.Fatalf("edits = %+v, want [%+v]", edits, want)
	}
}

// A range over well-formatted lines yields nothing, even when the rest of the
// file needs work.
func TestRangeFormattingCleanRegion(t *testing.T) {
	if edits := rangeEditsFor(t, rangeSource, lines(3, 5)); len(edits) != 0 {
		t.Fatalf("edits = %+v, want none", edits)
	}
	if edits := rangeEditsFor(t, rangeSource, protocol.Range{Start: pos(0, 0), End: pos(1, 17)}); len(edits) != 0 {
		t.Fatalf("edits = %+v, want none", edits)
	}
}

// A range spanning the whole file yields the same edits as Formatting.
func TestRangeFormattingWholeFileMatchesFormatting(t *testing.T) {
	src := mangleIndentation(rangeSource)
	full := formatEditsFor(t, src, spaces4)
	if len(full) < 2 {
		t.Fatalf("full formatting produced %d edits, want several to compare", len(full))
	}
	end := offsetToPosition([]byte(src), len(src))
	ranged := rangeEditsFor(t, src, protocol.Range{Start: pos(0, 0), End: end})
	if len(ranged) != len(full) {
		t.Fatalf("range edits = %+v, want %+v", ranged, full)
	}
	for i := range full {
		if ranged[i] != full[i] {
			t.Fatalf("range edit %d = %+v, want %+v", i, ranged[i], full[i])
		}
	}
}

// A partial-line selection widens to its whole lines: a cursor in the middle
// of the bad line still fixes it, and a selection ending at column 0 of the
// next line does not reach into that line.
func TestRangeFormattingWidensToWholeLines(t *testing.T) {
	src := "package P {\n  part def Car;\n  part def Bus;\n}\n"
	edits := rangeEditsFor(t, src, protocol.Range{Start: pos(1, 5), End: pos(1, 5)})
	if len(edits) != 1 || edits[0].Range.Start.Line != 1 {
		t.Fatalf("edits = %+v, want the edit for line 1 only", edits)
	}
	edits = rangeEditsFor(t, src, protocol.Range{Start: pos(1, 5), End: pos(2, 0)})
	if len(edits) != 1 || edits[0].Range.Start.Line != 1 {
		t.Fatalf("edits = %+v, want the edit for line 1 only", edits)
	}
	edits = rangeEditsFor(t, src, protocol.Range{Start: pos(1, 5), End: pos(2, 1)})
	if len(edits) != 2 {
		t.Fatalf("edits = %+v, want the edits for lines 1 and 2", edits)
	}
}

// Indentation inside the range follows the whole file's structure, so a
// selection deep in a nested block is indented to its real depth.
func TestRangeFormattingUsesWholeFileStructure(t *testing.T) {
	src := "package P {\n    part def Car {\n        part def Wheel {\nattribute n;\n        }\n    }\n}\n"
	edits := rangeEditsFor(t, src, lines(3, 4))
	want := protocol.TextEdit{Range: protocol.Range{Start: pos(3, 0), End: pos(3, 0)}, NewText: "            "}
	if len(edits) != 1 || edits[0] != want {
		t.Fatalf("edits = %+v, want [%+v]", edits, want)
	}
}

// A blank run the formatter collapses is only trimmed within the selection:
// selected surplus blanks go, lines outside the selection are never touched.
func TestRangeFormattingTrimsBlankRunWithinSelectionOnly(t *testing.T) {
	// Lines 2-4 are blank; the formatter keeps one of them.
	src := "package P {\n    part def Car;\n\n\n\n    part def Bus;\n}\n"
	full := formatEditsFor(t, src, spaces4)
	if want := []protocol.TextEdit{{Range: lines(3, 5), NewText: ""}}; len(full) != 1 || full[0] != want[0] {
		t.Fatalf("full edits = %+v, want %+v", full, want)
	}
	cases := []struct {
		name  string
		sel   protocol.Range
		edits []protocol.TextEdit
		want  string
	}{
		{"one blank inside the run", lines(3, 4), []protocol.TextEdit{{Range: lines(3, 4)}},
			"package P {\n    part def Car;\n\n\n    part def Bus;\n}\n"},
		{"the blank the whole-file edit keeps", lines(2, 3), []protocol.TextEdit{{Range: lines(2, 3)}},
			"package P {\n    part def Car;\n\n\n    part def Bus;\n}\n"},
		{"the whole run", lines(2, 5), full,
			"package P {\n    part def Car;\n\n    part def Bus;\n}\n"},
		{"the run's tail and the line after", lines(3, 6), full,
			"package P {\n    part def Car;\n\n    part def Bus;\n}\n"},
		{"the line before and the run's head", lines(1, 3), []protocol.TextEdit{{Range: lines(2, 3)}},
			"package P {\n    part def Car;\n\n\n    part def Bus;\n}\n"},
		{"a cursor on the middle blank", protocol.Range{Start: pos(3, 0), End: pos(3, 0)}, []protocol.TextEdit{{Range: lines(3, 4)}},
			"package P {\n    part def Car;\n\n\n    part def Bus;\n}\n"},
		{"the line before the run", lines(1, 2), nil, src},
		{"the line after the run", lines(5, 6), nil, src},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edits := rangeEditsFor(t, src, tc.sel)
			if len(edits) != len(tc.edits) {
				t.Fatalf("edits = %+v, want %+v", edits, tc.edits)
			}
			for i := range edits {
				if edits[i] != tc.edits[i] {
					t.Fatalf("edit %d = %+v, want %+v", i, edits[i], tc.edits[i])
				}
			}
			if got := applyOrderedEdits(t, src, edits); got != tc.want {
				t.Fatalf("applied = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every edit RangeFormatting returns lies on the selected lines, whatever the
// selection, and selecting everything reproduces the formatter's output.
func TestRangeFormattingNeverLeavesSelection(t *testing.T) {
	src := "package P {\n\n\n  part def Car;   \n\n\n\n\tpart def Bus {\n  \n\n  attribute n;\n\n  }\n\n\n}\n\n\n"
	want, err := format.Source("fmt.sysml", []byte(src), format.DefaultOptions)
	if err != nil {
		t.Fatal(err)
	}
	if got := applyOrderedEdits(t, src, formatEditsFor(t, src, spaces4)); got != string(want) {
		t.Fatalf("formatting gives %q, want %q", got, want)
	}
	total := strings.Count(src, "\n")
	for first := 0; first < total; first++ {
		for last := first; last < total; last++ {
			edits := rangeEditsFor(t, src, lines(first, last+1))
			for _, e := range edits {
				from, to := lineSpan(e.Range)
				if int(from) < first || int(to) > last {
					t.Fatalf("lines %d-%d: edit %+v leaves the selection", first, last, e)
				}
			}
			applyOrderedEdits(t, src, edits)
		}
	}
}

func TestRangeFormattingSkipsDocumentThatDoesNotParse(t *testing.T) {
	if edits := rangeEditsFor(t, "package P { part def\n", lines(0, 1)); len(edits) != 0 {
		t.Fatalf("edits = %v, want none for an unparseable document", edits)
	}
}

func TestRangeFormattingUnknownDocument(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	edits, err := s.RangeFormatting(context.Background(), &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("/tmp/missing.sysml")},
		Range:        lines(0, 1),
		Options:      spaces4,
	})
	if err != nil || edits != nil {
		t.Fatalf("RangeFormatting = %v, %v; want nil, nil", edits, err)
	}
}

// A formatter that declines with ErrNotIdempotent leaves the document alone,
// for both requests, while any other failure is reported.
func TestFormattingHonoursNotIdempotentGuard(t *testing.T) {
	saved := formatSource
	t.Cleanup(func() { formatSource = saved })

	formatSource = func(name string, src []byte, opts format.Options) ([]byte, error) {
		return src, format.ErrNotIdempotent
	}
	if edits := formatEditsFor(t, "package P {\npart def Car;\n}\n", spaces4); edits != nil {
		t.Fatalf("Formatting = %+v, want nil on ErrNotIdempotent", edits)
	}
	if edits := rangeEditsFor(t, "package P {\npart def Car;\n}\n", lines(0, 3)); edits != nil {
		t.Fatalf("RangeFormatting = %+v, want nil on ErrNotIdempotent", edits)
	}

	boom := errors.New("boom")
	formatSource = func(name string, src []byte, opts format.Options) ([]byte, error) { return nil, boom }
	s, name := openFormatDoc(t, "package P {\npart def Car;\n}\n")
	if _, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)}, Options: spaces4,
	}); !errors.Is(err, boom) {
		t.Fatalf("Formatting err = %v, want %v", err, boom)
	}
	if _, err := s.RangeFormatting(context.Background(), &protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)}, Range: lines(0, 3), Options: spaces4,
	}); !errors.Is(err, boom) {
		t.Fatalf("RangeFormatting err = %v, want %v", err, boom)
	}
}
