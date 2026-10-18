package runtimetools

import (
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// TestParseOfOneDocumentWithoutParseSources: a service without parse_sources
// still parses one document, through ParseFile, which the parse tool stands for
// as well; several documents, a base hash or a named inline document need
// ParseSources and are refused as the service refuses it.
func TestParseOfOneDocumentWithoutParseSources(t *testing.T) {
	h := attach(t, newService(t, sysmlgrpc.CapabilityParseSources))
	var resp pb.ParseSourcesResponse
	h.ok("parse", map[string]any{"paths": []string{"demo.sysml"}}, &resp)
	if resp.GetModelHash() == "" || len(resp.GetRoots()) != 1 {
		t.Errorf("one path: hash %q, %d roots; want a hash and one root", resp.GetModelHash(), len(resp.GetRoots()))
	}
	h.ok("parse", map[string]any{"sources": []map[string]any{{"path": "", "text": "package P;"}}}, &resp)
	if len(resp.GetRoots()) != 1 {
		t.Errorf("one inline source: %d roots, want one", len(resp.GetRoots()))
	}
	h.ok("parse", map[string]any{"sources": []map[string]any{{"path": "<content>", "text": "package Q;", "language": "kerml"}}}, &resp)
	if len(resp.GetRoots()) != 1 {
		t.Errorf("one <content> source: %d roots, want one", len(resp.GetRoots()))
	}
	for name, args := range map[string]map[string]any{
		"two documents":  {"paths": []string{"demo.sysml"}, "sources": []map[string]any{{"path": "", "text": "package P;"}}},
		"a base hash":    {"paths": []string{"demo.sysml"}, "baseModelHash": "h0"},
		"a named source": {"sources": []map[string]any{{"path": "p.sysml", "text": "package P;"}}},
	} {
		res := h.call("parse", args)
		if !res.IsError || !strings.HasPrefix(text(res), "UNIMPLEMENTED: ") {
			t.Errorf("%s: %v %q, want the service's UNIMPLEMENTED", name, res.IsError, text(res))
		}
	}
}
