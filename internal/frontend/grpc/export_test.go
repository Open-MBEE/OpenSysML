package grpc

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
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
		Source: &pb.ConvertRequest_Content{Content: `package Demo {
			@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
			attribute origin = "el-";
			part def A {
				@IdentityMetadata::ElementId { id = origin; }
			}
		}`},
		FromFormat: "sysml",
		ToFormat:   "ttl",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if refused.Error == "" {
		t.Fatalf("expected the mapping to refuse a non-constant ElementId:\n%s", refused.Content)
	}
	if !refused.Experimental {
		t.Error("a refusal is the experimental behavior, but was not marked")
	}

	const positionalName = "package P { part def A; part def '@2'; part def A; }"
	converted := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: positionalName},
		FromFormat: "sysml",
		ToFormat:   "ttl",
	})
	if !strings.Contains(converted.Content, `sysml:qualifiedName "P::'@2'"`) {
		t.Errorf("Turtle lacks the quoted name identity:\n%s", converted.Content)
	}
	positionalBack := mustConvert(t, srv, &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: converted.Content},
		FromFormat: "ttl",
		ToFormat:   "sysml",
	})
	if strings.Count(positionalBack.Content, "part def A") != 2 || !strings.Contains(positionalBack.Content, "part def '@2'") {
		t.Errorf("the positional-name model did not come back:\n%s", positionalBack.Content)
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

// TestConvertRefusesSysMLv1 verifies a v1 model is refused by Convert with the
// help every surface gives: it is migrated, by Migrate, not converted — whether
// from_format names a v1 form or the file's extension does.
func TestConvertRefusesSysMLv1(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, req := range map[string]*pb.ConvertRequest{
		"content xmi":   {Source: &pb.ConvertRequest_Content{Content: string(data)}, FromFormat: "xmi", ToFormat: "sysml"},
		"content uml":   {Source: &pb.ConvertRequest_Content{Content: string(data)}, FromFormat: "uml", ToFormat: "sysml"},
		"content mdzip": {Source: &pb.ConvertRequest_Content{Content: string(data)}, FromFormat: "mdzip", ToFormat: "ttl"},
		"file":          {Source: &pb.ConvertRequest_FilePath{FilePath: path}, ToFormat: "sysml"},
		"missing file":  {Source: &pb.ConvertRequest_FilePath{FilePath: "/nonexistent/Model.mdzip"}, ToFormat: "sysml"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := srv.Convert(context.Background(), req)
			if err == nil {
				t.Fatal("Convert migrated a SysML v1 model")
			}
			if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", code)
			}
			msg := connectMessage(err)
			if !strings.Contains(msg, convert.MigratedNotConverted) || !strings.Contains(msg, "call Migrate") {
				t.Errorf("message = %q, want the migrated-not-converted help naming Migrate", msg)
			}
		})
	}
}

// connectMessage is a status error's own wording, without its code.
func connectMessage(err error) string {
	var cerr *connect.Error
	if errors.As(err, &cerr) {
		return cerr.Message()
	}
	return err.Error()
}

// TestMigrateMigratesXMI verifies Migrate reads a v1 model as inline bytes
// with from_format xmi, and as a .xmi file whose form is inferred, and
// accounts for the migration: the summary and counts always, every element's
// verdict and the report text only when asked for.
func TestMigrateMigratesXMI(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, req := range map[string]*pb.MigrateRequest{
		"content":       {Source: &pb.MigrateRequest_Content{Content: data}, FromFormat: "xmi", ToFormat: "sysml"},
		"content uml":   {Source: &pb.MigrateRequest_Content{Content: data}, FromFormat: "uml", ToFormat: "sysml"},
		"file":          {Source: &pb.MigrateRequest_FilePath{FilePath: path}, ToFormat: "sysml"},
		"file reported": {Source: &pb.MigrateRequest_FilePath{FilePath: path}, ToFormat: "sysml", Report: true},
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := srv.Migrate(context.Background(), req)
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			if resp.Error != "" {
				t.Fatalf("migration refused: %s", resp.Error)
			}
			if resp.FromFormat != "xmi" || resp.ToFormat != "sysml" || !resp.Experimental || resp.ExperimentalNotice != convert.MigrationNotice {
				t.Errorf("from %q to %q, experimental %v, notice %q; want xmi, sysml, true, the migration notice",
					resp.FromFormat, resp.ToFormat, resp.Experimental, resp.ExperimentalNotice)
			}
			if !strings.Contains(resp.Content, "part def Vehicle") {
				t.Errorf("no migrated notation:\n%s", resp.Content)
			}
			report := resp.Report
			if report == nil {
				t.Fatal("no migration report")
			}
			if report.Mapped == 0 || !strings.HasPrefix(report.Summary, "migrated ") || !strings.Contains(report.Summary, "mapped") {
				t.Errorf("report counts %d/%d/%d/%d, summary %q; want mapped elements and the summary line",
					report.Mapped, report.Approximated, report.Unmapped, report.Skipped, report.Summary)
			}
			if report.Source == "" {
				t.Error("the report names no source")
			}
			if !req.Report {
				if len(report.Entries) != 0 || report.Text != "" {
					t.Errorf("entries and text came back unasked: %d entries, %q", len(report.Entries), report.Text)
				}
				return
			}
			if len(report.Entries) == 0 || !strings.HasPrefix(report.Text, "# SysML v1 to v2 migration report") {
				t.Fatalf("asked for the full report, got %d entries and text %q", len(report.Entries), report.Text)
			}
			verdicts := map[string]int{}
			for _, entry := range report.Entries {
				verdicts[entry.Verdict]++
				if entry.Id == "" || entry.Kind == "" {
					t.Errorf("entry without an id or kind: %+v", entry)
				}
			}
			if verdicts["mapped"] != int(report.Mapped) || verdicts["approximated"] != int(report.Approximated) ||
				verdicts["unmapped"] != int(report.Unmapped) || verdicts["skipped"] != int(report.Skipped) {
				t.Errorf("entry verdicts %v do not add up to the counts %d/%d/%d/%d",
					verdicts, report.Mapped, report.Approximated, report.Unmapped, report.Skipped)
			}
			if len(verdicts) != len(slices.DeleteFunc(slices.Collect(maps.Keys(verdicts)), func(v string) bool {
				return v != "mapped" && v != "approximated" && v != "unmapped" && v != "skipped"
			})) {
				t.Errorf("an entry carries a verdict outside the four: %v", verdicts)
			}
			if resp.Results != "" || len(resp.Files) != 0 {
				t.Errorf("results %q and %d files came back unasked", resp.Results, len(resp.Files))
			}
		})
	}
}

// TestMigrateLaysOutFromMTIP verifies an MTIP export, by path or inline, lays
// the migrated views out, and that the report says so.
func TestMigrateLaysOutFromMTIP(t *testing.T) {
	srv := mustNewService(t, 10)
	dir := filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi")
	layoutPath := filepath.Join(dir, "layout.layout.xml")
	layoutData, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, layout := range map[string]func(*pb.MigrateRequest){
		"path": func(req *pb.MigrateRequest) {
			req.Layout = &pb.MigrateRequest_LayoutPath{LayoutPath: layoutPath}
		},
		"inline": func(req *pb.MigrateRequest) {
			req.Layout = &pb.MigrateRequest_LayoutContent{LayoutContent: string(layoutData)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := &pb.MigrateRequest{
				Source:   &pb.MigrateRequest_FilePath{FilePath: filepath.Join(dir, "layout.xmi")},
				ToFormat: "sysml",
			}
			layout(req)
			resp, err := srv.Migrate(context.Background(), req)
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			if resp.Error != "" {
				t.Fatalf("migration refused: %s", resp.Error)
			}
			if !strings.Contains(resp.Report.Summary, "laid out") {
				t.Errorf("summary %q does not account for the layout", resp.Report.Summary)
			}
			if !strings.Contains(resp.Content, "DiagramLayout") {
				t.Errorf("no layout in the migrated notation:\n%s", resp.Content)
			}
		})
	}

	_, err = srv.Migrate(context.Background(), &pb.MigrateRequest{
		Source:   &pb.MigrateRequest_FilePath{FilePath: filepath.Join(dir, "layout.xmi")},
		ToFormat: "sysml",
		Layout:   &pb.MigrateRequest_LayoutContent{LayoutContent: "<not mtip"},
	})
	if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
		t.Errorf("an unreadable layout: err = %v, want InvalidArgument", err)
	}
}

// TestMigrateAttachesImageFilesAndResults verifies the image files an archive
// carries come back beside the notation, and the result index comes back when
// asked for, as `sysml -migrate` writes them.
func TestMigrateAttachesImageFilesAndResults(t *testing.T) {
	srv := mustNewService(t, 10)
	xmi, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "documents.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	fleet := []byte("\x89PNG\r\n\x1a\n fleet bytes")
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, content := range map[string][]byte{"documents.xmi": xmi, "attachments/fleet.png": fleet} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Migrate(context.Background(), &pb.MigrateRequest{
		Source:     &pb.MigrateRequest_Content{Content: archive.Bytes()},
		FromFormat: "mdzip",
		ToFormat:   "sysml",
		Results:    true,
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("migration refused: %s", resp.Error)
	}
	if resp.FromFormat != "xmi" {
		t.Errorf("from_format = %q, want xmi for an archive", resp.FromFormat)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "images/fleet.png" || !bytes.Equal(resp.Files[0].Content, fleet) {
		t.Errorf("files = %v, want images/fleet.png with the archive's bytes", resp.Files)
	}
	if !strings.Contains(resp.Content, "images/fleet.png") {
		t.Errorf("the notation does not refer to the image file:\n%s", resp.Content)
	}
	if !strings.Contains(resp.Report.Summary, "wrote 1 image file(s)") {
		t.Errorf("summary %q does not count the image file", resp.Report.Summary)
	}
	var results map[string]any
	if err := json.Unmarshal([]byte(resp.Results), &results); err != nil {
		t.Fatalf("results are not a JSON object: %v\n%s", err, resp.Results)
	}
}

// TestMigrateStrictWritesNoExtensionNotation verifies strict writes only
// pinned SysML v2 notation, reporting the extension-only constructs unmapped.
func TestMigrateStrictWritesNoExtensionNotation(t *testing.T) {
	srv := mustNewService(t, 10)
	path := filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "decision_property_probability.xmi")
	lax, err := srv.Migrate(context.Background(), &pb.MigrateRequest{
		Source: &pb.MigrateRequest_FilePath{FilePath: path}, ToFormat: "sysml",
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	strict, err := srv.Migrate(context.Background(), &pb.MigrateRequest{
		Source: &pb.MigrateRequest_FilePath{FilePath: path}, ToFormat: "sysml", Strict: true,
	})
	if err != nil {
		t.Fatalf("Migrate -strict: %v", err)
	}
	if lax.Error != "" || strict.Error != "" {
		t.Fatalf("migration refused: %q / %q", lax.Error, strict.Error)
	}
	for _, line := range strings.Split(strict.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, kw := range []string{"defer ", "choice ", "junction ", "history ", "deep history "} {
			if strings.HasPrefix(trimmed, kw) {
				t.Errorf("strict migration wrote an extension statement %q", trimmed)
			}
		}
	}
	if lax.Content == strict.Content {
		t.Error("strict changed nothing in a model whose probability names a property")
	}
}

// TestMigrateRejectsBadArguments verifies argument faults fail the call, and
// that a v2 model is refused: it is converted, not migrated.
func TestMigrateRejectsBadArguments(t *testing.T) {
	srv := mustNewService(t, 10)
	xmi := filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi")
	notation := &pb.MigrateRequest_Content{Content: []byte(convertModelSource)}
	v1 := &pb.MigrateRequest_FilePath{FilePath: xmi}

	cases := map[string]struct {
		req  *pb.MigrateRequest
		code connect.Code
		want string
	}{
		"no source":            {&pb.MigrateRequest{ToFormat: "sysml", FromFormat: "xmi"}, connect.CodeInvalidArgument, "source"},
		"content without from": {&pb.MigrateRequest{Source: notation, ToFormat: "sysml"}, connect.CodeInvalidArgument, "from_format"},
		"v2 content":           {&pb.MigrateRequest{Source: notation, FromFormat: "sysml", ToFormat: "sysml"}, connect.CodeInvalidArgument, "converted, not migrated"},
		"v2 file":              {&pb.MigrateRequest{Source: &pb.MigrateRequest_FilePath{FilePath: "model.sysml"}, ToFormat: "ttl"}, connect.CodeInvalidArgument, "converted, not migrated"},
		"unknown from":         {&pb.MigrateRequest{Source: notation, FromFormat: "docx", ToFormat: "sysml"}, connect.CodeInvalidArgument, "docx"},
		"no to_format":         {&pb.MigrateRequest{Source: v1}, connect.CodeInvalidArgument, "to_format"},
		"unknown to":           {&pb.MigrateRequest{Source: v1, ToFormat: "docx"}, connect.CodeInvalidArgument, "docx"},
		"xmi as target":        {&pb.MigrateRequest{Source: v1, ToFormat: "xmi"}, connect.CodeInvalidArgument, "xmi"},
		"missing file":         {&pb.MigrateRequest{Source: &pb.MigrateRequest_FilePath{FilePath: "/nonexistent/Model.mdzip"}, ToFormat: "sysml"}, connect.CodeNotFound, "Model.mdzip"},
		"unknown ext":          {&pb.MigrateRequest{Source: &pb.MigrateRequest_FilePath{FilePath: "model.json"}, ToFormat: "sysml"}, connect.CodeInvalidArgument, "model.json"},
		"missing layout":       {&pb.MigrateRequest{Source: v1, ToFormat: "sysml", Layout: &pb.MigrateRequest_LayoutPath{LayoutPath: "/nonexistent/Model_mtip.xml"}}, connect.CodeNotFound, "Model_mtip.xml"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := srv.Migrate(context.Background(), tc.req)
			if err == nil {
				t.Fatal("Migrate accepted a request it cannot serve")
			}
			if code := connect.CodeOf(err); code != tc.code {
				t.Errorf("code = %v, want %v: %v", code, tc.code, err)
			}
			if msg := connectMessage(err); !strings.Contains(msg, tc.want) {
				t.Errorf("message = %q, want it to mention %q", msg, tc.want)
			}
		})
	}
}
