package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRuntimeRobustnessStateTyping covers states typed by themselves, by library state
// actions or by unsupported members, and parallel states without regions.
func TestRuntimeRobustnessStateTyping(t *testing.T) {
	t.Run("parallel_state_body_unsupported_member", testParallelStateBodyUnsupportedMember)
	t.Run("parallel_state_region_without_initial", testParallelStateRegionWithoutInitial)
	t.Run("parallel_state_region_itself_parallel", testParallelStateRegionItselfParallel)
	t.Run("state_usage_typed_by_itself", testStateUsageTypedByItself)
	t.Run("state_usage_mutually_recursive_typing", testStateUsageMutuallyRecursiveTyping)
	t.Run("state_def_specializing_the_library_state_action", testStateDefSpecializingTheLibraryStateAction)
	t.Run("state_def_specializing_a_library_state_keeps_its_content", testStateDefSpecializingALibraryStateKeepsItsContent)
	t.Run("exhibited_state_typed_by_the_library_state_action", testExhibitedStateTypedByTheLibraryStateAction)
	t.Run("exhibited_state_typed_by_the_library_state_action_with_a_body", testExhibitedStateTypedByTheLibraryStateActionWithABody)
	t.Run("exhibited_state_typed_by_a_state_action_specialization", testExhibitedStateTypedByAStateActionSpecialization)
	t.Run("state_usage_inherits_unsupported_member", testStateUsageInheritsUnsupportedMember)
}

func testParallelStateBodyUnsupportedMember(t *testing.T) {
	src := `
		package test {
			action def Warm;
			state Machine parallel {
				state left;
				perform Warm;
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("parallel state not found")
	}
	_, err := ctx.CreateStateExecutor(sym)
	if err == nil {
		t.Fatal("parallel state with an unsupported member succeeded")
	}
	if !strings.Contains(err.Error(), "parallel state body contains unsupported member") {
		t.Fatalf("error = %v, want unsupported parallel-body member", err)
	}
}

// A region with substates of its own must name the one it starts in; a bare
// `state left;` is a region standing in that state and needs none.
func testParallelStateRegionWithoutInitial(t *testing.T) {
	src := `
		package test {
			state Machine parallel {
				state left {
					state a;
					state b;
				}
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("parallel state not found")
	}
	_, err := ctx.CreateStateExecutor(sym)
	if err == nil {
		t.Fatal("parallel state with no region initial succeeded")
	}
	if !strings.Contains(err.Error(), "region left has no initial state") {
		t.Fatalf("error = %v, want missing region initial", err)
	}
	if !strings.Contains(err.Error(), "entry; then <state>;") {
		t.Fatalf("error = %v, want initial-state notation guidance", err)
	}
}

// testParallelStateRegionItselfParallel: a region is a direct substate of a
// parallel body and starts in one of its own states, so a direct substate that
// is itself parallel has no state to start in and is refused, written inline or
// typed; the active configuration therefore never holds a region owned by a
// region's wrapper state.
func testParallelStateRegionItselfParallel(t *testing.T) {
	for name, src := range map[string]string{
		"inline": `
		package test {
			state def Machine {
				entry; then work;
				state work parallel {
					state left parallel {
						state r { entry; then r1; state r1; }
						state s { entry; then s1; state s1; }
					}
					state right { entry; then b1; state b1; }
				}
			}
		}
	`,
		"typed": `
		package test {
			state def Nested parallel {
				state r { entry; then r1; state r1; }
				state s { entry; then s1; state s1; }
			}
			state def Machine {
				entry; then work;
				state work parallel {
					state left : Nested;
					state right { entry; then b1; state b1; }
				}
			}
		}
	`,
	} {
		err := stateExecutorError(t, src, "Machine")
		if err == nil {
			t.Fatalf("%s: a parallel region of a parallel state succeeded", name)
		}
		if !strings.Contains(err.Error(), "region left has no initial state") {
			t.Fatalf("%s: error = %v, want missing region initial", name, err)
		}
	}
}

// testStateUsageTypedByItself: a definition whose substate is typed by it has no
// finite materialization and must report that, not recurse.
func testStateUsageTypedByItself(t *testing.T) {
	src := `
		package test {
			state def A {
				entry; then b;
				state b : A;
			}
		}
	`
	err := stateExecutorError(t, src, "A")
	if err == nil {
		t.Fatal("recursively typed state succeeded")
	}
	if !errors.Is(err, lower.ErrRecursiveStateTyping) {
		t.Fatalf("error = %v, want recursive state typing", err)
	}
}

// testStateUsageMutuallyRecursiveTyping: two definitions reaching each other
// through their substates report the cycle rather than materializing forever.
func testStateUsageMutuallyRecursiveTyping(t *testing.T) {
	src := `
		package test {
			state def A {
				entry; then b;
				state b : B;
			}
			state def B {
				entry; then a;
				state a : A;
			}
		}
	`
	err := stateExecutorError(t, src, "A")
	if err == nil {
		t.Fatal("mutually recursive state typing succeeded")
	}
	if !errors.Is(err, lower.ErrRecursiveStateTyping) {
		t.Fatalf("error = %v, want recursive state typing", err)
	}
}

// testStateDefSpecializingTheLibraryStateAction: a state definition written
// `:> StateAction` inherits no content from the library (whose `ref state self`
// is typed by StateAction itself), the same as the implicit specialization.
func testStateDefSpecializingTheLibraryStateAction(t *testing.T) {
	src := `
		package test {
			private import States::*;
			state def Phase :> StateAction;
			state def Prep :> Phase;
			state def Machine {
				entry; then prep;
				state prep : Prep;
				state launch : Phase;
				transition first prep then launch;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state Machine not found")
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if got := activeStateNames(exec); got != "launch" {
		t.Fatalf("active states = %q, want launch", got)
	}
}

// testStateDefSpecializingALibraryStateKeepsItsContent: only StateAction is
// withheld; a library's own state definition still contributes its substates,
// transitions, behaviors and attributes to what specializes it.
func testStateDefSpecializingALibraryStateKeepsItsContent(t *testing.T) {
	lib := `
		package Cycles {
			private import ScalarValues::*;
			state def Cycle {
				attribute seen : Integer = 0;
				entry; then warm;
				state warm;
				state hot {
					entry action mark { assign seen := seen + 1; }
				}
				transition first warm then hot;
			}
		}
	`
	src := `
		package test {
			private import Cycles::*;
			state def Burn :> Cycle;
			state def Machine {
				entry; then burn;
				state burn : Burn;
			}
		}
	`
	idx := libs.NewModelIndex()
	idx.AddDocument("<lib>", parser.New(source.New("<lib>", []byte(lib))).ParseFile())
	idx.MarkLibrary("<lib>")
	idx.AddDocument("<test>", parseAndBuild(t, src))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 10000)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state Machine not found")
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if got := exec.finalStateName(); got != "hot" {
		t.Fatalf("final state = %q, want hot", got)
	}
	seen, ok := exec.StateData()["burn.seen"]
	if !ok {
		t.Fatalf("burn.seen missing from %v", exec.StateData())
	}
	if got := FormatValue(seen); got != "1" {
		t.Fatalf("burn.seen = %s, want 1", got)
	}
}

// A body-less usage typed by the library's StateAction, however named, lowers
// without recursing into `ref state self` and fails only for no initial state.
func testExhibitedStateTypedByTheLibraryStateAction(t *testing.T) {
	for _, tc := range []struct{ name, typing string }{
		{"imported", "StateAction"},
		{"qualified", "States::StateAction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `
			package test {
				private import States::StateAction;
				part def Mission {
					attribute mass = 1;
					exhibit state phases : ` + tc.typing + `;
				}
			}`
			_, _, err := instantiateWithLibraries(t, src, "test::Mission")
			if err == nil {
				t.Fatal("expected the machine with no initial state to fail materialization")
			}
			if errors.Is(err, lower.ErrRecursiveStateTyping) {
				t.Fatalf("error = %v, want StateAction's content withheld rather than recursed into", err)
			}
			if !errors.Is(err, errNoInitialState) {
				t.Fatalf("error = %v, want errNoInitialState", err)
			}
			for _, want := range []string{"phases", "Mission", "StateAction"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v, want it to name %s", err, want)
				}
			}
		})
	}
}

// testExhibitedStateTypedByTheLibraryStateActionWithABody: a state usage typed
// by StateAction and stating its own body runs that body, inheriting nothing.
func testExhibitedStateTypedByTheLibraryStateActionWithABody(t *testing.T) {
	src := `
	package test {
		private import States::StateAction;
		part def Mission {
			attribute mass = 2;
			exhibit state phases : StateAction { entry; then x; state x; }
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Mission")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	assertExhibitedMachineIn(t, ctx, inst, "mass", 2, "x")
}

// A usage typed by a definition specializing StateAction inherits that
// definition's content and nothing of the library's.
func testExhibitedStateTypedByAStateActionSpecialization(t *testing.T) {
	t.Run("with_content", func(t *testing.T) {
		src := `
		package test {
			private import States::StateAction;
			state def Phase :> StateAction { entry; then x; state x; }
			part def Mission {
				attribute mass = 3;
				exhibit state phases : Phase;
			}
		}`
		ctx, inst, err := instantiateWithLibraries(t, src, "test::Mission")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		assertExhibitedMachineIn(t, ctx, inst, "mass", 3, "x")
	})
	t.Run("own_body", func(t *testing.T) {
		src := `
		package test {
			private import States::StateAction;
			state def Phase :> StateAction;
			part def Mission {
				attribute mass = 4;
				exhibit state phases : Phase { entry; then x; state x; }
			}
		}`
		ctx, inst, err := instantiateWithLibraries(t, src, "test::Mission")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		assertExhibitedMachineIn(t, ctx, inst, "mass", 4, "x")
	})
	t.Run("empty", func(t *testing.T) {
		src := `
		package test {
			private import States::StateAction;
			state def Phase :> StateAction;
			part def Mission {
				attribute mass = 5;
				exhibit state phases : Phase;
			}
		}`
		_, _, err := instantiateWithLibraries(t, src, "test::Mission")
		if !errors.Is(err, errNoInitialState) {
			t.Fatalf("error = %v, want errNoInitialState", err)
		}
		if !strings.Contains(err.Error(), "phases") || !strings.Contains(err.Error(), "Phase") {
			t.Errorf("error = %v, want it to name the machine and its definition", err)
		}
	})
}

// assertExhibitedMachineIn checks that an object exhibits a machine resting in
// the named state, and that an ordinary attribute of it still reads.
func assertExhibitedMachineIn(t *testing.T, ctx *Context, inst *Instance, attr string, want int64, state string) {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, attr)
	if err != nil {
		t.Fatalf("%s: %v", attr, err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Int != want {
		t.Errorf("%s = %s, want %d", attr, FormatValue(got), want)
	}
	machine, ok := inst.ExhibitedState()
	if !ok || machine.State == nil {
		t.Fatal("the object exhibits no machine")
	}
	if got := machine.State.finalStateName(); got != state {
		t.Errorf("final state = %q, want %q", got, state)
	}
}

// testStateUsageInheritsUnsupportedMember: content a usage inherits that state
// lowering cannot represent is reported, never dropped.
func testStateUsageInheritsUnsupportedMember(t *testing.T) {
	src := `
		package test {
			action def Warm;
			state def Inner {
				entry; then i1;
				state i1;
				perform Warm;
			}
			state def Machine {
				entry; then nested;
				state nested : Inner;
			}
		}
	`
	err := stateExecutorError(t, src, "Machine")
	if err == nil {
		t.Fatal("unsupported inherited member succeeded")
	}
	if !errors.Is(err, lower.ErrUnsupportedStateContent) {
		t.Fatalf("error = %v, want unsupported state content", err)
	}
	if !strings.Contains(err.Error(), "cannot be inherited by the state nested") {
		t.Fatalf("error = %v, want the inheriting state named", err)
	}
}
