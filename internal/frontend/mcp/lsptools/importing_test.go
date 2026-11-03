package lsptools

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
)

const plant = `package Plant {
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

func TestImportDiagramRoundTripsARenderedView(t *testing.T) {
	f := newFixture(t)
	write(t, f.root, "plant.sysml", plant)
	var render lsp.RenderResult
	f.ok("render_view", map[string]any{"path": "plant.sysml", "form": "drawio"}, &render)
	cell := regexp.MustCompile(`(?s)(<object id="[^"]*" [^>]*fqn="Plant::Loop::pump"[^>]*>.*?<mxGeometry x="[^"]*" y=")([^"]*)(")`)
	m := cell.FindStringSubmatch(render.Artifact)
	if m == nil {
		t.Fatalf("no pump cell in the export:\n%s", render.Artifact)
	}
	y, _ := strconv.ParseFloat(m[2], 64)
	moved := cell.ReplaceAllString(render.Artifact, "${1}"+strconv.FormatFloat(y+70, 'f', -1, 64)+"${3}")

	before := f.snapshot()
	var res ImportDiagramResult
	f.ok("import_diagram", map[string]any{"path": "plant.sysml", "content": moved}, &res)
	if res.Applied || len(res.Refused) != 0 || res.View != "Plant::loopView" {
		t.Fatalf("dry run = %+v", res)
	}
	// The documents the view was rendered from are pinned without edits.
	for _, change := range res.Changes {
		if (change.Path == "plant.sysml") != (len(change.Edits) > 0) {
			t.Fatalf("dry run changes = %+v", res.Changes)
		}
	}
	f.unchanged(before)

	f.ok("import_diagram", map[string]any{"path": "plant.sysml", "content": moved, "apply": true}, &res)
	if !res.Applied {
		t.Fatalf("apply = %+v", res)
	}
	if after := f.read("plant.sysml"); !strings.Contains(after, "metadata DiagramLayout::Layout about Plant::Loop::pump") {
		t.Errorf("plant.sysml has no Layout about the pump:\n%s", after)
	}
	f.ok("import_diagram", map[string]any{"path": "plant.sysml", "content": moved}, &res)
	if len(res.Edits) != 0 {
		t.Errorf("the written model still imports %+v", res.Edits)
	}
	f.fails("import_diagram", map[string]any{"path": "plant.sysml"})
}

func TestImportDiagramReadsFilesUnderTheRootAlone(t *testing.T) {
	f := newFixture(t)
	write(t, f.root, "plant.sysml", plant)
	write(t, filepath.Dir(f.root), "outside.drawio", "<mxfile/>")
	msg := f.fails("import_diagram", map[string]any{"path": "plant.sysml", "file": "../outside.drawio"})
	if !strings.Contains(msg, "outside.drawio") || strings.Contains(msg, "mxfile") {
		t.Fatalf("outside file: %s", msg)
	}
}
