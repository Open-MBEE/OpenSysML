package view

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
		Kind: KindTimeline, Run: true, RunUntil: 4, Stated: "the trace of a run to t = 4",
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
		"1.5 .. 4  open | parked (accept Open)  via a, b  (held at the end)",
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
	if !rendering.Data().Run || rendering.Data().RunUntil != 4 || len(rendering.Data().Lanes) != 1 {
		t.Errorf("timeline data = %+v, want its run and lane", rendering.Data())
	}
}

func TestRunTimelineMermaidWritesCompactGantt(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 3600.5, Stated: "the trace of a run to t = 3600.5",
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
		"  gantt:\n    displayMode: compact\n    leftPadding: 87\n",
		"%% run — timeline rendering (the trace of a run to t = 3600.5)",
		"%% t=0.125 guard: x not evaluated",
		"gantt",
		"dateFormat x",
		"axisFormat %M:%S.%L",
		"todayMarker off",
		"section lane#58; #35; #59;",
		"idle#58; #35;#59;",
		"ready (accept Go) :l0s1, 251, 3600500",
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
	if strings.Contains(mermaid, ":active") {
		t.Errorf("open timeline span uses the unstyled active tag:\n%s", mermaid)
	}
}

func TestRunTimelineMermaidLeftPaddingKeepsDefaultMinimum(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline,
		Run:  true,
		Lanes: []Lane{{
			ID: "l0", Name: "A",
		}},
	}
	if got := rendering.Mermaid(); !strings.Contains(got, "    leftPadding: 75\n") {
		t.Errorf("timeline Mermaid left padding is below the default:\n%s", got)
	}
}

func TestRunTimelinePlantUMLWritesTimingChanges(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 3, Stated: "the trace of a run to t = 3",
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
		"@3\nl0 is {hidden}\n@enduml",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("timeline PlantUML is missing %q:\n%s", want, puml)
		}
	}
	if strings.Contains(puml, "<style>") {
		t.Errorf("timing diagram contains the graph style preamble:\n%s", puml)
	}
}

func TestRunTimelinePlantUMLUsesRunUntilField(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 7, Stated: "the trace of a run to t = 4",
		Lanes: []Lane{{ID: "l0", Name: "Rover", Spans: []Span{{State: "moving", From: 1, To: 2, Open: true}}}},
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(puml, "@7\nl0 is {hidden}\n@enduml") {
		t.Errorf("PlantUML ignored RunUntil:\n%s", puml)
	}
}

func TestRunTimelinePlantUMLOmitsZeroDurationSpans(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 2,
		Lanes: []Lane{{ID: "l0", Name: "Rover", Spans: []Span{
			{State: "first", From: 0, To: 1},
			{State: "middle instant", From: 1, To: 1},
			{State: "following", From: 1, To: 2, Open: true},
			{State: "ending instant", From: 2, To: 2},
		}}},
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"' not represented: 2 states held for no time are listed in the text form",
		"@0\nl0 is \"first\"\n",
		"@1\nl0 is \"following\"\n",
		"@2\nl0 is {hidden}\n",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("timeline PlantUML is missing %q:\n%s", want, puml)
		}
	}
	if strings.Contains(puml, "middle instant") || strings.Contains(puml, "ending instant") {
		t.Errorf("timeline PlantUML rendered zero-duration state labels:\n%s", puml)
	}
}

func TestRunTimelinePlantUMLClosesBeforeTrailingZeroDurationSpan(t *testing.T) {
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 3,
		Lanes: []Lane{{ID: "l0", Name: "Rover", Spans: []Span{
			{State: "stopped", From: 0, To: 1},
			{State: "instant", From: 1, To: 1},
		}}},
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	for _, want := range []string{
		"@1\nl0 is {hidden}\n",
		"@3\nl0 is {hidden}\n",
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("timeline PlantUML is missing %q:\n%s", want, puml)
		}
	}
}

func TestRunTimelinePlantUMLScaleFitsLabelsAndCapsWidth(t *testing.T) {
	span := Span{
		State: "acknowledged | charged", From: 0, To: 1,
		Triggers: []string{"time", "accept Ack"},
	}
	lanes := []Lane{{Spans: []Span{span}}}
	scale := timelineScale(lanes, 6)
	labelWidth := max(140, len([]rune(timelineStateLabel(span)))*7+24)
	if scale < float64(labelWidth) {
		t.Errorf("scale = %g pixels/s, want at least %d pixels for the label", scale, labelWidth)
	}
	if width := scale * 6; width > 4000 {
		t.Errorf("timeline width = %g pixels, want at most 4000", width)
	}
	longScale := timelineScale(lanes, 10_000)
	if width := longScale * 10_000; width > 4000 {
		t.Errorf("long timeline width = %g pixels, want at most 4000", width)
	}
}

func TestRunTimelinePlantUMLLinksLanesAndSingleStateSpans(t *testing.T) {
	laneOrigin := Origin{Doc: "run.sysml", Span: source.Span{Offset: 1, Len: 1}}
	spanOrigin := Origin{Doc: "run.sysml", Span: source.Span{Offset: 2, Len: 1}}
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 3,
		Lanes: []Lane{{
			ID: "l0", Name: "Machine", Origin: laneOrigin,
			Spans: []Span{
				{State: "ready", Origin: spanOrigin, From: 0, To: 1},
				{State: "left.waiting | right.waiting", From: 1, To: 3, Open: true},
			},
		}},
	}
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: func(origin Origin) (Site, bool) {
			return Site{File: origin.Doc, Line: origin.Span.Offset}, true
		},
	}
	got, err := rendering.WriteWith(FormPlantUML, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`concise "[[https://example.test/src/run.sysml#L1 Machine]]" as l0`,
		`l0 is "[[https://example.test/src/run.sysml#L2 ready]]"`,
		`l0 is "left.waiting | right.waiting"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("run timeline PlantUML is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `[[https://example.test/src/run.sysml#L0 left.waiting`) {
		t.Errorf("multi-state span was linked:\n%s", got)
	}
}

func TestRunTimelinePlantUMLEscapesLinkedLabels(t *testing.T) {
	origin := Origin{Doc: "run.sysml", Span: source.Span{Offset: 1, Len: 1}}
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 1,
		Lanes: []Lane{{
			ID: "l0", Name: `machine # "quotes]`, Origin: origin,
			Spans: []Span{{State: `ready # "quotes]`, Origin: origin, From: 0, To: 1, Open: true}},
		}},
	}
	links := Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: func(origin Origin) (Site, bool) {
			return Site{File: origin.Doc, Line: 9}, true
		},
	}
	got, err := rendering.WriteWith(FormPlantUML, Options{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`concise "[[https://example.test/src/run.sysml#L9 machine <U+0023> <U+0022>quotes<U+005D>]]" as l0`,
		`l0 is "[[https://example.test/src/run.sysml#L9 ready <U+0023> <U+0022>quotes<U+005D>]]"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PlantUML label is missing safe link markup %q:\n%s", want, got)
		}
	}
}

func TestRunTimelinePlantUMLLeavesUnsafeLabelsUnlinked(t *testing.T) {
	origin := Origin{Doc: "run.sysml", Span: source.Span{Offset: 1, Len: 1}}
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 1,
		Lanes: []Lane{{ID: "l0", Name: "machine\tname", Origin: origin}},
	}
	got, err := rendering.WriteWith(FormPlantUML, Options{Links: Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: func(origin Origin) (Site, bool) {
			return Site{File: origin.Doc, Line: 1}, true
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "[[") || !strings.Contains(got, "concise \"machine\tname\" as l0") {
		t.Errorf("unsafe lane label should remain readable and unlinked:\n%s", got)
	}
}

func TestRunTimelineUnlinkedFormsIgnoreLinks(t *testing.T) {
	origin := Origin{Doc: "run.sysml", Span: source.Span{Offset: 1, Len: 1}}
	rendering := &Rendering{
		Kind: KindTimeline, Run: true, RunUntil: 1,
		Lanes: []Lane{{ID: "l0", Name: "Machine", Origin: origin, Spans: []Span{
			{State: "ready", Origin: origin, From: 0, To: 1, Open: true},
		}}},
	}
	options := Options{Links: Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: func(origin Origin) (Site, bool) {
			return Site{File: origin.Doc, Line: 1}, true
		},
	}}
	for _, form := range []Form{FormMermaid, FormText} {
		plain, err := rendering.WriteWith(form, Options{})
		if err != nil {
			t.Fatal(err)
		}
		linked, err := rendering.WriteWith(form, options)
		if err != nil {
			t.Fatal(err)
		}
		if linked != plain {
			t.Errorf("linked %s timeline differs from unlinked output:\nplain:\n%s\nlinked:\n%s", form, plain, linked)
		}
	}
}

func TestRunTimelineAndSequenceEmptyMessages(t *testing.T) {
	timeline := &Rendering{Kind: KindTimeline, Run: true, RunUntil: 4, Stated: "the trace of a run to t = 4"}
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
	sequence := &Rendering{Kind: KindSequence, Run: true, RunUntil: 4, Stated: "the trace of a run to t = 4"}
	if got := sequence.EmptyReason(); got != "the run recorded no message; the rendering is empty" {
		t.Errorf("sequence empty reason = %q", got)
	}
	if !strings.Contains(sequence.Text(), sequence.EmptyReason()) {
		t.Errorf("empty sequence text does not explain why:\n%s", sequence.Text())
	}
}

func TestRunSequenceHeadersDoNotChangeItsWriters(t *testing.T) {
	rendering := &Rendering{
		Kind: KindSequence, Run: true, RunUntil: 2, Stated: "the trace of a run to t = 2",
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

func TestRunSequenceParticipantLabelsPreserveSessionPaths(t *testing.T) {
	rendering := &Rendering{
		Kind: KindSequence,
		Run:  true,
		Roots: []*Node{
			{ID: "n0", Kind: "object", Name: "RunTimeline::mission.controller", Type: "Controller"},
			{ID: "n1", Kind: "object", Name: "RunTimeline::mission.instrument", Type: "Instrument"},
		},
	}
	mermaid := rendering.Mermaid()
	for _, want := range []string{
		"participant n0 as RunTimeline::mission.controller : Controller",
		"participant n1 as RunTimeline::mission.instrument : Instrument",
	} {
		if !strings.Contains(mermaid, want) {
			t.Errorf("run Mermaid is missing %q:\n%s", want, mermaid)
		}
	}
	puml, err := rendering.PlantUML()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`participant "RunTimeline::mission.controller : Controller" as n0`,
		`participant "RunTimeline::mission.instrument : Instrument" as n1`,
	} {
		if !strings.Contains(puml, want) {
			t.Errorf("run PlantUML is missing %q:\n%s", want, puml)
		}
	}

	model := *rendering
	model.Run = false
	if got, want := model.Mermaid(), "  participant n0 as «object»<br>'mission.controller' : Controller"; !strings.Contains(got, want) {
		t.Errorf("model Mermaid participant changed: want %q in\n%s", want, got)
	}
	modelPUML, err := model.PlantUML()
	if err != nil {
		t.Fatal(err)
	}
	if want := `**'mission.controller' : Controller**`; !strings.Contains(modelPUML, want) {
		t.Errorf("model PlantUML participant changed: want %q in\n%s", want, modelPUML)
	}
}
