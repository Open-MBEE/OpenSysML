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
	wants(t, run(t, fresh, "%render-run timeline"), "error: the session records no trace; %trace on before the run")
	wants(t, run(t, fresh, "%render-run"), renderRunUsage)
	wants(t, run(t, fresh, "%render-run neither"), `unknown run rendering "neither"`, "timeline or sequence")
	wants(t, run(t, fresh, "%render-run timeline svg"), `unknown form "svg"`)

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
	wants(t, run(t, s, "%render-run timeline"), "error: the session records no trace; %trace on before the run")
}

func TestRenderRunCompletesKindAndForm(t *testing.T) {
	s := NewSession()
	if got := s.Complete("%render-run ti", len("%render-run ti")); !slices.Equal(got.Candidates, []string{"timeline"}) {
		t.Errorf("completing run kind offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline p", len("%render-run timeline p")); !slices.Equal(got.Candidates, []string{"plantuml"}) {
		t.Errorf("completing run form offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline l", len("%render-run timeline l")); !slices.Equal(got.Candidates, []string{"link="}) {
		t.Errorf("completing run link offered %v", got.Candidates)
	}
	wants(t, strings.Join(helpText(), "\n"), "%render-run <timeline|sequence> [form] [link=<template>]")
}
