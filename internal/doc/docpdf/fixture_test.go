package docpdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The docrender fixtures the PDF tests evaluate: a report with two Mermaid
// diagrams, tables, lists and definitions, one with formulas, and one over a
// session's states and trace.
var (
	docrenderTestdata = filepath.Join("..", "docrender", "testdata")
	telescopeFixture  = filepath.Join(docrenderTestdata, "telescope_report.sysml")
	mathFixture       = filepath.Join(docrenderTestdata, "math_report.sysml")
	stateFixture      = filepath.Join(docrenderTestdata, "state_report.sysml")
)

// sourceModel is one SysML source text parsed, indexed and given a semantic model.
type sourceModel struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
}

// loadSource parses and indexes one SysML source text over the bundled library.
func loadSource(t *testing.T, file, content string) sourceModel {
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
	return sourceModel{index: index, model: model, resolver: resolver}
}

func (s sourceModel) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(s.index.LookupQualified(name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

// document compiles and evaluates the named document, over a session's runtime
// and roots when given.
func (s sourceModel) document(t *testing.T, name string, ctx *runtime.Context, roots []queryexec.Root) *docir.Document {
	t.Helper()
	plan, err := docplan.Compile(s.index, s.model, s.resolver, s.symbol(t, name))
	if err != nil {
		t.Fatalf("compile document %s: %v", name, err)
	}
	document, err := docir.Evaluate(plan,
		queryexec.Context{Index: s.index, Resolver: s.resolver, Model: s.model, Runtime: ctx, Roots: roots},
		queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document %s: %v", name, err)
	}
	return document
}

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
	return loadSource(t, file, content).document(t, name, nil, nil)
}

// stateDocument is the lamp report over a driven session — lamp1 on at 0 s,
// dimmed at 1 s, boosted at 2 s; lamp2 on at 2 s, off at 2.5 s; clock at 3 s —
// with state rows, the objects in a state, one lamp's events and bare summaries.
func stateDocument(t *testing.T) *docir.Document {
	t.Helper()
	content, err := os.ReadFile(stateFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	src := loadSource(t, filepath.Base(stateFixture), string(content))
	ctx := runtime.NewContext(runtime.NewModel(src.model, src.resolver), runtime.DefaultMaxSteps)
	ctx.SetTrace(runtime.NewTraceRecorder())
	var roots []queryexec.Root
	instantiate := func(name string) *runtime.Instance {
		inst, err := ctx.Instantiate(src.symbol(t, "Lamps::"+name))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", name, err)
		}
		roots = append(roots, queryexec.Root{Label: name, Object: inst})
		return inst
	}
	lamp1, lamp2 := instantiate("lamp1"), instantiate("lamp2")
	instantiate("panel")
	send := func(inst *runtime.Instance, signal string, args map[string]runtime.Value) {
		msg, err := ctx.SignalMessage(src.symbol(t, "Lamps::"+signal), args, inst)
		if err != nil {
			t.Fatalf("send %s: %v", signal, err)
		}
		ctx.PostMessage(msg)
	}
	advance := func(seconds float64) {
		if _, err := ctx.Advance(seconds); err != nil {
			t.Fatalf("advance %v: %v", seconds, err)
		}
	}
	level := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	send(lamp1, "Toggle", nil)
	advance(1)
	send(lamp1, "Dim", map[string]runtime.Value{"level": level})
	advance(1)
	send(lamp1, "Boost", nil)
	send(lamp2, "Toggle", nil)
	advance(0.5)
	send(lamp2, "Toggle", nil)
	advance(0.5)
	return src.document(t, "Lamps::LampReport", ctx, roots)
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

// proseDocument is a report of one section of running text, so the size most
// of its glyphs are set at is the body size.
func proseDocument(t *testing.T) *docir.Document {
	t.Helper()
	sentence := "The mirror segments are phased by actuators that hold the wavefront error within budget across the observing night. "
	return sourceDocument(t, "prose.sysml", `package Prose {
	private import DocumentQueries::*;
	part def Report :> Document {
		attribute redefines title = "Prose";
		part body : Section {
			attribute redefines title = "Text";
			part opening : Paragraph { part a : Span { attribute redefines text = "`+strings.Repeat(sentence, 6)+`"; } }
			part closing : Paragraph { part a : Span { attribute redefines text = "`+strings.Repeat(sentence, 6)+`"; } }
		}
	}
}
`, "Prose::Report")
}

// wideTableDocument is a report whose middle section holds a captioned,
// grouped seven-column table between two one-paragraph sections.
func wideTableDocument(t *testing.T) *docir.Document {
	t.Helper()
	return sourceDocument(t, "wide.sysml", `package Wide {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Cell {
		attribute team : String;
		attribute a : Integer;
		attribute b : Integer;
		attribute c : Integer;
		attribute d : Integer;
		attribute e : Integer;
		attribute f : Integer;
	}

	part matrix {
		part power : Cell {
			attribute redefines team = "Power";
			attribute redefines a = 1;
			attribute redefines b = 2;
			attribute redefines c = 3;
			attribute redefines d = 4;
			attribute redefines e = 5;
			attribute redefines f = 6;
		}
	}

	calc def Cells :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("team", "name", "a", "b", "c", "d", "e", "f")
		)
	}

	part def Report :> Document {
		attribute redefines title = "Wide Report";
		part intro : Paragraph {
			part lead : Span { attribute redefines text = "An opening paragraph."; }
		}
		part matrixSection : Section {
			attribute redefines title = "Matrix";
			part cells : Table {
				attribute redefines caption = "Every requirement";
				attribute redefines groupBy = "team";
				calc rows : Cells {
					in root = matrix;
				}
			}
		}
		part afterwards : Section {
			attribute redefines title = "Afterwards";
			part closing : Paragraph {
				part lead : Span { attribute redefines text = "A closing paragraph."; }
			}
		}
	}
}
`, "Wide::Report")
}
