package passes

import (
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// filterDiags returns the element-filter findings of src.
func filterDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range analyzeFilterSource(t, src) {
		if strings.HasPrefix(d.Code, "filter-") {
			out = append(out, d)
		}
	}
	return out
}

func analyzeFilterSource(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	const name = "<t>.kerml"
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	return Analyze(name, root, nil, idx)
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

// except returns the findings without one code, for tests whose subject is a
// different rule.
func except(diags []diag.Diagnostic, code string) []diag.Diagnostic {
	var out []diag.Diagnostic
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
			if diags[0].Severity != diag.SeverityError {
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
			if diags[0].Severity != diag.SeverityError {
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
		severity diag.Severity
		message  string
		text     string
	}
	tests := []struct {
		name string
		src  string
		want []wantDiagnostic
	}{
		{"metaclass Boolean chain", boolChain, []wantDiagnostic{
			{"filter-not-evaluated", diag.SeverityWarning, msgFilterNotEvaluated, "filter R1::N::m.a"},
			{"feature-reference-featuring-types", diag.SeverityError, msgSubsettingFeaturingTypes, "R1::N::m"},
		}},
		{"metaclass integer chain", intChain, []wantDiagnostic{
			{"filter-not-boolean", diag.SeverityError, msgFilterNotBoolean, "filter R1::N::m.a"},
		}},
		{"chain comparison", comparison, []wantDiagnostic{
			{"filter-not-evaluated", diag.SeverityWarning, msgFilterNotEvaluated, "filter R1::N::m.a > 2"},
			{"feature-reference-featuring-types", diag.SeverityError, msgSubsettingFeaturingTypes, "R1::N::m"},
		}},
		{"struct-featured chain", structChain, []wantDiagnostic{
			{"filter-not-evaluable", diag.SeverityError, msgFilterNotEvaluable, "filter R1::T::s.x"},
		}},
		{"library metaclass chain", libraryChain, []wantDiagnostic{
			{"filter-not-evaluated", diag.SeverityWarning, msgFilterNotEvaluated, "filter KerML::Root::Element::owner.name"},
		}},
		{"package-level chain", packageChain, []wantDiagnostic{
			{"filter-not-evaluated", diag.SeverityWarning, msgFilterNotEvaluated, "filter R1::p.a"},
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

// nestedNots writes a filter condition of n `not` operators around one classification test.
func nestedNots(n int) string {
	return filterMetadata + "package P { filter " + strings.Repeat("not (", n) + "@Safety" + strings.Repeat(")", n) + "; }\n"
}

// TestAFilterNestedPastTheBoundIsNotEvaluated: the pass recurses once per operator, so a
// condition past the bound is reported once as not evaluated instead of being walked.
func TestAFilterNestedPastTheBoundIsNotEvaluated(t *testing.T) {
	if diags := filterDiags(t, nestedNots(semantics.MaxFilterDepth-1)); len(diags) != 0 {
		t.Fatalf("a condition inside the bound: got %v, want no findings", diags)
	}
	diags := filterDiags(t, nestedNots(semantics.MaxFilterDepth+1))
	if len(diags) != 1 || diags[0].Code != "filter-not-evaluated" {
		t.Fatalf("a condition past the bound: got %v, want one filter-not-evaluated", diags)
	}
}

// TestAFilterNestedPastTheBoundInsideAConstructorIsNotEvaluated: the levels under a
// constructor's argument count toward the bound like any other.
func TestAFilterNestedPastTheBoundInsideAConstructorIsNotEvaluated(t *testing.T) {
	n := semantics.MaxFilterDepth + 1
	src := filterMetadata + "package P { filter new Safety(" + strings.Repeat("not (", n) + "true" + strings.Repeat(")", n) + ") == null; }\n"
	diags := filterDiags(t, src)
	if len(diags) != 1 || diags[0].Code != "filter-not-evaluated" {
		t.Fatalf("got %v, want one filter-not-evaluated", diags)
	}
}

// TestAFilterNestedPastTheBoundStillMustBeBoolean: the bound stops the evaluation of a
// condition, not the rule that it yields a truth value.
func TestAFilterNestedPastTheBoundStillMustBeBoolean(t *testing.T) {
	n := semantics.MaxFilterDepth + 1
	nots, closes := strings.Repeat("not (", n), strings.Repeat(")", n)
	for _, tc := range []struct{ name, condition string }{
		{"a sum", "1" + strings.Repeat(" + 1", n)},
		{"a negation", strings.Repeat("-(", n) + "1" + closes},
		{"an integer under nots", nots + "1" + closes},
		{"a sum under nots", nots + "1 + 1" + closes},
		{"an integer ending a conjunction", strings.Repeat("@Safety and ", n) + "1"},
		{"an integer heading a conjunction", "1" + strings.Repeat(" and @Safety", n)},
		{"a constructor", "new Safety(" + nots + "true" + closes + ")"},
		{"a constructor over a sum", "new Safety(1" + strings.Repeat(" + 1", n) + ")"},
		{"a conditional", strings.Repeat("if true ? 1 else ", n) + "2"},
		{"a conditional under nots", nots + strings.Repeat("if true ? 1 else ", n) + "2" + closes},
		{"a sum compared under nots", nots + "1" + strings.Repeat(" + 1", n) + closes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := filterDiags(t, filterMetadata+"package P { filter "+tc.condition+"; }\n")
			if len(diags) != 1 || diags[0].Code != "filter-not-boolean" || diags[0].Severity != diag.SeverityError {
				t.Fatalf("got %v, want one filter-not-boolean error", diags)
			}
		})
	}
}

// TestAFilterNestedPastTheBoundOnAnUnresolvedTypeSaysNothingMore: the lower tier's failure
// gates a condition at any depth.
func TestAFilterNestedPastTheBoundOnAnUnresolvedTypeSaysNothingMore(t *testing.T) {
	n := semantics.MaxFilterDepth + 1
	src := "package P { filter " + strings.Repeat("not (", n) + "@Missing" + strings.Repeat(")", n) + "; }\n"
	if diags := filterDiags(t, src); len(diags) != 0 {
		t.Fatalf("got %v, want no filter findings", diags)
	}
}

// TestAFilterNestedPastTheBoundOnAnUnresolvedNameIsNotEvaluable: an operand that names
// nothing leaves the condition without a result, as it does within the bound.
func TestAFilterNestedPastTheBoundOnAnUnresolvedNameIsNotEvaluable(t *testing.T) {
	n := semantics.MaxFilterDepth + 1
	for _, tc := range []struct{ name, condition string }{
		{"a sum", "missing" + strings.Repeat(" + missing", n)},
		{"a negation", strings.Repeat("-(", n) + "missing" + strings.Repeat(")", n)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := filterDiags(t, "package P { filter "+tc.condition+"; }\n")
			if len(diags) != 1 || diags[0].Code != "filter-not-evaluable" {
				t.Fatalf("got %v, want one filter-not-evaluable error", diags)
			}
		})
	}
}

// TestAFilterNestedPastTheBoundOnAnOperatorOutsideTheSubsetIsNotEvaluable: an operator a
// condition cannot be written with is refused at any depth.
func TestAFilterNestedPastTheBoundOnAnOperatorOutsideTheSubsetIsNotEvaluable(t *testing.T) {
	n := semantics.MaxFilterDepth + 1
	for _, tc := range []struct{ name, condition string }{
		{"a null-coalescing chain", "1" + strings.Repeat(" ?? 1", n)},
		{"a conditional on the candidate", strings.Repeat("if @Safety ? 1 else ", n) + "2"},
		{"a division by zero", "1 / 0" + strings.Repeat(" + 1", n)},
		{"a sequence", strings.Repeat("(1, ", n) + "1" + strings.Repeat(")", n)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := filterDiags(t, filterMetadata+"package P { filter "+tc.condition+"; }\n")
			if len(diags) != 1 || diags[0].Code != "filter-not-evaluable" || diags[0].Severity != diag.SeverityError {
				t.Fatalf("got %v, want one filter-not-evaluable error", diags)
			}
		})
	}
}

// TestAFilterNestedPastTheBoundThatFoldsToATruthValueIsOnlyNotEvaluated: a constant
// condition is in no error however deep it is written.
func TestAFilterNestedPastTheBoundThatFoldsToATruthValueIsOnlyNotEvaluated(t *testing.T) {
	n := semantics.MaxFilterDepth + 1
	for _, tc := range []struct{ name, condition string }{
		{"a conditional", strings.Repeat("if false ? 1 else ", n) + "true"},
		{"a comparison of sums", "1" + strings.Repeat(" + 1", n) + " == 1" + strings.Repeat(" + 1", n)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := filterDiags(t, "package P { filter "+tc.condition+"; }\n")
			if len(diags) != 1 || diags[0].Code != "filter-not-evaluated" || diags[0].Severity != diag.SeverityWarning {
				t.Fatalf("got %v, want one filter-not-evaluated warning", diags)
			}
		})
	}
}

// TestAFilterNestedPastTheBoundIsInTheErrorsItIsInWithinIt: the bound moves no error, so
// a shape reports the same filter errors written a few levels deep and past the bound.
func TestAFilterNestedPastTheBoundIsInTheErrorsItIsInWithinIt(t *testing.T) {
	const reflective = `package R1 {
			private import ScalarValues::*;
			metaclass M { var feature a : Boolean[1]; }
			feature p : M[1];
		}
	`
	const linked = `package Q {
			part def Link { part onward : Link; attribute ready : ScalarValues::Boolean; }
			part head : Link;
		}
	`
	errorsOf := func(src string) []string {
		var codes []string
		for _, d := range filterDiags(t, src) {
			if d.Severity == diag.SeverityError {
				codes = append(codes, d.Code)
			}
		}
		slices.Sort(codes)
		return codes
	}
	for _, tc := range []struct {
		name      string
		prelude   string
		condition func(n int) string
		want      []string
	}{
		{"a chain from a package feature", linked,
			func(n int) string { return "Q::head" + strings.Repeat(".onward", n) + ".ready" }, nil},
		{"a chain from a package feature to no feature", linked,
			func(n int) string { return "Q::head" + strings.Repeat(".onward", n) + ".absent" },
			[]string{"filter-not-boolean"}},
		{"a constructor over a conjunction on the candidate", filterMetadata,
			func(n int) string {
				return "new Safety((if @Safety ? true else false)" + strings.Repeat(" and @Safety", n) + ")"
			},
			[]string{"filter-not-boolean"}},
		{"a reflective chain beside a null-coalescing chain", reflective,
			func(n int) string { return "R1::p.a and (1" + strings.Repeat(" ?? 1", n) + ")" },
			[]string{"filter-not-evaluable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{8, semantics.MaxFilterDepth + 1} {
				got := errorsOf(tc.prelude + "package P { filter " + tc.condition(n) + "; }\n")
				if !slices.Equal(got, tc.want) {
					t.Errorf("nested %d deep: got filter errors %v, want %v", n, got, tc.want)
				}
			}
		})
	}
}

// TestAFilterNestedPastTheBoundCostsNothingForABranchNotTaken: finding the faults of a
// conditional folds the branch it takes, not the sum of a million-bit power beside it.
func TestAFilterNestedPastTheBoundCostsNothingForABranchNotTaken(t *testing.T) {
	src := "package P { filter if true ? true else (2 ** 1000000" + strings.Repeat(" + 1", 10_000) + " > 0); }\n"
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	diags := filterDiags(t, src)
	runtime.ReadMemStats(&after)
	if len(diags) != 1 || diags[0].Code != "filter-not-evaluated" {
		t.Fatalf("got %v, want one filter-not-evaluated warning", diags)
	}
	if spent := after.TotalAlloc - before.TotalAlloc; spent > 256<<20 {
		t.Errorf("checking it allocated %d MiB, want under 256: the sum is in the branch not taken", spent>>20)
	}
}
