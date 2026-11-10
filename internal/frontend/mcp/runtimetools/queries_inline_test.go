package runtimetools

import (
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

func TestRunDocumentQueryTakesAnInlineExpression(t *testing.T) {
	h := attach(t, newService(t))
	telescope := h.copyIn(telescopeFixture, "telescope.sysml")
	var rows pb.RunDocumentQueryResponse
	_, out := h.ok("run_document_query", map[string]any{
		"paths":       []string{telescope},
		"inlineQuery": `Project(WhereType(Descendants(Observatory::telescope), "PartUsage"), ("name", "mass"))`,
	}, &rows)
	if len(rows.GetColumns()) != 2 || len(rows.GetRows()) != 4 {
		t.Errorf("inline query: %d columns, %d rows\n%s", len(rows.GetColumns()), len(rows.GetRows()), out)
	}
	if !strings.Contains(out, "columns: element | name | mass") || !strings.Contains(out, "Observatory::telescope::mount") {
		t.Errorf("text:\n%s", out)
	}
	msg := h.fails("run_document_query", map[string]any{
		"paths": []string{telescope}, "inlineQuery": `Project(Named("Observatory") ("name"))`,
	})
	if !strings.Contains(msg, "inline query 1:") {
		t.Errorf("a fault in the expression: %s", msg)
	}
	msg = h.fails("run_document_query", map[string]any{
		"paths": []string{telescope}, "queryId": "Observatory::SubsystemTable", "inlineQuery": `Named("Observatory")`,
	})
	if !strings.Contains(msg, "not both") {
		t.Errorf("a name and an expression together: %s", msg)
	}
	msg = h.fails("run_document_query", map[string]any{"paths": []string{telescope}})
	if !strings.Contains(msg, "inline_query") {
		t.Errorf("neither a name nor an expression: %s", msg)
	}
}

func TestRunDocumentQueryInlineNeedsItsCapability(t *testing.T) {
	h := attach(t, newService(t, sysmlgrpc.CapabilityInlineDocumentQuery))
	telescope := h.copyIn(telescopeFixture, "telescope.sysml")
	msg := h.fails("run_document_query", map[string]any{"paths": []string{telescope}, "inlineQuery": `Named("Observatory")`})
	if !strings.Contains(msg, sysmlgrpc.CapabilityInlineDocumentQuery) {
		t.Errorf("without the capability: %s", msg)
	}
}
