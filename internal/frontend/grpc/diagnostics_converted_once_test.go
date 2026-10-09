package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

// warningLibrary, warningTop and warningOther are a model whose top document
// parses clean with a warning, so a parse of it reports diagnostics.
const (
	warningLibrary = "package Lib {\n\tpart def Engine;\n}\n"
	warningTop     = "package Top {\n\tprivate import Lib::*;\n\tcalc def F { 1 /* c */ }\n\tpart def Car {\n\t\tpart motor : Engine;\n\t}\n}\n"
	warningOther   = "package Other {\n\tpart def Wheel;\n}\n"
)

// requireWarnings fails unless diags hold a warning and nothing more severe.
func requireWarnings(t *testing.T, diags []*pb.Diagnostic) {
	t.Helper()
	if len(diags) == 0 {
		t.Fatal("no diagnostics to compare")
	}
	for _, d := range diags {
		if d.Severity != "warning" {
			t.Fatalf("diagnostic %v is not a warning", d)
		}
	}
}

// requireSameDiagnostics fails unless got repeats want message for message.
func requireSameDiagnostics(t *testing.T, got, want []*pb.Diagnostic) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d diagnostics, want %d:\n%v\n%v", len(got), len(want), got, want)
	}
	for i := range got {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("diagnostic %d is %v, want %v", i, got[i], want[i])
		}
	}
}

// A parse of a model parsed before answers the diagnostics the first parse
// did, converted once: the messages are the same, so a service seeing the model
// for the first time, a repeated parse and GetDiagnostics all agree, and a
// service that withholds diagnostic codes withholds them on every call.
func TestParseFileConvertsACachedModelsDiagnosticsOnce(t *testing.T) {
	ctx := context.Background()
	req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: misplacedComment}}
	fresh := mustNewService(t, 10)
	defer fresh.Close()
	want, err := fresh.ParseFile(ctx, req)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	requireWarnings(t, want.Diagnostics)
	requireEveryCoded(t, want.Diagnostics)

	srv := mustNewService(t, 10)
	defer srv.Close()
	first, err := srv.ParseFile(ctx, req)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	second, err := srv.ParseFile(ctx, req)
	if err != nil {
		t.Fatalf("ParseFile again: %v", err)
	}
	if !proto.Equal(first, want) {
		t.Errorf("first parse is\n%v\nfresh parse is\n%v", first, want)
	}
	if !proto.Equal(second, want) {
		t.Errorf("repeated parse is\n%v\nfresh parse is\n%v", second, want)
	}
	for i := range first.Diagnostics {
		if first.Diagnostics[i] != second.Diagnostics[i] {
			t.Errorf("diagnostic %d was converted again for the repeated parse", i)
		}
	}
	// The list is the response's own: growing one leaves the other as it was.
	second.Diagnostics = append(second.Diagnostics, &pb.Diagnostic{Severity: "error"})
	diags, err := srv.GetDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: first.ModelHash})
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	requireSameDiagnostics(t, diags.Diagnostics, want.Diagnostics)
	requireSameDiagnostics(t, first.Diagnostics, want.Diagnostics)

	withheld, err := NewServiceWithUnavailableCapabilitiesForTesting(10, "test", []string{CapabilityDiagnosticCodes})
	if err != nil {
		t.Fatal(err)
	}
	defer withheld.Close()
	for call := 0; call < 2; call++ {
		resp, err := withheld.ParseFile(ctx, req)
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		if len(resp.Diagnostics) != len(want.Diagnostics) {
			t.Fatalf("withheld service reported %d diagnostics, want %d", len(resp.Diagnostics), len(want.Diagnostics))
		}
		for i, d := range resp.Diagnostics {
			uncoded := proto.Clone(want.Diagnostics[i]).(*pb.Diagnostic)
			uncoded.Code = ""
			if !proto.Equal(d, uncoded) {
				t.Errorf("call %d: withheld diagnostic = %v, want %v", call, d, uncoded)
			}
		}
	}
}

// documentDiagnosticsOf are the diagnostics of resp located in the document named.
func documentDiagnosticsOf(resp *pb.ParseSourcesResponse, name string) []*pb.Diagnostic {
	var out []*pb.Diagnostic
	for _, d := range resp.Diagnostics {
		if d.GetSpan().GetFile() == name {
			out = append(out, d)
		}
	}
	return out
}

// A model answered from its lineage shares the converted diagnostics of every
// document the edit before it did not reach, and converts afresh those of the
// documents it did; either way it reports what a fresh parse of the same
// documents reports.
func TestParseSourcesSharesConvertedDiagnosticsOfDocumentsAnEditDidNotReach(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 32)
	defer srv.Close()
	parse := func(srv *Service, documents []*pb.SourceDocument) *pb.ParseSourcesResponse {
		t.Helper()
		resp, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{Documents: documents})
		if err != nil {
			t.Fatalf("ParseSources: %v", err)
		}
		for _, d := range resp.Diagnostics {
			if d.Severity != "warning" {
				t.Fatalf("diagnostic %v is not a warning", d)
			}
		}
		return resp
	}
	documents := func(other string) []*pb.SourceDocument {
		return inlineDocuments("lib.sysml", warningLibrary, "top.sysml", warningTop, "other.sysml", other)
	}
	// Parsed twice so the edits are answered from the set's lineage.
	parse(srv, documents(warningOther+"// first\n"))
	base := parse(srv, documents(warningOther))
	requireWarnings(t, documentDiagnosticsOf(base, "top.sysml"))

	untouched := parse(srv, documents(warningOther+"// a comment\n"))
	requireSameDiagnostics(t, untouched.Diagnostics, base.Diagnostics)
	before, after := documentDiagnosticsOf(base, "top.sysml"), documentDiagnosticsOf(untouched, "top.sysml")
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("top.sysml diagnostic %d was converted again though the edit did not reach the document", i)
		}
	}

	reached := inlineDocuments("lib.sysml", warningLibrary, "top.sysml", warningTop+"// edited\n", "other.sysml", warningOther)
	edited := parse(srv, reached)
	fresh := mustNewService(t, 10)
	defer fresh.Close()
	requireSameDiagnostics(t, edited.Diagnostics, parse(fresh, reached).Diagnostics)
	for _, d := range documentDiagnosticsOf(edited, "top.sysml") {
		for _, was := range before {
			if d == was {
				t.Errorf("top.sysml diagnostic %v is the base model's though the edit reached the document", d)
			}
		}
	}
}

// freshlyConverted converts the cached model's diagnostics again, past the cache.
func freshlyConverted(t *testing.T, srv *Service, hash string) []*pb.Diagnostic {
	t.Helper()
	cached, ok := srv.cache.Get(hash)
	if !ok {
		t.Fatalf("model %s is not cached", hash)
	}
	var fresh []*pb.Diagnostic
	for _, doc := range cached.Documents {
		fresh = append(fresh, srv.documentDiagnostics(doc)...)
	}
	return fresh
}

// The shared messages are never written after they are built: every later
// response still equals a conversion made afresh from the cached model.
func TestCachedDiagnosticsStayEqualToAFreshConversion(t *testing.T) {
	ctx := context.Background()
	req := &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: misplacedComment}}
	withheld, err := NewServiceWithUnavailableCapabilitiesForTesting(10, "test", []string{CapabilityDiagnosticCodes})
	if err != nil {
		t.Fatal(err)
	}
	defer withheld.Close()
	for name, srv := range map[string]*Service{"coded": mustNewService(t, 10), "codes withheld": withheld} {
		t.Run(name, func(t *testing.T) {
			defer srv.Close()
			first, err := srv.ParseFile(ctx, req)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			requireWarnings(t, first.Diagnostics)
			second, err := srv.ParseFile(ctx, req)
			if err != nil {
				t.Fatalf("ParseFile again: %v", err)
			}
			diags, err := srv.GetDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: first.ModelHash})
			if err != nil {
				t.Fatalf("GetDiagnostics: %v", err)
			}
			fresh := freshlyConverted(t, srv, first.ModelHash)
			requireSameDiagnostics(t, first.Diagnostics, fresh)
			requireSameDiagnostics(t, second.Diagnostics, fresh)
			requireSameDiagnostics(t, diags.Diagnostics, fresh)
		})
	}
}
