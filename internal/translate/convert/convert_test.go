package convert_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// SysMLToRDF parses and hands the tree to the mapping; the two steps taken by
// hand write the same graph.
func TestSysMLToRDFComposesTheParserAndTheMapping(t *testing.T) {
	text := []byte("package P { part def Q { attribute mass : Real = 3.0; } }")
	got, err := convert.SysMLToRDF("p.sysml", text)
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	file := source.New("p.sysml", text)
	p := parser.New(file)
	want, err := export.ToRDF(file, p.ParseFile())
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	if string(rdf.WriteTurtle(got)) != string(rdf.WriteTurtle(want)) {
		t.Errorf("SysMLToRDF and parse+ToRDF disagree:\n%s\n---\n%s", rdf.WriteTurtle(got), rdf.WriteTurtle(want))
	}
}

// The parser's diagnostics reach the caller as a SyntaxError from every entry
// point that parses.
func TestEntryPointErrors(t *testing.T) {
	bad := []byte("package P { part def Q {")
	var syntaxErr *convert.SyntaxError
	if _, err := convert.SysMLToRDF("bad.sysml", bad); !errors.As(err, &syntaxErr) || len(syntaxErr.Diags) == 0 {
		t.Errorf("SysMLToRDF on bad notation = %v, want a SyntaxError with diagnostics", err)
	}
	if _, err := convert.Convert("bad.sysml", bad, convert.FormatSysML, convert.FormatTurtle); !errors.As(err, &syntaxErr) {
		t.Errorf("Convert on bad notation = %v, want a SyntaxError", err)
	}
	if _, tolerated, err := convert.ConvertTolerant("bad.sysml", bad, convert.FormatSysML, convert.FormatSysML); err != nil || tolerated == nil {
		t.Errorf("ConvertTolerant notation to notation = (%v, %v), want output with the syntax error alongside", tolerated, err)
	}
	if _, err := convert.ParseFormat("nosuchformat"); err == nil || !strings.Contains(err.Error(), "nosuchformat") {
		t.Errorf("ParseFormat(nosuchformat) = %v, want an error naming it", err)
	}
}

// FromGraph is the conversion a graph that was never parsed — one read from a
// repository — gets, refusals to write included.
func TestFromGraphWritesNotationAndTurtle(t *testing.T) {
	graph, err := convert.SysMLToRDF("p.sysml", []byte("package P { part def Vehicle; }"))
	if err != nil {
		t.Fatal(err)
	}
	notation, err := convert.FromGraph(graph, convert.FormatSysML)
	if err != nil {
		t.Fatalf("FromGraph to sysml: %v", err)
	}
	if !strings.Contains(string(notation), "Vehicle") {
		t.Errorf("the exported notation lost the element:\n%s", notation)
	}
	turtle, err := convert.FromGraph(graph, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("FromGraph to ttl: %v", err)
	}
	if string(turtle) != string(rdf.WriteTurtle(graph)) {
		t.Errorf("FromGraph to ttl is not the graph's normalized document")
	}
	var notWritable *convert.NotWritableError
	if _, err := convert.FromGraph(graph, convert.FormatXMI); !errors.As(err, &notWritable) {
		t.Errorf("FromGraph to xmi = %v, want a NotWritableError", err)
	}
}
