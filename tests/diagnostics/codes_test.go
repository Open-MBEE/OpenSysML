// Package diagnostics_test sweeps every model in the tree for the diagnostics
// it raises and holds their codes to the registry in internal/syntax/diag/codes.
package diagnostics_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/codes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const repoRoot = "../.."

// emission is where a code was first seen in the sweep: a file and the message.
type emission struct {
	file, message string
	source        string
	severity      diag.Severity
	count         int
}

// trackedModels lists the model files the repository tracks, so the downloaded
// corpora under examples/ (gitignored) are left out of the sweep.
func trackedModels(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z", "--", "*.sysml", "*.kerml")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

// sweep opens every tracked model, one workspace per directory as `sysml -validate dir/*`
// would, parses the embedded standard library as the conformance gate does, and
// tallies the codes of every diagnostic raised.
func sweep(t *testing.T) map[string]*emission {
	t.Helper()
	seen := map[string]*emission{}
	note := func(file string, d diag.Diagnostic) {
		e, ok := seen[d.Code]
		if !ok {
			e = &emission{file: file, message: d.Message, source: d.Source, severity: d.Severity}
			seen[d.Code] = e
		}
		e.count++
	}

	byDir := map[string][]string{}
	for _, f := range trackedModels(t) {
		byDir[filepath.Dir(f)] = append(byDir[filepath.Dir(f)], f)
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		ws := model.NewWorkspace()
		var inputs []model.Input
		for _, f := range byDir[dir] {
			content, err := os.ReadFile(filepath.Join(repoRoot, f))
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			inputs = append(inputs, model.Input{Name: f, Content: content, Version: 1, Kind: source.KindOf(f)})
		}
		ws.OpenAll(inputs)
		names := make([]string, len(inputs))
		for i, in := range inputs {
			names[i] = in.Name
		}
		for i, diags := range ws.DiagnosticsAll(names) {
			for _, d := range diags {
				note(names[i], d)
			}
		}
	}

	_, lib := libs.SharedLibrary()
	for _, name := range lib.List() {
		data, err := lib.Read(name)
		if err != nil {
			t.Fatalf("read library %s: %v", name, err)
		}
		p := parser.New(source.New(name, data))
		_ = p.ParseFile()
		for _, d := range parser.AsDiagnostics(p.Diagnostics, nil) {
			note("stdlib/"+name, d)
		}
	}
	return seen
}

// TestEveryEmittedCodeIsRegistered sweeps the tree's models and the standard
// library: every diagnostic raised carries a code the registry knows, and every
// registered code is raised by some model or, failing that, named by the
// production source that emits it.
func TestEveryEmittedCodeIsRegistered(t *testing.T) {
	seen := sweep(t)
	emitted := make([]string, 0, len(seen))
	for c := range seen {
		emitted = append(emitted, c)
	}
	sort.Strings(emitted)
	for _, c := range emitted {
		e := seen[c]
		t.Logf("%-50s %-18s %-8s %5d  %s: %s", fmt.Sprintf("%q", c), e.source, e.severity, e.count, e.file, e.message)
		if c == "" {
			t.Errorf("a diagnostic carries no code: %s: %s (%s)", e.file, e.message, e.source)
			continue
		}
		if !codes.Known(c) {
			t.Errorf("code %q is emitted (%s: %s) but not registered in internal/check/codes", c, e.file, e.message)
		}
	}
	t.Logf("%d distinct codes emitted", len(emitted))

	literals := productionLiterals(t)
	var unseen []string
	for _, c := range codes.Codes() {
		if _, ok := seen[c]; ok {
			continue
		}
		family, kind, _ := codes.Lookup(c)
		switch {
		case kind != "" && literals[family.Code] && literals[kind]:
			unseen = append(unseen, c)
		case kind == "" && literals[c]:
			unseen = append(unseen, c)
		default:
			t.Errorf("code %q is registered but no model in the tree raises it and no production source names it", c)
		}
	}
	t.Logf("%d registered codes are named by production sources only: %s", len(unseen), strings.Join(unseen, " "))
}

// productionLiterals collects the string literals of the module's production Go
// sources (tests, testdata and the registry itself left out), the evidence that a
// code has an emitter when no model in the tree happens to raise it.
func productionLiterals(t *testing.T) map[string]bool {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z", "--", "*.go", ":!:*_test.go", ":!:*/testdata/*", ":!:tools/*", ":!:internal/check/codes/*")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	literal := regexp.MustCompile(`"([a-z][a-z0-9]*(?:[.-][a-z0-9]+)*)"`)
	found := map[string]bool{}
	for _, f := range strings.Split(string(out), "\x00") {
		if f == "" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(repoRoot, f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range literal.FindAllSubmatch(src, -1) {
			found[string(m[1])] = true
		}
	}
	return found
}
