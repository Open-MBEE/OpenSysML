package view

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestTimelineKindIsOnlyAStandaloneRunKind(t *testing.T) {
	if KindTimeline.Supported() || slices.Contains(Kinds(), KindTimeline) {
		t.Errorf("timeline is offered as a view kind: supported=%t kinds=%v", KindTimeline.Supported(), Kinds())
	}
	if _, ok := PseudoViewKind("timeline"); ok {
		t.Error("#timeline resolves to a pseudo-view")
	}
	if _, _, ok := ParsePseudoView("#timeline"); ok {
		t.Error("#timeline parses as a pseudo-view")
	}
	if KindTimeline.MachineForm() != FormMermaid {
		t.Errorf("timeline machine form = %s, want mermaid", KindTimeline.MachineForm())
	}
	for _, form := range []Form{FormText, FormMermaid, FormPlantUML} {
		if !KindTimeline.SupportsForm(form) {
			t.Errorf("timeline does not support %s", form)
		}
	}
	for _, form := range []Form{FormMarkdown, FormDot, FormCSV, FormTSV} {
		rendering := &Rendering{Kind: KindTimeline, Run: true}
		if KindTimeline.SupportsForm(form) {
			t.Errorf("timeline supports %s", form)
		}
		if _, err := rendering.Write(form); !errors.Is(err, ErrWrongForm) {
			t.Errorf("timeline in %s: error = %v, want ErrWrongForm", form, err)
		}
	}
}

func TestRunTimelineTextWritesLanesSpansTransitionsAndMarks(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, Stated: "the trace of a run to t = 4",
		Lanes: []Lane{{
			ID: "l0", Name: "T::rover.Machine",
			Spans: []Span{
				{State: "closed", From: 0, To: 1.5},
				{State: "open | parked", From: 1.5, To: 4, Triggers: []string{"accept Open"}, Through: []string{"a", "b"}, Open: true},
			},
			Transitions: []LaneTransition{{At: 1.5, From: "closed", To: "open", Event: "accept Open"}},
			Marks:       []Mark{{At: 1.5, Kind: "choice", Text: "choice entering open: parked(entry), opening(entry)"}},
		}},
		Notices: []string{"one earlier record was dropped"},
	}
	text := rendering.Text()
	for _, want := range []string{
		"run - timeline rendering (the trace of a run to t = 4)",
		"T::rover.Machine",
		"0 .. 1.5  closed",
		"1.5 .. 4  open | parked  on accept Open  (held at the end)",
		"  via a, b",
		"transitions:",
		"t=1.5 closed -> open (accept Open)",
		"noted:",
		"t=1.5 choice entering open: parked(entry), opening(entry)",
		"not represented:",
		"one earlier record was dropped",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("timeline text is missing %q:\n%s", want, text)
		}
	}
	if !rendering.Data().Run || len(rendering.Data().Lanes) != 1 {
		t.Errorf("timeline data = %+v, want its run and lane", rendering.Data())
	}
}

func TestRunTimelineMermaidWritesCompactGantt(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, Stated: "the trace of a run to t = 3600.5",
		Lanes: []Lane{{
			ID: "l0", Name: "lane: # ;",
			Spans: []Span{
				{State: "idle: #;", From: 0, To: 0.2505},
				{State: "ready", From: 0.2505, To: 3600.5, Triggers: []string{"accept Go"}, Open: true},
				{State: "instant", From: 3600.5, To: 3600.5},
			},
			Marks: []Mark{{At: 0.125, Kind: "guard", Text: "guard: x\nnot evaluated"}},
		}},
	}
	mermaid := rendering.Mermaid()
	for _, want := range []string{
		"fontFamily: \"Helvetica, Arial, sans-serif\"",
		"  gantt:\n    displayMode: compact\n",
		"%% run — timeline rendering (the trace of a run to t = 3600.5)",
		"%% t=0.125 guard: x not evaluated",
		"gantt",
		"dateFormat x",
		"axisFormat %M:%S.%L",
		"todayMarker off",
		"section lane#58; #35; #59;",
		"idle#58; #35;#59;",
		":active, l0s1, 251, 3600500",
		"1 state held for no time is listed in the text form",
		"instants are rounded to the nearest millisecond",
		"the axis reads minutes and seconds; it wraps past an hour",
		"Mermaid gantt draws no note; choice and guard records are comments",
	} {
		if !strings.Contains(mermaid, want) {
			t.Errorf("timeline Mermaid is missing %q:\n%s", want, mermaid)
		}
	}
	if !strings.Contains(mermaid, "%% t=0.125 guard: x not evaluated\ngantt") {
		t.Errorf("multiline mark escaped the Mermaid comment:\n%s", mermaid)
	}
}

func TestRunTimelinePlantUMLWritesTimingChanges(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, Stated: "the trace of a run to t = 3",
		Lanes: []Lane{{
			ID: "l0", Name: "Rover",
			Spans: []Span{
				{State: "idle", From: 0, To: 1.25},
				{State: "moving", From: 1.25, To: 3, Triggers: []string{"accept Go"}, Open: true},
			},
			Marks: []Mark{{At: 1.25, Kind: "choice", Text: "choice: moving"}},
		}},
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"@startuml\n' run — timeline rendering (the trace of a run to t = 3)",
		"scale 1 as 112 pixels",
		`concise "Rover" as l0`,
		"@0\nl0 is \"idle\"\n",
		"@1.25\nl0 is \"moving (accept Go)\"\nnote top of l0 : choice: moving\n",
		"@3\n@enduml",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("timeline PlantUML is missing %q:\n%s", want, puml)
		}
	}
	if strings.Contains(puml, "<style>") {
		t.Errorf("timing diagram contains the graph style preamble:\n%s", puml)
	}
}

func TestRunTimelineAndSequenceEmptyMessages(t *testing.T) {
	timeline := &Rendering{Kind: KindTimeline, Run: true, Stated: "the trace of a run to t = 4"}
	if !timeline.Empty() || !timeline.blank() {
		t.Errorf("empty timeline: Empty=%t blank=%t", timeline.Empty(), timeline.blank())
	}
	if got := timeline.EmptyReason(); got != "the run recorded no state; the rendering is empty" {
		t.Errorf("timeline empty reason = %q", got)
	}
	if !strings.Contains(timeline.Text(), timeline.EmptyReason()) ||
		!strings.Contains(timeline.Mermaid(), timeline.EmptyReason()) {
		t.Error("empty timeline forms do not explain why no state is shown")
	}
	puml, err := timeline.PlantUML()
	if err != nil || !strings.Contains(puml, timeline.EmptyReason()) {
		t.Errorf("empty timeline PlantUML = %q, %v", puml, err)
	}
	sequence := &Rendering{Kind: KindSequence, Run: true, Stated: "the trace of a run to t = 4"}
	if got := sequence.EmptyReason(); got != "the run recorded no message; the rendering is empty" {
		t.Errorf("sequence empty reason = %q", got)
	}
	if !strings.Contains(sequence.Text(), sequence.EmptyReason()) {
		t.Errorf("empty sequence text does not explain why:\n%s", sequence.Text())
	}
}

func TestRunSequenceHeadersDoNotChangeItsWriters(t *testing.T) {
	rendering := &Rendering{
		Kind: KindSequence, Run: true, Stated: "the trace of a run to t = 2",
		Roots: []*Node{{ID: "n0", Kind: "object", Name: "A"}, {ID: "n1", Kind: "object", Name: "B"}},
		Edges: []Edge{{From: "n0", To: "n1", Label: "t=1 Ping", Kind: EdgeFlow}},
	}
	if got := rendering.Text(); !strings.HasPrefix(got, "run - sequence rendering (the trace of a run to t = 2)") ||
		!strings.Contains(got, "A => B: t=1 Ping") {
		t.Errorf("run sequence text:\n%s", got)
	}
	if got := rendering.Mermaid(); !strings.Contains(got, "%% run — sequence rendering (the trace of a run to t = 2)") ||
		!strings.Contains(got, "n0->>n1: t=1 Ping") {
		t.Errorf("run sequence Mermaid:\n%s", got)
	}
	puml, err := rendering.PlantUML()
	if err != nil || !strings.Contains(puml, "' run — sequence rendering (the trace of a run to t = 2)") ||
		!strings.Contains(puml, "n0 -> n1 : t=1 Ping") {
		t.Errorf("run sequence PlantUML = %q, %v", puml, err)
	}
}
