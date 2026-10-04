package runtrace

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestKindParsingAndMissingTrace(t *testing.T) {
	if got := fmt.Sprint(Kinds()); got != "[timeline sequence]" {
		t.Fatalf("Kinds() = %s", got)
	}
	for _, want := range Kinds() {
		got, ok := ParseKind(string(want))
		if !ok || got != want {
			t.Errorf("ParseKind(%q) = %q, %t", want, got, ok)
		}
	}
	if _, ok := ParseKind("Timeline"); ok {
		t.Error("ParseKind accepted a differently cased kind")
	}
	if _, err := Render(KindTimeline, nil, Options{}); !errors.Is(err, ErrNoTrace) {
		t.Errorf("Render with no trace = %v, want ErrNoTrace", err)
	}
	if _, err := Render(Kind("other"), runtime.NewEventRecorder(0), Options{}); err == nil ||
		!strings.Contains(err.Error(), "timeline, sequence") {
		t.Errorf("Render with an unknown kind = %v", err)
	}
}

func TestTimelineGroupsStateChangesAndOrdersParallelLeaves(t *testing.T) {
	trace := runtime.NewEventRecorder(0)
	rover := testObject(1, "Rover")
	drone := testObject(2, "Drone")
	machine := &symbols.Symbol{Name: "Machine"}
	otherMachine := &symbols.Symbol{Name: "Other"}
	origin := func(at float64) runtime.TraceOrigin {
		return runtime.TraceOrigin{At: at, Object: rover, Behavior: machine}
	}
	trace.RecordStateEntry(origin(0), "open", "open", "", false)
	trace.RecordStateEntry(origin(0), "slewing", "open.slewing", "open.pointing", false)
	trace.RecordStateEntry(origin(0), "awake", "open.awake", "open.power", false)
	trace.RecordStateEntry(origin(0), "blink", "open.pointing.blink", "open.pointing", false)
	trace.RecordStateExit(origin(0), "blink", "open.pointing.blink", "open.pointing", false)
	trace.RecordNote(origin(1), runtime.ChoicePoint{
		Kind: runtime.ChoiceTransition, Where: "open", Alternatives: []string{"slewing", "settled"}, Taken: 0,
	})
	trace.RecordStateExit(origin(1.5), "slewing", "open.slewing", "open.pointing", false)
	trace.RecordStateTransition(origin(1.5), "slewing", "settled", "accept Ping")
	trace.RecordStateEntry(origin(1.5), "settled", "open.settled", "open.pointing", false)
	trace.RecordStateExit(origin(2), "awake", "open.awake", "open.power", false)
	trace.RecordStateTransition(origin(2), "awake", "awake", "after 1 s")
	trace.RecordStateEntry(origin(2), "awake", "open.awake", "open.power", false)
	trace.RecordNote(runtime.TraceOrigin{At: 2.5, Object: rover, Behavior: otherMachine},
		runtime.UnevaluableGuard{Where: "open", Alternative: "stalled", Reason: "value unavailable"})
	trace.RecordStateEntry(runtime.TraceOrigin{At: 0, Object: drone, Behavior: machine}, "ready", "ready", "", false)
	environmentChoice := runtime.ChoicePoint{Kind: runtime.ChoiceRegionOrder, Where: "dispatch"}
	trace.RecordNote(runtime.TraceOrigin{At: 3}, environmentChoice)

	rendering := Timeline(trace, Options{
		Until: 4,
		Label: func(instance *runtime.Instance) string {
			if instance.ID == rover.ID {
				return "rover"
			}
			return "drone"
		},
	})
	if !rendering.Run || rendering.Kind != view.KindTimeline || rendering.Stated != "the trace of a run to t = 4" {
		t.Fatalf("timeline metadata = %+v", rendering)
	}
	if len(rendering.Lanes) != 2 || rendering.Lanes[0].ID != "l0" || rendering.Lanes[1].ID != "l1" {
		t.Fatalf("lanes = %+v, want first-record order", rendering.Lanes)
	}
	lane := rendering.Lanes[0]
	if lane.Name != "rover.Machine" {
		t.Errorf("lane name = %q", lane.Name)
	}
	if len(lane.Spans) != 3 {
		t.Fatalf("spans = %+v, want initial, triggered and self-transition spans", lane.Spans)
	}
	want := []view.Span{
		{State: "slewing | awake", From: 0, To: 1.5, Through: []string{"blink"}},
		{State: "settled | awake", From: 1.5, To: 2, Triggers: []string{"accept Ping"}},
		{State: "settled | awake", From: 2, To: 4, Triggers: []string{"after 1 s"}, Open: true},
	}
	for i := range want {
		if got := lane.Spans[i]; got.State != want[i].State || got.From != want[i].From || got.To != want[i].To ||
			got.Open != want[i].Open || strings.Join(got.Triggers, "|") != strings.Join(want[i].Triggers, "|") ||
			strings.Join(got.Through, "|") != strings.Join(want[i].Through, "|") {
			t.Errorf("span %d = %+v, want %+v", i, got, want[i])
		}
	}
	if len(lane.Transitions) != 2 || lane.Transitions[0].Event != "accept Ping" ||
		lane.Transitions[1].From != "awake" || lane.Transitions[1].To != "awake" {
		t.Errorf("transitions = %+v", lane.Transitions)
	}
	if len(lane.Marks) != 2 || lane.Marks[0].Kind != "choice" ||
		lane.Marks[0].Text == "" || lane.Marks[1].Kind != "guard" || !strings.Contains(lane.Marks[1].Text, "unevaluable guard") {
		t.Errorf("marks = %+v", lane.Marks)
	}
	if rendering.Lanes[1].Name != "drone.Machine" || len(rendering.Lanes[1].Spans) != 1 ||
		rendering.Lanes[1].Spans[0].State != "ready" {
		t.Errorf("second lane = %+v", rendering.Lanes[1])
	}
	if !strings.Contains(strings.Join(rendering.Notices, "\n"), environmentChoice.String()) {
		t.Errorf("unattached mark notice = %v", rendering.Notices)
	}
}

func TestTimelineDistinguishesSameNamedStatesInSiblingRegions(t *testing.T) {
	trace, options, _, _ := fixtureRun(t, "parallel-regions.sysml", "box", 2)
	rendering, err := Render(KindTimeline, trace, options)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rendering.Write(view.FormText)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"0 .. 1  waiting | waiting", "1 .. 2  done | waiting"} {
		if !strings.Contains(got, want) {
			t.Errorf("timeline is missing %q:\n%s", want, got)
		}
	}
}

func TestTimelineTruncationAndSpanLimit(t *testing.T) {
	truncated := runtime.NewEventRecorder(3)
	object := testObject(1, "Rover")
	behavior := &symbols.Symbol{Name: "Machine"}
	origin := func(at float64) runtime.TraceOrigin {
		return runtime.TraceOrigin{At: at, Object: object, Behavior: behavior}
	}
	truncated.RecordStateEntry(origin(0), "lost", "lost", "", false)
	truncated.RecordStateExit(origin(1), "lost", "lost", "", false)
	truncated.RecordStateEntry(origin(2), "kept", "kept", "", false)
	truncated.RecordStateExit(origin(3), "kept", "kept", "", false)
	got := Timeline(truncated, Options{Until: 4})
	if len(got.Lanes) != 1 || len(got.Lanes[0].Spans) != 1 ||
		got.Lanes[0].Spans[0].State != "kept" || got.Lanes[0].Spans[0].From != 2 ||
		!strings.Contains(strings.Join(got.Notices, "\n"), "1 earlier records up to t = 0 were dropped") {
		t.Errorf("truncated rendering = %+v", got)
	}

	long := runtime.NewEventRecorder(0)
	long.RecordStateEntry(origin(0), "s0", "s0", "", false)
	for i := 1; i <= 202; i++ {
		at := float64(i)
		long.RecordStateExit(origin(at), fmt.Sprintf("s%d", i-1), fmt.Sprintf("s%d", i-1), "", false)
		long.RecordStateEntry(origin(at), fmt.Sprintf("s%d", i), fmt.Sprintf("s%d", i), "", false)
	}
	limited := Timeline(long, Options{Until: 203, Limit: 200})
	if count := len(limited.Lanes[0].Spans); count != 200 {
		t.Fatalf("limited spans = %d, want 200", count)
	}
	last := limited.Lanes[0].Spans[199]
	if last.From != 199 || last.To != 200 || last.Open {
		t.Errorf("last capped span = %+v", last)
	}
	if !strings.Contains(strings.Join(limited.Notices, "\n"),
		"3 later state changes from t = 200 are not drawn (at most 200 spans are)") {
		t.Errorf("cap notice = %v", limited.Notices)
	}
}

func TestTimelineCapUsesStableSpanIdentityForEqualInstants(t *testing.T) {
	trace := runtime.NewEventRecorder(0)
	behavior := &symbols.Symbol{Name: "Machine"}
	for i := int64(1); i <= 201; i++ {
		object := testObject(i, fmt.Sprintf("Object%d", i))
		trace.RecordStateEntry(runtime.TraceOrigin{At: 0, Object: object, Behavior: behavior},
			fmt.Sprintf("state%d", i), fmt.Sprintf("state%d", i), "", false)
	}

	rendering := Timeline(trace, Options{Until: 1, Limit: 200})
	total := 0
	for _, lane := range rendering.Lanes {
		total += len(lane.Spans)
	}
	if total != 200 {
		t.Fatalf("retained %d spans, want exactly 200", total)
	}
	for i := 0; i < 200; i++ {
		lane := rendering.Lanes[i]
		if lane.Name != fmt.Sprintf("#%d.Machine", i+1) || len(lane.Spans) != 1 ||
			lane.Spans[0].State != fmt.Sprintf("state%d", i+1) {
			t.Errorf("retained lane %d = %+v, want the corresponding first stable span", i, lane)
		}
	}
	if len(rendering.Notices) == 0 ||
		rendering.Notices[len(rendering.Notices)-1] != "1 later state change from t = 0 is not drawn (at most 200 spans are)" {
		t.Fatalf("cap notice = %v", rendering.Notices)
	}
}

func TestTimelineClosesTerminatedMachineAtTermination(t *testing.T) {
	trace, options, _, _ := fixtureRun(t, "termination.sysml", "mission", 10)
	rendering := Timeline(trace, options)
	var terminatedLane *view.Lane
	for i := range rendering.Lanes {
		if strings.Contains(rendering.Lanes[i].Name, "Stopper.modes") {
			terminatedLane = &rendering.Lanes[i]
			break
		}
	}
	if terminatedLane == nil {
		t.Fatalf("no stopper lane in %+v", rendering.Lanes)
	}
	if len(terminatedLane.Spans) != 1 {
		t.Fatalf("stopper spans = %+v, want one span ending at termination", terminatedLane.Spans)
	}
	span := terminatedLane.Spans[0]
	if span.From != 0 || span.To != 2 || span.Open {
		t.Errorf("stopper span = %+v, want a closed span from 0 to 2", span)
	}
	for _, mark := range terminatedLane.Marks {
		if mark.Kind == "terminate" && mark.At == 2 &&
			mark.Text == "terminated with occurrence: modes (do behavior abandoned: waiting)" {
			return
		}
	}
	t.Errorf("stopper marks = %+v, want a terminate mark at 2", terminatedLane.Marks)
}

func TestSequencePairsMessagesAndKeepsUnmatchedEndpoints(t *testing.T) {
	trace := runtime.NewEventRecorder(0)
	sender := testObject(1, "Sender")
	receiver := testObject(2, "Receiver")
	other := testObject(3, "Other")
	trace.RecordSend(runtime.TraceOrigin{At: 0, Object: sender}, runtime.Message{
		SignalType: "Ping", Payload: map[string]runtime.Value{"z": runtime.NewStringValue("last"), "a": runtime.NewStringValue("first")},
	}, receiver)
	trace.RecordSend(runtime.TraceOrigin{At: 0.5, Object: sender}, runtime.Message{SignalType: "Ping"}, other)
	trace.RecordAccept(runtime.TraceOrigin{At: 1, Object: other}, 0, "Ping", nil)
	trace.RecordAccept(runtime.TraceOrigin{At: 2, Object: receiver}, 0, "Ping", nil)
	trace.RecordSend(runtime.TraceOrigin{At: 3, Object: other}, runtime.Message{SignalType: "Notice"}, receiver)
	trace.RecordSend(runtime.TraceOrigin{At: 4}, runtime.Message{SignalType: "Broadcast"}, other)
	trace.RecordAccept(runtime.TraceOrigin{At: 5, Object: receiver}, 0, "Pong", nil)
	rendering := Sequence(trace, Options{
		Until: 6,
		Label: func(instance *runtime.Instance) string { return instance.Type.Name },
	})
	if !rendering.Run || rendering.Kind != view.KindSequence || len(rendering.Edges) != 5 {
		t.Fatalf("sequence = %+v", rendering)
	}
	wantLabels := []string{
		`t=0..2 Ping (a = "first", z = "last")`,
		"t=0.5..1 Ping",
		"t=3 Notice (not accepted)",
		"t=4 Broadcast (not accepted)",
		"t=5 Pong",
	}
	for i, want := range wantLabels {
		edge := rendering.Edges[i]
		if edge.Kind != view.EdgeFlow || edge.Label != want {
			t.Errorf("edge %d = %+v, want label %q", i, edge, want)
		}
	}
	if rendering.Edges[0].From != "n0" || rendering.Edges[0].To != "n1" ||
		rendering.Edges[1].From != "n0" || rendering.Edges[1].To != "n2" {
		t.Errorf("paired endpoints = %+v", rendering.Edges[:2])
	}
	if rendering.Edges[2].To != "n1" || rendering.Edges[3].From != "n3" ||
		rendering.Edges[3].To != "n2" || rendering.Edges[4].From != "n3" {
		t.Errorf("unmatched endpoints = %+v", rendering.Edges[2:])
	}
	if len(rendering.Roots) != 4 ||
		rendering.Roots[0].Kind != "object" || rendering.Roots[0].Name != "Sender" || rendering.Roots[0].Type != "Sender" ||
		rendering.Roots[3].Kind != "environment" || rendering.Roots[3].Name != "environment" {
		t.Errorf("participants = %+v", rendering.Roots)
	}
}

func TestSequencePairsNonzeroSerialsAndRetainsLegacyPairing(t *testing.T) {
	trace := runtime.NewEventRecorder(0)
	alpha, alphaBehavior := traceObject(1, "Alpha")
	beta, betaBehavior := traceObject(2, "Beta")
	legacy, legacyBehavior := traceObject(3, "Legacy")
	receiver, receiverBehavior := traceObject(4, "Receiver")
	trace.RecordSend(traceOrigin(0, alpha, alphaBehavior), runtime.Message{
		Serial: 11, SignalType: "Ping", Payload: map[string]runtime.Value{"seq": runtime.NewStringValue("one")},
	}, receiver)
	trace.RecordSend(traceOrigin(0.5, beta, betaBehavior), runtime.Message{
		Serial: 12, SignalType: "Ping", Payload: map[string]runtime.Value{"seq": runtime.NewStringValue("two")},
	}, receiver)
	trace.RecordSend(traceOrigin(1, legacy, legacyBehavior), runtime.Message{
		SignalType: "Ping", Payload: map[string]runtime.Value{"seq": runtime.NewStringValue("three")},
	}, receiver)
	trace.RecordAccept(traceOrigin(2, receiver, receiverBehavior), 12, "Ping", nil)
	trace.RecordAccept(traceOrigin(3, receiver, receiverBehavior), 0, "Ping", nil)

	rendering := Sequence(trace, Options{
		Until: 4,
		Label: func(instance *runtime.Instance) string { return instance.Type.Name },
	})
	if len(rendering.Edges) != 3 {
		t.Fatalf("edges = %+v, want the two sends and one serial-zero send", rendering.Edges)
	}
	if !strings.Contains(rendering.Edges[0].Label, `seq = "one"`) || !strings.Contains(rendering.Edges[0].Label, "not accepted") {
		t.Errorf("earlier same-event send = %+v, want unmatched Alpha message", rendering.Edges[0])
	}
	if !strings.Contains(rendering.Edges[1].Label, `seq = "two"`) || strings.Contains(rendering.Edges[1].Label, "not accepted") ||
		rendering.Edges[1].From != "n2" {
		t.Errorf("matching serial send = %+v, want the Beta message accepted", rendering.Edges[1])
	}
	if !strings.Contains(rendering.Edges[2].Label, `seq = "three"`) || strings.Contains(rendering.Edges[2].Label, "not accepted") ||
		rendering.Edges[2].From != "n3" {
		t.Errorf("serial-zero send = %+v, want legacy event/target pairing", rendering.Edges[2])
	}
}

func TestSequenceUsesAcceptedBetaMessageIdentityEndToEnd(t *testing.T) {
	trace, options, ctx, mission := fixtureRun(t, "serial-pairing.sysml", "mission", 2)
	labels := make(map[int64]string)
	for _, name := range []string{"alpha", "beta", "receiver"} {
		value, err := mission.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		id, ok := value.HeldValue().Object()
		if !ok {
			t.Fatalf("mission.%s does not hold an object", name)
		}
		labels[id] = "SerialPairing::mission." + name
	}
	options.Label = func(instance *runtime.Instance) string { return labels[instance.ID] }
	rendering := Sequence(trace, options)
	rootNames := make(map[string]string, len(rendering.Roots))
	for _, root := range rendering.Roots {
		rootNames[root.ID] = root.Name
	}
	alphaUnmatched, betaAccepted := false, false
	for _, edge := range rendering.Edges {
		switch {
		case strings.Contains(edge.Label, "seq = 1"):
			alphaUnmatched = rootNames[edge.From] == "SerialPairing::mission.alpha" &&
				strings.Contains(edge.Label, "not accepted")
		case strings.Contains(edge.Label, "seq = 2"):
			betaAccepted = rootNames[edge.From] == "SerialPairing::mission.beta" &&
				!strings.Contains(edge.Label, "not accepted")
		}
	}
	if !alphaUnmatched || !betaAccepted {
		t.Fatalf("sequence edges = %+v, roots = %+v; want Alpha unmatched and Beta's payload accepted",
			rendering.Edges, rendering.Roots)
	}
	artifact, err := rendering.WriteWith(view.FormText, view.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if text := string(artifact); !strings.Contains(text, "SerialPairing::mission.beta") ||
		!strings.Contains(text, "seq = 2") {
		t.Errorf("sequence diagram does not identify Beta and its payload:\n%s", text)
	}
}

func TestSequenceCapsMessagesAndReportsDroppedRecords(t *testing.T) {
	object := testObject(1, "Sender")
	trace := runtime.NewEventRecorder(0)
	for i := 0; i < 5; i++ {
		trace.RecordSend(runtime.TraceOrigin{At: float64(i), Object: object},
			runtime.Message{SignalType: "Ping"}, nil)
	}
	rendering := Sequence(trace, Options{Until: 5, Limit: 2})
	if len(rendering.Edges) != 2 {
		t.Fatalf("edges = %d, want 2", len(rendering.Edges))
	}
	notices := strings.Join(rendering.Notices, "\n")
	if !strings.Contains(notices, "3 later messages, t = 2 to 4, are not drawn (at most 2 are)") {
		t.Errorf("message cap notice = %s", notices)
	}
	equalInstants := runtime.NewEventRecorder(0)
	for i := 0; i < 4; i++ {
		equalInstants.RecordSend(runtime.TraceOrigin{At: 2, Object: object},
			runtime.Message{SignalType: "Ping"}, nil)
	}
	equal := Sequence(equalInstants, Options{Until: 3, Limit: 2})
	if got := strings.Join(equal.Notices, "\n"); !strings.Contains(got,
		"2 later messages, at t = 2, are not drawn (at most 2 are)") {
		t.Errorf("equal-instant cap notice = %s", got)
	}
	truncated := runtime.NewEventRecorder(3)
	for i := 0; i < 5; i++ {
		truncated.RecordSend(runtime.TraceOrigin{At: float64(i), Object: object},
			runtime.Message{SignalType: "Ping"}, nil)
	}
	truncatedRendering := Sequence(truncated, Options{Until: 5})
	truncatedNotices := strings.Join(truncatedRendering.Notices, "\n")
	if !strings.Contains(truncatedNotices, "2 earlier records up to t = 1 were dropped") ||
		!strings.Contains(truncatedNotices, "senders or acceptors of earlier messages may be missing") {
		t.Errorf("truncation notice = %s", truncatedNotices)
	}
}

func testObject(id int64, typeName string) *runtime.Instance {
	return &runtime.Instance{ID: id, Type: &symbols.Symbol{Name: typeName}}
}
