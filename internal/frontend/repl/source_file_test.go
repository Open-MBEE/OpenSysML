package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const locatedPicturesModel = `package Site {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;
	private import DocumentQueries::*;

	view bench {
		render asInterconnectionDiagram;
		@Picture { location = "images/bench.png"; x = 0; y = 0; width = 80; height = 60; }
	}

	part def Report :> Document {
		attribute redefines title = "Bench";
		part plate : Image {
			attribute redefines location = "images/bench.png";
			attribute redefines caption = "The bench";
		}
	}
}
`

// A location a loaded file states is relative to that file, not to the
// session buffer the file was joined into: the DOT form names the file beside
// the model, and a document written elsewhere refers back to it.
func TestLoadedFileLocationsResolveBesideTheFile(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model", "site.sysml")
	if err := os.MkdirAll(filepath.Dir(model), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte(locatedPicturesModel), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	s.Submit("package Typed { part def Lead; }")
	report, err := s.LoadPathsReport([]string{model})
	if err != nil {
		t.Fatalf("LoadPathsReport: %v", err)
	}
	if report.Errors {
		t.Fatalf("the model did not load clean: %+v", report)
	}

	beside := filepath.Join(dir, "model", "images", "bench.png")
	dot := run(t, s, "%render Site::bench dot")
	if !strings.Contains(dot, `image="`+beside+`"`) {
		t.Errorf("the DOT form names %q, not the picture beside the file:\n%s", "image=", dot)
	}

	out := filepath.Join(dir, "out")
	markdown, err := s.RenderDocumentMarkdown("Site::Report", docrender.MarkdownOptions{OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "](../model/images/bench.png)") {
		t.Errorf("the document written into %s does not refer back to the image beside the model:\n%s", out, markdown)
	}

	if got := s.sessionSourceFile(docName, source.Span{Offset: 0}); got != "" {
		t.Errorf("the typed submission is located in %q, want no file", got)
	}
	if got := s.sessionSourceFile("other.sysml", source.Span{}); got != "other.sysml" {
		t.Errorf("a document of its own is located in %q, want its name", got)
	}
}
