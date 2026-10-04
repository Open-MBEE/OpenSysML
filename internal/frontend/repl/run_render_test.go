package repl

import (
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

func TestRenderRunCompletesKindAndForm(t *testing.T) {
	s := NewSession()
	if got := s.Complete("%render-run ti", len("%render-run ti")); !slices.Equal(got.Candidates, []string{"timeline"}) {
		t.Errorf("completing run kind offered %v", got.Candidates)
	}
	if got := s.Complete("%render-run timeline p", len("%render-run timeline p")); !slices.Equal(got.Candidates, []string{"plantuml"}) {
		t.Errorf("completing run form offered %v", got.Candidates)
	}
	wants(t, strings.Join(helpText(), "\n"), "%render-run <timeline|sequence> [form]")
}
