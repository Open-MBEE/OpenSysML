package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
)

// Two parsed documents declaring one library package carry the workspace
// warning in GetDiagnostics, whether or not the request selects lints: it is no
// lint, so a selection leaves it in place.
func TestGetDiagnosticsReportsALibraryDeclaredTwiceUnderAnyLintSelection(t *testing.T) {
	srv := mustNewService(t, 10)
	src := "library package Foo { part def A; }"
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments("a.sysml", src, "b.sysml", src)})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	for _, req := range []*pb.DiagnosticsRequest{
		{},
		{DisableLint: []string{passes.CodeUndeclaredSignal}},
		{EnableLint: []string{passes.CodeRoundedRealLiteral}},
	} {
		req.ModelHash = parsed.ModelHash
		resp, err := srv.GetDiagnostics(context.Background(), req)
		if err != nil {
			t.Fatalf("GetDiagnostics %v: %v", req, err)
		}
		var found []*pb.Diagnostic
		for _, d := range resp.Diagnostics {
			if d.GetCode() == checkws.CodeLibraryDeclaredTwice {
				found = append(found, d)
			}
		}
		if len(found) != 1 || found[0].GetSpan().GetFile() != "b.sysml" || found[0].GetSeverity() != "warning" {
			t.Errorf("request %v: duplicate-library diagnostics = %v, want one warning on b.sysml", req, found)
		}
	}
}
