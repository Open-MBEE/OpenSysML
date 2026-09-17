package docpdf

import (
	"os"
	"path/filepath"
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

// The docrender fixtures the PDF tests evaluate: a report with two Mermaid
// diagrams, tables, lists and definitions, and one with formulas.
var (
	docrenderTestdata = filepath.Join("..", "core", "docrender", "testdata")
	telescopeFixture  = filepath.Join(docrenderTestdata, "telescope_report.sysml")
	mathFixture       = filepath.Join(docrenderTestdata, "math_report.sysml")
)

// fixtureDocument runs the pipeline on a fixture file: parse, resolve,
// semantics, docplan, then document IR evaluation of the named document.
func fixtureDocument(t *testing.T, path, name string) *docir.Document {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return sourceDocument(t, filepath.Base(path), string(content), name)
}

// sourceDocument evaluates the named document of one SysML source text.
func sourceDocument(t *testing.T, file, content, name string) *docir.Document {
	t.Helper()
	index := libs.NewModelIndex()
	sf := source.New(file, []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(sf.Name(), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	model.SetSourceText(func(doc string, span source.Span) string {
		if doc != sf.Name() {
			return ""
		}
		return sf.Text(span)
	})
	matches := symbols.PreferDeclared(index.LookupQualified(name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	plan, err := docplan.Compile(index, model, resolver, matches[0])
	if err != nil {
		t.Fatalf("compile document %s: %v", name, err)
	}
	document, err := docir.Evaluate(plan, queryexec.Context{Index: index, Resolver: resolver, Model: model}, queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document %s: %v", name, err)
	}
	return document
}

// telescopeDocument is the telescope report: sections, tables, lists,
// definitions, two Mermaid diagrams and metacharacter-laden content.
func telescopeDocument(t *testing.T) *docir.Document {
	t.Helper()
	return fixtureDocument(t, telescopeFixture, "Observatory::MassReport")
}

// mathDocument is the optics report: inline math runs, a display formula
// under a caption, and prose with a bare dollar.
func mathDocument(t *testing.T) *docir.Document {
	t.Helper()
	return fixtureDocument(t, mathFixture, "Optics::OpticsReport")
}

// plainDocument is a one-paragraph document with no diagram or formula.
func plainDocument(t *testing.T) *docir.Document {
	t.Helper()
	return sourceDocument(t, "plain.sysml", `package Plain {
	private import DocumentQueries::*;
	part def Report :> Document {
		attribute redefines title = "Smoke Test";
		part intro : Paragraph {
			part lead : Span { attribute redefines text = "One paragraph."; }
		}
	}
}
`, "Plain::Report")
}
