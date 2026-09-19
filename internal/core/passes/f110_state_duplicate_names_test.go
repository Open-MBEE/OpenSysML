package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// duplicateOwnedNameDiags returns the duplicate-owned-member-name warnings of src.
func duplicateOwnedNameDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	var duplicates []diag.Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Message == "Duplicate of other owned member name" {
			duplicates = append(duplicates, d)
		}
	}
	return duplicates
}

// A simple state declaration (`state <name>;`) names a member of its container
// like any other usage, so a repeated name is indistinguishable.
func TestDuplicateSimpleStateNames(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"states in state def", "package P { state def S { state red; state red; } }", 2},
		{"states in state usage", "package P { state s { state red; state red; } }", 2},
		{"states in part def", "package P { part def C { state red; state red; } }", 2},
		{"state against attribute", "package P { state def S { attribute red; state red; } }", 2},
		{"state in nested state", "package P { state def S { state r { state x; state x; } } }", 2},
		{"distinct states", "package P { state def S { state red; state green; } }", 0},
		{"same name in sibling states",
			"package P { state def S { state r1 { state x; } state r2 { state x; } } }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := duplicateOwnedNameDiags(t, tc.src)
			if len(diags) != tc.want {
				t.Fatalf("got %d duplicate-name diagnostics, want %d: %v", len(diags), tc.want, diags)
			}
		})
	}
}

// A named transition is a feature of the state that declares it, so two
// transitions of one name are indistinguishable members.
func TestDuplicateTransitionNames(t *testing.T) {
	const src = `package P {
		state def S {
			state a;
			state b;
			transition t first a then b;
			transition t first b then a;
		}
	}`
	if diags := duplicateOwnedNameDiags(t, src); len(diags) != 2 {
		t.Fatalf("got %d duplicate-name diagnostics, want 2: %v", len(diags), diags)
	}
}

func TestDistinctTransitionNamesOK(t *testing.T) {
	const src = `package P {
		state def S {
			state a;
			state b;
			transition t1 first a then b;
			transition t2 first b then a;
		}
	}`
	if diags := duplicateOwnedNameDiags(t, src); len(diags) != 0 {
		t.Fatalf("unexpected duplicate-name diagnostics: %v", diags)
	}
}
