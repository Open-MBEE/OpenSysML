package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func f23AllDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("<t>", root)
	return Analyze("<t>", root, nil, idx)
}

func TestF21MalformedFlowEndDoesNotPanic(t *testing.T) {
	const src = `package G {
		part def Fuel;
		part def Sys {
			flow of Fuel from to;
		}
	}`
	_ = constraintDiags(t, src)
}

func TestF22FilterErrorNodeDoesNotPanic(t *testing.T) {
	_ = filterDiags(t, `package H { filter (1 + ; }`)
}

func TestF23UnresolvedInvocationDoesNotPanic(t *testing.T) {
	_ = f23AllDiags(t, `package C { attribute x : Integer = Missing(1); }`)
}

func TestF22ConstantFilterOperatorsRemainEvaluable(t *testing.T) {
	const src = `package H {
		package Q {
			filter (1 + 2) * 3 > 0 and not (4 - 1 < 2);
		}
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 0 {
		t.Fatalf("literal arithmetic should be model-level evaluable, got %v", diags)
	}
}

func TestF22EnumerationIdentityRemainsEvaluable(t *testing.T) {
	const src = `package H {
		enum def Levels { high; low; }
		filter Levels::high == Levels::low;
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 0 {
		t.Fatalf("enumeration identity should remain evaluable, got %v", diags)
	}
}

// A chain rooted in a feature with no featuring type is model-level evaluable
// when what it reads is constant, which is what the pinned pilot accepts.
func TestF22ChainFromUnfeaturedRootIsEvaluable(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E2 {
		private import ScalarValues::*;
		part def P { attribute n : Integer = 1; }
		part p : P;
		package Q { filter E2::p.n > 0; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 0 {
		t.Fatalf("`E2::p.n > 0` is model-level evaluable, got %v", diags)
	}
}

// A chain rooted in a feature that a type features is not, and still says so.
func TestF22FeatureChainMessageNamesLimitation(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E3 {
		private import ScalarValues::*;
		part def P { attribute n : Integer = 1; part q : P; }
		part p : P;
		package Q { filter E3::P::q.n > 0; }
	}`
	diags := only(filterDiags(t, src), "filter-not-evaluable")
	if len(diags) != 1 {
		t.Fatalf("expected one not-evaluable diagnostic, got %v", diags)
	}
}

// A chain rooted in an unfeatured feature evaluates through any number of
// hops, parenthesised or not.
func TestFilterChainThroughNestedFeaturesIsEvaluable(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E {
		private import ScalarValues::*;
		attribute root { attribute inner { attribute k : Integer = 3; } }
		package Q { filter E::root.inner.k > 0; }
		package R { filter (E::root.inner).k > 0; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 0 {
		t.Fatalf("`E::root.inner.k > 0` is model-level evaluable, got %v", diags)
	}
}

// The read feature's value evaluates where it is written, so a sibling
// reference in it resolves.
func TestFilterChainDerivedValueIsEvaluable(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E {
		private import ScalarValues::*;
		attribute root { attribute n : Integer = 1; attribute m : Integer = n + 1; }
		package Q { filter E::root.m > 1; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 0 {
		t.Fatalf("`E::root.m > 1` is model-level evaluable, got %v", diags)
	}
}

// A valued feature before the last hop redirects the chain's evaluation, which
// is not followed: the condition reports that it is not evaluated.
func TestFilterChainThroughValuedHopReportsLimitation(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E {
		private import ScalarValues::*;
		attribute other { attribute k : Integer = 1; }
		attribute root { attribute inner = other; }
		package Q { filter E::root.inner.k > 0; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluated"); len(diags) != 1 {
		t.Fatalf("expected one not-evaluated diagnostic, got %v", diags)
	}
}

// A chain whose terminal is a feature of a metaclass is read reflectively
// from the candidate, which OpenSysML does not evaluate.
func TestFilterChainIntoMetaclassFeatureReportsLimitation(t *testing.T) {
	const src = `package ScalarValues { attribute def Boolean; }
	package E {
		private import ScalarValues::*;
		metaclass M { var feature a : Boolean[1]; }
		feature p : M[1];
		package Q { filter E::p.a; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluated"); len(diags) != 1 {
		t.Fatalf("expected one not-evaluated diagnostic, got %v", diags)
	}
}

// A read feature with no value of its own reports the limitation rather than
// comparing the feature itself.
func TestFilterChainUnvaluedTerminalReportsLimitation(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E {
		private import ScalarValues::*;
		attribute root { attribute inner { attribute k : Integer; } }
		package Q { filter E::root.inner.k == 3; }
	}`
	diags := filterDiags(t, src)
	if got := only(diags, "filter-not-evaluated"); len(got) != 1 {
		t.Fatalf("expected one not-evaluated diagnostic, got %v", diags)
	}
	if got := only(diags, "filter-not-evaluable"); len(got) != 0 {
		t.Fatalf("expected no not-evaluable diagnostic, got %v", diags)
	}
}

// A member segment written as a qualified name that names a member of the
// preceding feature still evaluates.
func TestFilterChainQualifiedMemberSegmentIsEvaluable(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E {
		private import ScalarValues::*;
		attribute root { attribute inner { attribute k : Integer = 3; } }
		package Q { filter E::root.inner::k == 3; }
	}`
	if diags := filterDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no filter diagnostics, got %v", diags)
	}
}

// A read feature whose value is not a number or boolean reports the same.
func TestFilterChainNonNumericValueReportsLimitation(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; attribute def String; }
	package E {
		private import ScalarValues::*;
		attribute root { attribute s : String = "x"; }
		package Q { filter E::root.s == "x"; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluated"); len(diags) != 1 {
		t.Fatalf("expected one not-evaluated diagnostic, got %v", diags)
	}
}

func TestF23BehavioralTargetsRemainInvocable(t *testing.T) {
	const src = `package C {
		calc def Twice {
			in x : Integer;
			return : Integer = x * 2;
		}
		attribute y : Integer = Twice(2);
		calc t : Twice;
		attribute z : Integer = t(3);
	}`
	for _, d := range f23AllDiags(t, src) {
		if d.Code == "invocation-not-behavior" {
			t.Fatalf("behavioral invocation rejected: %v", d)
		}
	}
}
