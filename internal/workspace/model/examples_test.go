package model

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const examplesDir = "../../../examples"

// markupToolMissingCode is the document-plan warning for a prose body whose
// converter is not installed; toolchainRequiredEnv makes that a failure.
const (
	markupToolMissingCode = "document-plan-" + string(docplan.ErrorMarkupToolMissing)
	toolchainRequiredEnv  = "OPENSYSML_REQUIRE_PDF_TOOLCHAIN"
)

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

			var errs, unchecked []string
			for _, d := range ws.Diagnostics(rel) {
				switch {
				case d.Severity == diag.SeverityError:
					errs = append(errs, d.Message)
				case d.Code == markupToolMissingCode:
					unchecked = append(unchecked, d.Message)
				}
			}
			if len(errs) > 0 {
				t.Errorf("%d error(s): %s", len(errs), strings.Join(errs, "; "))
			}
			if len(unchecked) > 0 {
				// The example's prose needs an external converter: the PDF
				// toolchain job installs it and requires the bodies checked.
				if required := os.Getenv(toolchainRequiredEnv); required != "" {
					t.Fatalf("%s=%s but %s", toolchainRequiredEnv, required, strings.Join(unchecked, "; "))
				}
				t.Logf("left unchecked: %s", strings.Join(unchecked, "; "))
			}
		})
	}
}

func TestViewsDemoResolvesCaseRenderingsWithSelfModelLoaded(t *testing.T) {
	ws := NewWorkspace()
	rel := "views-demo.sysml"
	content, err := os.ReadFile(filepath.Join(examplesDir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	ws.Open(rel, content, 1)
	for _, sibling := range siblings(t, "self-model/views.sysml") {
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
	if len(errs) > 0 {
		t.Errorf("%d error(s): %s", len(errs), strings.Join(errs, "; "))
	}
}

// siblings returns the files analysed together with one example: the other
// models in its directory and in the example directories enclosing it (a
// nested directory layers onto the model above it), or the file alone when it
// sits at the top level.
func siblings(t *testing.T, rel string) []string {
	t.Helper()

	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return []string{rel}
	}

	var group []string
	for _, candidate := range exampleFiles(t) {
		candidateDir := filepath.ToSlash(filepath.Dir(candidate))
		if candidateDir != "." && (candidateDir == dir || strings.HasPrefix(dir, candidateDir+"/")) {
			group = append(group, candidate)
		}
	}
	return group
}

// exampleFiles are the example models, the downloaded OMG corpora aside: the
// training corpus is a separate gate (TestTrainingExamplesSemanticErrors) and
// the corpora under pilot-corpora/ and spec-examples/ are inputs to the advisory
// differential harness, not models this repository ships.
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
			if rel == "sysml-v2-training" || rel == "pilot-corpora" || rel == "spec-examples" {
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
