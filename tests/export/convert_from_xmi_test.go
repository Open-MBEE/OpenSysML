//go:build !sysml_re && !sysml_nov1

package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestConvertFromXMI checks the XMI entry into the conversion paths: v1 XMI
// migrates to notation and to Turtle, the report comes back from Migrate, and
// nothing writes XMI.
func TestConvertFromXMI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "migrate", "testdata", "xmi", "vehicle.xmi"))
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := convert.Migrate("vehicle.xmi", data, convert.FormatSysML, migrate.Options{})
	if err != nil {
		t.Fatalf("Migrate to notation: %v", err)
	}
	if migrated.Report == nil || len(migrated.Report.Entries) == 0 {
		t.Fatal("Migrate returned no report")
	}
	if migrated.Results == nil {
		t.Fatal("Migrate returned no results index")
	}
	notation := migrated.Output
	if !strings.Contains(string(notation), "part def Vehicle") {
		t.Errorf("migrated notation lacks the Vehicle block:\n%s", notation)
	}
	if again, err := convert.Convert("vehicle.xmi", data, convert.FormatXMI, convert.FormatSysML); err != nil || string(again) != string(notation) {
		t.Errorf("Convert from XMI differs from Migrate: %v", err)
	}

	asTurtle, err := convert.Migrate("vehicle.xmi", data, convert.FormatTurtle, migrate.Options{})
	if err != nil {
		t.Fatalf("Migrate to Turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(asTurtle.Output)
	if err != nil {
		t.Fatalf("migrated Turtle does not parse: %v", err)
	}
	if len(graph.Triples()) == 0 {
		t.Error("migrated Turtle is empty")
	}

	var notWritable *convert.NotWritableError
	if _, err := convert.Convert("model.sysml", []byte("package P;"), convert.FormatSysML, convert.FormatXMI); !errors.As(err, &notWritable) {
		t.Errorf("writing XMI: got %v, want a NotWritableError", err)
	}
	if _, err := convert.Migrate("vehicle.xmi", data, convert.FormatXMI, migrate.Options{}); !errors.As(err, &notWritable) {
		t.Errorf("migrating to XMI: got %v, want a NotWritableError", err)
	}
	if _, err := convert.Convert("model.sysml", []byte("package P;"), convert.FormatXMI, convert.FormatSysML); err == nil {
		t.Error("notation read as XMI was accepted")
	}
}
