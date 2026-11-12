package repl

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRenderRunBuildsTimelineAndSequence(t *testing.T) {
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	wants(t, run(t, s, "%trace on"), "trace: on")
	wants(t, run(t, s, "%instantiate bulb"), "Created instance")
	wants(t, run(t, s, "%state bulb"), "Current state: off")
	run(t, s, "%send go")
	run(t, s, "%advance 1")

	timeline := run(t, s, "%render-run timeline")
	wants(t, timeline, "run - timeline rendering", "Lamps::bulb.lamp", "on")
	sequence := run(t, s, "%render-run sequence mermaid")
	wants(t, sequence, "sequenceDiagram", "go")
	wants(t, run(t, s, "%render-run timeline mermaid"), "gantt", "on (accept go)")
	wants(t, run(t, s, "%render-run sequence text"), "run - sequence rendering", "go")
}

func TestRenderRunUsesHeldObjectPathLabels(t *testing.T) {
	s := loadFixture(t, "testdata/run_render_nested.sysml")
	run(t, s, "%trace on")
	run(t, s, "%instantiate mission")
	wants(t, run(t, s, "%state NestedRun::Controller::modes NestedRun::mission.controller"), "Debugging state machine", "NestedRun::mission.controller")
	wants(t, run(t, s, "%advance 1"), "1.0")

	wants(t, run(t, s, "%render-run timeline"), "NestedRun::mission.controller.modes")
}

func TestRenderRunLinksSourceDeclarations(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", "run-timeline", "run-timeline.sysml"))
	if err != nil {
		t.Fatalf("resolve example path: %v", err)
	}
	s := NewSession()
	run(t, s, fmt.Sprintf("%%load %q", path))
	run(t, s, "%trace on")
	run(t, s, "%instantiate RunTimeline::mission")
	verdicts := s.RunFor(nil, []Behavior{
		{Name: "RunTimeline::Controller::modes", Performer: []string{"RunTimeline::mission.controller"}},
		{Name: "RunTimeline::Instrument::modes", Performer: []string{"RunTimeline::mission.instrument"}},
	}, 6)
	if len(verdicts) != 2 {
		t.Fatalf("behavior verdicts = %v, want both machine runs", verdicts)
	}
	for _, verdict := range verdicts {
		if verdict.Status != VerdictHolds {
			t.Fatalf("run verdict = %+v, want holds", verdict)
		}
	}

	template := "https://example.test/src/{file}#L{line}"
	sequence := run(t, s, "%render-run sequence mermaid link="+template)
	wants(t, sequence, "link n0: Source @ https://example.test/src/", "RunTimeline::mission.controller")
	timeline := run(t, s, "%render-run timeline plantuml link="+template)
	wants(t, timeline, `[[https://example.test/src/`, "#L", "RunTimeline::mission.controller.modes")
	wants(t, run(t, s, "%render-run timeline link=https://example.test/{missing}"), "unknown link template placeholder {missing}")
}

func TestRenderRunReportsMissingTraceAndRefusesDot(t *testing.T) {
	fresh := NewSession()
	wants(t, run(t, fresh, "%render-run timeline"), "error: the session records no trace; %trace on or %trace record before the run")
	wants(t, run(t, fresh, "%render-run"), renderRunUsage)
	wants(t, run(t, fresh, "%render-run neither"), `unknown run rendering "neither"`, "timeline or sequence")
	wants(t, run(t, fresh, "%render-run timeline png"), `unknown form "png"`)

	s := loadFixture(t, "testdata/lamp_signals.sysml")
	run(t, s, "%trace on")
	run(t, s, "%instantiate bulb")
	run(t, s, "%state bulb")
	run(t, s, "%advance 1")
	wants(t, run(t, s, "%render-run timeline dot"), "error:", "not written as dot")
}

func TestRenderRunWithoutRuntimeContext(t *testing.T) {
	examplePath, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", "run-timeline", "run-timeline.sysml"))
	if err != nil {
		t.Fatalf("resolve example path: %v", err)
	}
	s := NewSession()
	run(t, s, fmt.Sprintf("%%load %q", examplePath))
	run(t, s, "%trace on")
	if s.rtCtx != nil {
		t.Fatal("loading a model and enabling trace created a runtime context")
	}

	wants(t, run(t, s, "%render-run timeline"), "the run recorded no state; the rendering is empty")
	wants(t, run(t, s, "%render-run sequence"), "the run recorded no message; the rendering is empty")

	run(t, s, "%trace off")
	wants(t, run(t, s, "%render-run timeline"), "error: the session records no trace; %trace on or %trace record before the run")
}

func TestRenderRunCompletesKindAndForm(t *testing.T) {
	s := NewSession()
	if got := s.Complete("%render-run ti", len("%render-run ti")); !slices.Equal(got.Candidates, []string{"timeline"}) {
		t.Errorf("completing run kind offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline p", len("%render-run timeline p")); !slices.Equal(got.Candidates, []string{"plantuml"}) {
		t.Errorf("completing run form offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline l", len("%render-run timeline l")); !slices.Equal(got.Candidates, []string{"limit=", "link="}) {
		t.Errorf("completing run link offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline plantuml l", len("%render-run timeline plantuml l")); !slices.Equal(got.Candidates, []string{"limit=", "link="}) {
		t.Errorf("completing run link after form offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run sequence link=x m", len("%render-run sequence link=x m")); !slices.Equal(got.Candidates, []string{"mermaid"}) {
		t.Errorf("completing run form after link offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run sequence link=x limit=3 ", len("%render-run sequence link=x limit=3 ")); !slices.Equal(got.Candidates, []string{"dot", "mermaid", "plantuml", "svg", "svg-animated", "text"}) {
		t.Errorf("completing run form after link and limit offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline plantuml link=x ", len("%render-run timeline plantuml link=x ")); !slices.Equal(got.Candidates, []string{"limit="}) {
		t.Errorf("completion after form and link offered %v, want limit=", got.Candidates)
	}
	if got := s.Complete("%render-run timeline plantuml link=x limit=3 ", len("%render-run timeline plantuml link=x limit=3 ")); len(got.Candidates) != 0 {
		t.Errorf("completion after every run-render argument offered %v, want no candidates", got.Candidates)
	}
	wants(t, strings.Join(helpText(), "\n"), "%render-run <timeline|sequence> [form] [link=<template>] [limit=<count>]")
}

func TestRenderRunLimitBoundsTheDrawing(t *testing.T) {
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	run(t, s, "%trace on")
	run(t, s, "%instantiate bulb")
	wants(t, run(t, s, "%state bulb"), "Current state: off")
	run(t, s, "%advance 1")
	run(t, s, "%send go")
	run(t, s, "%advance 1")

	limited := run(t, s, "%render-run timeline limit=1")
	wants(t, limited, "run - timeline rendering", "0 .. 1  off", "1 later state change from t = 1 is not drawn (at most 1 spans are)")
	rejects(t, limited, "1 .. 2  on")
	full := run(t, s, "%render-run timeline limit=2")
	rejects(t, full, "not drawn")
	wants(t, full, "1 .. 2  on")
	wants(t, run(t, s, "%render-run sequence mermaid limit=5 link=https://example.test/{file}"), "sequenceDiagram", "go")
	wants(t, run(t, s, "%render-run timeline limit=0"), `limit must be a positive count, not "0"`)
	wants(t, run(t, s, "%render-run timeline limit=many"), `limit must be a positive count, not "many"`)
	wants(t, run(t, s, "%render-run timeline limit=1 limit=2"), "usage: %render-run")
	wants(t, run(t, s, "%render-run timeline text limit=1 link=x extra"), "usage: %render-run")
}

func TestTraceRecordKeepsTheRunWithoutPrintingIt(t *testing.T) {
	s := loadFixture(t, "testdata/lamp_signals.sysml")
	wants(t, run(t, s, "%render-run timeline"), "records no trace; %trace on or %trace record before the run")
	wants(t, run(t, s, "%trace record"), "trace: record")
	wants(t, run(t, s, "%trace"), "trace: record")
	if s.Tracing() {
		t.Error("recording reports as tracing")
	}
	run(t, s, "%instantiate bulb")
	wants(t, run(t, s, "%state bulb"), "Current state: off")
	run(t, s, "%send go")
	advanced := run(t, s, "%advance 1")
	rejects(t, advanced, "[trace]")
	wants(t, advanced, "Current state: on")
	wants(t, run(t, s, "%render-run timeline"), "run - timeline rendering", "off", "on")
	wants(t, run(t, s, "%render-run sequence"), "run - sequence rendering", "go")

	// Switching to on keeps the run recorded and prints only what follows.
	wants(t, run(t, s, "%trace on"), "trace: on")
	wants(t, run(t, s, "%instantiate plain"), "[trace]", "Created instance")
	wants(t, run(t, s, "%render-run sequence"), "go")
	// And back to record: nothing recorded in either mode is printed later.
	wants(t, run(t, s, "%trace record"), "trace: record")
	rejects(t, run(t, s, "%instantiate plain"), "[trace]")
	rejects(t, run(t, s, "%advance 1"), "[trace]")
	wants(t, run(t, s, "%render-run sequence"), "go")
	wants(t, run(t, s, "%trace off"), "trace: off")
	wants(t, run(t, s, "%trace maybe"), `unknown trace setting "maybe" (want on, off or record)`)
}

func TestTraceRecordKeepsBatchRunsSilent(t *testing.T) {
	s := runsSession(t)
	run(t, s, "%trace on")
	wants(t, run(t, s, "%runs 1 7 MC::acquire clock"), "[trace]", "runs MC::acquire")
	wants(t, run(t, s, "%trace record"), "trace: record")
	silent := run(t, s, "%runs 1 7 MC::acquire clock")
	rejects(t, silent, "[trace]")
	wants(t, silent, "runs MC::acquire")
}
