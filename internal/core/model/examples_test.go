package model

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

const examplesDir = "../../../examples"

// examplesKnownFailures records the shipped examples that are expected to
// report errors, keyed by path with the reason. README and the guide point new
// users at examples/, so an example that does not analyse cleanly is a bug
// unless it is listed here on purpose.
var examplesKnownFailures = map[string]string{}

// Every example the repository ships must analyse cleanly, so the CLI's
// -validate exit status stays meaningful for the files the docs point at.
func TestExamplesAnalyseCleanly(t *testing.T) {
	files := exampleFiles(t)
	if len(files) == 0 {
		t.Fatalf("no examples found under %s", examplesDir)
	}

	for _, rel := range files {
		t.Run(rel, func(t *testing.T) {
			// An example in its own directory is one model, whose files may
			// import across each other, so its siblings are opened with it.
			ws := NewWorkspace()
			for _, sibling := range siblings(t, rel) {
				content, err := os.ReadFile(filepath.Join(examplesDir, sibling))
				if err != nil {
					t.Fatalf("read %s: %v", sibling, err)
				}
				ws.Open(sibling, content, 1)
			}

			var errs []string
			for _, d := range ws.Diagnostics(rel) {
				if d.Severity == diag.SeverityError {
					errs = append(errs, d.Message)
				}
			}

			reason, known := examplesKnownFailures[rel]
			switch {
			case known && len(errs) == 0:
				t.Errorf("listed as a known failure (%s) but analyses cleanly; remove it from examplesKnownFailures", reason)
			case known:
				t.Logf("known failure (%s): %s", reason, strings.Join(errs, "; "))
			case len(errs) > 0:
				t.Errorf("%d error(s): %s", len(errs), strings.Join(errs, "; "))
			}
		})
	}
}

// siblings returns the files analysed together with one example: the other
// models in its directory, or the file alone when it sits at the top level.
func siblings(t *testing.T, rel string) []string {
	t.Helper()

	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return []string{rel}
	}

	var group []string
	for _, candidate := range exampleFiles(t) {
		if filepath.ToSlash(filepath.Dir(candidate)) == dir {
			group = append(group, candidate)
		}
	}
	return group
}

// exampleFiles are the example models, the downloaded OMG corpora aside: the
// training corpus is a separate gate (TestTrainingExamplesSemanticErrors) and
// the corpora under pilot-corpora/ are inputs to the advisory differential
// harness, not models this repository ships.
func exampleFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	err := filepath.Walk(examplesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(examplesDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if rel == "sysml-v2-training" || rel == "pilot-corpora" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".sysml") || strings.HasSuffix(rel, ".kerml") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", examplesDir, err)
	}
	sort.Strings(files)
	return files
}
