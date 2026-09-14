package runtime

import "testing"

const exactBudgetSrc = `package test {
	private import ScalarValues::Real;

	part def Engine {
		attribute power : Real = 300.0;
		assert constraint powered { power > 0.0 }
	}

	part def Car {
		attribute mass : Real = 1500.0;
		part engine : Engine;
		assert constraint massive { mass > 0.0 }
	}

	part car : Car;
}`

// A walk whose budget runs out on the last object it reaches has reached the
// whole graph: the object is validated whole, and only a walk with more to read
// is bounded.
func TestValidateObjectExactBudgetIsComplete(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, exactBudgetSrc))
	car, err := ctx.Instantiate(lookupOne(t, idx, "test::car"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	w := ctx.walkHeldObjects(car, maxMaterializeBudget)
	if w.bounded {
		t.Fatal("default budget: bounded, want the finite graph walked whole")
	}
	spent := maxMaterializeBudget - w.budget
	if spent < 2 {
		t.Fatalf("walk spent %d, want at least the engine's read and object", spent)
	}

	report, err := ctx.validateObjectWithin(car, nil, spent)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Bounded {
		t.Error("exact budget: bounded, want the finite graph walked whole")
	}
	if len(report.Verdicts) != 2 {
		t.Errorf("exact budget: %d verdict(s), want 2", len(report.Verdicts))
	}
	if !report.Valid() {
		t.Error("exact budget: valid = false, want true")
	}

	report, err = ctx.validateObjectWithin(car, nil, spent-1)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Bounded {
		t.Error("budget one short: not bounded")
	}
	if report.Valid() {
		t.Error("budget one short: valid = true, want false")
	}
}
