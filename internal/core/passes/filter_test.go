package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// filterDiags returns the element-filter findings of src.
func filterDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, d := range analyzeFilterSource(t, src) {
		if strings.HasPrefix(d.Code, "filter-") {
			out = append(out, d)
		}
	}
	return out
}

func analyzeFilterSource(t *testing.T, src string) []Diagnostic {
	t.Helper()
	const name = "<t>.kerml"
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	return Analyze(name, root, nil, idx)
}

// only returns the findings with one code.
func only(diags []Diagnostic, code string) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// except returns the findings without one code, for tests whose subject is a
// different rule.
func except(diags []Diagnostic, code string) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Code != code {
			out = append(out, d)
		}
	}
	return out
}

const filterMetadata = `metadata def Safety { attribute level; }
part def Belt;
`

// A filter condition is a predicate, so a condition yielding anything else can
// never select an element and is an error (KerML 8.2.4).
func TestFilterNotBooleanIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"a namespace filter", filterMetadata + "package P { public import Belt; filter 3; }"},
		{"an import filter", filterMetadata + "package P { public import Belt[42]; }"},
		{"an operand of a boolean operator", filterMetadata + "package P { filter @Safety and 7; }"},
		{"a typed element reference where a truth value is needed", filterMetadata + "package P { feature n : ScalarValues::Integer; filter n; }"},
		{"an unresolved bare reference", filterMetadata + "package P { filter Undefined; }"},
		{"an unresolved feature chain", filterMetadata + "package P { filter a.b.c; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := only(filterDiags(t, tc.src), "filter-not-boolean")
			if len(diags) != 1 {
				t.Fatalf("got %d filter-not-boolean diagnostics, want 1: %v", len(diags), diags)
			}
			if diags[0].Severity != SeverityError {
				t.Errorf("severity = %v, want an error", diags[0].Severity)
			}
			if diags[0].Span.Len == 0 {
				t.Errorf("the diagnostic has no span: %v", diags[0])
			}
			if diags[0].Message != msgFilterNotBoolean {
				t.Errorf("message = %q, want %q", diags[0].Message, msgFilterNotBoolean)
			}
		})
	}
}

// A condition outside the subset the evaluator decides is an error, as it is in
// the reference: it selects nothing, so the filter the model asked for is not
// the one it got.
func TestFilterNotEvaluableIsReported(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"an indexed condition", filterMetadata + "package P { filter @Safety[1]; }"},
		{"an invocation", filterMetadata + "package P { filter coll->select(x); }"},
		{"a Boolean-result operator", filterMetadata + "package P { filter ~(1 as Integer); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			all := filterDiags(t, tc.src)
			diags := only(all, "filter-not-evaluable")
			if len(diags) == 0 {
				t.Fatalf("the condition is not evaluable but nothing was reported")
			}
			if got := len(only(all, "filter-not-boolean")); got != 0 {
				t.Fatalf("got %d filter-not-boolean diagnostics, want none: %v", got, all)
			}
			if diags[0].Severity != SeverityError {
				t.Errorf("severity = %v, want an error", diags[0].Severity)
			}
			if diags[0].Span.Len == 0 {
				t.Errorf("the diagnostic has no span: %v", diags[0])
			}
			if diags[0].Message != msgFilterNotEvaluable {
				t.Errorf("message = %q, want %q", diags[0].Message, msgFilterNotEvaluable)
			}
		})
	}
}

func TestUnsupportedFilterResultReportsModelLevelRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		cond string
		code string
	}{
		{"an unsupported operator", "@Safety + 1", "filter-not-evaluable"},
		{"a feature chain", "a.b.c", "filter-not-boolean"},
		{"a conditional", "if c ? a else b", "filter-not-evaluable"},
		{"a cast", "x as Integer", "filter-not-evaluable"},
		{"an unresolved reference", "Undefined", "filter-not-boolean"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := filterDiags(t, filterMetadata+"package P { filter "+tc.cond+"; }")
			if got := len(only(diags, tc.code)); got != 1 {
				t.Fatalf("got %d %s diagnostics, want one: %v", got, tc.code, diags)
			}
			for _, code := range []string{"filter-not-evaluable", "filter-not-evaluated", "filter-not-boolean"} {
				if code == tc.code {
					continue
				}
				if got := len(only(diags, code)); got != 0 {
					t.Fatalf("got %d %s diagnostics, want none: %v", got, code, diags)
				}
			}
		})
	}
}

// The conditions filters are actually written with are reported by neither: a
// classification, its boolean composition, and a comparison of an annotation
// feature the evaluator reads once a candidate is at hand.
func TestFilterConditionsInTheSupportedSubsetAreClean(t *testing.T) {
	src := filterMetadata + `package P {
		public import Belt;
		filter @Safety;
		filter not @Safety or (@Safety and Safety::level >= 3);
		filter @@Safety implies @Safety;
	}
	package Q { public import P::*[@Safety and Safety::level == 3]; }`

	if diags := filterDiags(t, src); len(diags) != 0 {
		t.Fatalf("supported filter conditions were reported: %v", diags)
	}
}

// A constructor is model-level evaluable, so a filter built from one is reported
// for yielding an instance rather than a truth value (KerML 7.4.9, 8.2.4).
func TestFilterConstructorIsNotBooleanRatherThanInevaluable(t *testing.T) {
	src := filterMetadata + `package P { part def A { attribute n; }
		filter new A(null, 1, "", false); }`
	diags := filterDiags(t, src)
	if len(only(diags, "filter-not-boolean")) != 1 || len(only(diags, "filter-not-evaluable")) != 0 {
		t.Fatalf("want one filter-not-boolean and no filter-not-evaluable, got %v", diags)
	}
}

// An extent is a run's answer, not the model's (KerML §8.2.5.8.1 Table 5), so a
// filter over `all T` is inevaluable even where T declares every value it has.
func TestFilterExtentIsNotModelLevelEvaluable(t *testing.T) {
	src := `package P { datatype Color { feature red : Color; }
		package Q { filter (all Color) == null; } }`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 1 {
		t.Fatalf("want one filter-not-evaluable for the extent, got %v", diags)
	}
}

// The two faults are stated on the membership carrying the condition, once each,
// however many operands share the fault.
func TestFilterReportsEachFaultOnceOnTheMembership(t *testing.T) {
	src := filterMetadata + "package P { filter (3 + @Safety) and (4 + @Safety); }"
	diags := filterDiags(t, src)
	if len(only(diags, "filter-not-evaluable")) != 1 {
		t.Fatalf("want one filter-not-evaluable, got %v", diags)
	}
	for _, d := range diags {
		if d.Span.Len == 0 {
			t.Errorf("the diagnostic has no span: %v", d)
		}
	}
}

func TestChainFilterDiagnosticsUseSemanticResultAndEvaluability(t *testing.T) {
	const boolChain = `
		package R1 {
			private import ScalarValues::*;
			metaclass M { var feature a : Boolean[1]; }
			metaclass N { var feature m : M[1]; }
		}
		package Q { filter R1::N::m.a; }
	`
	const intChain = `
		package R1 {
			private import ScalarValues::*;
			metaclass M { var feature a : Integer[1]; }
			metaclass N { var feature m : M[1]; }
		}
		package Q { filter R1::N::m.a; }
	`
	const comparison = `
		package R1 {
			private import ScalarValues::*;
			metaclass M { var feature a : Integer[1]; }
			metaclass N { var feature m : M[1]; }
		}
		package Q { filter R1::N::m.a > 2; }
	`
	const structChain = `
		package R1 {
			private import ScalarValues::*;
			struct S { feature x : Boolean[1]; }
			struct T { feature s : S[1]; }
		}
		package Q { filter R1::T::s.x; }
	`
	const libraryChain = `package Q { filter KerML::Root::Element::owner.name == "P1"; }`
	const packageChain = `
		package R1 {
			private import ScalarValues::*;
			metaclass M { var feature a : Boolean[1]; }
			feature p : M[1];
		}
		package Q { filter R1::p.a; }
	`

	type wantDiagnostic struct {
		code     string
		severity Severity
		message  string
		text     string
	}
	tests := []struct {
		name string
		src  string
		want []wantDiagnostic
	}{
		{"metaclass Boolean chain", boolChain, []wantDiagnostic{
			{"filter-not-evaluated", SeverityWarning, msgFilterNotEvaluated, "filter R1::N::m.a"},
			{"feature-reference-featuring-types", SeverityError, msgSubsettingFeaturingTypes, "R1::N::m"},
		}},
		{"metaclass integer chain", intChain, []wantDiagnostic{
			{"filter-not-boolean", SeverityError, msgFilterNotBoolean, "filter R1::N::m.a"},
		}},
		{"chain comparison", comparison, []wantDiagnostic{
			{"filter-not-evaluated", SeverityWarning, msgFilterNotEvaluated, "filter R1::N::m.a > 2"},
			{"feature-reference-featuring-types", SeverityError, msgSubsettingFeaturingTypes, "R1::N::m"},
		}},
		{"struct-featured chain", structChain, []wantDiagnostic{
			{"filter-not-evaluable", SeverityError, msgFilterNotEvaluable, "filter R1::T::s.x"},
		}},
		{"library metaclass chain", libraryChain, []wantDiagnostic{
			{"filter-not-evaluated", SeverityWarning, msgFilterNotEvaluated, "filter KerML::Root::Element::owner.name"},
		}},
		{"package-level chain", packageChain, []wantDiagnostic{
			{"filter-not-evaluated", SeverityWarning, msgFilterNotEvaluated, "filter R1::p.a"},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := analyzeFilterSource(t, tc.src)
			if len(diags) != len(tc.want) {
				t.Fatalf("got %d diagnostics, want %d: %v", len(diags), len(tc.want), diags)
			}
			for i, want := range tc.want {
				got := diags[i]
				if got.Code != want.code || got.Severity != want.severity || got.Message != want.message {
					t.Errorf("diagnostic %d = %#v, want code=%q severity=%v message=%q", i, got, want.code, want.severity, want.message)
				}
				if !strings.Contains(stringForSpan(t, tc.src, got.Span), want.text) {
					t.Errorf("diagnostic %d span text = %q, want it to contain %q", i, stringForSpan(t, tc.src, got.Span), want.text)
				}
			}
		})
	}
}

func stringForSpan(t *testing.T, src string, span source.Span) string {
	t.Helper()
	if span.Offset < 0 || span.Offset+span.Len > len(src) {
		t.Fatalf("span %#v is outside source of length %d", span, len(src))
	}
	return src[span.Offset : span.Offset+span.Len]
}
