package grpc

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// The REPL imports this package for its feature-value serialization, so a test
// comparing the two runs as an external package and borrows these.
var (
	MustNewServiceForTest = mustNewService
	QueryModelForTest     = queryModel
)

const convertModelSource = `package Demo {
    // a comment, which notation keeps and RDF does not
part def Engine { attribute power : Real = 300.0; }
}
`

// mustConvert converts and fails the test on a transport error or a reported
// conversion error.
func mustConvert(t *testing.T, srv *Service, req *pb.ConvertRequest) *pb.ConvertResponse {
	t.Helper()
	resp, err := srv.Convert(context.Background(), req)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("Convert reported %q, diagnostics %v", resp.Error, resp.Diagnostics)
	}
	return resp
}

// TestConvertCapabilityReported verifies a client can require conversion before
// asking for it.
func TestConvertCapabilityReported(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	if !slices.Contains(info.Capabilities, CapabilityConvert) {
		t.Errorf("capabilities = %v, want it to contain %q", info.Capabilities, CapabilityConvert)
	}
}

// TestConvertNotationRoundTrip verifies notation written back out parses to the
// same model and keeps its comments.
func TestConvertNotationRoundTrip(t *testing.T) {
	srv := mustNewService(t, 10)

	resp := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: convertModelSource},
		FromFormat: "sysml",
		ToFormat:   "sysml",
	})
	if !strings.Contains(resp.Content, "part def Engine") {
		t.Errorf("output lost the model:\n%s", resp.Content)
	}
	if !strings.Contains(resp.Content, "// a comment") {
		t.Errorf("notation output dropped a lexical comment:\n%s", resp.Content)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none for a clean model", resp.Diagnostics)
	}

	// Converting the output again is stable: the formatter has a fixpoint.
	again := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: resp.Content},
		FromFormat: "sysml",
		ToFormat:   "sysml",
	})
	if again.Content != resp.Content {
		t.Errorf("second conversion differs:\n%s\nvs\n%s", again.Content, resp.Content)
	}
}

// TestConvertToTurtleAndBack verifies a model survives a trip through RDF.
func TestConvertToTurtleAndBack(t *testing.T) {
	srv := mustNewService(t, 10)

	turtle := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: convertModelSource},
		FromFormat: "sysml",
		ToFormat:   "turtle",
	})
	if turtle.ToFormat != "ttl" {
		t.Errorf("to_format = %q, want the canonical %q", turtle.ToFormat, "ttl")
	}
	if !strings.Contains(turtle.Content, "Demo::Engine") {
		t.Errorf("graph does not name the model's element:\n%s", turtle.Content)
	}

	back := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: turtle.Content},
		FromFormat: "ttl",
		ToFormat:   "sysml",
	})
	if !strings.Contains(back.Content, "part def Engine") {
		t.Errorf("notation from the graph lost the model:\n%s", back.Content)
	}
}

// TestConvertMarksRDFExperimental verifies the response carries the RDF
// mapping's status in both directions, carries it on a refusal too, and leaves
// it off a notation conversion.
func TestConvertMarksRDFExperimental(t *testing.T) {
	srv := mustNewService(t, 10)

	turtle := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: convertModelSource},
		FromFormat: "sysml",
		ToFormat:   "ttl",
	})
	if !turtle.Experimental {
		t.Error("experimental = false, want true for a conversion to RDF")
	}
	if !strings.Contains(turtle.ExperimentalNotice, "experimental") {
		t.Errorf("experimental_notice = %q, want it to state the status", turtle.ExperimentalNotice)
	}

	back := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: turtle.Content},
		FromFormat: "ttl",
		ToFormat:   "sysml",
	})
	if !back.Experimental {
		t.Error("reading RDF is experimental too, but was not marked")
	}

	notation := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: convertModelSource},
		FromFormat: "sysml",
		ToFormat:   "sysml",
	})
	if notation.Experimental || notation.ExperimentalNotice != "" {
		t.Errorf("a notation conversion is stable, but was marked: %t %q",
			notation.Experimental, notation.ExperimentalNotice)
	}

	refused, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: "package P { part def Seat; part seat : Seat; part seat : Seat; }"},
		FromFormat: "sysml",
		ToFormat:   "ttl",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if refused.Error == "" {
		t.Fatalf("expected the mapping to refuse the duplicate declaration:\n%s", refused.Content)
	}
	if !refused.Experimental {
		t.Error("a refusal is the experimental behavior, but was not marked")
	}
}

// TestConvertFilePathInfersFormat verifies a path source is read by the service
// and its extension names the input format.
func TestConvertFilePathInfersFormat(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(convertModelSource), 0o600); err != nil {
		t.Fatal(err)
	}

	resp := mustConvert(t, srv, &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_FilePath{FilePath: path},
		ToFormat: "ttl",
	})
	if resp.FromFormat != "sysml" {
		t.Errorf("from_format = %q, want it inferred as %q", resp.FromFormat, "sysml")
	}
}

// TestConvertModelHashConvertsWhatWasParsed verifies a hash converts the source
// the service parsed, so a file edited since the parse does not change it.
func TestConvertModelHashConvertsWhatWasParsed(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(convertModelSource), 0o600); err != nil {
		t.Fatal(err)
	}

	parsed, err := srv.ParseFile(context.Background(),
		&pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: path}})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("package Replaced { part def Other; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resp := mustConvert(t, srv, &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash},
		ToFormat: "sysml",
	})
	if resp.FromFormat != "sysml" {
		t.Errorf("from_format = %q, want notation without being told", resp.FromFormat)
	}
	if !strings.Contains(resp.Content, "part def Engine") || strings.Contains(resp.Content, "Replaced") {
		t.Errorf("converted the file as it stands, not the model parsed:\n%s", resp.Content)
	}

	// The path source, by contrast, is read afresh.
	current := mustConvert(t, srv, &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_FilePath{FilePath: path},
		ToFormat: "sysml",
	})
	if !strings.Contains(current.Content, "Replaced") {
		t.Errorf("file_path did not read the file as it stands:\n%s", current.Content)
	}
}

// TestConvertUncachedModelHashIsNotFound verifies an evicted or unknown model is
// named as such rather than converted as empty notation.
func TestConvertUncachedModelHashIsNotFound(t *testing.T) {
	srv := mustNewService(t, 10)

	_, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: "nosuchmodel"},
		ToFormat: "sysml",
	})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("err = %v, want NotFound", err)
	}
}

// TestConvertInlineContentNeedsFromFormat verifies inline content, which has no
// extension to infer from, is rejected rather than guessed at.
func TestConvertInlineContentNeedsFromFormat(t *testing.T) {
	srv := mustNewService(t, 10)

	_, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_Content{Content: convertModelSource},
		ToFormat: "ttl",
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("err = %v, want InvalidArgument", err)
	}
}

// TestConvertRejectsBadArguments verifies argument faults fail the call instead
// of being reported as a conversion that did not work.
func TestConvertRejectsBadArguments(t *testing.T) {
	srv := mustNewService(t, 10)
	content := &pb.ConvertRequest_Content{Content: convertModelSource}

	cases := map[string]*pb.ConvertRequest{
		"no source":        {ToFormat: "sysml", FromFormat: "sysml"},
		"no to_format":     {Source: content, FromFormat: "sysml"},
		"unknown to":       {Source: content, FromFormat: "sysml", ToFormat: "docx"},
		"unknown from":     {Source: content, FromFormat: "docx", ToFormat: "sysml"},
		"xmi as target":    {Source: content, FromFormat: "sysml", ToFormat: "xmi"},
		"missing file":     {Source: &pb.ConvertRequest_FilePath{FilePath: "/nonexistent/model.sysml"}, ToFormat: "ttl"},
		"unknown ext":      {Source: &pb.ConvertRequest_FilePath{FilePath: "model.json"}, ToFormat: "ttl"},
		"no format at all": {Source: content},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := srv.Convert(context.Background(), req); err == nil {
				t.Fatal("Convert accepted a request it cannot serve")
			} else if code := connect.CodeOf(err); code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
				t.Errorf("code = %v, want InvalidArgument or NotFound", code)
			}
		})
	}
}

// TestConvertReportsSyntaxErrors verifies a model the parser cannot read fails
// the conversion with its diagnostics, spans and all.
func TestConvertReportsSyntaxErrors(t *testing.T) {
	srv := mustNewService(t, 10)

	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: "package P { part def "},
		FromFormat: "sysml",
		ToFormat:   "ttl",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("unreadable notation converted to a graph without complaint")
	}
	if len(resp.Diagnostics) == 0 {
		t.Fatal("no diagnostics for a syntax error")
	}
	for _, diag := range resp.Diagnostics {
		if diag.Message == "" || diag.Span == nil {
			t.Errorf("diagnostic %v carries no message or span", diag)
		}
	}
}

// TestConvertTolerantWritesNotationAnyway verifies tolerated syntax errors are
// reported as diagnostics alongside the output, and only for notation output.
func TestConvertTolerantWritesNotationAnyway(t *testing.T) {
	srv := mustNewService(t, 10)
	broken := &pb.ConvertRequest_Content{Content: "package P { part def }"}

	resp := mustConvert(t, srv, &pb.ConvertRequest{
		Source:               broken,
		FromFormat:           "sysml",
		ToFormat:             "sysml",
		TolerateSyntaxErrors: true,
	})
	if resp.Content == "" {
		t.Error("tolerant notation conversion wrote nothing")
	}
	if len(resp.Diagnostics) == 0 {
		t.Error("tolerant conversion hid the syntax errors it tolerated")
	}

	// Tolerance does not extend to a graph, where a declaration the parser
	// could not read would simply be absent.
	graph, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:               broken,
		FromFormat:           "sysml",
		ToFormat:             "ttl",
		TolerateSyntaxErrors: true,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if graph.Error == "" {
		t.Error("tolerated syntax errors into RDF, which would drop declarations")
	}
}

// TestConvertMigratesXMI checks the service reads SysML v1 XMI: inline content
// with from_format xmi, and a .xmi file whose format is inferred.
func TestConvertMigratesXMI(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join("..", "core", "migrate", "testdata", "xmi", "vehicle.xmi")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, req := range map[string]*pb.ConvertRequest{
		"content": {Source: &pb.ConvertRequest_Content{Content: string(data)}, FromFormat: "xmi", ToFormat: "sysml"},
		"file":    {Source: &pb.ConvertRequest_FilePath{FilePath: path}, ToFormat: "sysml"},
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := srv.Convert(context.Background(), req)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if resp.Error != "" {
				t.Fatalf("conversion refused: %s", resp.Error)
			}
			if resp.FromFormat != "xmi" || !resp.Experimental || resp.ExperimentalNotice != export.MigrationNotice {
				t.Errorf("from_format %q, experimental %v, notice %q; want xmi, true, the migration notice", resp.FromFormat, resp.Experimental, resp.ExperimentalNotice)
			}
			if !strings.Contains(resp.Content, "part def Vehicle") {
				t.Errorf("no migrated notation:\n%s", resp.Content)
			}
		})
	}
}
