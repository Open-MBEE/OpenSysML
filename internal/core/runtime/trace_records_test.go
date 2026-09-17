package runtime

import (
	"strings"
	"testing"
)

// A run's records name the object and behavior that made them; a message posted
// from outside the run names neither.
func TestTraceRecordsCarryTheBehaviorThatMadeThem(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		attribute def Ping;
		attribute def Report;
		part def Sink;
		state def Relay {
			entry; then idle;
			state idle;
			transition first idle accept Ping then told;
			state told { entry send new Report() to sink; }
		}
		part def Rig {
			attribute mark : Integer = 0;
			part sink : Sink;
			part relay : Sink { exhibit state r : Relay; }
			perform action marking {
				first start;
				then decide route;
					if mark == 0 then split;
					if 1 / mark > 0 then split;
				fork split;
				action low { assign mark := 1; }
				action high { assign mark := 2; }
				join sync;
				action report send new Report() to sink;
				done;
				succession first split then low;
				succession first split then high;
				succession first low then sync;
				succession first high then sync;
				succession first sync then report;
				succession first report then done;
			}
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	rig, err := ctx.Instantiate(oneSymbol(t, idx, "test::Rig"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	relayID, ok := rig.FeatureValues["relay"].Value.Object()
	if !ok {
		t.Fatalf("relay = %v, want an object", rig.FeatureValues["relay"].Value)
	}
	relay, _ := ctx.Instance(relayID)
	ping, err := ctx.SignalMessage(oneSymbol(t, idx, "test::Ping"), nil, relay)
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	ctx.PostMessage(ping)
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance: %v", err)
	}

	marking := oneSymbol(t, idx, "test::Rig::marking")
	relayMachine := oneSymbol(t, idx, "test::Relay")
	var got []string
	for _, r := range trace.Records() {
		switch r.Kind {
		case TraceSend, TraceChoice, TraceGuard:
		default:
			continue
		}
		var origin strings.Builder
		switch {
		case r.Origin.Object == nil && r.Origin.Behavior == nil:
			origin.WriteString("outside")
		case r.Origin.Object == rig && r.Origin.Behavior == marking:
			origin.WriteString("rig/marking")
		case r.Origin.Object == relay && r.Origin.Behavior == relayMachine:
			origin.WriteString("relay/Relay")
		default:
			origin.WriteString("?")
		}
		name := r.Event
		switch n := r.Note.(type) {
		case ChoicePoint:
			name = n.Kind.String()
		case UnevaluableGuard:
			name = "guard " + n.Alternative
		}
		got = append(got, r.Kind.String()+" "+name+" from "+origin.String())
	}
	want := []string{
		"guard guard 2->split from rig/marking",
		"choice write order from rig/marking",
		"choice token order from rig/marking",
		"send Report from rig/marking",
		"send Ping from outside",
		"send Report from relay/Relay",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("origins:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An event recorder keeps the most recent limit records, no printed lines, and
// places a late accept by its whole-run mark.
func TestEventRecorderKeepsTheMostRecentRecords(t *testing.T) {
	tr := NewEventRecorder(3)
	at := func(t float64, object *Instance) TraceOrigin { return TraceOrigin{At: t, Object: object} }
	tr.line("printed only")
	tr.RecordStateEntry(at(0, nil), "a", false)
	tr.RecordStateEntry(at(1, nil), "b", false)
	if dropped, _ := tr.Dropped(); dropped != 0 || len(tr.Records()) != 2 {
		t.Fatalf("records = %d dropped = %d, want the two entries and no line", len(tr.Records()), dropped)
	}
	tr.Clear()
	mark := tr.Mark()
	tr.RecordStateEntry(at(2, nil), "c", false)
	tr.RecordStateEntry(at(3, nil), "d", false)
	tr.RecordAcceptAt(mark, at(2, nil), "Go", nil)
	states := func() string {
		var out []string
		for _, r := range tr.Records() {
			out = append(out, r.Kind.String()+":"+r.State+r.Event)
		}
		return strings.Join(out, " ")
	}
	if got := states(); got != "accept:Go entry:c entry:d" {
		t.Fatalf("records = %s, want the accept placed at its mark and the oldest two dropped", got)
	}
	if dropped, upTo := tr.Dropped(); dropped != 2 || upTo != 1 {
		t.Fatalf("dropped = %d up to %v, want 2 up to t = 1", dropped, upTo)
	}
	if entries := tr.Entries(); strings.Join(entries, ";") != "enter: c;enter: d" {
		t.Fatalf("entries since Clear = %q, want the two printed lines made after it", entries)
	}
	if got := NewEventRecorder(0); got.limit != 0 {
		t.Fatalf("limit 0 = %d, want unbounded", got.limit)
	}
}

// A capture of a bounded recorder restores what it kept and what it had dropped:
// records trimmed after the mark come back, and the horizon rewinds with them.
func TestEventRecorderCaptureRestoresTruncation(t *testing.T) {
	tr := NewEventRecorder(3)
	at := func(t float64) TraceOrigin { return TraceOrigin{At: t} }
	for i, s := range []string{"a", "b", "c", "d"} {
		tr.RecordStateEntry(at(float64(i)), s, false)
	}
	states := func() string {
		var out []string
		for _, r := range tr.Records() {
			out = append(out, r.State)
		}
		return strings.Join(out, " ")
	}
	capture := captureTrace(tr)
	tr.RecordStateEntry(at(4), "e", false)
	tr.RecordStateEntry(at(5), "f", false)
	if dropped, upTo := tr.Dropped(); states() != "d e f" || dropped != 3 || upTo != 2 {
		t.Fatalf("after the mark: %s dropped %d up to %v", states(), dropped, upTo)
	}
	capture.restore(tr)
	if dropped, upTo := tr.Dropped(); states() != "b c d" || dropped != 1 || upTo != 0 {
		t.Fatalf("restored: %s dropped %d up to %v, want b c d with one dropped up to t = 0", states(), dropped, upTo)
	}
	if tr.Mark() != 4 {
		t.Fatalf("mark after restore = %d, want 4", tr.Mark())
	}
}
