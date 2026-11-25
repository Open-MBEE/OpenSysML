package runtimetools

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// TestParseOfOneDocumentWithoutParseSources: a service without parse_sources
// still parses one document, through ParseFile, which the parse tool stands for
// as well; several documents, a base hash or an inline document not named
// <content> (an unnamed one would be <content-0>) need ParseSources and are
// refused as the service refuses it.
func TestParseOfOneDocumentWithoutParseSources(t *testing.T) {
	h := attach(t, newService(t, sysmlgrpc.CapabilityParseSources))
	var resp pb.ParseSourcesResponse
	h.ok("parse", map[string]any{"paths": []string{"demo.sysml"}}, &resp)
	if resp.GetModelHash() == "" || len(resp.GetRoots()) != 1 {
		t.Errorf("one path: hash %q, %d roots; want a hash and one root", resp.GetModelHash(), len(resp.GetRoots()))
	}
	h.ok("parse", map[string]any{"sources": []map[string]any{{"path": "<content>", "text": "package Q;", "language": "kerml"}}}, &resp)
	if len(resp.GetRoots()) != 1 {
		t.Errorf("one <content> source: %d roots, want one", len(resp.GetRoots()))
	}
	full := newService(t)
	want, err := full.ParseSources(t.Context(), &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{
		{Source: &pb.SourceDocument_Content{Content: "package Q;"}, Language: "kerml", Name: "<content>"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetModelHash() != want.GetModelHash() {
		t.Errorf("hash through ParseFile %s, through ParseSources %s; want the same document", resp.GetModelHash(), want.GetModelHash())
	}
	for name, args := range map[string]map[string]any{
		"two documents":     {"paths": []string{"demo.sysml"}, "sources": []map[string]any{{"path": "", "text": "package P;"}}},
		"a base hash":       {"paths": []string{"demo.sysml"}, "baseModelHash": "h0"},
		"a named source":    {"sources": []map[string]any{{"path": "p.sysml", "text": "package P;"}}},
		"an unnamed source": {"sources": []map[string]any{{"path": "", "text": "package P;"}}},
		"a keep option":     {"paths": []string{"demo.sysml"}, "keep": false},
	} {
		res := h.call("parse", args)
		if !res.IsError || !strings.HasPrefix(text(res), "UNIMPLEMENTED: ") {
			t.Errorf("%s: %v %q, want the service's UNIMPLEMENTED", name, res.IsError, text(res))
		}
	}
}

// The parse tool's keep argument reaches ParseSources, so the model it answers
// is not left in the cache under that hash.
func TestParseKeepFalseAnswersWithoutKeepingTheModel(t *testing.T) {
	svc := newService(t)
	h := attach(t, svc)
	var resp pb.ParseSourcesResponse
	h.ok("parse", map[string]any{"paths": []string{"demo.sysml"}, "keep": false}, &resp)
	if resp.GetModelHash() == "" {
		t.Fatal("the transient parse answered no model hash")
	}
	if _, err := svc.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: resp.GetModelHash()}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetDiagnostics on the transient hash: error %v, want NOT_FOUND", err)
	}
}

func TestLookupKindReadsTheProtojsonSpelling(t *testing.T) {
	for lookup, want := range map[string]pb.LookupKind{
		"":                           pb.LookupKind_LOOKUP_KIND_UNSPECIFIED,
		"LOOKUP_KIND_QUALIFIED_NAME": pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME,
		"LOOKUP_KIND_ELEMENT_ID":     pb.LookupKind_LOOKUP_KIND_ELEMENT_ID,
	} {
		if got, err := lookupKind(lookup); err != nil || got != want {
			t.Errorf("lookupKind(%q) = %v, %v; want %v", lookup, got, err, want)
		}
	}
	if _, err := lookupKind("element_id"); err == nil {
		t.Error("lookupKind accepted a spelling protojson does not write")
	}
}
