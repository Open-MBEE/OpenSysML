package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

const (
	// keywordAsName writes a reserved keyword where the grammar's ID terminal
	// admits only a name; the parser warns and the notation pass escalates.
	keywordAsName = "package P { part def X; part def M { part filter : X; } }"
	// quotedKeywordName spells the same name as the unrestricted name the
	// grammar admits.
	quotedKeywordName = "package P { part def X; part def M { part 'filter' : X; } }"
	// misplacedComment is notation the parser reads but the grammar does not
	// admit: the parser warns, and strict conformance escalates the warning.
	misplacedComment = "package P { calc def F { 1 /* c */ } }"
	// keywordAndSyntaxError has a syntax error alongside the keyword name, so
	// the parse's errors and warnings are both reported. The service analyzes
	// a clean parse only, so the warning is not escalated as the CLI's is.
	keywordAndSyntaxError = "package P { part def X; part def M { part filter : X; part q : ; } }"
)

// codeSeverity spells one diagnostic as the pair a client keys on.
func codeSeverity(d *pb.Diagnostic) string { return d.Code + "/" + strings.ToLower(d.Severity) }

// codeSeverities lists every diagnostic of a response as code/severity, sorted.
func codeSeverities(diags []*pb.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, codeSeverity(d))
	}
	sort.Strings(out)
	return out
}

// The service reports a reserved keyword written as a name the way the CLI
// does: as an error coded reserved-keyword-name, in every conformance mode.
func TestParseFileReportsReservedKeywordName(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	for _, strict := range []bool{false, true} {
		t.Run(fmt.Sprintf("strict_conformance=%v", strict), func(t *testing.T) {
			resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: keywordAsName},
				StrictConformance: strict,
			})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if resp.Error != "" {
				t.Fatalf("ParseFile error %q, want none", resp.Error)
			}
			got := codeSeverities(resp.Diagnostics)
			want := []string{"reserved-keyword-name/error"}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("diagnostics = %v, want %v\n%v", got, want, resp.Diagnostics)
			}
			if d := resp.Diagnostics[0]; !strings.Contains(d.Message, "reserved keyword") {
				t.Errorf("message %q does not name the reserved keyword", d.Message)
			}

			clean, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: quotedKeywordName},
				StrictConformance: strict,
			})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if len(clean.Diagnostics) != 0 {
				t.Errorf("quoted 'filter' reported %v, want no diagnostics", clean.Diagnostics)
			}
		})
	}
}

// ParseSources reports a reserved keyword written as a name in any of its
// documents, against the document that wrote it.
func TestParseSourcesReportsReservedKeywordName(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	resp, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents:         inlineDocuments("lib.sysml", sourcesLibrary, "kw.sysml", keywordAsName),
		StrictConformance: true,
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("ParseSources error %q, want none", resp.Error)
	}
	got := codeSeverities(resp.Diagnostics)
	if want := []string{"reserved-keyword-name/error"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("diagnostics = %v, want %v\n%v", got, want, resp.Diagnostics)
	}
	if file := resp.Diagnostics[0].Span.GetFile(); file != "kw.sysml" {
		t.Errorf("diagnostic reported against %q, want kw.sysml", file)
	}
}

// A parser nonstandard-notation warning is a warning by default and an error
// under strict conformance, reported once either way.
func TestParseFileStrictConformanceEscalatesParserNotationWarning(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	for _, tc := range []struct {
		strict bool
		want   string
	}{
		{false, "nonstandard-notation/warning"},
		{true, "nonstandard-notation/error"},
	} {
		t.Run(fmt.Sprintf("strict_conformance=%v", tc.strict), func(t *testing.T) {
			resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source:            &pb.ParseFileRequest_Content{Content: misplacedComment},
				StrictConformance: tc.strict,
			})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			got := codeSeverities(resp.Diagnostics)
			if strings.Join(got, ",") != tc.want {
				t.Fatalf("diagnostics = %v, want [%s]\n%v", got, tc.want, resp.Diagnostics)
			}
		})
	}
}

// Every diagnostic of a document is reported once, whether the parse was
// clean, warned, or failed: the parse's errors and warnings reach the client
// through the passes, not alongside them.
func TestModelDiagnosticsReportEachOnce(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()

	for name, tc := range map[string]struct {
		content string
		want    []string
	}{
		"warning":           {keywordAsName, []string{"reserved-keyword-name/error"}},
		"error_and_warning": {keywordAndSyntaxError, []string{"reserved-keyword-name/warning", "syntax/error"}},
		"comment":           {misplacedComment, []string{"nonstandard-notation/warning"}},
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
				Source: &pb.ParseFileRequest_Content{Content: tc.content},
			})
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			got := codeSeverities(resp.Diagnostics)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("diagnostics = %v, want %v\n%v", got, tc.want, resp.Diagnostics)
			}
			seen := map[string]int{}
			for _, d := range resp.Diagnostics {
				key := fmt.Sprintf("%s|%s|%d:%d", d.Code, d.Message, d.Span.GetStartLine(), d.Span.GetStartCol())
				seen[key]++
				if seen[key] > 1 {
					t.Errorf("diagnostic reported twice: %v", d)
				}
			}
			again, err := srv.GetDiagnostics(context.Background(), &pb.DiagnosticsRequest{ModelHash: resp.ModelHash})
			if err != nil {
				t.Fatalf("GetDiagnostics: %v", err)
			}
			if got, want := codeSeverities(again.Diagnostics), codeSeverities(resp.Diagnostics); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("GetDiagnostics = %v, want the ParseFile diagnostics %v", got, want)
			}
		})
	}
}

var (
	cliOnce sync.Once
	cliPath string
	cliErr  error
)

// sysmlCLI builds cmd/sysml once per test binary.
func sysmlCLI(t *testing.T) string {
	t.Helper()
	cliOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sysml-cli")
		if err != nil {
			cliErr = err
			return
		}
		cliPath = filepath.Join(dir, "sysml")
		build := exec.Command("go", gobuild.Args(cliPath)...)
		build.Dir = filepath.Join("..", "..", "..", "cmd", "sysml")
		if out, err := build.CombinedOutput(); err != nil {
			cliErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if cliErr != nil {
		t.Fatalf("building sysml: %v", cliErr)
	}
	return cliPath
}

// cliDiagnostics runs `sysml -validate -json` on the file and returns its
// diagnostics as code/severity, sorted.
func cliDiagnostics(t *testing.T, binary, file string, strict bool) []string {
	t.Helper()
	args := []string{"-validate", "-json"}
	if strict {
		args = append(args, "-strict")
	}
	out, err := exec.Command(binary, append(args, file)...).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || len(out) == 0 {
			t.Fatalf("sysml %v: %v\n%s", args, err, exit)
		}
	}
	var report struct {
		Diagnostics []struct {
			Severity string `json:"severity"`
			Code     string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("sysml -json output: %v\n%s", err, out)
	}
	codes := make([]string, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		codes = append(codes, d.Code+"/"+strings.ToLower(d.Severity))
	}
	sort.Strings(codes)
	return codes
}

// The CLI and the service report the same diagnostics, by code and severity,
// for the same document in the same conformance mode.
func TestParseFileMatchesCLIValidate(t *testing.T) {
	binary := sysmlCLI(t)
	srv := mustNewService(t, 10)
	defer srv.Close()

	dir := t.TempDir()
	for name, content := range map[string]string{
		"keyword.sysml": keywordAsName,
		"quoted.sysml":  quotedKeywordName,
		"comment.sysml": misplacedComment,
	} {
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/strict=%v", name, strict), func(t *testing.T) {
				want := cliDiagnostics(t, binary, file, strict)
				resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
					Source:            &pb.ParseFileRequest_Content{Content: content},
					StrictConformance: strict,
				})
				if err != nil {
					t.Fatalf("ParseFile: %v", err)
				}
				got := codeSeverities(resp.Diagnostics)
				if strings.Join(got, ",") != strings.Join(want, ",") {
					t.Errorf("gRPC reports %v, CLI reports %v\n%v", got, want, resp.Diagnostics)
				}
			})
		}
	}
}
