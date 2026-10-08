package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	checkpasses "github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestRuntimeRobustnessRepeatedStepCoverage(t *testing.T) {
	// A bind at a repeated step's out-pin takes the one value every performance
	// agrees on; differing outputs are the conflict a binding cannot resolve.
	t.Run("out-pin-binding-conflict", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute c : Integer = 0;
				attribute r : Integer[0..1];
				first start then b;
				action b[2] {
					out y : Integer;
					assign c := c + 1;
					assign y := c;
				}
				bind b.y = r;
				succession first [*] b then [1] done;
			}
		}`)
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("execution error = %v, want ErrBindingConflict", err)
		}
		var conflict *BindingConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("execution error = %v, want *BindingConflictError", err)
		}
		if !strings.Contains(err.Error(), "y") {
			t.Errorf("execution error = %q, want the pin named", err)
		}
		for _, held := range []Value{conflict.LeftValue, conflict.RightValue} {
			if held.Kind != ValConst || held.Const.Kind != semantics.ValInt {
				t.Fatalf("conflict ends = %v and %v, want the two performance outputs", conflict.LeftValue, conflict.RightValue)
			}
		}
		got := map[int64]bool{conflict.LeftValue.Const.Int: true, conflict.RightValue.Const.Int: true}
		if !got[1] || !got[2] {
			t.Errorf("conflict ends = %v and %v, want the outputs 1 and 2 of the two performances", conflict.LeftValue, conflict.RightValue)
		}
	})

	// A bind at a repeated step's in-pin takes a single value for every
	// performance; a multi-valued end is a distribution the model leaves open.
	t.Run("in-pin-multi-valued-end", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute k : Integer[2] = (1, 2);
				first start then a;
				action a[2] { in x : Integer; }
				bind a.x = k;
				succession first [*] a then [1] done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) {
			t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
		}
		if stepErr.Code != lower.StepMultiplicityUnsupportedCode {
			t.Errorf("step error code = %q, want %q", stepErr.Code, lower.StepMultiplicityUnsupportedCode)
		}
		const reason = "a binding distributes a multi-valued end over the performances in an assignment the model leaves open"
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("execution error = %q, want reason %q", err, reason)
		}
	})

	// A read of a repeated step's pin before every performance has ended is the
	// error a read of a not-yet-performed step gives.
	t.Run("read-before-performed", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			private import CollectionFunctions::*;
			action def A {
				attribute n : Integer = 0;
				first start then q;
				action q { assign n := size(a.x); }
				succession first q then [*] a;
				action a[2] { out x : Integer = 1; }
				succession first [*] a then [1] done;
			}
		}`)
		if !errors.Is(err, ErrNodeNotPerformed) {
			t.Fatalf("execution error = %v, want ErrNodeNotPerformed", err)
		}
	})

	// Object flows at pins of a repeated step stay refused.
	t.Run("flow-at-repeated-pin", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then a;
				action a[3] { out o : Integer = 1; }
				succession first [*] a then [1] b;
				action b { in i : Integer; }
				flow a.o to b.i;
				then done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) {
			t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
		}
		if stepErr.Code != lower.StepMultiplicityUnsupportedCode {
			t.Errorf("step error code = %q, want %q", stepErr.Code, lower.StepMultiplicityUnsupportedCode)
		}
	})

	// `perform action run[0]` enacts no performance; `run[2]` enacts two, each
	// adding one to the part's `count`.
	t.Run("perform-action-counts-on-part", func(t *testing.T) {
		for _, test := range []struct {
			multiplicity string
			want         int64
		}{
			{"[0]", 0},
			{"[2]", 2},
		} {
			t.Run("run"+test.multiplicity, func(t *testing.T) {
				file := parseAndBuild(t, `package test {
					private import ScalarValues::*;
					part def Host {
						attribute count : Integer = 0;
						perform action run`+test.multiplicity+` {
							action step { assign count := count + 1; }
							first step;
						}
					}
				}`)
				index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
				symbol := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
				if symbol == nil {
					t.Fatal("part Host not found")
				}
				inst, err := ctx.Instantiate(symbol)
				if err != nil {
					t.Fatalf("Instantiate: %v", err)
				}
				if got := featureIntValue(t, ctx, inst, "count"); got != test.want {
					t.Errorf("count = %d, want %d", got, test.want)
				}
				if test.want == 2 {
					assertDistinctRunOccurrences(t, ctx, inst)
				}
			})
		}
	})

	// `perform action run[2]` starts two performances, which PerformedActionsOf
	// returns both of; running `run` a third time is ambiguous under `run`.
	t.Run("perform-repeated-actions-listed-and-ambiguous", func(t *testing.T) {
		file := parseAndBuild(t, `package test {
			part def Host {
				perform action run[2];
			}
		}`)
		index, _, ctx := buildRuntime(t, "<test>", file)
		host := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
		inst, err := ctx.Instantiate(host)
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		run := resolveSymbol(t, host.Scope, "run")
		performed := inst.PerformedActionsOf(run)
		if len(performed) != 2 {
			t.Fatalf("PerformedActionsOf(run) = %d behaviors, want 2", len(performed))
		}
		if _, err := ctx.performanceOf(run, inst, nil); !errors.Is(err, ErrAmbiguousAction) ||
			!strings.Contains(err.Error(), "2 times, under run") {
			t.Errorf("performanceOf(run) = %v, want ErrAmbiguousAction wording the shared usage", err)
		}
	})

	// A repeated step inside a while inside a for counts its performances once
	// per pass of the innermost body.
	t.Run("nested-while-in-for", func(t *testing.T) {
		outputs, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute c : Integer = 0;
				attribute j : Integer = 0;
				first start then worker;
				action worker {
					for i : Integer in (1, 2) {
						assign j := 0;
						while j < 2 {
							first start then a;
							action a[3] { assign c := c + 1; }
							succession first [*] a then [1] t;
							action t { assign j := j + 1; }
						}
					}
				}
				then done;
			}
		}`)
		if err != nil {
			t.Fatalf("executeActionSource: %v", err)
		}
		assertIntOutput(t, outputs, "c", 12)
	})

	// A non-fixed count stays refused inside a loop body, same as at action
	// level.
	t.Run("non-fixed-in-loop-body", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute i : Integer = 0;
				first start then worker;
				action worker {
					while i < 1 {
						first start then a;
						action a[0..2] { }
						assign i := i + 1;
					}
				}
				then done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) {
			t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
		}
		if stepErr.Code != lower.StepMultiplicityNotFixedCode {
			t.Errorf("step error code = %q, want %q", stepErr.Code, lower.StepMultiplicityNotFixedCode)
		}
	})

	// A join with another incoming succession cannot order under the repeated
	// step's count: the other source performs once, the join per performance.
	t.Run("join-with-another-incoming", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then b;
				action b;
				action a[3];
				succession first a then j;
				succession first b then j;
				join j;
				then done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderUnsatisfiableCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepOrderUnsatisfiableCode)
		}
	})

	// A fork's outgoing succession fixes its count to the repeated step's, so a
	// predecessor performing once cannot order every crossing at the fork.
	t.Run("fork-drives-repeated-step", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then b;
				action b;
				then f;
				fork f;
				action a[3];
				succession first f then a;
				succession first [*] a then [1] done;
			}
		}`)
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderUnsatisfiableCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepOrderUnsatisfiableCode)
		}
	})

	// Under the one-performance reading a merge carrying another incoming
	// succession beside the repeated step's cannot order its one performance
	// against three.
	t.Run("merge-another-incoming-unsatisfiable", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then b;
				action b;
				action a[3];
				succession first a then m;
				succession first b then m;
				merge m;
				then done;
			}
		}`)
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderUnsatisfiableCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepOrderUnsatisfiableCode)
		}
	})

	// A body's declaration order is the executor's own, not a stated order:
	// lone statements beside a repeated step are the open order it reports.
	t.Run("loop-body-declaration-order", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute i : Integer = 0;
				first start then worker;
				action worker {
					while i < 1 {
						action a[2] { assign i := i + 1; }
						assign i := i + 10;
					}
				}
				then done;
			}
		}`)
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderOpenCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepOrderOpenCode)
		}
		const reason = "the body states no succession, so its declaration order is the executor's and does not order every performance"
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("execution error = %v, want reason %q", err, reason)
		}
	})

	// A repeated step inside a loop or if body is each run as one move, so the
	// orders between its performances are the ones exploration never varies:
	// explore and check record the note rather than claim the orders covered.
	t.Run("block-body-repetition-is-observed", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			private import ScalarValues::*;
			action def LoopRace {
				attribute c : Integer = 0;
				attribute passes : Integer = 0;
				first start then worker;
				action worker {
					while passes < 1 {
						first start then a;
						action a[2] {
							attribute t : Integer := c;
							assign c := t + 1;
						}
						succession first [*] a then [1] tally;
						action tally { assign passes := passes + 1; }
					}
				}
				then done;
			}
			action def LoneOnly {
				attribute c : Integer = 0;
				first start then worker;
				action worker {
					if true {
						action a[2] {
							attribute t : Integer := c;
							assign c := t + 1;
						}
					}
				}
				then done;
			}
			action def FlatAlone {
				attribute c : Integer = 0;
				first start then a;
				action a[2] { assign c := c + 1; }
				succession first [*] a then [1] done;
			}
		}`)
		notes := func(name string) []string {
			x := m.exploreAction(t, "explore", name)
			if !x.Complete() {
				t.Fatalf("exploration incomplete: %s", x.Status())
			}
			return x.Notes
		}
		for _, name := range []string{"LoopRace", "LoneOnly"} {
			if got := notes(name); len(got) != 1 || got[0] != ReasonBlockBodyRepetition {
				t.Errorf("%s notes = %v, want [%q]", name, got, ReasonBlockBodyRepetition)
			}
		}
		// Check records the same note where it searches the body at all.
		report := checkStart(t, m, starterOf(m.action(t, "LoneOnly")), unreduced())
		if len(report.Notes) != 1 || report.Notes[0] != ReasonBlockBodyRepetition {
			t.Errorf("LoneOnly check notes = %v, want [%q]", report.Notes, ReasonBlockBodyRepetition)
		}
		if got := notes("FlatAlone"); len(got) != 0 {
			t.Errorf("FlatAlone notes = %v, want none", got)
		}
		flatReport := checkStart(t, m, starterOf(m.action(t, "FlatAlone")), unreduced())
		if len(flatReport.Notes) != 0 {
			t.Errorf("FlatAlone check notes = %v, want none", flatReport.Notes)
		}
	})

	// A snapshot restores the coverage notes it captured: one taken before the
	// run clears them, one taken after keeps them.
	t.Run("block-body-notes-restore", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
			action def LoneOnly {
				first start then worker;
				action worker {
					if true {
						action a[2];
					}
				}
				then done;
			}
		}`))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "LoneOnly", ast.DefAction)
		before, err := ctx.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if _, err := ctx.ExecuteAction(sym); err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if len(ctx.coverageReasons()) == 0 {
			t.Fatalf("no coverage note recorded")
		}
		after, err := ctx.Snapshot()
		if err != nil {
			t.Fatalf("Snapshot after the run: %v", err)
		}
		after.Restore()
		if got := ctx.coverageReasons(); !slices.Contains(got, ReasonBlockBodyRepetition) {
			t.Errorf("notes after restoring the later snapshot = %v, want the block-body reason", got)
		}
		before.Restore()
		if got := ctx.coverageReasons(); len(got) != 0 {
			t.Errorf("notes after restoring the earlier snapshot = %v, want none", got)
		}
	})

	// A written [*] end into a join contradicts the end multiplicity SysML
	// mandates there, and is unsatisfiable rather than a barrier.
	t.Run("wildcard-into-join-contradicts-mandate", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then a;
				action a[3];
				succession first [*] a then j;
				join j;
				then done;
			}
		}`)
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderUnsatisfiableCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepOrderUnsatisfiableCode)
		}
		if !strings.Contains(err.Error(), "contradicts the one SysML requires at a join node") {
			t.Errorf("execution error = %q, want the mandated-end reason", err)
		}
	})

	// A guard on an edge whose source is a repeated step stays refused: the
	// grammar admits no source-end multiplicity there.
	t.Run("guard-out-of-repeated-step", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then a;
				action a[3];
				action q;
				succession first a if true then q;
				succession first [*] a then [1] done;
			}
		}`)
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepMultiplicityUnsupportedCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepMultiplicityUnsupportedCode)
		}
	})

	// The merge every performance crosses performs n times, which a successor
	// performing once cannot order.
	t.Run("merge-successor-under-per-performance-count", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				first start then a;
				action a[3];
				succession first a then m;
				merge m;
				action q;
				succession first m then q;
				then done;
			}
		}`)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
	})

	// Explore agrees with run: the false guard is the open order one error
	// outcome reports.
	t.Run("explore-guard-false", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			private import ScalarValues::*;
			action def GuardFalse {
				attribute c : Integer = 0;
				attribute g : Boolean = false;
				first start then p;
				action p;
				succession first p if g then [*] a;
				action a[3] { assign c := c + 1; }
				succession first [*] a then [1] done;
			}
		}`)
		x := m.exploreAction(t, "explore", "GuardFalse")
		if len(x.Outcomes) != 1 {
			t.Fatalf("outcomes %v, want exactly one", outcomeTexts(x))
		}
		outcome := x.Outcomes[0].Outcome
		if outcome.Err == nil {
			t.Fatalf("outcome error = nil, want the open-order error")
		}
		if !strings.Contains(outcome.Err.Error(), "a false guard leaves the performances of the repeated step unordered") {
			t.Errorf("outcome error = %v, want the guard's open-order reason", outcome.Err)
		}
	})

	// A nested repeated read yields the sequence over performances, which a
	// single-valued target cannot take.
	t.Run("repeated-read-into-single-valued", func(t *testing.T) {
		_, err := executeActionSource(t, "A", `package test {
			private import ScalarValues::*;
			action def A {
				attribute total : Integer = 0;
				first start then outer;
				action outer {
					first start then inner;
					action inner[3] { attribute x : Integer = 1; }
					then done;
				}
				succession first outer then q;
				action q { assign total := outer.inner.x; }
				then done;
			}
		}`)
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("execution error = %v, want ErrMultiplicityViolation", err)
		}
	})

	// Explore agrees with run: both sibling performances of `a` interleave but
	// accrue to one outcome.
	t.Run("explore-pin-value", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			private import ScalarValues::*;
			private import NumericalFunctions::*;

			action def Inc {
				in x : Integer;
				out y : Integer;
				first start then bump;
				action bump { assign y := x + 1; }
				then done;
			}

			action def PinValue {
				attribute c : Integer = 4;
				attribute total : Integer = 0;
				first start then a;
				action a : Inc[2] { in x = c; }
				succession first [*] a then [1] q;
				action q { assign total := sum(a.y); }
				succession first q then done;
			}
		}`)
		x := m.exploreAction(t, "explore", "PinValue")
		if !x.Complete() {
			t.Fatalf("exploration incomplete: %s", x.Status())
		}
		if len(x.Outcomes) != 1 {
			t.Fatalf("outcomes %v, want exactly one", outcomeTexts(x))
		}
		outcome := x.Outcomes[0].Outcome
		if outcome.Err != nil {
			t.Fatalf("outcome error = %v", outcome.Err)
		}
		if got := intValue(t, outcome.Outputs, "total"); got != 10 {
			t.Errorf("total = %d, want 10", got)
		}
	})

	// A succession flow out of a repeated step's pin is an object flow at a
	// repeated pin: run refuses it, and the pass warns the same.
	t.Run("succession-flow-at-repeated-pin", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			action def A {
				first start then a;
				action a[2] { out y : Integer; assign y := 1; }
				succession flow of Integer from a.y to b.x;
				action b { in x : Integer; }
				then done;
			}
		}`
		file := parseAndBuild(t, src)
		index, _, _ := buildRuntimeWithLibraries(t, "<test>", file)
		diagnostics := checkpasses.Analyze("<test>", file, nil, index)
		var checked *diag.Diagnostic
		for i := range diagnostics {
			if diagnostics[i].Code == "action-step-multiplicity-unsupported" {
				checked = &diagnostics[i]
				break
			}
		}
		if checked == nil {
			t.Fatalf("check diagnostics have no action-step-multiplicity-unsupported warning")
		}
		if !strings.Contains(checked.Message, "object flows at pins of a repeated action step") {
			t.Errorf("check warning = %q, want the repeated-pin flow refusal", checked.Message)
		}
		_, err := executeActionSource(t, "A", src)
		if !errors.Is(err, ErrActionStepMultiplicity) {
			t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
		}
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepMultiplicityUnsupportedCode {
			t.Fatalf("execution error = %v, want StepMultiplicityUnsupportedCode", err)
		}
	})

	// `perform action run[2]` as a succession's later end cannot pair under
	// KERML-29: a behavior-order finding notes it, and both performances run.
	t.Run("namespace-succession-later-end-repeated", func(t *testing.T) {
		file := parseAndBuild(t, `package test {
			private import ScalarValues::*;
			part def Host {
				attribute n : Integer = 0;
				perform action prep {
					first start;
					then action one { assign n := n + 10; }
					then done;
				}
				perform action run[2] {
					first start;
					then action one { assign n := n + 1; }
					then done;
				}
				first prep then run;
			}
		}`)
		index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
		host := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
		inst, err := ctx.Instantiate(host)
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		notes := ctx.Notes()
		run := resolveSymbol(t, host.Scope, "run")
		performed := inst.PerformedActionsOf(run)
		if len(performed) != 2 {
			t.Fatalf("PerformedActionsOf(run) = %d behaviors, want 2", len(performed))
		}
		for i, behavior := range performed {
			if behavior.deferred != nil {
				t.Errorf("run performance %d remains held", i)
			}
		}
		if got := featureIntValue(t, ctx, inst, "n"); got != 12 {
			t.Errorf("n = %d, want 12 (prep once and both run performances)", got)
		}
		var findings int
		for _, note := range notes {
			if finding, ok := note.(SuccessionOrdersNothing); ok &&
				strings.Contains(finding.Reason, "2 later-end performances") {
				findings++
			}
		}
		if findings != 1 {
			t.Errorf("later-end findings = %d, want one KERML-29 pairing note", findings)
		}
	})

	// `perform action run[2]` as a succession's earlier end cannot pair either:
	// the later end releases with a finding, and both performances still run.
	t.Run("namespace-succession-earlier-end-repeated", func(t *testing.T) {
		file := parseAndBuild(t, `package test {
			private import ScalarValues::*;
			part def Host {
				attribute n : Integer = 0;
				attribute m : Integer = 0;
				perform action run[2] {
					first start;
					then action one { assign n := n + 1; }
					then done;
				}
				perform action prep {
					first start;
					then action one { assign m := m + 1; }
					then done;
				}
				first run then prep;
			}
		}`)
		index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
		host := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
		inst, err := ctx.Instantiate(host)
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		notes := ctx.Notes()
		run := resolveSymbol(t, host.Scope, "run")
		if performed := inst.PerformedActionsOf(run); len(performed) != 2 {
			t.Fatalf("PerformedActionsOf(run) = %d behaviors, want 2", len(performed))
		}
		if got := featureIntValue(t, ctx, inst, "n"); got != 2 {
			t.Errorf("n = %d, want 2 (both run performances)", got)
		}
		if got := featureIntValue(t, ctx, inst, "m"); got != 1 {
			t.Errorf("m = %d, want 1 (prep released and ran)", got)
		}
		var findings int
		for _, note := range notes {
			if finding, ok := note.(SuccessionOrdersNothing); ok &&
				strings.Contains(finding.Reason, "2 earlier-end performances") {
				findings++
			}
		}
		if findings != 1 {
			t.Errorf("earlier-end findings = %d, want one KERML-29 pairing note", findings)
		}
	})

	// A redefining step declaring no multiplicity of its own takes the redefined
	// step's `[n]`: the effective count performs n times, and an external read
	// sees every performance.
	t.Run("inherited-step-multiplicity", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			private import NumericalFunctions::*;
			action def Base {
				action a[3] {
					out x : Integer;
				}
			}
			action def Reads specializes Base {
				attribute c : Integer = 0;
				attribute n : Integer = 0;
				action :>> a {
					assign c := c + 1;
					assign x := c;
				}
				first start then a;
				succession first [*] a then [1] q;
				action q {
					assign n := size(a.x);
				}
				succession first q then done;
			}
		}`
		file := parseAndBuild(t, src)
		index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
		for _, d := range checkpasses.Analyze("<test>", file, nil, index) {
			if strings.Contains(string(d.Code), "action-step-multiplicity") {
				t.Errorf("check diagnostic = %v, want no action-step-multiplicity finding", d)
			}
		}
		reads := findSymbolByName(index.DocumentRoot("<test>"), "Reads", ast.DefAction)
		outputs, err := ctx.ExecuteAction(reads)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		got, ok := outputs["n"]
		if !ok || got.Kind != ValConst || got.Const.Kind != semantics.ValInt || got.Const.Int != 3 {
			t.Fatalf("outputs[n] = %v (present %v), want 3 (every performance of the inherited count)", got, ok)
		}
	})

	// An inherited loop body whose steps are ordered with written multiplicities
	// still performs the repeated step its count per pass.
	t.Run("inherited-loop-body-ordered-repetition", func(t *testing.T) {
		outputs, err := executeActionSource(t, "Derived", `package test {
			private import ScalarValues::*;
			action def Base {
				attribute n : Integer = 0;
				first start then worker;
				action worker {
					attribute i : Integer = 0;
					while i < 2 {
						first start then tick;
						action tick[2] { assign n := n + 1; }
						action bump { assign i := i + 1; }
						succession first [*] tick then [1] bump;
					}
				}
				then done;
			}
			action def Derived :> Base { action :>> worker; }
		}`)
		if err != nil {
			t.Fatalf("executeActionSource: %v", err)
		}
		assertIntOutput(t, outputs, "n", 4)
	})

	// A false guard into an inherited repeated step leaves its performances as
	// unordered as an own-count one's: run and check report the same open order.
	t.Run("inherited-guard-false", func(t *testing.T) {
		src := `package test {
			private import ScalarValues::*;
			action def Base {
				action a[3];
			}
			action def Derived specializes Base {
				first start then p;
				action p;
				succession first p if false then [*] a;
				action :>> a;
				succession first [*] a then [1] done;
			}
		}`
		file := parseAndBuild(t, src)
		index, _, _ := buildRuntimeWithLibraries(t, "<test>", file)
		var found bool
		for _, d := range checkpasses.Analyze("<test>", file, nil, index) {
			if d.Code == "action-step-order-open" && strings.Contains(d.Message, "a[3]") {
				found = true
			}
		}
		if !found {
			t.Errorf("check diagnostics lack the false guard's open-order warning")
		}
		_, err := executeActionSource(t, "Derived", src)
		var stepErr *lower.StepMultiplicityError
		if !errors.As(err, &stepErr) || stepErr.Code != lower.StepOrderOpenCode {
			t.Fatalf("execution error = %v, want %s", err, lower.StepOrderOpenCode)
		}
	})

	// A performed action's count is bounded before the behaviors it would
	// mint are allocated.
	t.Run("perform-action-count-budget", func(t *testing.T) {
		file := parseAndBuild(t, `package test {
			private import ScalarValues::*;
			part def Host {
				perform action run[1000000000] {
					first start;
					then done;
				}
			}
		}`)
		index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
		ctx.maxActionSteps = 4
		host := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
		if _, err := ctx.Instantiate(host); !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("Instantiate = %v, want ErrActionStepLimitExceeded", err)
		}
	})
}

// assertDistinctRunOccurrences checks a `perform action run[n]`'s part gives
// each attached behavior its own performance occurrence: the `run` feature
// holds one instance value per behavior, all distinct, matching what each
// behavior's executor binds.
func assertDistinctRunOccurrences(t *testing.T, ctx *Context, inst *Instance) {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, "run")
	if err != nil {
		t.Fatalf("GetFeatureValue(run): %v", err)
	}
	held := fv.HeldValue()
	if held.Kind != ValSequence {
		t.Fatalf("run = %v, want a sequence of performance occurrences", held)
	}
	elements := held.Sequence().Elements()
	var runs []*ObjectBehavior
	for _, behavior := range inst.Behaviors() {
		if behavior.Name == "run" {
			runs = append(runs, behavior)
		}
	}
	if len(elements) != len(runs) {
		t.Fatalf("run holds %d occurrence values against %d run behaviors", len(elements), len(runs))
	}
	seen := make(map[int64]bool, len(elements))
	for i, element := range elements {
		if element.Kind != ValInstance {
			t.Fatalf("run[%d] = %v, want an occurrence instance", i, element)
		}
		if seen[element.Instance] {
			t.Errorf("run[%d] repeats occurrence #%d", i, element.Instance)
		}
		seen[element.Instance] = true
	}
	for i, behavior := range runs {
		if behavior.Action == nil || behavior.Action.occurrence == nil {
			t.Fatalf("run behavior %d binds no occurrence", i)
		}
		if behavior.Action.occurrence.ID != elements[i].Instance {
			t.Errorf("run behavior %d binds occurrence #%d, want #%d at its index in the run feature",
				i, behavior.Action.occurrence.ID, elements[i].Instance)
		}
	}
}

// featureIntValue reads an integer-valued feature of an instance.
func featureIntValue(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", name, err)
	}
	value := fv.HeldValue()
	if value.Kind != ValConst || value.Const.Kind != semantics.ValInt {
		t.Fatalf("feature %q = %v, want an integer", name, value)
	}
	return value.Const.Int
}
