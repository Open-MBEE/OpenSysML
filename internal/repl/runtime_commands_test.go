package repl

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// loadFixture submits a testdata model into a fresh session.
func loadFixture(t *testing.T, path string) *Session {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s := NewSession()
	// Warnings are expected: these fixtures are written in our state notation,
	// which NonstandardNotationPass reports as an OpenSysML extension.
	if errs := errorDiagnostics(s.Submit(string(data)).Diagnostics); len(errs) > 0 {
		t.Fatalf("fixture %s has errors: %v", path, errs)
	}
	return s
}

// errorDiagnostics returns only the error-severity diagnostics of diags.
func errorDiagnostics(diags []passes.Diagnostic) []passes.Diagnostic {
	var out []passes.Diagnostic
	for _, d := range diags {
		if d.Severity == passes.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// run executes one meta command and returns its output as a single string.
func run(t *testing.T, s *Session, line string) string {
	t.Helper()
	out, quit, err := s.RunMeta(line)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	if quit {
		t.Fatalf("%s unexpectedly quit", line)
	}
	return strings.Join(out, "\n")
}

// wants asserts every fragment appears in got.
func wants(t *testing.T, got string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(got, fragment) {
			t.Errorf("expected %q in output:\n%s", fragment, got)
		}
	}
}

// wantsInOrder asserts every fragment appears in got, each after the one before it.
func wantsInOrder(t *testing.T, got string, fragments ...string) {
	t.Helper()
	from := 0
	for _, fragment := range fragments {
		at := strings.Index(got[from:], fragment)
		if at < 0 {
			t.Errorf("expected %q after the fragments before it in output:\n%s", fragment, got)
			return
		}
		from += at + len(fragment)
	}
}

// hasNotice reports whether any of a submission's notices mentions fragment.
func hasNotice(res Result, fragment string) bool {
	for _, n := range res.Notices {
		if strings.Contains(n, fragment) {
			return true
		}
	}
	return false
}

// rejects asserts no fragment appears in got.
func rejects(t *testing.T, got string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if strings.Contains(got, fragment) {
			t.Errorf("did not expect %q in output:\n%s", fragment, got)
		}
	}
}

// TestREPLPackagedModelWorkflow is the end-to-end smoke test for the advertised
// workflow on a packaged model: nothing in the fixture is declared at the top
// level, so every command has to resolve a package member.
func TestREPLPackagedModelWorkflow(t *testing.T) {
	script := []string{
		"%load testdata/vehicle_package.sysml",
		"%instantiate Vehicle",
		"%features Vehicle",
		"%eval Demo::Vehicle::mass",
		"%calc add 2 3",
		"%constraint withinMassLimit",
		"%instances",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()

	wants(t, got,
		"✓ Created instance of Demo::Vehicle",
		"Instance: Demo::Vehicle",
		"mass = 1500.0",
		"engine = Instance(",
		"✓ Demo::Vehicle::mass",
		"= 1500.0",
		"✓ add(2, 3)",
		"= 5",
		"✓ Constraint withinMassLimit passed",
		"Instances:",
	)
	rejects(t, got, "unresolved reference", "error:")
}

func TestInstantiateFindsPackageMemberBySimpleAndQualifiedName(t *testing.T) {
	for _, name := range []string{"Vehicle", "Demo::Vehicle"} {
		t.Run(name, func(t *testing.T) {
			s := loadFixture(t, "testdata/vehicle_package.sysml")
			got := run(t, s, "%instantiate "+name)
			wants(t, got, "✓ Created instance of Demo::Vehicle")

			// Instances are keyed by the resolved name, so either spelling
			// reaches the instance the other created.
			wants(t, run(t, s, "%features Vehicle"), "mass = 1500.0")
			wants(t, run(t, s, "%features Demo::Vehicle"), "mass = 1500.0")
		})
	}
}

func TestInstantiateUnknownSymbol(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Nope"), `error: unresolved reference: Nope`)
	wants(t, run(t, s, "%instantiate Demo::Nope"), `error: unresolved reference: Demo::Nope`)
}

// A simple name matching in two packages is reported with the candidates rather
// than resolved to whichever the scope walk reached first.
func TestAmbiguousSimpleNameReportsCandidates(t *testing.T) {
	s := NewSession()
	s.Submit(`package A { part def Widget { attribute size = 1.0; } }
package B { part def Widget { attribute size = 2.0; } }`)

	got := run(t, s, "%instantiate Widget")
	wants(t, got, `symbol "Widget" is ambiguous`, "A::Widget", "B::Widget")

	// The qualified name disambiguates.
	wants(t, run(t, s, "%instantiate B::Widget"), "✓ Created instance of B::Widget")
	wants(t, run(t, s, "%features B::Widget"), "size = 2.0")
}

// Declarations submitted after a lookup must be visible: the symbol index and
// runtime context are derived from a document that Submit replaces.
func TestLookupSeesDeclarationsAddedAfterFirstLookup(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo { part def Vehicle { attribute mass = 1500.0; } }`)
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "✓ Created instance of Demo::Vehicle")

	s.Submit(`package Demo { part def Trailer { attribute mass = 900.0; } }`)
	wants(t, run(t, s, "%instantiate Demo::Trailer"), "✓ Created instance of Demo::Trailer")
	wants(t, run(t, s, "%features Trailer"), "mass = 900.0")
	wants(t, run(t, s, "%eval Demo::Trailer::mass"), "900.0")
}

// An instance does not outlive the declaration it is of: redeclaring that
// definition rewrites what its feature values mean, so the object built from the old one
// goes, and the listing says why rather than reading like a fresh session.
func TestInstancesDoNotOutliveTheirDeclaration(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo { part def Vehicle { attribute mass = 1500.0; } }`)
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "ID: 1")
	wants(t, run(t, s, "%instances"), "Demo::Vehicle")

	s.Submit(`package Demo { part def Vehicle { attribute mass = 900.0; } }`)
	wants(t, run(t, s, "%instances"),
		"no instances created", "1 instance was dropped when the declarations changed at submission 2")
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "ID: 1")
	wants(t, run(t, s, "%features Demo::Vehicle"), "mass = 900.0")
}

func TestFeatureValuesWithoutInstance(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%features Vehicle"), "no instance of", "%instantiate")
}

func TestInstancesEmptyAndPopulated(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instances"), "(no instances created)")
	run(t, s, "%instantiate Engine")
	wants(t, run(t, s, "%instances"), "Demo::Engine")
}

func TestEvalLiteralFeatureAndCompound(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")

	// Literal: evaluated without any session context.
	wants(t, run(t, s, "%eval 6 * 7"), "= 42")
	// Feature reference by qualified name, and by simple name through the
	// scope-tree walk.
	wants(t, run(t, s, "%eval Demo::Engine::power"), "= 300.0")
	wants(t, run(t, s, "%eval power"), "= 300.0")

	// Compound expression over a top-level feature.
	flat := NewSession()
	flat.Submit("attribute mass = 3.0;")
	wants(t, run(t, flat, "%eval mass + 1.0"), "= 4.0")
}

func TestEvalErrors(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%eval missing"), `unresolved reference: missing`)
	// A part def is a symbol, but not one with a value.
	wants(t, run(t, s, "%eval Demo::Vehicle"), "has no value to evaluate")

	empty := NewSession()
	wants(t, run(t, empty, "%eval mass"), "no declarations loaded")
}

// An expression of literals alone is answered without any declarations, so a
// failure of one is the answer and is reported as it is: declaring something
// would not change it, and "no declarations loaded" would say to try.
func TestEvalReportsTheAnswerOfALiteralExpressionThatFails(t *testing.T) {
	empty := NewSession()
	wants(t, run(t, empty, "%eval (1, 2, 3)#(0)"), "sequence index 0 is outside 1..3")
	wants(t, run(t, empty, "%eval (1, 2, 3)#(1.5)"),
		"sequence index requires an Integer index")
	wants(t, run(t, empty, "%eval (1, 2, 3).{in x; in y; x}"),
		"calls its body with 1 argument(s), but it declares 2 parameter(s)")
	// A budget the literal expression itself spends is the answer too, and the
	// session's bounds are the ones in force.
	budgets := runtime.DefaultBudgets()
	budgets.MaxElements = 10
	if err := empty.SetBudgets(budgets); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	wants(t, run(t, empty, "%eval (1..100)->size()"),
		"collection element limit exceeded (10 elements; raise "+runtime.MaxElementsEnvVar)
	// A name is the one failure declarations do answer, so it still says so.
	wants(t, run(t, empty, "%eval mass + 1"), "no declarations loaded")
}

// A library operation is answered by its unqualified name only where the
// session imports its package, as the checker resolves it; the qualified name is
// answered anywhere. A calc the session wrote under the same name is that
// declaration's, never the library's.
func TestEvalResolvesALibraryOperationAsTheCheckerDoes(t *testing.T) {
	empty := NewSession()
	wants(t, run(t, empty, "%eval SequenceFunctions::size((1, 2, 3))"), "= 3")
	wants(t, run(t, empty, "%eval RealFunctions::sqrt(16.0)"), "= 4.0")
	wants(t, run(t, empty, "%eval size((1, 2, 3))"),
		"error: no declarations loaded",
		"unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?")
	wants(t, run(t, empty, "%eval sqrt(16.0)"),
		"unresolved reference: sqrt — did you mean RealFunctions::sqrt or QuantityCalculations::sqrt?")

	imported := NewSession()
	imported.Submit("private import SequenceFunctions::*;")
	imported.Submit("private import NumericalFunctions::*;")
	wants(t, run(t, imported, "%eval size((1, 2, 3))"), "= 3")
	wants(t, run(t, imported, "%eval (1, 2, 3)->size()"), "= 3")
	wants(t, run(t, imported, "%eval sum((1, 2, 3))"), "= 6")
	wants(t, run(t, imported, "%eval sqrt(16.0)"),
		"unresolved reference: sqrt — did you mean RealFunctions::sqrt or QuantityCalculations::sqrt?")
	rejects(t, run(t, imported, "%eval sqrt(16.0)"), "no declarations loaded")

	own := NewSession()
	own.Submit("private import NumericalFunctions::*;")
	own.Submit("calc sum { in a; in b; return : Integer = a + b; }")
	wants(t, run(t, own, "%eval sum(1, 2)"), "= 3")
	wants(t, run(t, own, "%eval sum((1, 2, 3))"), "error:")
}

// A model's own calc that calls a library function by its unqualified name is
// answered only once the model imports the package, whether it is read through
// a feature it computes or invoked with %calc.
func TestEvalAndCalcOfAnUnimportedLibraryFunction(t *testing.T) {
	const wheels = `
		package Demo {
			part def Car {
				attribute wheels : ScalarValues::Integer[*] = (1, 2, 3, 4);
				attribute wheelCount = wheels->size();
			}
			calc def Count { in xs : ScalarValues::Integer[*]; return : ScalarValues::Integer = xs->size(); }
		}`
	unimported := NewSession()
	unimported.Submit(wheels)
	wants(t, run(t, unimported, "%eval Demo::Car::wheelCount"),
		"error:", "unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?")
	wants(t, run(t, unimported, "%calc Demo::Count((1, 2, 3))"),
		"unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?")

	imported := NewSession()
	imported.Submit(strings.Replace(wheels, "package Demo {", "package Demo {\n\t\t\tprivate import SequenceFunctions::*;", 1))
	wants(t, run(t, imported, "%eval Demo::Car::wheelCount"), "= 4")
	wants(t, run(t, imported, "%calc Demo::Count((1, 2, 3))"), "= 3")
}

func TestCalcWithPositionalArgs(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%calc add 20 22"), "✓ add(20, 22)", "= 42")
	wants(t, run(t, s, "%calc Demo::add 1 2"), "= 3")
	wants(t, run(t, s, "%calc nosuch 1"), `unresolved reference: nosuch`)
}

func TestConstraintPassAndFail(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%constraint withinMassLimit"), "✓ Constraint withinMassLimit passed")
	wants(t, run(t, s, "%constraint Demo::overMassLimit"), "✗ Constraint Demo::overMassLimit failed")
	wants(t, run(t, s, "%constraint nosuch"), `unresolved reference: nosuch`)
}

// A condition is evaluated in the scope the element was declared in, not in the
// document root: a measurement unit an inner package imports is visible to the
// condition that package writes, with or without an instance to evaluate against.
func TestConstraintResolvesUnitsOfItsOwnPackage(t *testing.T) {
	s := NewSession()
	s.Submit(`package QTest {
		public import SI::*;
		constraint def SpeedOK {
			attribute d = 100.0 [m];
			attribute t = 10.0 [s];
			d / t < 20.0 [m] / 1.0 [s]
		}
	}`)
	wants(t, run(t, s, "%constraint QTest::SpeedOK"), "✓ Constraint QTest::SpeedOK passed")
}

func TestRequirement(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%requirement SafeMass"), "✓ Requirement SafeMass satisfied")
	wants(t, run(t, s, "%requirement Demo::SafeMass"), "satisfied")
	wants(t, run(t, s, "%requirement nosuch"), `unresolved reference: nosuch`)
}

func TestFormatValue(t *testing.T) {
	sequence := runtime.NewSequence()
	sequence.Append(runtime.Value{
		Kind:  runtime.ValConst,
		Const: semantics.Value{Kind: semantics.ValInt, Int: 1},
	})
	sequence.Append(runtime.NewStringValue("hi"))
	set := runtime.NewSet()
	set.Add(runtime.NewStringValue("z"))
	set.Add(runtime.NewStringValue("a"))
	cases := []struct {
		name string
		val  runtime.Value
		want string
	}{
		{"int", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 7}}, "7"},
		{"real", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}, "1.5"},
		{"bool", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}, "true"},
		{"infinity", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}, "*"},
		{"null", runtime.Value{Kind: runtime.ValNull}, "null"},
		{"string", runtime.NewStringValue("hi"), `"hi"`},
		{"instance", runtime.Value{Kind: runtime.ValInstance, Instance: 3}, "Instance(ID: 3)"},
		{"sequence", runtime.NewSequenceValue(sequence), `[1, "hi"]`},
		{"set", runtime.NewSetValue(set), `Set{"z", "a"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatValue(nil, tc.val); got != tc.want {
				t.Errorf("formatValue = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- Action debugger ---

func TestActionDebuggerRunsToResult(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")

	wants(t, run(t, s, "%action tally"), `✓ Started action executor for "tally"`, "Tokens: 1")
	wants(t, run(t, s, "%tokens"), "Active tokens (1):", "Token 1 @ start")
	wants(t, run(t, s, "%step"), "✓ Step complete")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
	wants(t, run(t, s, "%continue"), "already completed")
	wants(t, run(t, s, "%stop"), `✓ Stopped debugging session for "tally"`)
}

// A perform usage states no flow of its own: the debugger runs the flow of the
// action definition typing it, rather than reporting the usage as flowless.
func TestActionDebuggerRunsAPerformUsage(t *testing.T) {
	s := loadFixture(t, "testdata/action_perform_usage.sysml")

	run(t, s, "%instantiate Perform::hiker")
	wants(t, run(t, s, "%action Perform::hiker::walk"), `✓ Started action executor for "Perform::hiker::walk"`, "Tokens: 1")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "legs = 3")
}

// An action reports the values it produced the same way however it was driven,
// so a run stepped to its end reads like one run to completion.
func TestSteppedActionReportsResultsLikeContinue(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")

	run(t, s, "%action tally")
	var stepped string
	for i := 0; i < 10; i++ {
		stepped = run(t, s, "%step")
		if strings.Contains(stepped, "✓ Action completed") {
			break
		}
	}
	wants(t, stepped, "\n  Results:\n    total = 5")

	run(t, s, "%stop")
	run(t, s, "%action tally")
	wants(t, run(t, s, "%continue"), "\n  Results:\n    total = 5")
}

// %tokens names the node a token sits on, not its Go type.
func TestTokensShowNodeNames(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	got := run(t, s, "%tokens")
	wants(t, got, "Token 1 @ start")
	rejects(t, got, "*ast.")
}

// A token held at a join says which succession it came over and which it waits for.
func TestTokensShowHeldJoinArrivals(t *testing.T) {
	s := loadSource(t, `package test {
		action gather {
			first start;
			fork split;
			action quick;
			action slow1;
			action slow2;
			join sync;
			done;
			succession first start then split;
			succession first split then quick;
			succession first split then slow1;
			succession first slow1 then slow2;
			succession first quick then sync;
			succession first slow2 then sync;
			succession first sync then done;
		}
	}`)
	run(t, s, "%action gather")
	run(t, s, "%step")
	run(t, s, "%step")
	run(t, s, "%step")
	got := run(t, s, "%tokens")
	wants(t, got, "Token 2 @ sync (arrived from quick; awaiting slow2)", "Token 3 @ slow2")
	rejects(t, got, "slow2 (arrived")
	wants(t, run(t, s, "%continue"), "✓ Action completed")
}

// A loop through a merge with no exit is stepped one node at a time; %continue then
// stops at the action step budget instead of spinning.
func TestActionDebuggerStepsAnUnguardedMergeLoop(t *testing.T) {
	s := loadSource(t, `package test {
		action spin {
			first start;
			merge m;
			action a;
			succession first start then m;
			succession first m then a;
			succession first a then m;
		}
	}`)
	budgets := runtime.DefaultBudgets()
	budgets.MaxActionSteps = 20
	if err := s.SetBudgets(budgets); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	run(t, s, "%action spin")
	for i := 0; i < 2*int(budgets.MaxActionSteps); i++ {
		wants(t, run(t, s, "%step"), "✓ Step complete", "Tokens: 1")
		at := "Token 1 @ m"
		if i%2 == 1 {
			at = "Token 1 @ a"
		}
		wants(t, run(t, s, "%tokens"), at)
	}
	wants(t, run(t, s, "%continue"),
		"error: execution failed:",
		"exceeded max steps (20 steps; raise "+runtime.MaxActionStepsEnvVar)
}

func TestActionDebuggerRejectsNonAction(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%action Vehicle"), "is not an action")
	wants(t, run(t, s, "%action nosuch"), `unresolved reference: nosuch`)
}

// %break stops a run when a token reaches the node, and the run resumes from
// there with the tokens intact.
func TestBreakpointStopsAndResumes(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")

	wants(t, run(t, s, "%break accumulate"), `✓ Breakpoint set at node "accumulate"`)

	paused := run(t, s, "%continue")
	wants(t, paused, `⏸ Paused at breakpoint "accumulate"`, "Tokens: 1")
	rejects(t, paused, "Action completed")

	wants(t, run(t, s, "%tokens"), "Token 1 @ accumulate", "total = 0")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}

// The initial node already holds a token when the run starts, so a breakpoint
// on it must still stop before the first step.
func TestBreakpointOnInitialNodeStops(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	run(t, s, "%break start")

	paused := run(t, s, "%continue")
	wants(t, paused, `⏸ Paused at breakpoint "start"`, "Tokens: 1")
	rejects(t, paused, "Action completed")

	wants(t, run(t, s, "%tokens"), "Token 1 @ start")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}

// %break names a node a loop body declares: the run pauses before each of its
// performances, with the token at the node running the loop, and resumes once per pause.
func TestBreakpointOnABlockNodePausesEachIteration(t *testing.T) {
	s := loadFixture(t, "testdata/action_block_debug.sysml")
	run(t, s, "%action count")

	wants(t, run(t, s, "%break add"), `✓ Breakpoint set at node "add"`)

	paused := run(t, s, "%continue")
	wants(t, paused, `⏸ Paused at breakpoint "add"`, "Tokens: 1")
	rejects(t, paused, "Action completed")
	wants(t, run(t, s, "%tokens"), "Token 1 @ iterate", "total = 0")

	wants(t, run(t, s, "%step"), "✓ Step complete", `⏸ Paused at breakpoint "add"`)
	wants(t, run(t, s, "%tokens"), "Token 1 @ iterate", "total = 1")

	wants(t, run(t, s, "%continue"), `⏸ Paused at breakpoint "add"`)
	wants(t, run(t, s, "%tokens"), "total = 3")
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 6")
}

// %break on a node two fork branches converge on — a join, or a plain node reached over
// two successions — pauses once, when both arrivals are in, and once resumed runs on.
func TestBreakpointOnAConvergingNodePausesOnce(t *testing.T) {
	cases := []struct{ node, decl, result string }{
		{"sync", "join sync;", "hits = 2"},
		{"scale", "action scale { assign hits := hits * 10; }", "hits = 20"},
	}
	for _, tc := range cases {
		t.Run(tc.node, func(t *testing.T) {
			s := loadSource(t, `package test {
				private import ScalarValues::*;
				action meet {
					out attribute hits : Integer = 0;
					first start;
					fork split;
					action l { assign hits := hits + 1; }
					action r { assign hits := hits + 1; }
					`+tc.decl+`
					done;
					succession first start then split;
					succession first split then l;
					succession first split then r;
					succession first l then `+tc.node+`;
					succession first r then `+tc.node+`;
					succession first `+tc.node+` then done;
				}
			}`)
			run(t, s, "%action meet")
			wants(t, run(t, s, "%break "+tc.node), `✓ Breakpoint set at node "`+tc.node+`"`)

			paused := run(t, s, "%continue")
			wants(t, paused, `⏸ Paused at breakpoint "`+tc.node+`"`, "Tokens: 2")
			rejects(t, paused, "Action completed")
			tokens := run(t, s, "%tokens")
			wants(t, tokens, "Token 2 @ "+tc.node, "Token 3 @ "+tc.node, "hits = 2")
			rejects(t, tokens, "awaiting")

			wants(t, run(t, s, "%continue"), "✓ Action completed", tc.result)
		})
	}
}

// Ending a session paused in a block node — by %stop, by starting another, or
// by redeclaring the action — releases its executor: the paused run ends and
// the executor takes no further step.
func TestEndingAPausedSessionReleasesItsRun(t *testing.T) {
	pauseAtAdd := func(t *testing.T) (*Session, *runtime.ActionExecutor) {
		t.Helper()
		s := loadFixture(t, "testdata/action_block_debug.sysml")
		run(t, s, "%action count")
		run(t, s, "%break add")
		wants(t, run(t, s, "%continue"), `⏸ Paused at breakpoint "add"`)
		return s, s.actionExec.executor
	}
	released := func(t *testing.T, exec *runtime.ActionExecutor) {
		t.Helper()
		if err := exec.Step(); !errors.Is(err, runtime.ErrExecutorReleased) {
			t.Errorf("Step of the ended session's executor = %v, want ErrExecutorReleased", err)
		}
	}

	t.Run("stop", func(t *testing.T) {
		s, exec := pauseAtAdd(t)
		wants(t, run(t, s, "%stop"), `✓ Stopped debugging session for "count"`)
		released(t, exec)
	})
	t.Run("another session", func(t *testing.T) {
		s, exec := pauseAtAdd(t)
		wants(t, run(t, s, "%action count"), `✓ Started action executor for "count"`)
		released(t, exec)
	})
	t.Run("redeclared", func(t *testing.T) {
		s, exec := pauseAtAdd(t)
		res := s.Submit(`package Debug { action count { attribute total : Integer = 1; } }`)
		wants(t, strings.Join(res.Notices, "\n"), `action debugging session for "count" ended`)
		released(t, exec)
	})
	t.Run("unrelated declaration keeps it", func(t *testing.T) {
		s, _ := pauseAtAdd(t)
		if res := s.Submit(`package Other { attribute x = 1; }`); len(res.Notices) > 0 {
			t.Fatalf("unrelated declaration ended the session: %v", res.Notices)
		}
		wants(t, run(t, s, "%step"), "✓ Step complete", `⏸ Paused at breakpoint "add"`)
	})
}

func TestBreakpointRejectsUnknownNode(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	got := run(t, s, "%break nosuchnode")
	wants(t, got, `has no node named "nosuchnode"`, "nodes: ")
}

// --- State machine debugger ---

func TestStateDebuggerAdvancesByTime(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")

	wants(t, run(t, s, "%state Cycle"), `✓ Started state machine executor for "Cycle"`, "Current state: init")
	wants(t, run(t, s, "%events"), "Event queue: 1 events")
	wants(t, run(t, s, "%current"), "Current state: init", "Time: 0.0")

	// The completion transition out of `init` is due now; the transition out of
	// `waiting` is scheduled at 10, so a shorter advance stops before it.
	wants(t, run(t, s, "%advance 1"), "Advanced to 1.0 (1 event(s) processed)", "Current state: waiting", "Last event at: 0.0")

	// Durations accumulate: the second advance reaches 10 even though no event
	// moved the executor's own clock during the first.
	wants(t, run(t, s, "%advance 9"), "Advanced to 10.0 (1 event(s) processed)", "Current state: working")
	wants(t, run(t, s, "%current"), "Time: 10.0")

	wants(t, run(t, s, "%advance 5"), "Current state: done", "Last event at: 15.0", "State machine completed")
	wants(t, run(t, s, "%advance 5"), "No pending work - simulation time is now 20.0")
}

// A do behavior is due now, so a small advance must run it even when the only
// queued event is far past the deadline.
func TestAdvanceRunsDoWorkWithFarFutureEvent(t *testing.T) {
	s := loadFixture(t, "testdata/state_do_far_event.sysml")
	run(t, s, "%state Slow")

	wants(t, run(t, s, "%advance 1"),
		"Current state: working",
		"Do behavior actions run: 2")
	wants(t, run(t, s, "%current"), "count = 2", "Time: 1.0")
}

// Do behavior with an empty event queue is do work, not an event: counting it
// as one made %advance report events that were never dispatched.
func TestAdvanceCountsDoWorkSeparatelyFromEvents(t *testing.T) {
	s := loadFixture(t, "testdata/state_do_no_event.sysml")
	run(t, s, "%state Work")

	out := run(t, s, "%advance 1")
	// Two transitions fire (init -> busy, busy -> done) and the three do actions
	// are reported as do work; counting them as events reported four.
	wants(t, out, "Advanced to 1.0 (2 event(s) processed)", "Do behavior actions run: 3")
	wants(t, run(t, s, "%current"), "count = 3")
}

// A change condition a do action has just made true is taken in the same
// advance, and a condition that never holds is reported rather than read as a
// machine that settled.
func TestAdvanceTakesChangeTriggerAndReportsAFalseCondition(t *testing.T) {
	s := loadFixture(t, "testdata/state_change_trigger.sysml")
	run(t, s, "%state Watch")

	wants(t, run(t, s, "%advance 1"), "Current state: armed", "Do behavior actions run: 1")
	wants(t, run(t, s, "%advance 1"),
		"No pending work",
		"waiting on change condition: armed: accept when blocked (condition is false)")
	wants(t, run(t, s, "%current"), "Current state: armed", "Cannot progress: waiting on change condition")
}

// Advancing less than the next event's timestamp is not "no pending work": the
// event is still queued, so the drain is reported with the state and what is left.
func TestAdvanceShorterThanNextEventReportsRemainingWork(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	run(t, s, "%state Cycle")
	run(t, s, "%advance 1") // leaves the transition out of `waiting` queued at 10

	wants(t, run(t, s, "%advance 1"),
		"Advanced to 2.0 (0 event(s) processed)",
		"Current state: waiting",
		"Remaining events: 1")
}

func TestAdvanceRejectsBadDuration(t *testing.T) {
	s := loadFixture(t, "testdata/state_debug.sysml")
	run(t, s, "%state Cycle")
	wants(t, run(t, s, "%advance soon"), "invalid time")
	wants(t, run(t, s, "%advance -1"), "must not be negative")
	// A trailing unit is accepted.
	wants(t, run(t, s, "%advance 10s"), "Current state: working")
}

// %current reports the whole active configuration of a machine whose top level
// is orthogonal regions, rather than <unknown>.
func TestCurrentShowsOrthogonalRegions(t *testing.T) {
	s := loadFixture(t, "../core/runtime/testdata/conformance/state_orthogonal_regions.sysml")
	wants(t, run(t, s, "%state TrafficLight"), "✓ Started state machine executor")
	got := run(t, s, "%current")
	wants(t, got, "Current state: start | start")
	rejects(t, got, "<unknown>")
}

func TestStateDebuggerRejectsNonStateMachine(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%state Vehicle"), "is not a state machine")
	wants(t, run(t, s, "%state nosuch"), `unresolved reference: nosuch`)
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"30", 30, false},
		{"30s", 30, false},
		{"0.5", 0.5, false},
		{"", 0, true},
		{"later", 0, true},
		{"-1", 0, true},
		// A non-finite duration would poison the debugger clock for good.
		{"NaN", 0, true},
		{"inf", 0, true},
		{"-Inf", 0, true},
	}
	for _, tc := range cases {
		got, err := parseDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDuration(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
	}
}

func TestAnonymousNodeLabel(t *testing.T) {
	cases := []struct {
		node ast.Node
		want string
	}{
		{&ast.InitialNode{}, "<initial>"},
		{&ast.FinalNode{}, "<final>"},
		{&ast.ForkNode{}, "<fork>"},
		{&ast.JoinNode{}, "<join>"},
		{&ast.MergeNode{}, "<merge>"},
		{&ast.DecisionNode{}, "<decision>"},
		{&ast.ActionExecutionNode{}, "<action>"},
		{nil, "<none>"},
		{&ast.Usage{}, "<anonymous>"},
	}
	for _, tc := range cases {
		if got := anonymousNodeLabel(tc.node); got != tc.want {
			t.Errorf("anonymousNodeLabel(%T) = %q, want %q", tc.node, got, tc.want)
		}
	}
}

func TestIsSymbolReference(t *testing.T) {
	for _, expr := range []string{"mass", "Demo::Vehicle::mass"} {
		if !isSymbolReference(expr) {
			t.Errorf("%q should be a symbol reference", expr)
		}
	}
	for _, expr := range []string{"", "a + b", "f(x)", "a.b", "a:b"} {
		if isSymbolReference(expr) {
			t.Errorf("%q should not be a symbol reference", expr)
		}
	}
}

// quantitySession is a document whose imports bring in the units and a calc
// taking quantity arguments, which is what the prompt is asked about below.
func quantitySession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(`package QSession {
		public import SI::*;
		public import ISQ::*;
		calc def Fall {
			in v0 : ISQSpaceTime::SpeedValue;
			in tb : ISQSpaceTime::TimeValue;
			return : ISQBase::LengthValue = v0 * tb;
		}
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// An expression typed at the prompt is evaluated in the namespace the session is
// working in, so the units that namespace imports resolve unqualified.
func TestEvalResolvesImportedUnitsUnqualified(t *testing.T) {
	s := quantitySession(t)
	wants(t, run(t, s, "%eval 1.0 [m/s]"), "= 1.0 [m/s]")
	wants(t, run(t, s, "%eval 2.0 [km] + 500.0 [m]"), "= 2.5 [km]")
	// The unit itself is a declaration the imports make visible: it resolves
	// to the measurement reference it declares.
	wants(t, run(t, s, "%eval m"), "= m")
	rejects(t, run(t, s, "%eval m"), "unresolved reference", "has no value to evaluate")
	// A name nothing declares still reports that it is unknown.
	wants(t, run(t, s, "%eval nosuch"), `unresolved reference: nosuch`)

	// The same scope is what a compound expression names its members in.
	pkg := NewSession()
	pkg.Submit("package Demo { attribute mass = 3.0; }")
	wants(t, run(t, pkg, "%eval mass * 2"), "= 6.0")
}

// An attribute shaped as a Collections::Array by its own dimensions and elements
// binds no value expression, yet names one Array value; its derived features
// read out of that value, and a valueless attribute of another type still
// reports that it holds none.
func TestEvalArrayShapedByItsFeatures(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package Grid {
		private import ScalarValues::*;
		private import Collections::*;
		attribute cells : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
		attribute bare : Integer;
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	wants(t, run(t, s, "%eval Grid::cells"), "= Array(2, 3)[1, 2, 3, 4, 5, 6]")
	wants(t, run(t, s, "%eval Grid::cells.rank"), "= 2")
	wants(t, run(t, s, "%eval Grid::cells.flattenedSize"), "= 6")
	wants(t, run(t, s, "%eval CollectionFunctions::'array#'(Grid::cells, (2, 1))"), "= 4")
	wants(t, run(t, s, "%eval Grid::bare"), "has no value to evaluate")
}

// A require/assume constraint binding a value reads that value, as a constraint
// usage's `= expr` does; one binding none holds no value to evaluate.
func TestEvalReadsRequirementConstraintValues(t *testing.T) {
	s := NewSession()
	s.Submit(`package Demo {
		constraint plain = false;
		requirement def R {
			require constraint failed = false;
			assume constraint granted = true;
			require constraint bare;
		}
	}`)
	wants(t, run(t, s, "%eval Demo::plain"), "= false")
	wants(t, run(t, s, "%eval Demo::R::failed"), "= false")
	wants(t, run(t, s, "%eval Demo::R::granted"), "= true")
	wants(t, run(t, s, "%eval Demo::R::bare"), "has no value to evaluate")
}

// The namespace the session works in is the last one it declared, so declaring
// another moves it: the earlier package is then reached by qualified name.
func TestPromptScopeIsTheLastNamespaceDeclared(t *testing.T) {
	s := NewSession()
	s.Submit("package P1 { public import SI::*; attribute a = 1.0; }")
	wants(t, run(t, s, "%eval 1.0 [m]"), "= 1.0 [m]")

	s.Submit("package P2 { attribute b = 2.0; }")
	wants(t, run(t, s, "%eval b * 3"), "= 6.0")
	wants(t, run(t, s, "%eval 1.0 [m]"), "unresolved unit m")
	wants(t, run(t, s, "%eval P1::a + P2::b"), "= 3.0")

	// A name two packages declare is reported, not answered from whichever of
	// them the prompt scope reaches.
	s.Submit("package P3 { attribute b = 5.0; }")
	wants(t, run(t, s, "%eval b"), "is ambiguous", "P2::b", "P3::b")
}

// %calc parses its arguments as expressions, so an argument that contains
// spaces — a quantity, a parenthesized expression, a nested call — survives.
func TestCalcParsesExpressionArguments(t *testing.T) {
	s := quantitySession(t)
	wants(t, run(t, s, "%calc Fall -15.0 [m/s] 8.5 [s]"), "✓ Fall(-15.0 [m/s], 8.5 [s])", "= -127.5 [m]")
	// The same invocation written the way the notation writes one.
	wants(t, run(t, s, "%calc Fall(-15.0 [m/s], 8.5 [s])"), "= -127.5 [m]")
	// A parenthesized subexpression, and a call standing as an argument.
	wants(t, run(t, s, "%calc Fall (-5.0 [m/s] - 10.0 [m/s]) 8.5 [s]"), "= -127.5 [m]")
	wants(t, run(t, s, "%calc Fall -15.0 [m/s] (4.0 [s] + 4.5 [s])"), "= -127.5 [m]")
	// Named arguments are a different production; the limitation is reported.
	wants(t, run(t, s, "%calc Fall v0=-15.0 [m/s] tb=8.5 [s]"), "named arguments are not supported")
}

// %calc runs a calculation whose body is statements — locals, a loop, an early
// return — the same way it runs one that is a single expression.
func TestCalcRunsAStatementBody(t *testing.T) {
	s := NewSession()
	s.Submit(`package P {
		calc def factorial {
			in n;
			attribute acc = 1;
			attribute i = 1;
			while i <= n {
				assign acc := acc * i;
				assign i := i + 1;
			}
			return : Integer = acc;
		}
		calc def firstOver {
			in limit;
			for x in (1, 5, 9) {
				if x > limit { return : Integer = x; }
			}
			return : Integer = 0;
		}
	}`)
	wants(t, run(t, s, "%calc P::factorial 6"), "✓ P::factorial(6)", "= 720")
	wants(t, run(t, s, "%calc P::factorial 0"), "= 1")
	wants(t, run(t, s, "%calc P::firstOver 3"), "= 5")
	wants(t, run(t, s, "%calc P::firstOver 100"), "= 0")
}

// A whitespace-separated argument may be signed: `5 -3` is two arguments, while
// `5 - 3` — an expression left unfinished across the space — is one.
func TestCalcSeparatesSignedArguments(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%calc add 5 -3"), "✓ add(5, -3)", "= 2")
	wants(t, run(t, s, "%calc add 5, -3"), "✓ add(5, -3)", "= 2")
	wants(t, run(t, s, "%calc add -5 -3"), "✓ add(-5, -3)", "= -8")
	wants(t, run(t, s, "%calc add (2 + 3) -3"), "✓ add((2 + 3), -3)", "= 2")
	wants(t, run(t, s, "%calc add 5 - 3"), `parameter "y" has no argument`)
	// An `=` inside an argument's own parentheses binds that call's parameter,
	// not the argument, so only the argument's own binding is refused.
	wants(t, run(t, s, "%calc add add(x = 1, y = 2) 4"), "✓ add(add(x = 1, y = 2), 4)", "= 7")
	wants(t, run(t, s, "%calc add a=1 2"), "named arguments are not supported")
	// Malformed input is diagnosed rather than parsed past.
	wants(t, run(t, s, "%calc add (5 3"), "failed to parse argument")
}

// A quantity's magnitude is a number in a result table like any other, so it is
// rendered by the same convention as a bare Real.
func TestFormatValueQuantityUsesRealFormatting(t *testing.T) {
	s := quantitySession(t)
	wants(t, run(t, s, "%eval -15.200531548598184 [m/s]"), "= -15.200531548598184 [m/s]")
	wants(t, run(t, s, "%eval 32.99999999999993 [s]"), "= 32.99999999999993 [s]")
	// A whole magnitude keeps its ".0", as a bare Real does.
	wants(t, run(t, s, "%eval 2.0 [m] * 3.0 [m]"), "= 6.0 [m**2]")
	// A magnitude below what two decimals can show reads as itself, not as zero.
	wants(t, run(t, s, "%eval 0.0001 [m]"), "= 0.0001 [m]")
}

// A calc usage computes output features rather than one result, so %calc lists
// every output of one evaluation and %eval reads them as features.
func TestCalcUsageOutputsAtThePrompt(t *testing.T) {
	s := NewSession()
	s.Submit(`package M {
		calc def Two { in n; out a = n + 1; out b = n * 2; }
		calc c : Two { in n = 5; }
		attribute z = c.b;
		part p {
			calc d : Two { in n = 7; }
			attribute q = d.a;
		}
	}`)
	wants(t, run(t, s, "%calc M::c"), "✓ M::c", "a = 6", "b = 10")
	wants(t, run(t, s, "%eval M::c.a"), "= 6")
	wants(t, run(t, s, "%eval M::c.b"), "= 10")
	wants(t, run(t, s, "%eval M::z"), "= 10")
	wants(t, run(t, s, "%eval M::p::q"), "= 8")
}

// A calculation yields exactly one result, so invoking one that computes several
// outputs and designates none is reported instead of answering with whichever
// output comes first.
func TestCalcWithSeveralOutputsIsNotInvocable(t *testing.T) {
	s := NewSession()
	s.Submit(`package M {
		calc def Two { in n; out a = n + 1; out b = n * 2; }
	}`)
	wants(t, run(t, s, "%calc M::Two 5"), "has no single result", "a, b")
	wants(t, run(t, s, "%eval M::Two(5)"), "has no single result")
}

// A signal a state's entry action sent through a port is in flight and due now:
// %advance must dispatch it, so stepping the debugger reaches the same state
// running the machine to completion does.
func TestAdvanceDeliversPendingPortSignal(t *testing.T) {
	s := loadFixture(t, "../core/runtime/testdata/conformance/state_transition_accept_via_port.sysml")
	run(t, s, "%state Radio")

	wants(t, run(t, s, "%advance 1"), "Current state: done", "State machine completed")
	wants(t, run(t, s, "%current"), "received = 1")
}

// A machine performed by an object routes over that object's connections, so
// naming the object is how the debugger reaches a variant selection: two objects
// of one type each drive the machine to the state their own variant connects to.
func TestStateDebuggerRoutesForThePerformingObject(t *testing.T) {
	s := loadFixture(t, "../core/runtime/testdata/conformance/variant_connection_per_owner.sysml")

	run(t, s, "%instantiate VariantRouting::alpha")
	run(t, s, "%instantiate VariantRouting::beta")

	wants(t, run(t, s, "%state VariantRouting::Router::Route VariantRouting::alpha"), "✓ Started state machine executor")
	wants(t, run(t, s, "%advance 1"), "Current state: arrived")

	wants(t, run(t, s, "%state VariantRouting::Router::Route VariantRouting::beta"), "✓ Started state machine executor")
	wants(t, run(t, s, "%advance 1"), "Current state: diverted")
}

// A behavior performed by nothing routes over its own connections only, and an
// object named for a behavior that was never instantiated is reported.
func TestStateDebuggerReportsAnUninstantiatedPerformer(t *testing.T) {
	s := loadFixture(t, "../core/runtime/testdata/conformance/variant_connection_per_owner.sysml")
	wants(t, run(t, s, "%state VariantRouting::Router::Route VariantRouting::alpha"), "no instance of")
}

// A machine whose substates are typed usages: the debugger enters the state the
// definition declares initial, and each usage keeps its own attribute values.
func TestStateDebuggerStepsThroughInheritedContent(t *testing.T) {
	s := loadFixture(t, "testdata/state_typed_usage.sysml")

	wants(t, run(t, s, "%state Machine"), "Current state: i1")
	wants(t, run(t, s, "%current"), "one.hits = 1", "two.hits = 0")
	wants(t, run(t, s, "%advance 5"), "Current state: i2")
	wants(t, run(t, s, "%current"), "one.hits = 1", "two.hits = 0", "Time: 5.0")
}
