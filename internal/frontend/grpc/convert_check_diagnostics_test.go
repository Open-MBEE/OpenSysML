package grpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// The models the semantic check reports on deterministically: a duplicate
// owned member name is a warning, an unresolved type an error. Both parse.
const (
	nameConflictModel   = "package P {\n  part def A;\n  part def A;\n}\n"
	unresolvedTypeModel = "package P {\n  part x : NoSuchType;\n}\n"
)

func convertContent(t *testing.T, srv *Service, content string, check bool) *pb.ConvertResponse {
	t.Helper()
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:                  &pb.ConvertRequest_Content{Content: content},
		FromFormat:              "sysml",
		ToFormat:                "api-json",
		IncludeCheckDiagnostics: check,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return resp
}

func severitiesByCode(diags []*pb.Diagnostic) map[string]string {
	codes := map[string]string{}
	for _, d := range diags {
		codes[d.Code] = d.Severity
	}
	return codes
}

// Without the field, a model the check only warns about or finds an
// unresolved type in converts with no diagnostics, as it always has.
func TestConvertCheckDiagnosticsDefaultUnchanged(t *testing.T) {
	srv := mustNewService(t, 10)
	for name, content := range map[string]string{"warning": nameConflictModel, "unresolved": unresolvedTypeModel} {
		resp := convertContent(t, srv, content, false)
		if resp.Error != "" || resp.Content == "" {
			t.Fatalf("%s: default Convert error %q, content %d bytes", name, resp.Error, len(resp.Content))
		}
		if len(resp.Diagnostics) != 0 {
			t.Fatalf("%s: default Convert reported %d diagnostics, want none", name, len(resp.Diagnostics))
		}
	}
}

// With the field, the check's warning is reported with its own severity and
// code, and the output is the one the default request writes.
func TestConvertCheckDiagnosticsWarning(t *testing.T) {
	srv := mustNewService(t, 10)
	plain := convertContent(t, srv, nameConflictModel, false)
	resp := convertContent(t, srv, nameConflictModel, true)
	if resp.Error != "" || resp.Content != plain.Content {
		t.Fatalf("opt-in changed the conversion: error %q, content equal %v", resp.Error, resp.Content == plain.Content)
	}
	if got := severitiesByCode(resp.Diagnostics); got["name-conflict"] != "warning" {
		t.Fatalf("diagnostics %v, want a name-conflict warning", resp.Diagnostics)
	}
	for _, d := range resp.Diagnostics {
		if d.Span == nil || d.Span.File != "<content>" || d.Span.StartLine == 0 {
			t.Fatalf("diagnostic without a located span: %v", d)
		}
	}
}

// An unresolved type is an error-severity check diagnostic, while the
// conversion itself still succeeds as it does without the field.
func TestConvertCheckDiagnosticsUnresolvedType(t *testing.T) {
	srv := mustNewService(t, 10)
	plain := convertContent(t, srv, unresolvedTypeModel, false)
	resp := convertContent(t, srv, unresolvedTypeModel, true)
	if resp.Error != "" || resp.Content != plain.Content {
		t.Fatalf("opt-in changed the conversion: error %q, content equal %v", resp.Error, resp.Content == plain.Content)
	}
	if got := severitiesByCode(resp.Diagnostics); got["unresolved"] != "error" {
		t.Fatalf("diagnostics %v, want an unresolved error", resp.Diagnostics)
	}
}

// A source with syntax errors reports those alone, whether the conversion
// refused it or tolerated it: the check passes do not run on it.
func TestConvertCheckDiagnosticsSyntaxErrorsOnly(t *testing.T) {
	srv := mustNewService(t, 10)
	broken := "package P {\n  part def A;\n  part def A\n"
	resp := convertContent(t, srv, broken, true)
	if resp.Error == "" || resp.Content != "" {
		t.Fatalf("broken notation converted: error %q", resp.Error)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity != "error" || d.Code == "name-conflict" {
			t.Fatalf("check diagnostic reported for unparsed notation: %v", d)
		}
	}
	tolerated, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:                  &pb.ConvertRequest_Content{Content: broken},
		FromFormat:              "sysml",
		ToFormat:                "sysml",
		TolerateSyntaxErrors:    true,
		IncludeCheckDiagnostics: true,
	})
	if err != nil {
		t.Fatalf("tolerant Convert: %v", err)
	}
	if tolerated.Error != "" || len(tolerated.Diagnostics) == 0 {
		t.Fatalf("tolerant Convert: error %q, %d diagnostics", tolerated.Error, len(tolerated.Diagnostics))
	}
	if got := severitiesByCode(tolerated.Diagnostics); got["name-conflict"] != "" {
		t.Fatalf("check diagnostic reported for tolerated syntax errors: %v", tolerated.Diagnostics)
	}
}

// A graph source has no notation to check: the field is refused for it.
func TestConvertCheckDiagnosticsRefusesGraphSource(t *testing.T) {
	srv := mustNewService(t, 10)
	plain := convertContent(t, srv, nameConflictModel, false)
	ttl, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_Content{Content: plain.Content}, FromFormat: "api-json", ToFormat: "sysml",
		IncludeCheckDiagnostics: true,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("api-json source with include_check_diagnostics: %v, %v; want INVALID_ARGUMENT", ttl, err)
	}
}

// A file is analyzed from the bytes the conversion read, by the format the
// request names: notation in a file with no notation extension still converts,
// and its check report comes with it.
func TestConvertCheckDiagnosticsFilePathByFromFormat(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join(t.TempDir(), "model.json")
	if err := os.WriteFile(path, []byte(nameConflictModel), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source: &pb.ConvertRequest_FilePath{FilePath: path}, FromFormat: "sysml", ToFormat: "sysml",
			IncludeCheckDiagnostics: check,
		})
		if err != nil || resp.Error != "" || resp.Content == "" {
			t.Fatalf("check %v: Convert of notation named .json: %v, error %q", check, err, resp.GetError())
		}
		warned := severitiesByCode(resp.Diagnostics)["name-conflict"] == "warning"
		if warned != check {
			t.Fatalf("check %v: diagnostics %v", check, resp.Diagnostics)
		}
		if check && resp.Diagnostics[0].Span.File != path {
			t.Fatalf("diagnostic located in %q, want %q", resp.Diagnostics[0].Span.File, path)
		}
	}
}

// A conversion the writer refuses still reports what the check found, since the
// model parsed: two documents declaring one package are refused as a graph,
// and the duplicate-name warning of one of them is reported with the refusal.
func TestConvertCheckDiagnosticsWithRefusedConversion(t *testing.T) {
	srv := mustNewService(t, 10)
	sources, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("warn.sysml", nameConflictModel, "again.sysml", "package P { part def B; }"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	for _, check := range []bool{false, true} {
		resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source: &pb.ConvertRequest_ModelHash{ModelHash: sources.ModelHash}, ToFormat: "api-json",
			IncludeCheckDiagnostics: check,
		})
		if err != nil || resp.Error == "" || resp.Content != "" {
			t.Fatalf("check %v: two declarations of P converted: %v, error %q", check, err, resp.GetError())
		}
		if (severitiesByCode(resp.Diagnostics)["name-conflict"] == "warning") != check {
			t.Fatalf("check %v: diagnostics %v", check, resp.Diagnostics)
		}
	}
}

// A model_hash reports the analysis its parse cached; with documents, the
// written documents' only.
func TestConvertCheckDiagnosticsCachedModel(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: nameConflictModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "api-json",
		IncludeCheckDiagnostics: true,
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("Convert of model_hash: %v, error %q", err, resp.GetError())
	}
	if got := severitiesByCode(resp.Diagnostics); got["name-conflict"] != "warning" {
		t.Fatalf("model_hash diagnostics %v, want a name-conflict warning", resp.Diagnostics)
	}

	sources, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("warn.sysml", nameConflictModel, "clean.sysml", "package Q { part def B; }"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	for _, tc := range []struct {
		documents []string
		warned    bool
	}{{nil, true}, {[]string{"warn.sysml"}, true}, {[]string{"clean.sysml"}, false}} {
		resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source: &pb.ConvertRequest_ModelHash{ModelHash: sources.ModelHash}, ToFormat: "api-json",
			Documents: tc.documents, IncludeCheckDiagnostics: true,
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("Convert documents %v: %v, error %q", tc.documents, err, resp.GetError())
		}
		if got := severitiesByCode(resp.Diagnostics)["name-conflict"] == "warning"; got != tc.warned {
			t.Fatalf("documents %v: diagnostics %v, warned %v want %v", tc.documents, resp.Diagnostics, got, tc.warned)
		}
	}
}
