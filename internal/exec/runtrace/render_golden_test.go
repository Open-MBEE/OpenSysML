package runtrace

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

var updateRunGoldens = flag.Bool("update", false, "rewrite run-rendering goldens")

type renderingGolden struct {
	name string
	form view.Form
	run  *view.Rendering
}

func TestRunRenderingGoldens(t *testing.T) {
	example, options := exampleRun(t)
	cases := make([]renderingGolden, 0)
	for _, kind := range []Kind{KindTimeline, KindSequence} {
		run, err := Render(kind, example, options)
		if err != nil {
			t.Fatal(err)
		}
		cases = appendForms(cases, "example-"+string(kind), run)
		empty, err := Render(kind, runtime.NewTraceRecorder(), Options{})
		if err != nil {
			t.Fatal(err)
		}
		cases = appendForms(cases, "empty-"+string(kind), empty)
	}

	parallelTrace, parallelOptions, _, _ := fixtureRun(t, "parallel-regions.sysml", "box", 2)
	parallelTimeline, err := Render(KindTimeline, parallelTrace, parallelOptions)
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "parallel-regions-timeline", form: view.FormText, run: parallelTimeline})

	serialTrace, serialOptions, serialCtx, mission := fixtureRun(t, "serial-pairing.sysml", "mission", 2)
	serialLabels := make(map[int64]string)
	for _, name := range []string{"alpha", "beta", "receiver"} {
		value, err := mission.GetFeatureValue(serialCtx, name)
		if err != nil {
			t.Fatal(err)
		}
		id, ok := value.HeldValue().Object()
		if !ok {
			t.Fatalf("mission.%s does not hold an object", name)
		}
		serialLabels[id] = "SerialPairing::mission." + name
	}
	serialOptions.Label = func(inst *runtime.Instance) string { return serialLabels[inst.ID] }
	serialSequence, err := Render(KindSequence, serialTrace, serialOptions)
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "serial-pairing-sequence", form: view.FormText, run: serialSequence})

	terminationTrace, terminationOptions, _, _ := fixtureRun(t, "termination.sysml", "mission", 10)
	terminationTimeline, err := Render(KindTimeline, terminationTrace, terminationOptions)
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "termination-timeline", form: view.FormText, run: terminationTimeline})

	cappedTimeline := longTimeline()
	cappedTimelineOptions := Options{Until: 201, Limit: 200}
	capped, err := Render(KindTimeline, cappedTimeline, cappedTimelineOptions)
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "capped-timeline", form: view.FormText, run: capped})
	cappedSequence, err := Render(KindSequence, longSequence(), Options{Until: 201, Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "capped-sequence", form: view.FormText, run: cappedSequence})

	truncatedTimeline := truncatedTimelineTrace()
	truncated, err := Render(KindTimeline, truncatedTimeline, Options{Until: 4})
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "truncated-timeline", form: view.FormText, run: truncated})
	truncatedSequence, err := Render(KindSequence, truncatedSequenceTrace(), Options{Until: 4})
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "truncated-sequence", form: view.FormText, run: truncatedSequence})

	guard, guardOptions := guardTrace()
	guardRun, err := Render(KindTimeline, guard, guardOptions)
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "guard-timeline", form: view.FormText, run: guardRun})
	messages := messageEdgeCases()
	messageRun, err := Render(KindSequence, messages, Options{Until: 4})
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "messages-sequence", form: view.FormText, run: messageRun})
	selfTransition, err := Render(KindTimeline, selfTransitionTrace(), Options{Until: 2})
	if err != nil {
		t.Fatal(err)
	}
	cases = append(cases, renderingGolden{name: "self-transition-timeline", form: view.FormText, run: selfTransition})
	hashLabel := runtime.NewTraceRecorder()
	hashObject, hashBehavior := traceObject(7, "receiver")
	hashLabel.RecordStateEntry(traceOrigin(0, hashObject, hashBehavior), "ready", "ready", "", false)
	hashRun, err := Render(KindTimeline, hashLabel, Options{Until: 1})
	if err != nil {
		t.Fatal(err)
	}
	cases = appendForms(cases, "hash-label-timeline", hashRun)
	for _, kind := range []Kind{KindTimeline, KindSequence} {
		run, err := Render(kind, example, options)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, renderingGolden{name: "dot-" + string(kind), form: view.FormDot, run: run})
	}

	for _, tc := range cases {
		t.Run(tc.name+"."+string(tc.form), func(t *testing.T) {
			got, err := tc.run.Write(tc.form)
			if tc.form == view.FormDot {
				if err == nil {
					t.Fatal("DOT rendering unexpectedly succeeded")
				}
				got = err.Error() + "\n"
			} else if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", tc.name+"."+string(tc.form)+".golden")
			if *updateRunGoldens {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != got {
				t.Errorf("golden mismatch; run with -update to accept:\n%s", firstDifference(string(want), got))
			}
		})
	}
}

func appendForms(cases []renderingGolden, name string, run *view.Rendering) []renderingGolden {
	for _, form := range []view.Form{view.FormText, view.FormMermaid, view.FormPlantUML} {
		cases = append(cases, renderingGolden{name: name, form: form, run: run})
	}
	return cases
}

func exampleRun(t *testing.T) (*runtime.TraceRecorder, Options) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "examples", "run-timeline", "run-timeline.sysml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := source.New(path, data)
	file := parser.New(src).ParseFile()
	index := libs.NewModelIndex()
	index.AddDocument(path, file)
	resolver := resolve.New(index)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 100_000)
	ctx.SetTrace(runtime.NewTraceRecorder())
	if err := ctx.SetSchedule(runtime.DefaultSchedulePolicy); err != nil {
		t.Fatal(err)
	}
	packageScope := index.DocumentRoot(path).Children()[0]
	root, err := ctx.Instantiate(mustSymbol(t, packageScope, "mission"))
	if err != nil {
		t.Fatal(err)
	}
	labels := make(map[int64]string)
	for _, feature := range []string{"controller", "instrument"} {
		fv, err := root.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Fatal(err)
		}
		id, ok := fv.HeldValue().Object()
		if !ok {
			t.Fatalf("mission.%s does not hold an object", feature)
		}
		labels[id] = "RunTimeline::mission." + feature
	}
	if _, err := ctx.Advance(6); err != nil {
		t.Fatal(err)
	}
	return ctx.Trace(), Options{
		Until: 6,
		Label: func(inst *runtime.Instance) string { return labels[inst.ID] },
	}
}

func fixtureRun(t *testing.T, filename, instanceName string, until float64) (*runtime.TraceRecorder, Options, *runtime.Context, *runtime.Instance) {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := source.New(path, data)
	file := parser.New(src).ParseFile()
	index := libs.NewModelIndex()
	index.AddDocument(path, file)
	resolver := resolve.New(index)
	ctx := runtime.NewContext(runtime.NewModel(semantics.NewModel(resolver), resolver), 100_000)
	ctx.SetTrace(runtime.NewTraceRecorder())
	if err := ctx.SetSchedule(runtime.DefaultSchedulePolicy); err != nil {
		t.Fatal(err)
	}
	packageScope := index.DocumentRoot(path).Children()[0]
	root, err := ctx.Instantiate(mustSymbol(t, packageScope, instanceName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.Advance(until); err != nil {
		t.Fatal(err)
	}
	return ctx.Trace(), Options{
		Until: until,
		Label: func(inst *runtime.Instance) string {
			if inst.Type != nil {
				return inst.Type.Name
			}
			return ""
		},
	}, ctx, root
}

func mustSymbol(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	if scope == nil {
		t.Fatalf("no scope while looking up %q", name)
	}
	symbol, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("no symbol %q", name)
	}
	return symbol
}

func traceOrigin(at float64, object *runtime.Instance, behavior *symbols.Symbol) runtime.TraceOrigin {
	return runtime.TraceOrigin{At: at, Object: object, Behavior: behavior}
}

func traceObject(id int64, name string) (*runtime.Instance, *symbols.Symbol) {
	typeSymbol := &symbols.Symbol{Name: name}
	return &runtime.Instance{ID: id, Type: typeSymbol}, &symbols.Symbol{Name: "modes"}
}

func longTimeline() *runtime.TraceRecorder {
	trace := runtime.NewTraceRecorder()
	object, behavior := traceObject(1, "sender")
	trace.RecordStateEntry(traceOrigin(0, object, behavior), "s0", "s0", "", false)
	for at := 1; at <= 201; at++ {
		from := fmt.Sprintf("s%d", at-1)
		to := fmt.Sprintf("s%d", at)
		trace.RecordStateExit(traceOrigin(float64(at), object, behavior), from, from, "", false)
		trace.RecordStateTransition(traceOrigin(float64(at), object, behavior), from, to, "tick")
		trace.RecordStateEntry(traceOrigin(float64(at), object, behavior), to, to, "", false)
	}
	return trace
}

func longSequence() *runtime.TraceRecorder {
	trace := runtime.NewTraceRecorder()
	sender, senderBehavior := traceObject(1, "sender")
	receiver, receiverBehavior := traceObject(2, "receiver")
	for at := 0; at < 201; at++ {
		trace.RecordSend(traceOrigin(float64(at), sender, senderBehavior), runtime.Message{SignalType: "Ping"}, receiver)
		trace.RecordAccept(traceOrigin(float64(at), receiver, receiverBehavior), 0, "Ping", nil)
	}
	return trace
}

func truncatedTimelineTrace() *runtime.TraceRecorder {
	trace := runtime.NewEventRecorder(2)
	object, behavior := traceObject(1, "sender")
	trace.RecordStateEntry(traceOrigin(0, object, behavior), "old", "old", "", false)
	trace.RecordStateExit(traceOrigin(1, object, behavior), "old", "old", "", false)
	trace.RecordStateEntry(traceOrigin(2, object, behavior), "next", "next", "", false)
	trace.RecordStateExit(traceOrigin(3, object, behavior), "next", "next", "", false)
	trace.RecordStateEntry(traceOrigin(4, object, behavior), "held", "held", "", false)
	return trace
}

func truncatedSequenceTrace() *runtime.TraceRecorder {
	trace := runtime.NewEventRecorder(2)
	sender, senderBehavior := traceObject(1, "sender")
	receiver, receiverBehavior := traceObject(2, "receiver")
	trace.RecordSend(traceOrigin(0, sender, senderBehavior), runtime.Message{SignalType: "Old"}, receiver)
	trace.RecordAccept(traceOrigin(1, receiver, receiverBehavior), 0, "Old", nil)
	trace.RecordSend(traceOrigin(2, sender, senderBehavior), runtime.Message{SignalType: "Ping"}, receiver)
	trace.RecordAccept(traceOrigin(3, receiver, receiverBehavior), 0, "Ping", nil)
	return trace
}

func guardTrace() (*runtime.TraceRecorder, Options) {
	trace := runtime.NewTraceRecorder()
	object, behavior := traceObject(1, "sender")
	origin := traceOrigin(1, object, behavior)
	trace.RecordStateEntry(traceOrigin(0, object, behavior), "waiting", "waiting", "", false)
	trace.RecordNote(origin, runtime.UnevaluableGuard{Where: "waiting", Alternative: "accept Ping", Reason: "unbound"})
	return trace, Options{Until: 2}
}

func messageEdgeCases() *runtime.TraceRecorder {
	trace := runtime.NewTraceRecorder()
	sender, senderBehavior := traceObject(1, "sender")
	receiver, receiverBehavior := traceObject(2, "receiver")
	trace.RecordSend(traceOrigin(0, sender, senderBehavior), runtime.Message{SignalType: "Unmatched"}, receiver)
	trace.RecordSend(traceOrigin(1, nil, nil), runtime.Message{SignalType: "Outside"}, nil)
	trace.RecordSend(traceOrigin(2, sender, senderBehavior), runtime.Message{SignalType: "Broadcast"}, nil)
	trace.RecordAccept(traceOrigin(2.5, receiver, receiverBehavior), 0, "Broadcast", nil)
	trace.RecordAccept(traceOrigin(3, receiver, receiverBehavior), 0, "Missing", nil)
	return trace
}

func selfTransitionTrace() *runtime.TraceRecorder {
	trace := runtime.NewTraceRecorder()
	object, behavior := traceObject(1, "sender")
	trace.RecordStateEntry(traceOrigin(0, object, behavior), "idle", "idle", "", false)
	trace.RecordStateExit(traceOrigin(1, object, behavior), "idle", "idle", "", false)
	trace.RecordStateTransition(traceOrigin(1, object, behavior), "idle", "idle", "accept Tick")
	trace.RecordStateEntry(traceOrigin(1, object, behavior), "idle", "idle", "", false)
	return trace
}

func firstDifference(want, got string) string {
	for i := 0; i < len(want) && i < len(got); i++ {
		if want[i] != got[i] {
			return fmt.Sprintf("first difference at byte %d\nwant: %q\ngot:  %q", i, want[i:], got[i:])
		}
	}
	return fmt.Sprintf("want %d bytes, got %d", len(want), len(got))
}
