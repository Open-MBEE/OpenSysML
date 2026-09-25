package runtime

import "testing"

// Two activations of one action run in one context, each writing the part the
// action declares. The usage denotes one object for the context, which both read
// and write, so each reports `target.total` rather than dropping the part as several.
func TestResultsReportPartsOfEachActivation(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
	private import ScalarValues::*;
	part def Probe {
		attribute total : Real = 0.0;
	}
	action def Cfg {
		in amount : Real;
		part target : Probe;
		action set { assign target.total := amount; }
		first start then set;
		first set then done;
	}
}`)
	cfg := lookupOne(t, idx, "test::Cfg")
	run := func(amount float64) *ActionExecutor {
		exec, err := newActionExecutor(ctx, cfg, nil)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		exec.SetInputs(map[string]Value{"amount": realOf(amount)})
		if err := exec.initialize(); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		for exec.State() == StateRunning {
			if err := exec.Step(); err != nil {
				t.Fatalf("step: %v", err)
			}
		}
		return exec
	}
	first := run(1)
	if got, ok := first.Results()["target.total"]; !ok || got.Const.Real != 1 {
		t.Fatalf("first target.total = %v, %v; want 1", got, ok)
	}
	second := run(2)
	if got, ok := second.Results()["target.total"]; !ok || got.Const.Real != 2 {
		t.Fatalf("second target.total = %v, %v; want 2", got, ok)
	}
	if got, ok := first.Results()["target.total"]; !ok || got.Const.Real != 2 {
		t.Fatalf("first target.total after the second run = %v, %v; want 2", got, ok)
	}
}
