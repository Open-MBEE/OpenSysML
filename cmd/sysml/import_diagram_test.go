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
	for _, args := range [][]string{{"-dry-run"}, {"-view", "Plant::loopView"}, {"-import-diagram", ""}} {
		got := runStreams(t, binary, importDiagramModel, args...)
		if got.status != 2 || !strings.Contains(got.stderr, "-import-diagram") {
			t.Errorf("%v: status %d\n%s", args, got.status, got.output())
		}
	}
}

// A model reached through a symlink is edited where the link points, the link
// kept; a document the import leaves as it was is not rewritten.
func TestImportDiagramWritesThroughSymlinksAndLeavesUnchangedFilesAlone(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	real := filepath.Join(dir, "src", "plant.sysml")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte(importDiagramModel), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "plant.sysml")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	other := filepath.Join(dir, "other.sysml")
	if err := os.WriteFile(other, []byte("package Other;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(other)
	if err != nil {
		t.Fatal(err)
	}
	file := exportDrawio(t, binary, dir, link)
	got := runFiles(t, binary, []string{link, other}, "-import-diagram", file)
	if got.status != exitHolds || !strings.Contains(got.stderr, link+": written") || strings.Contains(got.stderr, other+": written") {
		t.Fatalf("import: status %d\n%s", got.status, got.output())
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("plant.sysml is no longer a symlink: %v %v", info, err)
	}
	if text, _ := os.ReadFile(real); !strings.Contains(string(text), "Layout about Plant::Loop::pump") {
		t.Errorf("linked model not edited:\n%s", text)
	}
	if after, err := os.Stat(other); err != nil || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("other.sysml rewritten though unchanged: %v %v", after, err)
	}
}

const importDiagramPortsModel = `package PumpStation {
	private import Views::*;
	private import OpenSysMLRenderings::*;
	part def Tank { port outlet; }
	part def Pump { port inlet; }
	part station {
		part tank : Tank;
		part pump : Pump;
		connect tank.outlet to pump.inlet;
	}
	view stationView {
		expose station;
		render asInterconnectionDiagram;
	}
}
`

// The export names ports as the import's baseline does, so an untouched export
// of a view with ports and a connection imports nothing and reports nothing; a
// port dragged along its owner is a notice, a re-routed connection a Route.
func TestImportDiagramPortsRoundTripSilently(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "pumps.sysml")
	if err := os.WriteFile(modelPath, []byte(importDiagramPortsModel), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "station.drawio")
	got := runFiles(t, binary, []string{modelPath}, "-render", "PumpStation::stationView", "-render-form", "drawio", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("export: exit status = %d\n%s", got.status, got.output())
	}
	exported, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exported), `fqn="PumpStation::Tank::outlet"`) {
		t.Errorf("the export does not name the port by its declaration:\n%s", exported)
	}
	untouched := runFiles(t, binary, []string{modelPath}, "-import-diagram", out, "-dry-run")
	if untouched.status != exitHolds || !strings.Contains(untouched.stderr, "nothing to import") ||
		strings.Contains(untouched.stderr, "not in the rendering") || strings.Contains(untouched.stderr, "deleted from the file") {
		t.Fatalf("untouched export: status %d\n%s", untouched.status, untouched.output())
	}

	port := regexp.MustCompile(`(?s)(<object id="[^"]*" [^>]*fqn="PumpStation::Tank::outlet"[^>]*>.*?<mxGeometry x=")([^"]*)(")`)
	if port.FindStringSubmatch(string(exported)) == nil {
		t.Fatalf("no outlet port cell with a geometry:\n%s", exported)
	}
	movedPort := filepath.Join(dir, "moved-port.drawio")
	if err := os.WriteFile(movedPort, []byte(port.ReplaceAllString(string(exported), "${1}0.9${3}")), 0o600); err != nil {
		t.Fatal(err)
	}
	moved := runFiles(t, binary, []string{modelPath}, "-import-diagram", movedPort, "-dry-run")
	if moved.status != exitHolds || !strings.Contains(moved.stderr, "drawn on its owner's border") {
		t.Errorf("moved port: status %d\n%s", moved.status, moved.output())
	}

	bend := regexp.MustCompile(`(?s)(kind="connection".*?<mxPoint x="[^"]*" y=")([^"]*)(")`)
	if bend.FindStringSubmatch(string(exported)) == nil {
		t.Fatalf("no connection waypoint:\n%s", exported)
	}
	rerouted := filepath.Join(dir, "rerouted.drawio")
	if err := os.WriteFile(rerouted, []byte(bend.ReplaceAllString(string(exported), "${1}300${3}")), 0o600); err != nil {
		t.Fatal(err)
	}
	routed := runFiles(t, binary, []string{modelPath}, "-import-diagram", rerouted, "-dry-run")
	if routed.status != exitHolds || !strings.Contains(routed.stdout, "Route about connection") || !strings.Contains(routed.stdout, "DiagramLayout::Route { points = (83, 300, 177, 101); }") {
		t.Errorf("re-routed connection: status %d\n%s", routed.status, routed.output())
	}
}
