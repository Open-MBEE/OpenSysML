package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const importDiagramModel = `package Plant {
	private import Views::*;
	private import OpenSysMLRenderings::*;
	part def Pump;
	part def Loop {
		part pump : Pump;
		part spare : Pump;
	}
	view loopView {
		expose Loop;
		render asInterconnectionDiagram;
	}
}
`

// exportDrawio renders the model's view as a draw.io file beside the model and
// moves the pump down in it, as a user dragging the cell would.
func exportDrawio(t *testing.T, binary, dir, modelPath string) string {
	t.Helper()
	out := filepath.Join(dir, "loop.drawio")
	got := runFiles(t, binary, []string{modelPath}, "-render", "Plant::loopView", "-render-form", "drawio", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("export: exit status = %d\n%s", got.status, got.output())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	cell := regexp.MustCompile(`(?s)(<object id="[^"]*" [^>]*fqn="Plant::Loop::pump"[^>]*>.*?<mxGeometry x="[^"]*" y=")([^"]*)(")`)
	m := cell.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatalf("no pump cell with a geometry in the export:\n%s", data)
	}
	y, _ := strconv.ParseFloat(m[2], 64)
	moved := cell.ReplaceAllString(string(data), "${1}"+strconv.FormatFloat(y+70, 'f', -1, 64)+"${3}")
	if err := os.WriteFile(out, []byte(moved), 0o600); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestImportDiagramDryRunPrintsTheDiffAndWritesNothing(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "plant.sysml")
	if err := os.WriteFile(modelPath, []byte(importDiagramModel), 0o600); err != nil {
		t.Fatal(err)
	}
	file := exportDrawio(t, binary, dir, modelPath)

	got := runFiles(t, binary, []string{modelPath}, "-import-diagram", file, "-dry-run")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"Layout about Plant::Loop::pump", "--- a/" + modelPath, "@@", "+\t\tmetadata DiagramLayout::Layout about"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	if !strings.Contains(got.stderr, "edits planned, nothing written") {
		t.Errorf("stderr does not say it was a dry run:\n%s", got.stderr)
	}
	if after, _ := os.ReadFile(modelPath); string(after) != importDiagramModel {
		t.Errorf("the dry run changed the model:\n%s", after)
	}
}

func TestImportDiagramWritesTheLayoutIntoTheModel(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "plant.sysml")
	if err := os.WriteFile(modelPath, []byte(importDiagramModel), 0o600); err != nil {
		t.Fatal(err)
	}
	file := exportDrawio(t, binary, dir, modelPath)

	got := runFiles(t, binary, []string{modelPath}, "-import-diagram", file)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stderr, modelPath+": written") {
		t.Errorf("stderr does not say the model was written:\n%s", got.stderr)
	}
	after, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "metadata DiagramLayout::Layout about") || !strings.Contains(string(after), "pump") {
		t.Errorf("the model has no Layout about the pump:\n%s", after)
	}
	// The written model reproduces the edited file: importing it again plans nothing.
	again := runFiles(t, binary, []string{modelPath}, "-import-diagram", file, "-dry-run")
	if again.status != exitHolds || !strings.Contains(again.stderr, "nothing to import") {
		t.Errorf("second import: status %d\n%s", again.status, again.output())
	}
}

func TestImportDiagramFlagsNeedTheFile(t *testing.T) {
	binary := buildCLI(t)
	for _, args := range [][]string{{"-dry-run"}, {"-view", "Plant::loopView"}} {
		got := runStreams(t, binary, importDiagramModel, args...)
		if got.status != 2 || !strings.Contains(got.stderr, "-import-diagram") {
			t.Errorf("%v: status %d\n%s", args, got.status, got.output())
		}
	}
}
