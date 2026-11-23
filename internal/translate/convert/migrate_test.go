//go:build !sysml_re && !sysml_nov1

package convert_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A conversion from XMI is the migration, then a notation conversion of what it wrote,
// so the migrated notation and the converted Turtle agree with the migration's own output.
func TestConvertFromXMIComposesTheMigration(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "migrate", "testdata", "xmi", "vehicle.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := migrate.Migrate("vehicle.xmi", data)
	if err != nil {
		t.Fatalf("migrate.Migrate: %v", err)
	}
	asNotation, err := convert.Migrate("vehicle.xmi", data, convert.FormatSysML, migrate.Options{})
	if err != nil {
		t.Fatalf("Migrate to notation: %v", err)
	}
	notation, report := asNotation.Output, asNotation.Report
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

// The migration reads XMI and never writes it.
func TestMigrateRefusesToWriteXMI(t *testing.T) {
	var notWritable *convert.NotWritableError
	if _, err := convert.Migrate("m.xmi", nil, convert.FormatXMI, migrate.Options{}); !errors.As(err, &notWritable) {
		t.Errorf("Migrate to XMI = %v, want a NotWritableError", err)
	}
}
