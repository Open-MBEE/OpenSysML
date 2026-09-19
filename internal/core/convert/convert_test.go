package convert_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/convert"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// A conversion from XMI is the migration followed by a notation conversion of
// what it wrote, so the migrated notation and the converted Turtle agree with
// the migration package's own output.
func TestConvertFromXMIComposesTheMigration(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := migrate.Migrate("vehicle.xmi", data)
	if err != nil {
		t.Fatalf("migrate.Migrate: %v", err)
	}
	notation, report, err := convert.Migrate("vehicle.xmi", data, convert.FormatSysML)
	if err != nil {
		t.Fatalf("Migrate to notation: %v", err)
	}
	if len(report.Entries) != len(migrated.Report.Entries) {
		t.Errorf("Migrate reported %d entries, the migration %d", len(report.Entries), len(migrated.Report.Entries))
	}
	reformatted, err := convert.Convert("vehicle.sysml", migrated.Notation, convert.FormatSysML, convert.FormatSysML)
	if err != nil {
		t.Fatalf("Convert the migration's notation: %v", err)
	}
	if string(notation) != string(reformatted) {
		t.Errorf("Migrate to notation differs from converting the migration's notation:\n%s\n---\n%s", notation, reformatted)
	}
	turtle, err := convert.Convert("vehicle.xmi", data, convert.FormatXMI, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("Convert XMI to Turtle: %v", err)
	}
	direct, err := convert.SysMLToRDF("vehicle.sysml", migrated.Notation)
	if err != nil {
		t.Fatalf("SysMLToRDF of the migrated notation: %v", err)
	}
	if string(turtle) != string(rdf.WriteTurtle(direct)) {
		t.Error("Convert XMI to Turtle differs from the Turtle of the migrated notation")
	}
}

// The parser's diagnostics reach the caller as a SyntaxError from every entry
// point that parses, and XMI cannot be written.
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
	var notWritable *convert.NotWritableError
	if _, _, err := convert.Migrate("m.xmi", nil, convert.FormatXMI); !errors.As(err, &notWritable) {
		t.Errorf("Migrate to XMI = %v, want a NotWritableError", err)
	}
	if _, err := convert.ParseFormat("nosuchformat"); err == nil || !strings.Contains(err.Error(), "nosuchformat") {
		t.Errorf("ParseFormat(nosuchformat) = %v, want an error naming it", err)
	}
}
