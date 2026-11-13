package runtime

import "testing"

// A context no check or exploration runs has its coverage read by nothing, so it
// keeps none; one asked to keep it notes each state entered and transition taken.
func TestCoverageIsKeptOnlyWhereItIsRead(t *testing.T) {
	m := parseExploreModel(t, `package test {
		part def M;
		state def Run {
			entry; then s0;
			state s0;
			transition first s0 then fin;
			state fin;
		}
	}`)
	machine := oneSymbol(t, m.idx, "test::M")
	stateSym := oneSymbol(t, m.idx, "test::Run")
	run := func(keep bool) *Context {
		t.Helper()
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		if keep {
			ctx.keepCoverage()
		}
		self, err := ctx.Instantiate(machine)
		if err != nil {
			t.Fatal(err)
		}
		exec, err := newStateExecutor(ctx, stateSym, self)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.initialize(); err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatal(err)
		}
		return ctx
	}
	if plain := run(false); len(plain.coverage) != 0 {
		t.Errorf("a context nothing reads the coverage of kept %d entries, want none", len(plain.coverage))
	}
	kept := run(true).runCoverage()
	if len(kept.States) != 2 || len(kept.Transitions) != 1 {
		t.Errorf("kept states %+v and transitions %+v, want s0 and fin entered and s0 -> fin taken", kept.States, kept.Transitions)
	}
}
