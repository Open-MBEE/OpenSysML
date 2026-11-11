package runtimetools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestQueriesWriteTheirAnswerAsAResultView(t *testing.T) {
	h := attach(t, newService(t))
	telescope := h.copyIn(telescopeFixture, "telescope.sysml")
	var rows pb.RunDocumentQueryResponse
	h.ok("run_document_query", map[string]any{
		"paths": []string{telescope}, "inlineQuery": `WhereType(Descendants(Observatory::telescope), "PartUsage")`, "asView": true,
	}, &rows)
	view := rows.GetResultView()
	for _, want := range []string{" : QueryResult {", "expose Observatory::telescope::mount;", "outcome = Outcome::established;"} {
		if !strings.Contains(view, want) {
			t.Errorf("run_document_query view lacks %q:\n%s", want, view)
		}
	}
	var selected pb.QueryResponse
	h.ok("query", map[string]any{"paths": []string{telescope}, "oslcQuery": `oslc.where=sysml:name="mount"`, "asView": true}, &selected)
	if view := selected.GetResultView(); !strings.Contains(view, "expose Observatory::telescope::mount;") {
		t.Errorf("query view:\n%s", view)
	}
	h.ok("query", map[string]any{"paths": []string{telescope}, "oslcQuery": `oslc.where=sysml:name="mount"`}, &selected)
	if selected.GetResultView() != "" {
		t.Errorf("a query not asked for a view answered %q", selected.GetResultView())
	}
}

func TestRenderModelViewRendersAResultViewAsADocument(t *testing.T) {
	h := attach(t, newService(t))
	telescope := h.copyIn(telescopeFixture, "telescope.sysml")
	var rows pb.RunDocumentQueryResponse
	h.ok("run_document_query", map[string]any{
		"paths": []string{telescope}, "inlineQuery": `WhereType(Descendants(Observatory::telescope), "PartUsage")`, "asView": true,
	}, &rows)
	if err := os.WriteFile(filepath.Join(h.root, "answer.sysml"), []byte(rows.GetResultView()), 0o600); err != nil {
		t.Fatal(err)
	}
	var view pb.RenderViewResponse
	_, out := h.ok("render_model_view", map[string]any{
		"paths": []string{telescope, "answer.sysml"}, "view": "AnswerResult::answer", "form": "markdown",
	}, &view)
	if view.GetMarkdown() == "" || !strings.Contains(out, strings.TrimSpace(view.GetMarkdown())) || !strings.Contains(out, "| Observatory::telescope::mount |") {
		t.Errorf("text:\n%s\nmarkdown:\n%s", out, view.GetMarkdown())
	}
}
