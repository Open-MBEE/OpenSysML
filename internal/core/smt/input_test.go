package smt

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// inputsSrc: n is bound by no default, x by one; the requirement holds at the
// default and fails for other values of n.
const inputsSrc = `package test {
	private import ScalarValues::*;
	enum def Mode { Fast; Slow; }
	action def A {
		attribute n : Integer;
		attribute x : Integer = 1;
		attribute u : Natural;
		attribute m : Mode;
		attribute s : String;
		requirement positive { require x + n > 0; }
		requirement natural { require u >= 0; }
		requirement fast { require m == Mode::Fast; }
		first start;
		action set { assign x := x + 1; }
		done;
		succession first start then set;
		succession first set then done;
	}
}`

// inputByName is the encoding's input named name.
func inputByName(t *testing.T, enc *Encoding, name string) Input {
	t.Helper()
	for _, in := range enc.Inputs {
		if in.Name == name {
			return in
		}
	}
	t.Fatalf("no input %s among %d", name, len(enc.Inputs))
	return Input{}
}

// TestUnboundInputsAreFreeAndBoundOnesPinned: a feature with no default is free
// in its declared domain, one with a default is pinned, and the domains spell
// what the declared type narrows.
func TestUnboundInputsAreFreeAndBoundOnesPinned(t *testing.T) {
	src := strings.Replace(inputsSrc, "attribute s : String;", "", 1)
	ctx, _, action, graph, held := loweredWithIndex(t, "inputs.sysml", src, "test::A")
	enc, err := Encode(ctx, action, graph, held, nil, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	cases := []struct {
		name   string
		free   bool
		domain string
	}{
		{"n", true, ""},
		{"x", false, ""},
		{"u", true, ">= 0"},
		{"m", true, "{Fast, Slow}"},
	}
	for _, c := range cases {
		in := inputByName(t, enc, c.name)
		if in.Free != c.free || in.Released || in.Domain != c.domain {
			t.Errorf("%s: free=%v released=%v domain=%q, want free=%v released=false domain=%q",
				c.name, in.Free, in.Released, in.Domain, c.free, c.domain)
		}
	}
	if in := inputByName(t, enc, "x"); in.Type != "Integer" {
		t.Errorf("x declares %q, want Integer", in.Type)
	}
}

// TestFreeInputRangesOverItsDomain: a requirement that holds at the default
// input is violated for another value; the same violation at a negative value
// alone is proved over Natural and violated over Integer.
func TestFreeInputRangesOverItsDomain(t *testing.T) {
	solver := requireSolver(t)
	src := strings.Replace(inputsSrc, "attribute s : String;", "", 1)
	ctx, idx, action, graph, held := loweredWithIndex(t, "inputs.sysml", src, "test::A")
	enc, err := Encode(ctx, action, graph, held, nil, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	positive, err := enc.Conditions(ctx, []*symbols.Symbol{lookup(t, idx, "test::A::positive")}, nil)
	if err != nil {
		t.Fatalf("positive: %v", err)
	}
	if got := solveStatus(t, solver, enc.Violation(positive)); got != solve.StatusSat {
		t.Errorf("positive over a free n: %v, want sat (n = -1 violates)", got)
	}
	natural, err := enc.Conditions(ctx, []*symbols.Symbol{lookup(t, idx, "test::A::natural")}, nil)
	if err != nil {
		t.Fatalf("natural: %v", err)
	}
	if got := solveStatus(t, solver, enc.Violation(natural)); got != solve.StatusUnsat {
		t.Errorf("u >= 0 over a free Natural: %v, want unsat", got)
	}
	if got := solveStatus(t, solver, enc.Failure(natural)); got != solve.StatusUnsat {
		t.Errorf("failure reading a free Natural: %v, want unsat (it has a value)", got)
	}
	fast, err := enc.Conditions(ctx, []*symbols.Symbol{lookup(t, idx, "test::A::fast")}, nil)
	if err != nil {
		t.Fatalf("fast: %v", err)
	}
	if got := solveStatus(t, solver, enc.Violation(fast)); got != solve.StatusSat {
		t.Errorf("m == Fast over a free Mode: %v, want sat (Slow violates)", got)
	}

	integer := strings.Replace(src, "attribute u : Natural;", "attribute u : Integer;", 1)
	ctx, idx, action, graph, held = loweredWithIndex(t, "integer.sysml", integer, "test::A")
	enc, err = Encode(ctx, action, graph, held, nil, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	natural, err = enc.Conditions(ctx, []*symbols.Symbol{lookup(t, idx, "test::A::natural")}, nil)
	if err != nil {
		t.Fatalf("natural: %v", err)
	}
	if got := solveStatus(t, solver, enc.Violation(natural)); got != solve.StatusSat {
		t.Errorf("u >= 0 over a free Integer: %v, want sat", got)
	}
}

// TestReleasingABoundInputDropsItsBinding: naming a feature with a default frees
// it; naming no feature, or an output, is a typed refusal before any query.
func TestReleasingABoundInputDropsItsBinding(t *testing.T) {
	solver := requireSolver(t)
	// Both n and x bound, so x + n > 0 holds; releasing x lets the binding go.
	src := strings.NewReplacer("attribute s : String;", "out y : Integer;",
		"attribute n : Integer;", "attribute n : Integer = 5;").Replace(inputsSrc)
	ctx, idx, action, graph, held := loweredWithIndex(t, "release.sysml", src, "test::A")
	enc, err := Encode(ctx, action, graph, held, nil, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	positive, err := enc.Conditions(ctx, []*symbols.Symbol{lookup(t, idx, "test::A::positive")}, nil)
	if err != nil {
		t.Fatalf("positive: %v", err)
	}
	if got := solveStatus(t, solver, enc.Violation(positive)); got != solve.StatusUnsat {
		t.Errorf("x + n > 0 with both pinned: %v, want unsat", got)
	}
	released, err := Encode(ctx, action, graph, held, []string{"x"}, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode releasing x: %v", err)
	}
	if in := inputByName(t, released, "x"); !in.Free || !in.Released {
		t.Errorf("x released: free=%v released=%v, want both", in.Free, in.Released)
	}
	positive, err = released.Conditions(ctx, []*symbols.Symbol{lookup(t, idx, "test::A::positive")}, nil)
	if err != nil {
		t.Fatalf("positive: %v", err)
	}
	if got := solveStatus(t, solver, released.Violation(positive)); got != solve.StatusSat {
		t.Errorf("x + n > 0 with x released: %v, want sat (x = -5 violates)", got)
	}

	var refused *analysis.InputError
	if _, err := Encode(ctx, action, graph, held, []string{"nothing"}, 2, DefaultUnroll); !errors.As(err, &refused) || refused.Feature != "nothing" {
		t.Errorf("releasing nothing: %v, want an InputError naming it", err)
	}
	if _, err := Encode(ctx, action, graph, held, []string{"y"}, 2, DefaultUnroll); !errors.As(err, &refused) || refused.Feature != "y" {
		t.Errorf("releasing an output: %v, want an InputError naming it", err)
	}
}

// TestFreeInputWithoutADomainIsRefused: an unbound feature of a type the encoding
// cannot narrow to a sort is refused naming the type, before any query.
func TestFreeInputWithoutADomainIsRefused(t *testing.T) {
	ctx, _, action, graph, held := loweredWithIndex(t, "string.sysml", inputsSrc, "test::A")
	_, err := Encode(ctx, action, graph, held, nil, 2, DefaultUnroll)
	var refused *analysis.DomainError
	if !errors.As(err, &refused) || refused.Feature != "s" || refused.Type != "String" {
		t.Fatalf("encode: %v, want a DomainError naming s : String", err)
	}
	if !errors.Is(err, analysis.ErrDomain) {
		t.Errorf("%v is not ErrNoDomain", err)
	}

	// The same feature bound by a default is pinned, and asks for no domain.
	bound := strings.Replace(inputsSrc, "attribute s : String;", `attribute s : String = "a";`, 1)
	ctx, _, action, graph, held = loweredWithIndex(t, "bound.sysml", bound, "test::A")
	if _, err := Encode(ctx, action, graph, held, nil, 2, DefaultUnroll); err != nil {
		t.Errorf("encode with s bound: %v", err)
	}
}
