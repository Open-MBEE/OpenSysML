package parser

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// `assert`, `assume` and `require` state a constraint reference or a
// `constraint` declaration, so a condition written after the keyword is
// rejected (SysML.xtext AssertConstraintUsage, RequirementConstraintUsage).
func TestKeywordedConditionIsRejected(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"assert", "constraint c { assert total <= limit; }"},
		{"assert not", "constraint c { assert not total <= limit; }"},
		{"assert literal", "constraint c { assert true; }"},
		{"assume", "constraint c { assume total != 0; }"},
		{"assert with parameters", "constraint c { in x : Real; assert x > 0; }"},
		{"require in requirement", "requirement r { require power > 0; }"},
		{"assume in requirement", "requirement r { assume power > 0; }"},
		{"require in requirement def", "requirement def R { require power > 0; }"},
		{"require in concern", "concern def C { require power > 0; }"},
		{"require in viewpoint", "viewpoint v { require power > 0; }"},
		{"require in objective", "case def C { objective o { require power > 0; } }"},
		{"require in nested requirement", "case def C { requirement r { require power > 0; } }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			if p.ParseFile() == nil {
				t.Fatal("nil root")
			}
			if len(p.Diagnostics) == 0 {
				t.Fatal("no diagnostic for a keyworded condition")
			}
			if got := p.Diagnostics[0].Message; !strings.Contains(got, "not a condition") {
				t.Errorf("diagnostic = %q, want it to name the condition", got)
			}
		})
	}
}

// The rejection carries the migration as a quick fix, and a negated condition
// keeps its truth value as `not (…)`.
func TestKeywordedConditionFixWritesTheStandardSpelling(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"assert", "constraint c { assert total <= limit; }", "constraint c { total <= limit }"},
		{"assert not", "constraint c { assert not total <= limit; }", "constraint c { not (total <= limit) }"},
		{"require", "requirement r { require power > 0; }", "requirement r { require constraint { power > 0 } }"},
		{"assume", "requirement r { assume power > 0; }", "requirement r { assume constraint { power > 0 } }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := diagnose(t, c.src)
			if len(diags) != 1 {
				t.Fatalf("diagnostics = %v, want one", diags)
			}
			if len(diags[0].Fixes) != 1 {
				t.Fatalf("fixes = %+v, want one", diags[0].Fixes)
			}
			if got := applyEdits(c.src, diags[0].Fixes[0].Edits); got != c.want {
				t.Errorf("fixed source = %q, want %q", got, c.want)
			}
		})
	}
}

// applyEdits applies edits to src back to front, so earlier offsets stay valid.
func applyEdits(src string, edits []diag.Edit) string {
	sorted := append([]diag.Edit(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Span.Offset > sorted[j].Span.Offset })
	for _, e := range sorted {
		src = src[:e.Span.Offset] + e.NewText + src[e.Span.Offset+e.Span.Len:]
	}
	return src
}

// The forms the grammar admits keep parsing clean: the constraint a keyword
// names, its negation, a nested `constraint` declaration, and the keyword-less
// condition the removed spelling migrates to.
func TestStandardConditionFormsStayClean(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"bare condition", "constraint c { total <= limit }"},
		{"bare negated condition", "constraint c { not (total <= limit) }"},
		{"bare condition with parameters", "constraint c { in x : Real; x > 0 }"},
		{"asserted reference", "package P { constraint def C; constraint c { assert P::C; } }"},
		{"asserted negated reference", "package P { constraint def C; constraint c { assert not P::C; } }"},
		{"nested constraint", "constraint c { assert constraint { total <= limit } }"},
		{"named nested constraint", "constraint c { assert constraint budget { total <= limit } }"},
		{"required reference", "package P { constraint def C; requirement r { require P::C; } }"},
		{"required nested constraint", "requirement r { require constraint { power > 0 } }"},
		{"assumed nested constraint", "requirement r { assume constraint { power > 0 } }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(c.src)))
			p.ParseFile()
			for _, d := range p.Diagnostics {
				t.Errorf("unexpected diagnostic: %s", d.Message)
			}
		})
	}
}
