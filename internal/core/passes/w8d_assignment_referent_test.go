package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func assignmentReferentFindings(t *testing.T, src string, warm bool) []diag.Diagnostic {
	t.Helper()
	return assignmentDiags(t, src, warm, "assignment-referent-time-varying")
}

func assignmentDiags(t *testing.T, src string, warm bool, code string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, diag := range w9cLibraryDiags(t, src, warm) {
		if diag.Code == code {
			out = append(out, diag)
		}
	}
	return out
}

func TestAssignmentReferentMayTimeVary(t *testing.T) {
	src := `package Test {
	item i {
		attribute a;
		action d;
		portion attribute p;
	}
	action def A {
		assign i.a := null;
		assign i.d := null;
		assign i.p := null;
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentReferentFindings(t, src, warm)
		if len(got) != 2 {
			t.Fatalf("warm=%v: got %v, want action and portion diagnostics", warm, got)
		}
		want := []int{
			strings.Index(src, "d := null"),
			strings.Index(src, "p := null"),
		}
		for i, diag := range got {
			if diag.Message != msgAssignmentReferentTimeVarying || diag.Span.Offset != want[i] {
				t.Errorf("warm=%v diagnostic %d: got %v, want offset %d", warm, i, diag, want[i])
			}
		}
	}
}

func TestAssignmentReferentIsElementScoped(t *testing.T) {
	src := `package Test {
	item i {
		private attribute hidden;
		action d;
	}
	action def A {
		assign i.hidden := null;
		assign i.d := null;
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentReferentFindings(t, src, warm)
		if len(got) != 1 || got[0].Span.Offset != strings.Index(src, "d := null") {
			t.Fatalf("warm=%v: got %v, want only the independent d diagnostic", warm, got)
		}
	}
}

func TestAssignmentReferentUnresolvedOrNonFeatureDoesNotCascade(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "unresolved",
			src: `package Test {
				action def A { assign missing := null; }
			}`,
		},
		{
			name: "definition",
			src: `package Test {
				part def P;
				action def A { assign P := null; }
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, warm := range []bool{false, true} {
				if got := assignmentReferentFindings(t, tc.src, warm); len(got) != 0 {
					t.Errorf("warm=%v: got %v, want no cascading time-varying diagnostic", warm, got)
				}
			}
		})
	}
}

// An assignment's referent is the feature its target names (SysML v2 §8.3.16.2,
// validateAssignmentActionUsageReferent): a type or a namespace is none.
func TestAssignmentReferentNonFeatureRejected(t *testing.T) {
	tests := []struct {
		name, target, want string
	}{
		{"part definition", "P", "P is declared `part def`"},
		{"attribute definition", "AD", "AD is declared `attribute def`"},
		{"package", "Q", "Q is declared `package`"},
		{"library datatype", "ScalarValues::Integer", "ScalarValues::Integer is declared `datatype`"},
		{"nested definition", "Q::N", "Q::N is declared `part def`"},
	}
	prefix := "package Test { part def P; attribute def AD; package Q { part def N; } item i { attribute a; } action def A { assign "
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := prefix + tc.target + " := null; assign i.a := null; } }"
			for _, warm := range []bool{false, true} {
				all := w9cLibraryDiags(t, src, warm)
				var got []diag.Diagnostic
				for _, d := range all {
					if d.Severity == diag.SeverityError {
						got = append(got, d)
					}
				}
				if len(got) != 1 || got[0].Code != "assignment-referent" {
					t.Fatalf("warm=%v: got %v, want the one referent diagnostic", warm, got)
				}
				wantMsg := msgAssignmentReferent + " " + tc.want + ", not a feature."
				if got[0].Message != wantMsg {
					t.Errorf("warm=%v: got %q, want %q", warm, got[0].Message, wantMsg)
				}
				last := tc.target[strings.LastIndex(tc.target, ":")+1:]
				if got[0].Span.Offset != strings.Index(src, last+" := null") {
					t.Errorf("warm=%v: offset %d, want the target's last segment", warm, got[0].Span.Offset)
				}
			}
		})
	}
}

// The referent rule needs no library; only the time-varying rule, derived from
// Occurrences::Occurrence, waits for one.
func TestAssignmentReferentNonFeatureRejectedWithoutLibrary(t *testing.T) {
	src := "package Test { part def P; package Q; part def PD { attribute a; } action def A { assign P := null; assign Q := null; assign PD::a := null; } }"
	root, pd, idx := analyzeInputs(t, "a.sysml", src)
	var got []string
	for _, d := range Analyze("a.sysml", root, pd, idx) {
		if d.Severity == diag.SeverityError {
			got = append(got, d.Code+": "+d.Message)
		}
	}
	want := []string{
		"assignment-referent: " + msgAssignmentReferent + " P is declared `part def`, not a feature.",
		"assignment-referent: " + msgAssignmentReferent + " Q is declared `package`, not a feature.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A named multiplicity is a feature (KerML §8.3.3.3), so it is a referent; its
// value never varies, so the time-varying rule is what rejects assigning to it.
func TestAssignmentReferentMultiplicityIsNotTimeVarying(t *testing.T) {
	src := `package Test {
	private import Base::*;
	item i { attribute a; }
	action def A {
		assign exactlyOne := null;
		assign Base::zeroToMany := null;
		assign i.a := null;
	}
}`
	for _, warm := range []bool{false, true} {
		if got := assignmentDiags(t, src, warm, "assignment-referent"); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want no referent diagnostic", warm, got)
		}
		got := assignmentReferentFindings(t, src, warm)
		want := []int{
			strings.Index(src, "exactlyOne := null"),
			strings.Index(src, "zeroToMany := null"),
		}
		if len(got) != len(want) {
			t.Fatalf("warm=%v: got %v, want the two multiplicity diagnostics", warm, got)
		}
		for i, diag := range got {
			if diag.Span.Offset != want[i] {
				t.Errorf("warm=%v diagnostic %d: offset %d, want %d", warm, i, diag.Span.Offset, want[i])
			}
		}
	}
}

// A target that resolves to a feature is a referent, whatever else the rest of
// the pass says about it; an unresolved one is the name-resolution tier's alone.
func TestAssignmentReferentFeatureOrUnresolvedIsNotReported(t *testing.T) {
	src := `package Test {
	part def P { attribute x; }
	item i { attribute a; part p : P; action d; binding b bind a = p.x; }
	action def A {
		assign i.a := null;
		assign i.p := null;
		assign i.p.x := null;
		assign i.d := null;
		assign i.b := null;
		assign missing := null;
		assign i.missing := null;
	}
}`
	for _, warm := range []bool{false, true} {
		if got := assignmentDiags(t, src, warm, "assignment-referent"); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want no referent diagnostic", warm, got)
		}
	}
}

// An alias names the element it is for (KerML 8.2.3.2): an alias to a feature
// is a referent, an alias to a definition is none, directly and through a chain.
func TestAssignmentReferentThroughAliasJudgesAliasedElement(t *testing.T) {
	prefix := `package Test {
	part def P { attribute x; alias ax for x; }
	item i { attribute a; part p : P; alias aa for a; }
	alias ai for i; alias aai for ai; alias aP for P;
	package Q { alias qa for i::a; }
	action def A { `
	silent := "assign ai.a := null; assign aai.a := null; assign i.aa := null; assign ai.p.x := null; assign i.p.ax := null; assign Q::qa := null; } }"
	for _, warm := range []bool{false, true} {
		if got := assignmentDiags(t, prefix+silent, warm, "assignment-referent"); len(got) != 0 {
			t.Errorf("warm=%v: got %v, want no referent diagnostic", warm, got)
		}
	}
	for _, target := range []string{"aP", "Test::aP"} {
		src := prefix + "assign " + target + " := null; } }"
		for _, warm := range []bool{false, true} {
			got := assignmentDiags(t, src, warm, "assignment-referent")
			want := msgAssignmentReferent + " " + target + " is declared `part def`, not a feature."
			if len(got) != 1 || got[0].Message != want {
				t.Errorf("warm=%v target %s: got %v, want %q", warm, target, got, want)
			}
		}
	}
}

// A target that is an expression rather than a feature name is the syntax
// tier's error (SysML.xtext TargetParameter), so the pass adds nothing to it.
func TestAssignmentReferentExpressionTargetIsSyntaxError(t *testing.T) {
	for _, target := range []string{"1", "i.a + 1", "pick()", "i.a[1]"} {
		src := "package Test { item i { attribute a; } calc def pick { return : Integer = 1; } action def A { assign " + target + " := null; } }"
		root, pd, idx := analyzeInputs(t, "a.sysml", src)
		var codes []string
		for _, d := range Analyze("a.sysml", root, pd, idx) {
			if d.Severity == diag.SeverityError {
				codes = append(codes, d.Code)
			}
		}
		if len(codes) != 1 || codes[0] != "syntax" {
			t.Errorf("target %q: error codes %v, want the one syntax error", target, codes)
		}
	}
}

func TestAssignmentReferentWithoutOccurrenceOwnerFails(t *testing.T) {
	src := `package Test {
	attribute fixed;
	action def A {
		assign fixed := null;
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentReferentFindings(t, src, warm)
		if len(got) != 1 || got[0].Span.Offset != strings.Index(src, "fixed := null") {
			t.Fatalf("warm=%v: got %v, want the package-owned referent diagnostic", warm, got)
		}
	}
}
