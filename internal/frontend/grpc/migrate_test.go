//go:build !sysml_re && !sysml_nov1

package grpc

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
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
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
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
