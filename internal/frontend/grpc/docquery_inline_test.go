package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
)

const inlineSubsystems = `Project(WhereType(Descendants(Observatory::telescope), "PartUsage"), ("name", "mass"))`

func TestRunDocumentQueryInlineAnswersTypedRows(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{ModelHash: hash, InlineQuery: inlineSubsystems})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	if len(resp.Columns) != 2 || resp.Columns[0].Name != "name" || resp.Columns[1].Name != "mass" {
		t.Fatalf("columns = %v, want [name mass]", resp.Columns)
	}
	// Unordered by the expression, the rows come in document order.
	wantElements := []string{
		"Observatory::telescope::optics",
		"Observatory::telescope::segmentControl",
		"Observatory::telescope::mount",
		"Observatory::telescope::baffle|shroud *tricky*",
	}
	if len(resp.Rows) != len(wantElements) {
		t.Fatalf("rows = %d, want %d", len(resp.Rows), len(wantElements))
	}
	for i, row := range resp.Rows {
		if got := row.Element.GetElementId(); got != wantElements[i] {
			t.Errorf("row %d element = %q, want %q", i, got, wantElements[i])
		}
		if got := row.Element.GetElementType(); got != "PartUsage" {
			t.Errorf("row %d element type = %q, want PartUsage", i, got)
		}
	}
	// The same rows the named query answers, and the named query still runs after.
	named, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Observatory::SubsystemTable",
		Bindings: []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	})
	if err != nil {
		t.Fatalf("named RunDocumentQuery failed: %v", err)
	}
	if len(named.Rows) != len(resp.Rows) {
		t.Errorf("named query answers %d rows, inline %d", len(named.Rows), len(resp.Rows))
	}
	// The inline definition is indexed beside the model, not in it.
	if cached, _ := srv.cache.Get(hash); cached.Index.DocumentRoot(queryplan.InlineDocument) != nil {
		t.Errorf("the model's index holds the inline document")
	}
}

func TestRunDocumentQueryInlineBindsAParameterItDeclares(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)
	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash:   hash,
		InlineQuery: `in root : KerML::Root::Element; Project(WhereType(Descendants(root), "PartUsage"), ("name"))`,
		Bindings:    []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	if len(resp.Rows) != 4 {
		t.Errorf("rows = %d, want 4", len(resp.Rows))
	}
	_, err = srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, InlineQuery: inlineSubsystems,
		Bindings: []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "unknown binding root") {
		t.Errorf("binding an expression declaring no parameter: %v, want InvalidArgument unknown binding", err)
	}
}

func TestRunDocumentQueryInlineReadsHeldObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, `package Garage {
	private import DocumentQueries::*;
	part def Wheel { attribute pressure : Integer default 30; }
	part def Car { part wheels : Wheel[2]; }
	part car : Car;
}`)
	request := &pb.RunDocumentQueryRequest{ModelHash: hash, InlineQuery: `Project(Objects(type = "Wheel"), ("pressure"))`}
	resp, err := srv.RunDocumentQuery(context.Background(), request)
	if err != nil || len(resp.Rows) != 0 {
		t.Fatalf("before Instantiate: %d rows, %v", len(resp.GetRows()), err)
	}
	if _, err := srv.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: hash, SymbolId: "Garage::car"}); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	resp, err = srv.RunDocumentQuery(context.Background(), request)
	if err != nil {
		t.Fatalf("after Instantiate: %v", err)
	}
	if len(resp.Rows) != 2 || resp.Rows[0].Element.GetObject().GetPath() != "Garage::car.wheels[1]" {
		t.Errorf("rows = %v, want the two wheels held", resp.Rows)
	}
}

func TestRunDocumentQueryInlineFaultsArePositionedInTheExpression(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)
	cases := []struct {
		name, text, want string
		code             connect.Code
	}{
		{"parse", `Project(Named("Observatory") ("name"))`, "inline query 1:", connect.CodeInvalidArgument},
		{"planning", `Union(Named("Observatory"), Frobnicate(Named("Observatory")))`, "inline query 1:29: query <inline query> invokes unknown operation Frobnicate", connect.CodeInvalidArgument},
		{"blank", "  ", "inline query 1:1:", connect.CodeInvalidArgument},
		{"declaring more", `Named("Observatory") } package Evil {`, "inline query 1:1: an inline query is one query expression", connect.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{ModelHash: hash, InlineQuery: tc.text})
			if connect.CodeOf(err) != tc.code || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v (%v), want %v containing %q", err, connect.CodeOf(err), tc.code, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), "declared in") {
				t.Errorf("the fault is reported as the model's: %v", err)
			}
		})
	}
}

func TestRunDocumentQueryInlineAttachesTheOutcomeDetail(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, `package Observatory { part telescope; }`)
	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, InlineQuery: `Named(qualifiedName = "Observatory::telescop")`,
	})
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("error = %v, want FailedPrecondition", err)
	}
	if !strings.HasPrefix(connErr.Message(), "inline query 1:1: ") {
		t.Errorf("message %q is not positioned in the expression", connErr.Message())
	}
	var outcome *pb.QueryOutcome
	for _, detail := range connErr.Details() {
		if msg, valueErr := detail.Value(); valueErr == nil {
			if typed, ok := msg.(*pb.QueryOutcome); ok {
				outcome = typed
			}
		}
	}
	if outcome == nil || outcome.Kind != "pathError" || outcome.Step != "Named(Observatory::telescop)" {
		t.Fatalf("outcome = %v, want the path error at Named", outcome)
	}
}

func TestRunDocumentQueryNamedAndInlineAreExclusive(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseTelescope(t, srv)
	for name, request := range map[string]*pb.RunDocumentQueryRequest{
		"both":    {ModelHash: hash, QueryId: "Observatory::SubsystemTable", InlineQuery: inlineSubsystems},
		"neither": {ModelHash: hash},
	} {
		_, err := srv.RunDocumentQuery(context.Background(), request)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "inline_query") {
			t.Errorf("%s: %v, want InvalidArgument naming inline_query", name, err)
		}
	}
}

func TestRunDocumentQueryInlineRequiresItsCapability(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityInlineDocumentQuery)
	hash := parseTelescope(t, srv)
	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{ModelHash: hash, InlineQuery: inlineSubsystems})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityInlineDocumentQuery) {
		t.Errorf("error = %v, want Unimplemented naming the capability", err)
	}
	if _, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Observatory::SubsystemTable",
		Bindings: []*pb.DocumentQueryBinding{binding("root", element("Observatory::telescope"))},
	}); err != nil {
		t.Errorf("a named query without the inline capability: %v", err)
	}
}
