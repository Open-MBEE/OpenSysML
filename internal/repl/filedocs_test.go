package repl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// A file loaded from the command line is a document of its own: a root-level
// import in one file surfaces its names in that file's root namespace only, so
// the other files on the command line do not see them, exactly as the editor
// and the workspace report it.
func TestLoadedFilesDoNotShareRootImports(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeFile(t, filepath.Join(dir, "a.sysml"), "import ScalarValues::*;\npackage A { attribute x : Real; }\n"),
		writeFile(t, filepath.Join(dir, "b.sysml"), "package B { attribute y : Real; }\n"),
	}
	s := NewSession()
	if _, err := s.LoadFilesSummary(paths); err != nil {
		t.Fatal(err)
	}
	var inB []string
	for _, d := range s.LocatedDiagnostics() {
		if d.File == paths[1] {
			inB = append(inB, d.Message)
		}
	}
	if len(inB) != 1 || !strings.Contains(inB[0], "unresolved reference: Real") {
		t.Errorf("b.sysml should not see a.sysml's root import; its diagnostics: %q", inB)
	}
	if got, want := cliDiagnostics(t, paths), workspaceDiagnostics(t, paths); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the CLI reported:\n%s\nwant, as the workspace does:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Two files declaring the same root package are two root namespaces of one name,
// not a duplicate; a reference resolves to the first declaration.
func TestLoadedFilesDeclaringOneRootPackageAreNotDuplicates(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeFile(t, filepath.Join(dir, "a.sysml"), "package A { part def X; }\n"),
		writeFile(t, filepath.Join(dir, "b.sysml"), "package A { part def Y; }\n"),
		writeFile(t, filepath.Join(dir, "c.sysml"), "package C { part x : A::X; }\n"),
	}
	s := NewSession()
	out, err := s.LoadFilesSummary(paths)
	if err != nil {
		t.Fatal(err)
	}
	if s.HasErrors() {
		t.Errorf("the files did not validate clean:\n%s", strings.Join(s.DiagnosticLines(), "\n"))
	}
	for _, line := range out {
		if strings.Contains(line, "Duplicate") {
			t.Errorf("a repeated root package was reported as a duplicate: %s", line)
		}
	}
	if got, want := cliDiagnostics(t, paths), workspaceDiagnostics(t, paths); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the CLI reported:\n%s\nwant, as the workspace does:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The prompt's transcript is a document of its own too: a root-level import in
// a loaded file does not serve what is typed after %load, though the file's
// root packages are reachable through the global namespace as before.
func TestPromptDoesNotSeeALoadedFilesRootImports(t *testing.T) {
	s := NewSession()
	path := tempFile(t, "a.sysml", "import ScalarValues::*;\npackage A { attribute x : Real; }\n")
	if _, _, err := s.runMeta("%load " + path); err != nil {
		t.Fatal(err)
	}
	res := s.Submit("package P { attribute y : Real; part a : A; }")
	var messages []string
	for _, d := range res.Diagnostics {
		if res.mine(d.Span) {
			messages = append(messages, d.Message)
		}
	}
	if len(messages) != 1 || !strings.Contains(messages[0], "unresolved reference: Real") {
		t.Errorf("the prompt should resolve A but not the file's import of Real; it reported %q", messages)
	}
}

// An error in a loaded file gates the deeper checks of that file only: a clean
// prompt submission is fully analyzed in its own document, so no blocker is named
// for it, and a later loaded file is not blocked by what the prompt holds either.
func TestLoadedFileErrorsDoNotBlockOtherDocuments(t *testing.T) {
	s := NewSession()
	bad := tempFile(t, "bad.sysml", "package Bad { part a : Missing; }\n")
	if _, _, err := s.runMeta("%load " + bad); err != nil {
		t.Fatal(err)
	}
	res := s.Submit("package Clean { part def A; }")
	if note := res.Blocked.note(); note != "" {
		t.Errorf("a loaded file's error should not block the prompt's document: %s", note)
	}

	s.Submit("package Typed { part b : Absent::B; }")
	good := tempFile(t, "good.sysml", "package Good { part def B; }\n")
	if note := s.submit(good, "package Good { part def B; }\n").Blocked.note(); note != "" {
		t.Errorf("the prompt's error should not block a loaded file's document: %s", note)
	}
	// Within the transcript, an earlier typed error still gates the deeper checks,
	// and is named once: a load in between does not make it worth saying again.
	if s.Submit("package Also { part def C; }").Blocked.note() == "" {
		t.Error("the typed unresolved reference should still be named as blocking the prompt")
	}
	s.submit(good, "package Good { part def B; }\n")
	if note := s.Submit("package More { part def D; }").Blocked.note(); note != "" {
		t.Errorf("the standing error was named already; a load does not renew it: %s", note)
	}
	// A load that resolves the standing error ends its interval: should a reload
	// bring the error back, the next prompt is told again.
	s.submit(good, "package Absent { part def B; }\n")
	s.submit(good, "package Good { part def B; }\n")
	if s.Submit("package Yet { part def E; }").Blocked.note() == "" {
		t.Error("an error resolved by a load and brought back by a reload should be named again")
	}
}

// Every multi-file directory of the fixtures and of the OMG corpora reports the
// same diagnostics loaded from the command line as opened in a workspace.
func TestCommandLineLoadMatchesWorkspace(t *testing.T) {
	roots := []struct {
		dir     string
		require string // set in CI, where an absent corpus fails instead of skipping
		fetch   string
	}{
		{dir: "../../testdata"},
		{dir: "../../examples"},
		{
			dir:     "../../examples/sysml-v2-training",
			require: "OPENSYSML_REQUIRE_TRAINING_CORPUS",
			fetch:   "./scripts/download-training-examples.sh",
		},
		{
			dir:     "../../examples/pilot-corpora/kerml-examples",
			require: "OPENSYSML_REQUIRE_PILOT_CORPORA",
			fetch:   "./scripts/download-pilot-corpora.sh",
		},
		{
			dir:     "../../examples/pilot-corpora/sysml-examples",
			require: "OPENSYSML_REQUIRE_PILOT_CORPORA",
			fetch:   "./scripts/download-pilot-corpora.sh",
		},
		{
			dir:     "../../examples/pilot-corpora/sysml-validation",
			require: "OPENSYSML_REQUIRE_PILOT_CORPORA",
			fetch:   "./scripts/download-pilot-corpora.sh",
		},
	}
	seen := map[string]bool{}
	for _, root := range roots {
		if _, err := os.Stat(root.dir); os.IsNotExist(err) {
			if os.Getenv(root.require) != "" {
				t.Fatalf("%s is missing and %s is set; fetch it with %s", root.dir, root.require, root.fetch)
			}
			t.Logf("%s is absent (fetch it with %s), so this run proves nothing about it", root.dir, root.fetch)
			continue
		}
		for _, files := range modelDirectories(t, root.dir) {
			dir := filepath.Dir(files[0])
			if seen[dir] {
				continue
			}
			seen[dir] = true
			t.Run(filepath.ToSlash(dir), func(t *testing.T) {
				got, want := cliDiagnostics(t, files), workspaceDiagnostics(t, files)
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("the CLI reported:\n%s\nwant, as the workspace does:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
				}
			})
		}
	}
}

// modelDirectories walks root and returns the model files of every directory
// holding more than one, each directory's files sorted, the corpora's directories
// under a root that contains them included.
func modelDirectories(t *testing.T, root string) [][]string {
	t.Helper()
	byDir := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || source.KindOf(path) == source.KindUnknown {
			return nil
		}
		byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	dirs := make([]string, 0, len(byDir))
	for dir, files := range byDir {
		if len(files) > 1 {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	out := make([][]string, 0, len(dirs))
	for _, dir := range dirs {
		files := byDir[dir]
		sort.Strings(files)
		out = append(out, files)
	}
	return out
}

// cliDiagnostics loads the files as the command line does and returns what it
// reports, one sorted line per diagnostic.
func cliDiagnostics(t *testing.T, paths []string) []string {
	t.Helper()
	s := NewSession()
	if _, err := s.LoadFilesSummary(paths); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range s.LocatedDiagnostics() {
		out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s [%s]", filepath.Base(d.File), d.Line, d.Column, d.Severity, d.Message, d.Code))
	}
	sort.Strings(out)
	return out
}

// workspaceDiagnostics opens the files in one workspace, as the editor and the
// corpus gates do, and returns what it reports in the same form as cliDiagnostics.
func workspaceDiagnostics(t *testing.T, paths []string) []string {
	t.Helper()
	ws := model.NewWorkspace()
	contents := make(map[string][]byte, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents[path] = content
		ws.Open(path, content, 1)
	}
	var out []string
	for _, path := range paths {
		lines := source.New(path, contents[path]).Lines()
		for _, d := range ws.Diagnostics(path) {
			p := lines.PosAt(d.Span.Offset)
			out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s [%s]", filepath.Base(path), p.Line, p.Col, d.Severity, d.Message, d.Code))
		}
	}
	sort.Strings(out)
	return out
}
