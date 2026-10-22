package runtime

import (
	"fmt"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Shared helpers and fixtures for the TestRuntimeRobustness* suites, one of
// which lives in each robustness_<feature>_test.go file. Every case must return a
// typed error, never panic or hang.

// placementBody declares a frame placed in datum, for the transform failure modes.
const placementBody = `
	attribute shifted : CartesianSpatial3dCoordinateFrame {
		:>> mRefs = (mm, mm, mm);
		:>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (10.0, 20.0, 30.0) [datum]; }
	}`

// instantiateHolder instantiates the `Holder` part def the source declares, for
// a case whose failure surfaces when one of its feature values is read.
func instantiateHolder(t *testing.T, src string) (*Instance, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return inst, ctx
}

// stateRunErrorForSource runs the named machine in src to completion and answers
// the first error it reports, failing if the machine hangs.
func stateRunErrorForSource(t *testing.T, name, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}

	done := make(chan error, 1)
	go func() {
		exec, err := newStateExecutor(ctx, sym, nil)
		if err == nil {
			err = exec.initialize()
		}
		if err == nil {
			err = exec.RunToCompletion()
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-watchdog(10 * time.Second):
		t.Fatalf("running %s did not terminate", name)
		return nil
	}
}

// stateExecutorForSource builds an executor for the named machine in src, for
// tests that drive it event by event.
func stateExecutorForSource(t *testing.T, name, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return exec
}

// stateExecutorError builds a state executor for a named state definition and
// returns what creating it reports.
func stateExecutorError(t *testing.T, src, name string) error {
	t.Helper()
	return stateExecutorErrorIn(t, src, name, buildRuntime)
}

// libraryStateExecutorError is stateExecutorError over an index carrying the
// standard library, for a model that names library elements.
func libraryStateExecutorError(t *testing.T, src, name string) error {
	t.Helper()
	return stateExecutorErrorIn(t, src, name, buildRuntimeWithLibraries)
}

func stateExecutorErrorIn(t *testing.T, src, name string, build func(*testing.T, string, *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context)) error {
	t.Helper()
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := build(t, "<test>", file)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state %s not found", name)
	}
	_, err := ctx.CreateStateExecutor(sym)
	return err
}

// nestedWaitModel waits on the clock in the flow of a block a body statement runs.
const nestedWaitModel = `
	package test {
		private import SI::*;
		private import ScalarValues::*;
		action outer {
			attribute n : Integer = 0;
			first start;
			then action body {
				if true {
					action inner {
						first start;
						then action nap accept after 5 [s];
						then action mark assign n := 1;
						then done;
					}
				}
			}
			then done;
		}
	}
`

// performedWaitModel waits on the clock in an action a node of the flow performs,
// after a wait of the flow's own.
const performedWaitModel = `
	package test {
		private import SI::*;
		private import ScalarValues::*;
		action def Napper {
			out attribute n : Integer = 0;
			first start;
			then action nap accept after 5 [s];
			then action mark assign n := 1;
			then done;
		}
		action outer {
			attribute n : Integer = 0;
			first start;
			then action early accept after 1 [s];
			then perform action call : Napper;
			then action copy assign n := call.n;
			then done;
		}
	}
`

// Helper: parse source into AST RootNamespace
func parseAndBuild(t *testing.T, src string) *ast.RootNamespace {
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	return file
}

// Helper: build runtime context from file
func buildRuntime(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context) {
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(typedModel(model, resolver), 10000)
	return idx, model, ctx
}

// buildRuntimeWithLibraries builds a runtime context over an index that carries
// the standard library, for a model that names its elements.
func buildRuntimeWithLibraries(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument(path, file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return idx, model, NewContext(parsingModel(model, resolver), 10000)
}

// Helper: find symbol by name and kind
func findSymbolByName(scope *symbols.Scope, name string, kind ast.DefinitionKind) *symbols.Symbol {
	// Map DefKind to UsageKind
	var usageKind ast.UsageKind
	switch kind {
	case ast.DefCalc:
		usageKind = ast.UsageCalc
	case ast.DefAction:
		usageKind = ast.UsageAction
	case ast.DefState:
		usageKind = ast.UsageState
	case ast.DefConstraint:
		usageKind = ast.UsageConstraint
	case ast.DefRequirement:
		usageKind = ast.UsageRequirement
	}

	// Check all child scopes (packages/namespaces)
	for _, child := range scope.Children() {
		for _, memberName := range child.MemberNames() {
			sym, _ := child.LookupLocal(memberName)
			if sym == nil {
				continue
			}

			if sym.Name == name {
				switch decl := sym.Decl.(type) {
				case *ast.Definition:
					if decl.Kind == kind {
						return sym
					}
				case *ast.Usage:
					if decl.Kind == usageKind {
						return sym
					}
				}
			}
		}
	}

	// Also check root scope directly
	for _, memberName := range scope.MemberNames() {
		sym, _ := scope.LookupLocal(memberName)
		if sym == nil {
			continue
		}

		if sym.Name == name {
			switch decl := sym.Decl.(type) {
			case *ast.Definition:
				if decl.Kind == kind {
					return sym
				}
			case *ast.Usage:
				if decl.Kind == usageKind {
					return sym
				}
			}
		}
	}
	return nil
}

// invokeCalcInSource invokes calcName with one Integer argument and returns the
// error, on its own goroutine so a body that never terminates fails the case
// instead of stalling the suite. maxSteps bounds the run.
func invokeCalcInSource(t *testing.T, src, calcName string, arg int64, maxSteps int64) error {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		value := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: arg}}
		result, err := ctx.InvokeCalc(sym, []Value{value}, rootScope)
		if err == nil {
			err = fmt.Errorf("calc %s returned %s, expected it to fail", calcName, FormatTraceValue(result))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-watchdog(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
		return nil
	}
}

// calcUsageOutputInSource reads one output feature of the named calc usage and
// returns the error the read reports, on its own goroutine so a body that never
// terminates fails the case instead of stalling the suite. maxSteps bounds the
// run.
func calcUsageOutputInSource(t *testing.T, src, usageName, output string, maxSteps int64, budgets ...func(*Context)) error {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	for _, set := range budgets {
		set(ctx)
	}
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, usageName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc usage %s not found", usageName)
	}

	done := make(chan error, 1)
	go func() {
		value, err := ctx.calcUsageOutput(sym, output, sym.OwnerScope, nil)
		if err == nil {
			err = fmt.Errorf("output %s of %s answered %s, expected the read to fail",
				output, usageName, FormatTraceValue(value))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-watchdog(10 * time.Second):
		t.Fatalf("reading output %s of %s did not terminate", output, usageName)
		return nil
	}
}

// variationFeatureValueInSource instantiates a usage and returns the value its named
// feature value holds, so a variation's failure modes are read where a model reads them.
func variationFeatureValueInSource(t *testing.T, src, usage, fv string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	matches := idx.LookupQualified(usage)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", usage, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		return Value{}, err
	}
	got, err := inst.GetFeatureValue(ctx, fv)
	if err != nil {
		return Value{}, err
	}
	return got.Value, nil
}

// variationFamily is a variation point with three variants, over which the
// selection a specialization makes is varied per case.
const variationFamily = `
	package test {
		part def Diamond { attribute cut; attribute color; }
		abstract part family : Diamond {
			variation attribute :>> cut {
				variant attribute cutShallow { attribute cost = 200.0; }
				variant attribute cutIdeal { attribute cost = 250.0; }
			}
			variation attribute :>> color {
				variant attribute colorWhite { attribute cost = 100.0; }
			}
		}
		%s
	}`

// oneSymbol returns the single symbol a qualified name denotes.
func oneSymbol(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

// readGo answers the sole value of self's attribute go, which the robustness
// fixtures bind to the pin or parameter under test.
func readGo(t *testing.T, ctx *Context, self *Instance) Value {
	t.Helper()
	fv, err := self.GetFeatureValue(ctx, "go")
	if err != nil {
		t.Fatalf("read go of object #%d: %v", self.ID, err)
	}
	return soleElement(fv.HeldValue())
}

// calcErrorWithLibraries invokes the named calc of package test in src, with the
// standard library indexed, and answers the error it fails with.
func calcErrorWithLibraries(t *testing.T, src, calcName string, args []Value, maxSteps int64) error {
	t.Helper()

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calcName)

	done := make(chan error, 1)
	go func() {
		result, err := ctx.InvokeCalc(sym, args, scope)
		if err == nil {
			err = fmt.Errorf("calc %s returned %s, expected it to fail", calcName, FormatTraceValue(result))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-watchdog(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
		return nil
	}
}

// instantiateInSource materializes the named type declared in src, so a case can
// state the failure materializing an object reports.
func instantiateInSource(t *testing.T, src, fqn string) (*Context, *Instance, error) {
	t.Helper()
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	inst, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
	return ctx, inst, err
}

// instantiateWithLibraries materializes the named type declared in src over an
// index carrying the standard library, so a case may name its scalar types.
func instantiateWithLibraries(t *testing.T, src, fqn string) (*Context, *Instance, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
	return ctx, inst, err
}

// runOuterAction runs action test::outer of src to completion and returns the
// error, for a case whose contract is the error a nested node reports.
func runOuterAction(t *testing.T, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	return exec.RunToCompletion()
}

const adderActionDef = `
	action def Adder {
		in a : Integer[1];
		in b : Integer[1];
		out sum : Integer;
		first step;
		action step { assign sum := a + b; }
	}
`

// stateWithDoBody is a machine whose state active runs body as its inline do
// action, counting in total what the body's nodes did.
func stateWithDoBody(t *testing.T, body string) *StateExecutor {
	t.Helper()
	return stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active { do action ops { `+body+` } }
			succession first init then active;
			succession first active then done;
		}
	}`)
}

// functionValueFixture declares Fn, which applies its calc-typed parameter f to a.
const functionValueFixture = `
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Add { in x : Real; in y : Real[1]; return : Real = x + y; }
	calc def Fn { in calc f[1] { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
`

// invokeCalcExpecting evaluates the calc call expr against src with the standard library,
// on its own goroutine so a body that never terminates fails the case instead of stalling it.
func invokeCalcExpecting(t *testing.T, src, expr string) error {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = 100000
	scope := idx.DocumentRoot("<test>")

	done := make(chan error, 1)
	go func() {
		node := parser.New(source.New("<expr>", []byte(expr))).ParseExpression()
		result, err := ctx.EvalWithScopeOn(node, scope, nil)
		if err == nil {
			err = fmt.Errorf("%s = %s, expected it to fail", expr, FormatTraceValue(result))
		}
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-watchdog(10 * time.Second):
		t.Fatalf("%s did not terminate", expr)
		return nil
	}
}

// verificationRobustnessModel states a case whose subject nothing binds, one
// whose body reads a feature holding no value, and a part that is no case.
const verificationRobustnessModel = `
	package test {
		part def Sensor { attribute reading : ScalarValues::Integer; }
		part unread : Sensor;

		verification def Unbound {
			subject sensor : Sensor;
			VerificationCases::PassIf(sensor.reading == 0)
		}

		action def Adder {
			in a : ScalarValues::Integer;
			in b : ScalarValues::Integer;
			out sum : ScalarValues::Integer;
			first step;
			action step { assign sum := a + b; }
		}
		action adder : Adder;

		verification def Stepping {
			subject sensor : Sensor;
			first start;
			then perform adder;
			then done;
		}

		verification stepping : Stepping { subject sensor = unread; }

		verification def Thresholded {
			subject sensor : Sensor;
			in threshold : ScalarValues::Integer[1];
			VerificationCases::PassIf(sensor.reading == threshold)
		}

		verification def Plan {
			subject sensor : Sensor;
			verification sub : Thresholded;
			VerificationCases::PassIf(true)
		}

		verification plan : Plan { subject sensor = unread; }
	}
`

// verificationObjectiveModel states a verification case whose objective is a
// requirement on a Rover while the case verifies a Lander, and one that
// verifies nothing bound.
const verificationObjectiveModel = `
	package test {
		private import ScalarValues::*;
		part def Lander { attribute touchdownSpeed : Real; }
		part def Rover { attribute touchdownSpeed : Real; }
		part scout : Lander { attribute :>> touchdownSpeed = 1.2; }

		requirement def SoftRoving {
			subject rover : Rover;
			in attribute limit : Real default = 1.5;
			require constraint { rover.touchdownSpeed <= limit }
		}

		verification def RoverCheck {
			subject lander : Lander;
			in attribute limit : Real = 1.5;
			objective : SoftRoving { in limit = limit; }
			VerificationCases::PassIf(lander.touchdownSpeed <= limit)
		}
		verification checkRover : RoverCheck { subject lander = scout; }
		verification checkNothing : RoverCheck;
	}
`

// tradeStudyRobustnessModel states trade studies that cannot finish: an
// evaluation function left abstract, one that divides by an alternative's zero,
// a subject listing nothing, one redefined to a single value yet bound to two,
// and an evaluation reading a feature no alternative gives a value.
const tradeStudyRobustnessModel = `
	package test {
		private import ScalarValues::*;
		private import TradeStudies::*;

		part def Engine { attribute mass : Real; attribute cylinders : Integer; attribute cost : Real; }
		part heavy : Engine { attribute :>> mass = 20.0; attribute :>> cylinders = 0; }
		part light : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 2; }

		analysis abstractEval : TradeStudy {
			subject : Engine[1..*] = (heavy, light);
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real; }
			return part :>> selectedAlternative : Engine;
		}

		analysis perCylinder : TradeStudy {
			subject : Engine[1..*] = (light, heavy);
			objective : MinimizeObjective;
			calc :>> evaluationFunction {
				in part e :>> alternative : Engine;
				return :>> result : Real = e.mass / e.cylinders;
			}
			return part :>> selectedAlternative : Engine;
		}

		analysis none : TradeStudy {
			subject : Engine[1..*] = ();
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real = e.mass; }
			return part :>> selectedAlternative : Engine;
		}

		analysis single : TradeStudy {
			subject : Engine[1] = (heavy, light);
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real = e.mass; }
			return part :>> selectedAlternative : Engine;
		}

		analysis unpriced : TradeStudy {
			subject : Engine[1..*] = (heavy, light);
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real = e.cost; }
			return part :>> selectedAlternative : Engine;
		}
	}
`

// stateMachineWithLibraries builds Machine from src over the standard library,
// for state bodies that name units, and returns it before initialization.
func stateMachineWithLibraries(t *testing.T, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	return exec
}

// weightedActionError runs the action `route` of a body naming the Stochastic
// and RandomFunctions libraries and returns how it failed.
func weightedActionError(t *testing.T, body string, seed ...uint64) error {
	t.Helper()
	src := "package test {\n private import ScalarValues::*;\n private import SI::*;\n private import Stochastic::*;\n private import RandomFunctions::*;\n action route {" + body + "}\n}"
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
	if sym == nil {
		t.Fatal("action route not found")
	}
	if len(seed) > 0 {
		ctx.SetModelSeed(seed[0])
	}
	_, err := ctx.ExecuteAction(sym)
	return err
}

// calcNamed is the calc definition of that name in the scope.
func calcNamed(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("calc %s not indexed", name)
	}
	return sym
}

// intOf and realOf are numbers as an argument carries them.
func intOf(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

func realOf(f float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

// invokeValue invokes a calc and reads the one value its result holds: a result stating
// no multiplicity is `[0..*]` (SysML v2 §7.6.3) and holds that value as a one-element sequence.
func invokeValue(ctx *Context, sym *symbols.Symbol, args []Value, scope *symbols.Scope) (Value, error) {
	result, err := ctx.InvokeCalc(sym, args, scope)
	return soleElement(result), err
}
