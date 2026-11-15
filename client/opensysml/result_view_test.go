package opensysml_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
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
	rendered, err := client.RenderView(context.Background(), model, "Answers::mount", opensysml.WithForm(opensysml.DocumentFormMarkdown))
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if len(rendered.Header) != 4 || rendered.Header[2] != (opensysml.RenderHeaderField{Name: "outcome", Value: "established"}) {
		t.Errorf("header = %v, want query, source, outcome, provenance", rendered.Header)
	}
	if len(rendered.Rows) != 1 || rendered.Rows[0].Cells[0] != "Observatory::telescope::mount" {
		t.Errorf("rows = %v, want the exposed mount", rendered.Rows)
	}
	if !strings.Contains(rendered.Text, "| Observatory::telescope::mount |") {
		t.Errorf("document has no row for the mount:\n%s", rendered.Text)
	}
}

// Only a QVTS::QueryResult view is written in html or asciidoc, so the client asks no service that cannot write one.
func TestRenderViewDocumentFormNeedsTheResultViewCapability(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityQueryResultView})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := dialClient(t, server.URL)
	model := parse(t, client, resultViewSource)
	for _, form := range []string{opensysml.DocumentFormHTML, opensysml.DocumentFormAsciiDoc} {
		_, err := client.RenderView(context.Background(), model, "Answers::mount", opensysml.WithForm(form))
		wantCapabilityRefusal(t, "RenderView in "+form, err, opensysml.CapabilityQueryResultView)
	}
}
