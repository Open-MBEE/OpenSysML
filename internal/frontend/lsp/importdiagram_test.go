package lsp

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// movePump shifts the pump's cell down in a draw.io export, as a user dragging it would.
func movePump(t *testing.T, document string) string {
	t.Helper()
	cell := regexp.MustCompile(`(?s)(<object id="[^"]*" [^>]*fqn="Plant::Loop::pump"[^>]*>.*?<mxGeometry x="[^"]*" y=")([^"]*)(")`)
	m := cell.FindStringSubmatch(document)
	if m == nil {
		t.Fatalf("no pump cell in the export:\n%s", document)
	}
	y, _ := strconv.ParseFloat(m[2], 64)
	return cell.ReplaceAllString(document, "${1}"+strconv.FormatFloat(y+70, 'f', -1, 64)+"${3}")
}

func TestImportDiagramAnswersTheLayoutsAsAWorkspaceEdit(t *testing.T) {
	s, docURI := renderServer(t, "plant.sysml", plantModel)
	exported, err := s.Render(&RenderParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "PlantViews::loopView", Form: "drawio"})
	if err != nil {
		t.Fatal(err)
	}
	untouched, err := s.ImportDiagram(&ImportDiagramParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, Content: exported.Artifact})
	if err != nil {
		t.Fatal(err)
	}
	if untouched.View != "PlantViews::loopView" || len(untouched.Edits) != 0 || untouched.Edit != nil || len(untouched.Notices)+len(untouched.Unmatched) != 0 {
		t.Fatalf("untouched export imports %+v, want nothing", untouched)
	}
	out, err := s.ImportDiagram(&ImportDiagramParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, Content: movePump(t, exported.Artifact), DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Edit == nil || out.Refused != nil || !out.DryRun {
		t.Fatalf("result = %+v, want a dry-run edit", out)
	}
	var pump bool
	for _, e := range out.Edits {
		pump = pump || e.Kind == "Layout" && e.Target == "Plant::Loop::pump"
	}
	if !pump {
		t.Errorf("edits %+v, want a Layout about Plant::Loop::pump", out.Edits)
	}
	applied := applyWorkspaceEdit(t, plantModel, out.Edit, docURI)
	if !strings.Contains(applied, "metadata DiagramLayout::Layout about Plant::Loop::pump") {
		t.Errorf("applied edit has no Layout about the pump:\n%s", applied)
	}
	if _, err := s.ImportDiagram(&ImportDiagramParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}}); err == nil {
		t.Error("a request without a file was answered")
	}
}
