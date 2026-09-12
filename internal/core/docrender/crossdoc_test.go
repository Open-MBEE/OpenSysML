package docrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

type renderFixture struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
}

func loadRenderFixture(t *testing.T, path string) renderFixture {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	index := libs.NewModelIndex()
	p := parser.New(source.New(filepath.Base(path), content))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(filepath.Base(path), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	return renderFixture{index: index, model: semantics.NewModel(resolver), resolver: resolver}
}

func (f renderFixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified(name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

// renderFixtureDocumentSet runs the whole pipeline on every named document of
// a fixture, evaluating them together so cross-document anchors are stamped.
func renderFixtureDocumentSet(t *testing.T, path string, names []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(names))
	for _, document := range fixtureDocumentSet(t, path, names) {
		markdown, err := Markdown(document, MarkdownOptions{})
		if err != nil {
			t.Fatalf("render document %s: %v", document.Name(), err)
		}
		out[DocumentFileName(document.Name())] = markdown
	}
	return out
}

// fixtureDocumentSet evaluates every named document of a fixture together, so
// cross-document anchors are stamped.
func fixtureDocumentSet(t *testing.T, path string, names []string) []*docir.Document {
	t.Helper()
	fixture := loadRenderFixture(t, path)
	plans := make([]*docplan.Plan, 0, len(names))
	for _, name := range names {
		plan, err := docplan.Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, name))
		if err != nil {
			t.Fatalf("compile document %s: %v", name, err)
		}
		plans = append(plans, plan)
	}
	documents, err := docir.EvaluateSet(plans,
		queryexec.Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model},
		queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document set: %v", err)
	}
	return documents
}

// TestMarkdownLinkedDocumentsGolden locks the end-to-end rendering of a
// linked multi-document set: relative links between files, anchors on the
// referenced blocks, root references, and in-document references unchanged.
func TestMarkdownLinkedDocumentsGolden(t *testing.T) {
	rendered := renderFixtureDocumentSet(t,
		filepath.Join("testdata", "linked_reports.sysml"),
		[]string{"Observatory::SystemReport", "Observatory::Mass Appendix"})
	// The goldens keep the deterministic output names so the links
	// between them resolve on disk exactly as a rendered set's do.
	for _, file := range []string{
		"Observatory-SystemReport.md",
		"Observatory-Mass.20Appendix.md",
	} {
		got, ok := rendered[file]
		if !ok {
			t.Fatalf("no document rendered as %s; got %v", file, keys(rendered))
		}
		path := filepath.Join("testdata", "linked", file)
		if *update {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatalf("update golden: %v", err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden (run with -update to create): %v", err)
		}
		if got != string(want) {
			t.Errorf("rendered Markdown differs from %s (run with -update after intentional changes)\ngot:\n%s", path, got)
		}
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestMarkdownCrossDocumentLinksResolve checks every cross-document link in
// the rendered set points at a rendered file, and at an anchor that file
// declares when the link names one.
func TestMarkdownCrossDocumentLinksResolve(t *testing.T) {
	rendered := renderFixtureDocumentSet(t,
		filepath.Join("testdata", "linked_reports.sysml"),
		[]string{"Observatory::SystemReport", "Observatory::Mass Appendix"})
	report := rendered["Observatory-SystemReport.md"]
	if !strings.Contains(report, "](Observatory-Mass.20Appendix.md#tables-masses)") {
		t.Errorf("report lacks content-block link:\n%s", report)
	}
	if !strings.Contains(report, "](Observatory-Mass.20Appendix.md)") {
		t.Errorf("report lacks root link:\n%s", report)
	}
	appendix := rendered["Observatory-Mass.20Appendix.md"]
	if !strings.Contains(appendix, `<a id="tables-masses"></a>`) {
		t.Errorf("appendix lacks referenced anchor:\n%s", appendix)
	}
	if !strings.Contains(appendix, "](Observatory-SystemReport.md)") {
		t.Errorf("appendix lacks back link:\n%s", appendix)
	}
}

// TestDocumentFileNameEncoding checks file names derive deterministically
// from qualified names and stay within a safe character set.
func TestDocumentFileNameEncoding(t *testing.T) {
	for fqn, want := range map[string]string{
		"Reports::MassReport":   "Reports-MassReport.md",
		"Observatory::Mass 1/2": "Observatory-Mass.201.2F2.md",
		"Solo":                  "Solo.md",
	} {
		if got := DocumentFileName(fqn); got != want {
			t.Errorf("DocumentFileName(%q) = %q, want %q", fqn, got, want)
		}
	}
}
