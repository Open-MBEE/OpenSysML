package diagram_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// w8dDiags analyses src with the default registry, as the CLI does.
func w8dDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", root)
	return passes.Analyze("<t>", root, nil, idx)
}

// identityDiagsAcross analyses two workspace documents with the default
// registry and returns each document's diagnostics.
func identityDiagsAcross(t *testing.T, srcA, srcB string) ([]diag.Diagnostic, []diag.Diagnostic) {
	t.Helper()
	rootA := parser.New(source.New("<a>", []byte(srcA))).ParseFile()
	rootB := parser.New(source.New("<b>", []byte(srcB))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("<a>", rootA)
	idx.AddDocument("<b>", rootB)
	return passes.Analyze("<a>", rootA, nil, idx), passes.Analyze("<b>", rootB, nil, idx)
}

// w8dLine returns the 1-based line of a span in src.
func w8dLine(src string, span source.Span) int {
	return strings.Count(src[:span.Offset], "\n") + 1
}

// w8dLines returns the 1-based lines carrying a diagnostic with code.
func w8dLines(t *testing.T, src, code string) []int {
	t.Helper()
	var lines []int
	for _, d := range only(w8dDiags(t, src), code) {
		lines = append(lines, w8dLine(src, d.Span))
	}
	return lines
}

func w8dWantLines(t *testing.T, src, code string, want ...int) {
	t.Helper()
	got := w8dLines(t, src, code)
	if len(got) != len(want) {
		t.Fatalf("%s: got lines %v, want %v", code, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got lines %v, want %v", code, got, want)
		}
	}
}

// only returns the findings with one code.
func only(diags []diag.Diagnostic, code string) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}
