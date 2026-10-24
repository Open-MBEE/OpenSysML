package codes

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the catalogue in docs/reference/diagnostics.md from the registry")

const cataloguePage = "../../../docs/reference/diagnostics.md"

// Drift gate: the catalogue section of docs/reference/diagnostics.md is what
// the registry renders, so a code added, reworded or removed fails here until
// `make diagnostics-catalogue` rewrites the page.
func TestCatalogueIsCurrent(t *testing.T) {
	page, err := os.ReadFile(cataloguePage)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Splice(page)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(cataloguePage, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !bytes.Equal(page, want) {
		t.Errorf("%s has drifted from the registry; run `make diagnostics-catalogue`", filepath.Base(cataloguePage))
	}
}

// Every code's section is reachable at the anchor URL names, and the page
// keeps the catalogue between its markers once.
func TestCatalogueAnchorsEveryCode(t *testing.T) {
	text := Catalogue()
	for _, e := range All() {
		if !bytes.Contains([]byte(text), []byte("\n#### `"+e.Code+"`\n")) {
			t.Errorf("the catalogue has no section for %s", e.Code)
		}
	}
	if got := URL("document-plan-missing-title"); got != CatalogueURL+"#document-plan" {
		t.Errorf("URL of a family member = %s", got)
	}
	if got := URL("runtime.materialize.bounded"); got != CatalogueURL+"#runtimematerializebounded" {
		t.Errorf("URL of a dotted code = %s", got)
	}
	if _, err := Splice([]byte("no markers")); err == nil {
		t.Error("Splice accepted a page without markers")
	}
}
