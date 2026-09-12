package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A requirement nested in an action reads the performance's values as the run
// holds them: it holds until a step writes a value that violates it, and the
// violation names the condition.
func TestHoldsReadsThePerformance(t *testing.T) {
	ctx, idx := contextForSource(t, `package test {
	action def A {
		attribute x : Integer = 1;
		requirement positive { require x > 0; }
		constraint small { x < 10 }
		first start;
		action drop { assign x := x - 2; }
		action raise { assign x := x + 20; }
		done;
		succession first start then drop;
		succession first drop then raise;
		succession first raise then done;
	}
}`)
	exec, err := ctx.CreateActionExecutor(lookupOne(t, idx, "test::A"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	positive := lookupOne(t, idx, "test::A::positive")
	small := lookupOne(t, idx, "test::A::small")
	holds := func(sym *symbols.Symbol) (bool, error) { return exec.Holds(sym, nil) }
	for _, sym := range []*symbols.Symbol{positive, small} {
		if ok, err := holds(sym); err != nil || !ok {
			t.Fatalf("%s before the run: %v, %v", sym.Name, ok, err)
		}
	}
	// Move the token off the start node, then perform drop: x becomes -1.
	for range 2 {
		if err := exec.Step(); err != nil {
			t.Fatalf("step: %v", err)
		}
	}
	if got := exec.Data()["x"]; got.Const.Int != -1 {
		t.Fatalf("x = %v after drop, want -1", got)
	}
	ok, err := holds(positive)
	var violation *ViolationError
	if ok || !errors.As(err, &violation) || violation.Condition != "x > 0" {
		t.Fatalf("positive after drop: %v, %v; want a violation of x > 0", ok, err)
	}
	if ok, err := holds(small); err != nil || !ok {
		t.Fatalf("small after drop: %v, %v", ok, err)
	}
	if err := exec.Step(); err != nil {
		t.Fatalf("step: %v", err)
	}
	if ok, err := holds(positive); err != nil || !ok {
		t.Fatalf("positive after raise: %v, %v", ok, err)
	}
	if ok, err := holds(small); ok || !errors.As(err, &violation) || violation.Condition != "x < 10" {
		t.Fatalf("small after raise: %v, %v; want a violation of x < 10", ok, err)
	}
	if _, err := exec.Holds(nil, nil); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("no condition: %v, want ErrNoConditions", err)
	}
}
