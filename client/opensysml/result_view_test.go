package opensysml_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const resultViewSource = documentSource + `
package Answers {
	private import QVTS::*;

	view mount : QueryResult {
		expose Observatory::telescope::mount;
		attribute :>> query = "Observatory::Subsystems";
		attribute :>> source = "observatory.sysml";
		attribute :>> outcome = Outcome::established;
		attribute :>> provenance = "urn:prov:bundle:mount";
	}
}
`

func TestQueriesAnswerAsResultViews(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, resultViewSource)
	ctx := context.Background()

	rows, err := client.RunInlineDocumentQueryAsView(ctx, model, `Project(Named("Observatory::telescope::mount"), ("name"))`)
	if err != nil {
		t.Fatalf("RunInlineDocumentQueryAsView: %v", err)
	}
	for _, want := range []string{"view answer : QueryResult", "expose Observatory::telescope::mount;", "Outcome::established"} {
		if !strings.Contains(rows.ResultView, want) {
			t.Errorf("result view has no %q:\n%s", want, rows.ResultView)
		}
	}
	plain, err := client.RunInlineDocumentQuery(ctx, model, `Project(Named("Observatory::telescope::mount"), ("name"))`)
	if err != nil || plain.ResultView != "" {
		t.Errorf("unasked view = %q, err %v; want none", plain.ResultView, err)
	}

	answer, err := client.QueryOSLCAsView(ctx, model, `oslc.where=sysml:name="optics"`)
	if err != nil {
		t.Fatalf("QueryOSLCAsView: %v", err)
	}
	if len(answer.Elements) != 1 || !strings.Contains(answer.ResultView, "expose Observatory::telescope::optics;") {
		t.Errorf("elements = %v, view:\n%s", answer.Elements, answer.ResultView)
	}

	// A path error is an outcome: its view rides the refusal.
	_, err = client.QueryOSLCAsView(ctx, model, `oslc.where=sysml:nme="optics"`)
	var refused *opensysml.StatusError
	if !errors.As(err, &refused) || !strings.Contains(refused.ResultView, "Outcome::pathError") {
		t.Errorf("err = %v, want a refusal carrying a pathError view", err)
	}
	_, err = client.RunInlineDocumentQueryAsView(ctx, model, `Project(Named("Observatory::telescope::mnt"), ("name"))`)
	if !errors.As(err, &refused) || !strings.Contains(refused.ResultView, "expose Observatory::telescope;") {
		t.Errorf("err = %v, want a refusal whose view exposes the frontier", err)
	}
}

func TestRenderViewRendersAResultViewAsADocument(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, resultViewSource)
	rendered, err := client.RenderView(context.Background(), model, "Answers::mount", opensysml.WithDocumentForm("markdown"))
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if len(rendered.Header) != 4 || rendered.Header[2] != (opensysml.RenderHeaderField{Name: "outcome", Value: "established"}) {
		t.Errorf("header = %v, want query, source, outcome, provenance", rendered.Header)
	}
	if len(rendered.Rows) != 1 || rendered.Rows[0].Cells[0] != "Observatory::telescope::mount" {
		t.Errorf("rows = %v, want the exposed mount", rendered.Rows)
	}
	if !strings.Contains(rendered.Document, "| Observatory::telescope::mount |") {
		t.Errorf("document has no row for the mount:\n%s", rendered.Document)
	}
}
