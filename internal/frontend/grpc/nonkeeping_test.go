package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// The fixture's two projects and the derived union a client reads for its results,
// which must not cost the open project its place in the cache.
const (
	nonKeepingA = "package A { attribute x = 1; }"
	nonKeepingB = "package B { attribute y = 2; }"
	nonKeepingC = "package C { attribute z = 3; }"
)

func parseSourcesWithKeep(t *testing.T, srv *Service, keep *bool, base string,
	documents []*pb.SourceDocument) *pb.ParseSourcesResponse {
	t.Helper()
	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: documents, Keep: keep, BaseModelHash: base,
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	return resp
}

// A derived union read twice, with the other project between the reads, keeps the
// open model cached; a kept union proves the fixture can evict one of the three.
func TestParseSourcesKeepFalseLeavesTheOpenModelCached(t *testing.T) {
	srv := mustNewService(t, 3)
	defer srv.Close()
	ctx := context.Background()

	// Fill the cache with three kept models, the open one last.
	b := parseSourcesWithKeep(t, srv, nil, "", inlineDocuments("b.sysml", nonKeepingB))
	c := parseSourcesWithKeep(t, srv, nil, "", inlineDocuments("c.sysml", nonKeepingC))
	a := parseSourcesWithKeep(t, srv, nil, "", inlineDocuments("a.sysml", nonKeepingA))

	union := inlineDocuments("a.sysml", nonKeepingA, "b.sysml", nonKeepingB)
	keepFalse, keepTrue := false, true
	transient := parseSourcesWithKeep(t, srv, &keepFalse, a.ModelHash, union)
	if len(transient.Affected) == 0 {
		t.Fatal("a transient union answering a base reported no affected documents")
	}
	// Read the other project between two reads of the open one, as a client
	// reading evidence for two projects in turn does.
	_ = parseSourcesWithKeep(t, srv, &keepFalse, b.ModelHash, inlineDocuments("b.sysml", nonKeepingB))
	again := parseSourcesWithKeep(t, srv, &keepFalse, a.ModelHash,
		inlineDocuments("a.sysml", nonKeepingA))

	// Every kept model is still in the cache, and the transient models are not.
	if got := len(srv.cache.items); got != 3 {
		t.Errorf("cached models after the transient reads = %d, want 3", got)
	}
	for _, hash := range []string{a.ModelHash, b.ModelHash} {
		if _, ok := srv.cache.items[hash]; !ok {
			t.Errorf("model %s was displaced by a transient parse", hash)
		}
	}
	if _, ok := srv.cache.items[transient.ModelHash]; ok {
		t.Error("the transient union was retained in the parsed-model cache")
	}
	if again.ModelHash != a.ModelHash {
		t.Error("a transient read of the open documents changed its model hash")
	}
	// The open model is still usable through the RPC, not just in the map: a
	// later request on its hash answers rather than reporting CodeNotFound.
	if _, err := srv.GetDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: a.ModelHash}); err != nil {
		t.Errorf("GetDiagnostics on the open model after the transient reads: %v", err)
	}

	// A kept parse of the same union does evict one of the three, so the test
	// above is about the transient option and not about a cache that never fills.
	kept := parseSourcesWithKeep(t, srv, &keepTrue, a.ModelHash, union)
	if kept.ModelHash != transient.ModelHash {
		t.Errorf("transient hash %s, kept hash %s; want the same model", transient.ModelHash, kept.ModelHash)
	}
	if _, ok := srv.cache.items[transient.ModelHash]; !ok {
		t.Error("a kept parse of the union did not insert it")
	}
	if _, ok := srv.cache.items[c.ModelHash]; ok {
		t.Error("a kept parse of the union evicted no model: the no-eviction check was vacuous")
	}
}

// A transient parse answers the same hash, roots, diagnostics and affected set
// as a kept parse; only its cache effect differs.
func TestParseSourcesKeepFalseAnswersWhatAKeptParseAnswers(t *testing.T) {
	kept := mustNewService(t, 10)
	defer kept.Close()
	transient := mustNewService(t, 10)
	defer transient.Close()

	documents := inlineDocuments("lib.sysml", sourcesLibrary, "top.sysml", sourcesTop)
	base := parseSourcesWithKeep(t, kept, nil, "", documents).ModelHash
	keepFalse, keepTrue := false, true
	want := parseSourcesWithKeep(t, kept, &keepTrue, base, documents)
	got := parseSourcesWithKeep(t, transient, &keepFalse, base, documents)
	if !proto.Equal(want, got) {
		t.Errorf("transient answer = %v, kept answer = %v", got, want)
	}
	if _, ok := transient.cache.items[got.ModelHash]; ok {
		t.Error("the transient answer inserted a model")
	}
}

// A transient parse stopped at any cancellation point leaves nothing cached,
// even when the stop was during analysis.
func TestParseSourcesKeepFalseCancelledParseKeepsNothing(t *testing.T) {
	srv := mustNewService(t, 3)
	defer srv.Close()
	keepFalse := false
	req := &pb.ParseSourcesRequest{Documents: cancellationDocuments("T"), Keep: &keepFalse}
	resp, _ := cancelAtEveryCheck(t, context.Canceled, func(ctx context.Context) (*pb.ParseSourcesResponse, error) {
		return srv.ParseSources(ctx, req)
	})
	if resp == nil || resp.ModelHash == "" {
		t.Fatal("a transient parse that stayed live answered no model")
	}
	if got := cachedModels(srv); got != 0 {
		t.Errorf("cached models after transient parses = %d, want none", got)
	}
	if _, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: resp.ModelHash}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetDiagnostics on the transient hash: error %v, want NOT_FOUND", err)
	}
}

// A service without the capability refuses a stated field, so a transient caller
// is never silently given a kept model; an ordinary parse still works.
func TestParseSourcesKeepNeedsTheCapability(t *testing.T) {
	ctx := context.Background()
	srv := mustNewServiceWithout(t, CapabilityParseSourcesKeep)
	defer srv.Close()
	documents := inlineDocuments("a.sysml", nonKeepingA)
	keepFalse, keepTrue := false, true

	for name, keep := range map[string]*bool{
		"keep false": &keepFalse,
		"keep true":  &keepTrue,
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{Documents: documents, Keep: keep})
			if connect.CodeOf(err) != connect.CodeUnimplemented || resp != nil || !strings.Contains(err.Error(), CapabilityParseSourcesKeep) {
				t.Errorf("ParseSources: response %v, error %v, want UNIMPLEMENTED naming %s", resp, err, CapabilityParseSourcesKeep)
			}
		})
	}
	if got := len(srv.cache.items); got != 0 {
		t.Errorf("refused parses cached %d models", got)
	}
	// A request without the field is still the old, kept parse.
	plain := parseSourcesWithKeep(t, srv, nil, "", documents)
	if _, ok := srv.cache.items[plain.ModelHash]; !ok {
		t.Error("a plain parse was not kept by the service without the capability")
	}
}
