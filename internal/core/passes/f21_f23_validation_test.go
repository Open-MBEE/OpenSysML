package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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
