package runtime

import (
	"testing"
)

const nodeOccurrenceFixture = `package test {
	private import ScalarValues::*;
	action def Callee { in a : Integer[1]; out r : Integer[1]; assign r := a + 1; }
	part ordered {
		perform action run {
			action one { out v : Integer[1]; assign v := 1; }
			action two { out v : Integer[1]; assign v := one.v + 1; }
			first start then one;
			first one then two;
			first two then done;
		}
	}
	part branched {
		attribute flag : Boolean = true;
		perform action run {
			out attribute k : Integer[1] = 0;
			action br {
				if flag { action inner { out w : Integer[1]; assign w := 7; } assign k := inner.w; }
			}
			first start then br;
			first br then done;
		}
	}
	part invoking {
		perform action run {
			out attribute total : Integer[1] = 0;
			action n = Callee(3);
			action fin { assign total := 1; }
			first start then n;
			first n then fin;
			first fin then done;
		}
	}
	part stopped {
		perform action run {
			action outer {
				fork split;
				action slow { action w accept sig : Integer; first start then w; first w then done; }
				action pre;
				action quit { terminate outer; }
				first start;
				succession first start then split;
				succession first split then slow;
				succession first split then pre;
				succession first pre then quit;
				succession first slow then done;
				succession first quit then done;
			}
			first start then outer;
			first outer then done;
		}
	}
	part nested {
		perform action run {
			action outer {
				fork split;
				action mid {
					fork inner;
					action s1 { action w accept sig : Integer; first start then w; first w then done; }
					action s2 { action w accept sig : Integer; first start then w; first w then done; }
					first start;
					succession first start then inner;
					succession first inner then s1;
					succession first inner then s2;
					succession first s1 then done;
					succession first s2 then done;
				}
				action pre;
				action pre2;
				action pre3;
				action pre4;
				action quit { terminate outer; }
				first start;
				succession first start then split;
				succession first split then mid;
				succession first split then pre;
				succession first pre then pre2;
				succession first pre2 then pre3;
				succession first pre3 then pre4;
				succession first pre4 then quit;
				succession first mid then done;
				succession first quit then done;
			}
			first start then outer;
			first outer then done;
		}
	}
	part unreached {
		perform action run {
			attribute go : Boolean = false;
			action one { out v : Integer[1]; assign v := 1; }
			action never { out v : Integer[1]; assign v := 2; }
			decide pick;
			first start then one;
			first one then pick;
			first pick if go then never;
			first pick if not go then done;
			first never then done;
		}
	}
}`

// heldNodeObject is the object the path from self holds, an action node's performance.
func heldNodeObject(t *testing.T, ctx *Context, self *Instance, path string) *Instance {
	t.Helper()
	fv, err := featureValueAtPath(t, ctx, self, path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	id, ok := fv.HeldValue().Object()
	if !ok {
		t.Fatalf("%s holds %s, want one object", path, fv.HeldValue().Kind)
	}
	inst, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("%s names unknown object #%d", path, id)
	}
	return inst
}

func TestRuntimeRobustnessNodeOccurrence(t *testing.T) {
	instantiate := func(t *testing.T, fqn string) (*Context, *Instance) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nodeOccurrenceFixture))
		self, err := instantiateReporting(ctx, oneSymbol(t, idx, fqn))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", fqn, err)
		}
		return ctx, self
	}

	t.Run("performances_begin_and_end_in_succession_order", func(t *testing.T) {
		ctx, self := instantiate(t, "test::ordered")
		one, two := heldNodeObject(t, ctx, self, "run.one"), heldNodeObject(t, ctx, self, "run.two")
		first, _ := ctx.OccurrenceLife(one.ID)
		second, _ := ctx.OccurrenceLife(two.ID)
		if first.Ended == 0 || second.Ended == 0 {
			t.Fatalf("lives = %v, %v: a completed performance has ended", first, second)
		}
		if first.Ended >= second.Began {
			t.Errorf("one ended at %d, two began at %d: the succession orders them", first.Ended, second.Began)
		}
		run := heldNodeObject(t, ctx, self, "run")
		whole, _ := ctx.OccurrenceLife(run.ID)
		if first.Began < whole.Began || second.Ended > whole.Ended {
			t.Errorf("node lives %v, %v lie outside the performance's %v", first, second, whole)
		}
	})

	t.Run("write_to_the_object_reaches_the_performance", func(t *testing.T) {
		ctx, self := instantiate(t, "test::ordered")
		one := heldNodeObject(t, ctx, self, "run.one")
		if err := one.SetFeatureValue(ctx, "v", intConst(9)); err != nil {
			t.Fatalf("write run.one.v: %v", err)
		}
		results := self.Behaviors()[0].Action.Results()
		if got := results["one.v"]; !valueEqual(got, intConst(9)) {
			t.Errorf("one.v of the run = %v, want the 9 written to its object", got)
		}
	})

	t.Run("branch_node_no_object_holds_still_performs", func(t *testing.T) {
		ctx, self := instantiate(t, "test::branched")
		fv, err := featureValueAtPath(t, ctx, self, "run.k")
		if err != nil {
			t.Fatalf("read run.k: %v", err)
		}
		if got := fv.HeldValue(); !valueEqual(got, intConst(7)) {
			t.Errorf("run.k = %v, want 7", got)
		}
	})

	t.Run("invocation_valued_node_holds_no_object", func(t *testing.T) {
		ctx, self := instantiate(t, "test::invoking")
		fv, err := featureValueAtPath(t, ctx, self, "run.total")
		if err != nil {
			t.Fatalf("read run.total: %v", err)
		}
		if got := fv.HeldValue(); !valueEqual(got, intConst(1)) {
			t.Errorf("run.total = %v, want 1: the run goes on past a node valued by an invocation", got)
		}
	})

	t.Run("terminated_node_performance_has_ended", func(t *testing.T) {
		ctx, self := instantiate(t, "test::stopped")
		slow := heldNodeObject(t, ctx, self, "run.outer.slow")
		if life, _ := ctx.OccurrenceLife(slow.ID); life.Began == 0 || life.Ended == 0 {
			t.Errorf("run.outer.slow life = %v, want one begun and ended by the terminate around it", life)
		}
	})

	t.Run("terminated_performance_ends_after_those_within_it", func(t *testing.T) {
		ctx, self := instantiate(t, "test::nested")
		mid, _ := ctx.OccurrenceLife(heldNodeObject(t, ctx, self, "run.outer.mid").ID)
		for _, path := range []string{"run.outer.mid.s1", "run.outer.mid.s2"} {
			sub, _ := ctx.OccurrenceLife(heldNodeObject(t, ctx, self, path).ID)
			if sub.Began == 0 || sub.Ended == 0 || sub.Ended > mid.Ended {
				t.Errorf("%s life = %v, want one ended no later than mid's %v", path, sub, mid)
			}
		}
	})

	t.Run("node_never_reached_has_not_begun", func(t *testing.T) {
		ctx, self := instantiate(t, "test::unreached")
		never := heldNodeObject(t, ctx, self, "run.never")
		if life, _ := ctx.OccurrenceLife(never.ID); life.Began != 0 {
			t.Errorf("run.never life = %v, want one that never began", life)
		}
		fv, err := never.GetFeatureValue(ctx, "v")
		if err == nil && valueEqual(fv.HeldValue(), intConst(2)) {
			t.Errorf("run.never.v = %v, want no value of a performance that never was", fv.HeldValue())
		}
	})
}
