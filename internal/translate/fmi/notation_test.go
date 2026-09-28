package fmi

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	execfmi "github.com/Open-MBEE/OpenSysML/internal/exec/fmi"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current import")

// notationOf parses the fixture's model description and imports it.
func notationOf(t *testing.T, fixture string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "exec", "fmi", "testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	d, err := execfmi.ParseModelDescription(data)
	if err != nil {
		t.Fatalf("ParseModelDescription: %v", err)
	}
	out, err := Notation(d, Options{URI: "file:///opt/fmus/" + strings.TrimSuffix(fixture, ".xml") + ".fmu"})
	if err != nil {
		t.Fatalf("Notation: %v", err)
	}
	return out
}

func checkGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the import output (run with -update after reviewing):\n%s", path, got)
	}
}

func TestGoldenNotationFMI2(t *testing.T) {
	checkGolden(t, "testdata/bouncingball-2.0.golden.sysml", notationOf(t, "bouncingball-2.0.xml"))
}

func TestGoldenNotationFMI3(t *testing.T) {
	checkGolden(t, "testdata/mixed-3.0.golden.sysml", notationOf(t, "mixed-3.0.xml"))
}

// TestImportedNotationAnalysesClean: the notation the importer writes must
// survive the workspace's own checks against the bundled standard library.
func TestImportedNotationAnalysesClean(t *testing.T) {
	for _, fixture := range []string{"bouncingball-2.0.xml", "mixed-3.0.xml"} {
		t.Run(fixture, func(t *testing.T) {
			ws := model.NewWorkspace()
			name := fixture + ".sysml"
			ws.Open(name, notationOf(t, fixture), 1)
			var errs []string
			for _, d := range ws.Diagnostics(name) {
				if d.Severity == diag.SeverityError {
					errs = append(errs, fmt.Sprint(d))
				}
			}
			if len(errs) > 0 {
				t.Errorf("imported notation has %d error(s): %v", len(errs), errs)
			}
		})
	}
}

func TestNotationRequiresURI(t *testing.T) {
	d := &execfmi.Description{ModelName: "X"}
	if _, err := Notation(d, Options{}); err == nil {
		t.Fatal("a missing URI must fail")
	}
}

// TestNamingCollisions covers the identifier rules: sanitizing, keyword names,
// and a model variable colliding with a reserved experiment name.
func TestNamingCollisions(t *testing.T) {
	d := &execfmi.Description{
		FMIVersion: execfmi.Version2,
		ModelName:  "2-stroke",
		Variables: []execfmi.Variable{
			{Name: "in", Causality: execfmi.CausalityInput, Kind: execfmi.KindReal},
			{Name: "stopTime", Causality: execfmi.CausalityInput, Kind: execfmi.KindReal},
			{Name: "weird-name", Causality: execfmi.CausalityInput, Kind: execfmi.KindReal},
			{Name: "weird_name", Causality: execfmi.CausalityInput, Kind: execfmi.KindReal},
			{Name: "h", Causality: execfmi.CausalityOutput, Kind: execfmi.KindReal},
		},
	}
	out, err := Notation(d, Options{URI: "file:///x.fmu"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"package _2_stroke", "calc def _2_stroke",
		"in in_ : Real", "in stopTime_2 : Real", "in weird_name : Real", "in weird_name_2 : Real",
		"in stopTime : Real", // the reserved experiment parameter claims its name first
	} {
		if !bytes.Contains([]byte(text), []byte(want)) {
			t.Errorf("notation lacks %q:\n%s", want, text)
		}
	}
}

// TestNoOutputsReturnsTime: an FMU with no output still yields a result.
func TestNoOutputsReturnsTime(t *testing.T) {
	d := &execfmi.Description{FMIVersion: execfmi.Version2, ModelName: "Empty"}
	out, err := Notation(d, Options{URI: "file:///x.fmu"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`return time : Real { @ToolVariable { name = "fmi:time"; } }`)) {
		t.Errorf("notation lacks the fmi:time return:\n%s", out)
	}
}

// TestSkipsDocumented: structural parameters, arrays and unsupported kinds are
// skipped with a comment rather than imported.
func TestSkipsDocumented(t *testing.T) {
	d := &execfmi.Description{
		FMIVersion: execfmi.Version3,
		ModelName:  "Mixed",
		Variables: []execfmi.Variable{
			{Name: "n", Causality: execfmi.CausalityStructuralParameter, Kind: execfmi.KindInteger},
			{Name: "grid", Causality: execfmi.CausalityOutput, Kind: execfmi.KindReal, Array: true},
			{Name: "tick", Causality: execfmi.CausalityInput, Kind: execfmi.KindClock},
			{Name: "blob", Causality: execfmi.CausalityLocal, Kind: execfmi.KindBinary},
			{Name: "y", Causality: execfmi.CausalityOutput, Kind: execfmi.KindReal},
		},
	}
	out, err := Notation(d, Options{URI: "file:///x.fmu"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"// structural parameter n not imported",
		"// array grid not imported",
		"// tick Clock not imported",
		"return y : Real",
	} {
		if !bytes.Contains([]byte(text), []byte(want)) {
			t.Errorf("notation lacks %q:\n%s", want, text)
		}
	}
}
