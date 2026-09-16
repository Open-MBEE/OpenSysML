package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// grpcExtension uses notation of ours: a warning by default, an error when the
// request asks strictly.
const grpcExtension = "package P { state def S { choice c; state a; } }"

func TestServerAdvertisesStrictConformance(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(info.Capabilities, CapabilityStrictConformance) {
		t.Fatalf("capabilities = %v, want %q among them", info.Capabilities, CapabilityStrictConformance)
	}
}

func TestParseFileStrictConformanceEscalatesOurNotation(t *testing.T) {
	srv := mustNewService(t, 10)
	for _, tc := range []struct {
		name   string
		strict bool
		want   string
	}{
		{"default", false, "warning"},
		{"strict", true, "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: grpcExtension},
				StrictConformance: tc.strict,
			})
			if err != nil {
				t.Fatal(err)
			}
			d := notationDiagnostic(t, resp)
			if d.Severity != tc.want {
				t.Fatalf("severity = %q, want %q", d.Severity, tc.want)
			}
		})
	}
}

// The two modes must not share a cache entry, or the second caller is answered
// with the first one's question.
func TestParseFileCachesTheModesSeparately(t *testing.T) {
	srv := mustNewService(t, 10)
	def, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: grpcExtension},
	})
	if err != nil {
		t.Fatal(err)
	}
	strict, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:            &pb.ParseFileRequest_Content{Content: grpcExtension},
		StrictConformance: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if def.ModelHash == strict.ModelHash {
		t.Fatalf("both modes hashed to %q", def.ModelHash)
	}
	if got := notationDiagnostic(t, def).Severity; got != "warning" {
		t.Errorf("default severity = %q, want warning", got)
	}
	if got := notationDiagnostic(t, strict).Severity; got != "error" {
		t.Errorf("strict severity = %q, want error", got)
	}

	// Re-asking the default question after the strict one must still answer it.
	again, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: grpcExtension},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := notationDiagnostic(t, again).Severity; got != "warning" {
		t.Errorf("default severity after the strict call = %q, want warning", got)
	}
}

// requireOutsideRequirement is a model an edit can make nonstandard: moving the
// `require` constraint into the part def is notation of ours, a warning by
// default and an error when the model was parsed strictly.
const requireOutsideRequirement = "package P {\n    requirement def R {\n        require constraint c { 1 > 0 }\n    }\n    part def V;\n}\n"

// An edit's notation is judged at the strictness the model was parsed at: the
// same move is applied to the default model and refused for the strict one.
func TestApplyEditsJudgesTheEditAtTheParsedStrictness(t *testing.T) {
	srv := mustNewService(t, 10)
	for _, tc := range []struct {
		name   string
		strict bool
	}{
		{"default", false},
		{"strict", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: requireOutsideRequirement},
				StrictConformance: tc.strict,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Diagnostics) != 0 {
				t.Fatalf("fixture has diagnostics: %v", parsed.Diagnostics)
			}
			resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash: parsed.ModelHash, Operations: []*pb.EditOperation{moveOp("P::R::c", "P::V")},
			})
			if err != nil {
				t.Fatal(err)
			}
			assertStrictnessVerdict(t, resp, tc.strict)
		})
	}
}

// The strictness a ParseSources request asked for follows its model into an
// edit of any of its documents.
func TestApplyEditsJudgesEachDocumentAtTheParsedStrictness(t *testing.T) {
	srv := mustNewService(t, 10)
	for _, tc := range []struct {
		name   string
		strict bool
	}{
		{"default", false},
		{"strict", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
				Documents:         inlineDocuments("lib.sysml", "package Lib {\n    part def Base;\n}\n", "req.sysml", requireOutsideRequirement),
				StrictConformance: tc.strict,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Diagnostics) != 0 {
				t.Fatalf("fixture has diagnostics: %v", parsed.Diagnostics)
			}
			resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash: parsed.ModelHash, AcceptDocuments: true, Document: "req.sysml",
				Operations: []*pb.EditOperation{moveOp("P::R::c", "P::V")},
			})
			if err != nil {
				t.Fatal(err)
			}
			assertStrictnessVerdict(t, resp, tc.strict)
		})
	}
}

// assertStrictnessVerdict checks the move of the `require` constraint was
// applied in the default mode and refused as nonstandard in the strict one.
func assertStrictnessVerdict(t *testing.T, resp *pb.ApplyEditsResponse, strict bool) {
	t.Helper()
	if !strict {
		if resp.Failure != pb.EditFailure_EDIT_FAILURE_UNSPECIFIED || resp.Error != "" {
			t.Fatalf("default-mode edit refused (%s): %s", resp.Failure, resp.Error)
		}
		if len(resp.Documents) != 1 || !strings.Contains(resp.Documents[0].Content, "part def V {\n        require constraint c") {
			t.Fatalf("documents = %v, want the constraint moved into V", resp.Documents)
		}
		return
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_RESULT_INVALID {
		t.Fatalf("strict-mode failure = %s (%s), want RESULT_INVALID", resp.Failure, resp.Error)
	}
	if !strings.Contains(resp.Error, "OpenSysML extension") {
		t.Fatalf("strict-mode error %q does not name the extension", resp.Error)
	}
	if resp.Content != "" || len(resp.Documents) != 0 || len(resp.Applied) != 0 {
		t.Fatalf("a refusal returned notation: content=%q documents=%v applied=%v", resp.Content, resp.Documents, resp.Applied)
	}
	var found *pb.Diagnostic
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "OpenSysML extension") {
			found = d
		}
	}
	if found == nil || found.Severity != "error" {
		t.Fatalf("diagnostics = %v, want the extension reported as an error", resp.Diagnostics)
	}
}

// notationDiagnostic is the response's single nonstandard-notation finding.
func notationDiagnostic(t *testing.T, resp *pb.ParseFileResponse) *pb.Diagnostic {
	t.Helper()
	var found []*pb.Diagnostic
	for _, d := range resp.Diagnostics {
		if strings.Contains(d.Message, "OpenSysML extension") {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("got %d extension diagnostic(s) in %+v, want 1", len(found), resp.Diagnostics)
	}
	return found[0]
}
