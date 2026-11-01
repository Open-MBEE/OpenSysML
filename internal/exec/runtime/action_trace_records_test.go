package runtime

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// An action's run leaves typed records of each token step, wait, accept, decision,
// assignment and its completion, none of which the printed trace shows.
func TestActionTraceRecordsFollowTheTokens(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		item def Go;
		part def Craft;
		part craft : Craft;
		action def Flow {
			in budget : Integer;
			attribute remaining : Integer = budget;
			out landed : Boolean = false;
			first start;
			then action burn assign remaining := remaining - 3;
			then decide route;
				if remaining >= 5 then waiting;
				else finish;
			action waiting accept go : Go;
			then fork split;
			action land assign landed := true;
			action survey;
			join meet;
			action finish;
			then done;
			succession first split then land;
			succession first split then survey;
			succession first land then meet;
			succession first survey then meet;
			succession first meet then finish;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	craft, err := ctx.Instantiate(oneSymbol(t, idx, "test::craft"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	flow := oneSymbol(t, idx, "test::Flow")
	exec, _, err := ctx.StartActionPerformedBy(flow, craft, map[string]Value{"budget": {Kind: ValConst, Const: semantics.IntValue(10)}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	goSignal, err := ctx.SignalMessage(oneSymbol(t, idx, "test::Go"), nil, craft)
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	if err := ctx.DeliverEvents(exec, []Message{goSignal}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	var got []string
	for _, r := range trace.Records() {
		switch r.Kind {
		case TraceStep, TraceWait, TraceAccept, TraceDecide, TraceAssign, TraceComplete:
		default:
			continue
		}
		if r.Origin.Object != craft || r.Origin.Behavior != flow {
			t.Errorf("%s record made by %v/%v, want craft/Flow", r.Kind, r.Origin.Object, r.Origin.Behavior)
		}
		if r.Kind != TraceComplete && r.Token == 0 {
			t.Errorf("%s record names no token: %+v", r.Kind, r)
		}
		got = append(got, r.Text())
	}
	// The two branches of the fork step in the order the schedule picks, so the
	// records between the fork and the join are compared as a set.
	head := []string{
		"step start",
		"step start -> burn",
		"assign remaining = 7",
		"step burn -> route",
		"decide route -> 1->waiting",
		"step route -> waiting",
		"wait Go",
		"accept Go",
		"step waiting -> split",
		"step split -> land",
		"step split -> survey",
	}
	branches := []string{"assign landed = [true]", "step land -> meet", "step survey -> meet"}
	tail := []string{"step meet", "step meet -> finish", "step finish -> done", "complete"}
	if len(got) != len(head)+len(branches)+len(tail) {
		t.Fatalf("records:\n%s", strings.Join(got, "\n"))
	}
	middle := append([]string(nil), got[len(head):len(head)+len(branches)]...)
	sort.Strings(middle)
	sort.Strings(branches)
	if strings.Join(got[:len(head)], "\n") != strings.Join(head, "\n") ||
		strings.Join(middle, "\n") != strings.Join(branches, "\n") ||
		strings.Join(got[len(got)-len(tail):], "\n") != strings.Join(tail, "\n") {
		t.Fatalf("records:\n%s\nwant %v, then %v in any order, then %v", strings.Join(got, "\n"), head, branches, tail)
	}
	for _, line := range trace.Entries() {
		for _, kind := range []string{"step start", "wait ", "assign remaining =", "decide ", "complete"} {
			if strings.HasPrefix(line, kind) {
				t.Errorf("printed trace shows the typed record %q", line)
			}
		}
	}
}

// Each new kind reads as its query name and prints no line.
func TestActionTraceKindsPrintNoLine(t *testing.T) {
	for kind, name := range map[TraceKind]string{
		TraceStep: "step", TraceWait: "wait", TraceAssign: "assign", TraceDecide: "decide", TraceComplete: "complete",
	} {
		if kind.String() != name {
			t.Errorf("%d.String() = %q, want %q", kind, kind.String(), name)
		}
		if line, printed := (TraceRecord{Kind: kind}).Line(); printed || line != "" {
			t.Errorf("%s prints %q", name, line)
		}
	}
	record := TraceRecord{Kind: TraceAssign, Feature: "remaining", Value: Value{Kind: ValConst, Const: semantics.IntValue(7)}}
	if got := record.Text(); got != "assign remaining = 7" {
		t.Errorf("assign text = %q", got)
	}
	record = TraceRecord{Kind: TraceDecide, State: "route", To: "1->waiting", Alternatives: []string{"1->waiting", "2->finish"}}
	if got := record.Text(); got != "decide route -> 1->waiting" {
		t.Errorf("decide text = %q", got)
	}
}

// A qualified target under a shadowing local is recorded with the value written to it,
// not with the local's.
func TestActionTraceQualifiedAssignUnderShadowingLocal(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Craft;
		part craft : Craft;
		action def Act {
			attribute a : Integer := 1;
			out attribute r : Integer;
			if true {
				attribute a : Integer := 9;
				assign r := Act::a;
				assign Act::a := 4;
			}
		}
		action def Shaped {
			attribute a : Integer[1..2] := 1;
			if true {
				attribute a : Integer := 9;
				assign Shaped::a := 4;
			}
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	craft, err := ctx.Instantiate(oneSymbol(t, idx, "test::craft"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	outputs, _, err := ctx.ExecuteActionWithEvents(oneSymbol(t, idx, "test::Act"), craft, map[string]Value{}, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := FormatValue(outputs["r"]); got != "1" && got != "[1]" {
		t.Errorf("r = %s, want Act::a's 1", got)
	}
	var assigns []string
	for _, r := range trace.Records() {
		if r.Kind == TraceAssign {
			assigns = append(assigns, r.Text())
		}
	}
	joined := strings.Join(assigns, "\n")
	if !strings.Contains(joined, "assign a = 4") || strings.Contains(joined, "assign a = 9") {
		t.Errorf("assign records:\n%s\nwant Act::a recorded as 4, never as the local's 9", joined)
	}
	trace = NewTraceRecorder()
	ctx.SetTrace(trace)
	if _, _, err := ctx.ExecuteActionWithEvents(oneSymbol(t, idx, "test::Shaped"), craft, map[string]Value{}, nil); err != nil {
		t.Fatalf("run Shaped: %v", err)
	}
	assigns = assigns[:0]
	for _, r := range trace.Records() {
		if r.Kind == TraceAssign {
			assigns = append(assigns, r.Text())
		}
	}
	if joined := strings.Join(assigns, "\n"); joined != "assign a = [4]" {
		t.Errorf("assign records:\n%s\nwant Shaped::a recorded as its held [4]", joined)
	}
}
