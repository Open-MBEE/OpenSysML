package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// coupledDynamics is a ramp and an integrator of it, each performed by a part of site;
// connect joins them in site, and inside is the integrator's own extra.
func coupledDynamics(connect, inside string) string {
	return `package test {
		private import ScalarValues::*;
		private import SI::*;
		private import ISQ::DurationValue;
		private import VectorFunctions::*;
		private import StateSpaceRepresentation::*;

		part site {
			part source {
				perform action ramp : ContinuousStateSpaceDynamics {
					in :>> input = VectorOf((0.0));
					:>> stateSpace = VectorOf((0.0));
					out :>> output;
					attribute timeStep : DurationValue = 0.5 [s];
					attribute stopTime : DurationValue = 2 [s];
					calc :>> getDerivative {
						in input : Input;
						in stateSpace : StateSpace;
						return : StateDerivative = VectorOf((1.0)) / 1 [s];
					}
					calc :>> getOutput {
						in input : Input;
						in stateSpace : StateSpace;
						return : Output = 2.0 * stateSpace;
					}
				}
				attribute label : String = "ramp";
			}
			part sink {
				perform action sum : ContinuousStateSpaceDynamics {
					in :>> input;
					:>> stateSpace = VectorOf((0.0));
					out :>> output;
					attribute timeStep : DurationValue = 0.5 [s];
					attribute stopTime : DurationValue = 2 [s];
					` + inside + `
					calc :>> getDerivative {
						in input : Input;
						in stateSpace : StateSpace;
						return : StateDerivative = input / 1 [s];
					}
					calc :>> getOutput {
						in input : Input;
						in stateSpace : StateSpace;
						return : Output = 1.0 * stateSpace;
					}
				}
			}
			` + connect + `
		}
	}`
}

// runCoupled materializes test::site of src, runs the clock for 3 s and returns the
// integrator's performance with the error it failed with, if any.
func runCoupled(t *testing.T, src string) (*ActionExecutor, error) {
	t.Helper()
	return runCoupledIn(t, src, "test::site", "sink")
}

// runCoupledIn is runCoupled over the object declared as object, a site or one specializing it.
func runCoupledIn(t *testing.T, src, object, part string) (*ActionExecutor, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	site := performerObject(t, ctx, idx, object)
	if _, err := ctx.Advance(3); err != nil {
		t.Fatalf("advance: %v", err)
	}
	sink, ok := ctx.Instance(objectAtPath(t, ctx, site, part))
	if !ok {
		t.Fatal("site holds no sink")
	}
	behaviors := sink.Behaviors()
	if len(behaviors) != 1 || behaviors[0].Action == nil {
		t.Fatalf("sink performs %d behaviors, want the one action", len(behaviors))
	}
	return behaviors[0].Action, behaviors[0].Err
}

// A flow from one dynamics' output to another's input carries each sample across:
// the integrator ends where it ends when a binding holds the two equal.
func TestStateSpaceFlowCarriesTheOutputToTheInput(t *testing.T) {
	results := make(map[string]string)
	for name, connect := range map[string]string{
		"flow":    "flow source.ramp.output to sink.sum.input;",
		"binding": "bind source.ramp.output = sink.sum.input;",
	} {
		sum, err := runCoupled(t, coupledDynamics(connect, ""))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !sum.state.Ended() {
			t.Fatalf("%s: the integrator is %v after its stopTime", name, sum.state)
		}
		results[name] = FormatValue(sum.Results()["stateSpace"])
	}
	// The ramp's output is 2t; held over each 0.5 s step from its start it sums to 3.
	if want := "⟨3.0⟩"; results["flow"] != want {
		t.Errorf("flow: the integrator ended at %s, want %s", results["flow"], want)
	}
	if results["flow"] != results["binding"] {
		t.Errorf("flow ended at %s, binding at %s", results["flow"], results["binding"])
	}
}

// A source that ended before the dynamics first step still feeds them what it left.
func TestStateSpaceFlowFromAnEndedAction(t *testing.T) {
	sum, err := runCoupled(t, coupledDynamics(`
		part fixed {
			perform action emit { out level = VectorOf((2.0)); }
		}
		flow fixed.emit.level to sink.sum.input;`, ""))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := FormatValue(sum.Results()["stateSpace"]), "⟨4.0⟩"; got != want {
		t.Errorf("the integrator ended at %s, want %s", got, want)
	}
}

// A flow stated `[0]` carries nothing: beside the one that does, it is no second flow into the input.
func TestStateSpaceFlowOfNoneIsNoFlow(t *testing.T) {
	sum, err := runCoupled(t, coupledDynamics(`
		flow source.ramp.output to sink.sum.input;
		flow none [0] from source.label to sink.sum.input;`, ""))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := FormatValue(sum.Results()["stateSpace"]), "⟨3.0⟩"; got != want {
		t.Errorf("the integrator ended at %s, want %s", got, want)
	}
}

// A flow into an action an inheriting object performs under another name, redefining
// it, is still the flow into that action's input.
func TestStateSpaceFlowIntoARedefinedAction(t *testing.T) {
	src := coupledDynamics("flow source.ramp.output to sink.sum.input;", "")
	src = strings.Replace(src, "part site {", "part def Site {", 1)
	src = src[:strings.LastIndex(src, "}")] + `
		part later : Later;
		part def Later :> Site {
			part :>> sink {
				perform action total :>> sum {
					in :>> input;
					:>> stateSpace = VectorOf((0.0));
					out :>> output;
					calc :>> getDerivative {
						in input : Input;
						in stateSpace : StateSpace;
						return : StateDerivative = input / 1 [s];
					}
					calc :>> getOutput {
						in input : Input;
						in stateSpace : StateSpace;
						return : Output = 1.0 * stateSpace;
					}
				}
			}
		}
	}`
	total, err := runCoupledIn(t, src, "test::later", "sink")
	if err != nil {
		t.Fatal(err)
	}
	// The ramp samples first at each instant, so the integrator holds its output from the start of each step.
	if got := FormatValue(total.Results()["stateSpace"]); got != "⟨3.0⟩" {
		t.Errorf("the integrator ended at %s, want ⟨3.0⟩", got)
	}
}

// A flow into a part an inheriting object holds under another name, redefining it, is
// still the flow into the input of the action that part performs.
func TestStateSpaceFlowIntoARedefinedPart(t *testing.T) {
	src := coupledDynamics("flow source.ramp.output to sink.sum.input;", "")
	src = strings.Replace(src, "part site {", "part def Site {", 1)
	src = src[:strings.LastIndex(src, "}")] + `
		part later : Later;
		part def Later :> Site {
			part drain :>> sink;
		}
	}`
	sum, err := runCoupledIn(t, src, "test::later", "drain")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatValue(sum.Results()["stateSpace"]); got != "⟨3.0⟩" {
		t.Errorf("the integrator ended at %s, want ⟨3.0⟩", got)
	}
}

// A flow out of a part an inheriting object holds under another name, redefining it, is
// still the flow out of the output of the action that part performs.
func TestStateSpaceFlowOutOfARedefinedPart(t *testing.T) {
	src := coupledDynamics("flow source.ramp.output to sink.sum.input;", "")
	src = strings.Replace(src, "part site {", "part def Site {", 1)
	src = src[:strings.LastIndex(src, "}")] + `
		part later : Later;
		part def Later :> Site {
			part feed :>> source;
		}
	}`
	sum, err := runCoupledIn(t, src, "test::later", "sink")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatValue(sum.Results()["stateSpace"]); got != "⟨3.0⟩" {
		t.Errorf("the integrator ended at %s, want ⟨3.0⟩", got)
	}
}

// A flow out of an action an inheriting part performs under another name, redefining it,
// is still the flow out of that performance's output, read as it stands at each step.
func TestStateSpaceFlowOutOfARedefinedAction(t *testing.T) {
	src := coupledDynamics("flow source.ramp.output to sink.sum.input;", "")
	src = strings.Replace(src, "part site {", "part def Site {", 1)
	src = src[:strings.LastIndex(src, "}")] + `
		part later : Later;
		part def Later :> Site {
			part feed :>> source {
				perform action incline :>> ramp {
					in :>> input = VectorOf((0.0));
					:>> stateSpace = VectorOf((0.0));
					out :>> output;
					calc :>> getDerivative {
						in input : Input;
						in stateSpace : StateSpace;
						return : StateDerivative = VectorOf((1.0)) / 1 [s];
					}
					calc :>> getOutput {
						in input : Input;
						in stateSpace : StateSpace;
						return : Output = 2.0 * stateSpace;
					}
				}
			}
		}
	}`
	sum, err := runCoupledIn(t, src, "test::later", "sink")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatValue(sum.Results()["stateSpace"]); got != "⟨3.0⟩" {
		t.Errorf("the integrator ended at %s, want ⟨3.0⟩", got)
	}
}

// A flow a specializing type redefines replaces the one it inherits: the input has one feed, not two.
func TestStateSpaceFlowRedefiningAnInheritedFlow(t *testing.T) {
	src := coupledDynamics("flow feedIn from source.ramp.output to sink.sum.input;", "")
	src = strings.Replace(src, "part site {", "part def Site {", 1)
	src = src[:strings.LastIndex(src, "}")] + `
		part later : Later;
		part def Later :> Site {
			flow :>> feedIn from source.ramp.output to sink.sum.input;
		}
	}`
	sum, err := runCoupledIn(t, src, "test::later", "sink")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := FormatValue(sum.Results()["stateSpace"]), "⟨3.0⟩"; got != want {
		t.Errorf("the integrator ended at %s, want %s", got, want)
	}
}

// A guardless crossing whose type states terminal ends the dynamics at the step that detected it.
func TestStateSpaceDerivativeCrossingEndsATerminalRun(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_space_derivative_zero_crossing.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	model := strings.Replace(string(src), "action def Apex :> ZeroCrossingEventDef;",
		"action def Apex :> ZeroCrossingEventDef { attribute terminal : Boolean = true; }", 1)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
	ball := performerObject(t, ctx, idx, "test::ball")
	if _, err := ctx.Advance(3); err != nil {
		t.Fatalf("advance: %v", err)
	}
	for _, behavior := range ball.Behaviors() {
		if behavior.Action == nil {
			continue
		}
		if behavior.Err != nil {
			t.Fatal(behavior.Err)
		}
		if got, want := FormatValue(behavior.Action.Results()["stateSpace"]), "⟨2.5, -2.5⟩"; got != want {
			t.Errorf("the flight ended at %s, want %s, the state at its apex", got, want)
		}
	}
}

// An event occurrence typed by a StateSpaceEventDef that is no zero crossing has
// no condition the library states: the dynamics run to their stop and raise nothing.
func TestStateSpaceEventDefWithoutAConditionRaisesNothing(t *testing.T) {
	src := strings.Replace(coupledDynamics("flow source.ramp.output to sink.sum.input;",
		"event occurrence saturated : Saturation;"),
		"part site {", "action def Saturation :> StateSpaceEventDef;\n part site {", 1)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	performerObject(t, ctx, idx, "test::site")
	if _, err := ctx.Advance(3); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("the run posted %d messages, want none", len(pending))
	}
}

// The three OMG state-space examples are sketches of the protocol: every dynamics in them
// leaves out a calc or the step the run needs, which the runner reports by name.
func TestOMGStateSpaceExamplesLeaveTheProtocolAbstract(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "examples", "pilot-corpora", "sysml-examples", "State Space Representation Examples")
	for file, actions := range map[string]map[string]string{
		"CartSample.sysml": {
			"CartSample::context::cart::cartBehavior":     "states no timeStep",
			"CartSample::context::pusher::pusherBehavior": "leaves getDerivative abstract",
		},
		"EVSample.sysml": {
			"EVSample::vehicle::vehicleBehavior":          "leaves getDerivative abstract",
			"EVSample::vehicle::battery::batteryBehavior": "leaves getDerivative abstract",
			"EVSample::vehicle::motor::motorBehavior":     "leaves getDerivative abstract",
			"EVSample::vehicle::tire::tireBehavior":       "leaves getDerivative abstract",
		},
		"EVSample1.sysml": {
			"EVSample1::vehicle::vehicleBehavior":          "leaves getDerivative abstract",
			"EVSample1::vehicle::battery::batteryBehavior": "leaves getDerivative abstract",
			"EVSample1::vehicle::motor::motorBehavior":     "leaves getDerivative abstract",
			"EVSample1::vehicle::tire::tireBehavior":       "leaves getDerivative abstract",
			"EVSample1::resister":                          "leaves getDerivative abstract",
			"EVSample1::battery":                           "leaves getDerivative abstract",
		},
	} {
		path := filepath.Join(dir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.Getenv("OPENSYSML_REQUIRE_PILOT_CORPORA") != "" {
				t.Fatalf("pilot corpora required but absent: %v", err)
			}
			t.Skipf("pilot corpora absent; run ./scripts/download-pilot-corpora.sh: %v", err)
		}
		idx, _, ctx := buildRuntimeWithLibraries(t, path, parser.New(source.New(path, data)).ParseFile())
		for fqn, says := range actions {
			matches := idx.LookupQualified(fqn)
			if len(matches) != 1 {
				t.Errorf("%s: %d matching symbols, want 1", fqn, len(matches))
				continue
			}
			exec, err := ctx.CreateActionExecutor(matches[0])
			if err == nil {
				err = exec.RunToCompletion()
			}
			if !errors.Is(err, lower.ErrUnsupportedStateSpace) && !errors.Is(err, errStateSpaceStep) {
				t.Errorf("%s: %v, want a typed state-space refusal", fqn, err)
				continue
			}
			if !strings.Contains(err.Error(), says) {
				t.Errorf("%s: %q, want it to say %q", fqn, err, says)
			}
		}
	}
}

// sinkBeforeSource is src with the fed part declared ahead of the one feeding it.
func sinkBeforeSource(t *testing.T, src string) string {
	t.Helper()
	source, sink, flow := strings.Index(src, "part source {"), strings.Index(src, "part sink {"), strings.Index(src, "flow ")
	if source < 0 || sink < source || flow < sink {
		t.Fatalf("the model does not declare source, sink and flow in that order")
	}
	return src[:source] + src[sink:flow] + src[source:sink] + src[flow:]
}

// The dynamics feeding another sample first at each instant, whichever is declared first: an
// output computed from the input reads the source's from the start on.
func TestStateSpaceFlowIsIndependentOfDeclarationOrder(t *testing.T) {
	src := coupledDynamics("flow source.ramp.output to sink.sum.input;", "")
	src = strings.Replace(src, "return : Output = 1.0 * stateSpace;", "return : Output = 1.0 * input;", 1)
	for name, model := range map[string]string{"source first": src, "sink first": sinkBeforeSource(t, src)} {
		t.Run(name, func(t *testing.T) {
			sum, err := runCoupled(t, model)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := FormatValue(sum.Results()["stateSpace"]), "⟨3.0⟩"; got != want {
				t.Errorf("the integrator ended at %s, want %s", got, want)
			}
			if got, want := FormatValue(sum.Results()["output"]), "⟨4.0⟩"; got != want {
				t.Errorf("the integrator's output ended at %s, want %s", got, want)
			}
		})
	}
}

// Two dynamics feeding each other wait for neither: each runs on what the other last wrote.
func TestStateSpaceFlowsInACycleRun(t *testing.T) {
	src := coupledDynamics("flow source.ramp.output to sink.sum.input;\n flow sink.sum.output to source.ramp.input;", "")
	for name, model := range map[string]string{"source first": src, "sink first": sinkBeforeSource(t, src)} {
		t.Run(name, func(t *testing.T) {
			sum, err := runCoupled(t, model)
			if err != nil {
				t.Fatal(err)
			}
			// Which of the two samples first at an instant is the scheduler's draw, so the value is not pinned.
			if got := sum.Results()["stateSpace"]; !isVectorKind(soleElement(got)) {
				t.Errorf("the integrator ended at %s, want a vector", FormatValue(got))
			}
		})
	}
}

// Dynamics released mid-run sample no more, so the dynamics they fed do not wait for them.
func TestStateSpaceFlowFromAReleasedAction(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, coupledDynamics("flow source.ramp.output to sink.sum.input;", "")))
	site := performerObject(t, ctx, idx, "test::site")
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance: %v", err)
	}
	performance := func(part string) *ObjectBehavior {
		inst, ok := ctx.Instance(objectAtPath(t, ctx, site, part))
		if !ok || len(inst.Behaviors()) != 1 || inst.Behaviors()[0].Action == nil {
			t.Fatalf("%s performs no one action", part)
		}
		return inst.Behaviors()[0]
	}
	performance("source").Action.Release()
	if _, err := ctx.Advance(2); err != nil {
		t.Fatalf("advance: %v", err)
	}
	sum := performance("sink")
	if sum.Err != nil {
		t.Fatal(sum.Err)
	}
	if !sum.Action.state.Ended() {
		t.Fatalf("the integrator is %v after its stopTime", sum.Action.state)
	}
	// The ramp left 2.0 at 1 s: 0, 1 and 2 over the first three steps, 2 held over the last.
	if got, want := FormatValue(sum.Action.Results()["stateSpace"]), "⟨2.5⟩"; got != want {
		t.Errorf("the integrator ended at %s, want %s", got, want)
	}
}

// A flow stating its ends in its body carries the feature each end owns, as one stating them after from and to does.
func TestStateSpaceFlowWithEndsInItsBody(t *testing.T) {
	for name, flow := range map[string]string{
		"redefining the feature": "flow { end ::> source.ramp { ref :>> output; } end ::> sink.sum { ref :>> input; } }",
		"declaring a payload":    "flow of ScalarValues::Real { end ::> source.ramp { ref :>> output; } end ::> sink.sum { ref :>> input; } }",
		"as the clause expands it": `flow {
			end redefines Transfers::Transfer::source references source.ramp { ref redefines Transfers::Transfer::source::sourceOutput, output; }
			end references sink.sum redefines Transfers::Transfer::target { ref redefines Transfers::Transfer::target::targetInput, input; }
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			sum, err := runCoupled(t, coupledDynamics(flow, ""))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := FormatValue(sum.Results()["stateSpace"]), "⟨3.0⟩"; got != want {
				t.Errorf("the integrator ended at %s, want %s", got, want)
			}
		})
	}
}

// A flow leaving the source's stateSpace orders the two as one leaving its output does.
func TestStateSpaceFlowOfAStateIsIndependentOfDeclarationOrder(t *testing.T) {
	src := coupledDynamics("flow source.ramp.stateSpace to sink.sum.input;", "")
	for name, model := range map[string]string{"source first": src, "sink first": sinkBeforeSource(t, src)} {
		t.Run(name, func(t *testing.T) {
			sum, err := runCoupled(t, model)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := FormatValue(sum.Results()["stateSpace"]), "⟨1.5⟩"; got != want {
				t.Errorf("the integrator ended at %s, want %s", got, want)
			}
		})
	}
}
