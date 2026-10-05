package perf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

type importShape struct {
	name   string
	nested bool
}

type importSize struct {
	parts int
	attrs int
}

var importShapes = []importShape{
	{name: "flat"},
	{name: "nested", nested: true},
}

var importSizes = []importSize{
	{parts: 100, attrs: 3},
	{parts: 500, attrs: 3},
	{parts: 2000, attrs: 3},
	{parts: 2000, attrs: 5},
}

var (
	vehicleAttributes = []string{"mass", "power", "range", "cost", "drag"}
	batteryAttributes = []string{"capacity", "voltage", "charge", "temp", "cycles"}
)

func importModel(shape importShape, parts, attrs int) []byte {
	var model bytes.Buffer
	fmt.Fprintln(&model, "package Perf {")
	fmt.Fprintln(&model, "    private import ScalarValues::*;")
	fmt.Fprintln(&model, "    part def Battery {")
	for _, attr := range batteryAttributes[:attrs] {
		fmt.Fprintf(&model, "        attribute %s : Real;\n", attr)
	}
	fmt.Fprintln(&model, "    }")
	fmt.Fprintln(&model, "    part def Vehicle {")
	for _, attr := range vehicleAttributes[:attrs] {
		fmt.Fprintf(&model, "        attribute %s : Real;\n", attr)
	}
	fmt.Fprintln(&model, "        part battery : Battery;")
	fmt.Fprintln(&model, "    }")
	for i := 0; i < parts; i++ {
		if shape.nested {
			fmt.Fprintf(&model, "    part v%d : Vehicle {\n        part :>> battery;\n    }\n", i)
		} else {
			fmt.Fprintf(&model, "    part v%d : Vehicle;\n", i)
		}
	}
	fmt.Fprintln(&model, "}")
	return model.Bytes()
}

func importCSV(shape importShape, parts, attrs int) []byte {
	columns := vehicleAttributes[:attrs]
	if shape.nested {
		columns = batteryAttributes[:attrs]
	}
	var csv bytes.Buffer
	fmt.Fprint(&csv, "element")
	for _, column := range columns {
		fmt.Fprintf(&csv, ",%s", column)
	}
	fmt.Fprintln(&csv)
	for i := 0; i < parts; i++ {
		element := fmt.Sprintf("Perf::v%d", i)
		if shape.nested {
			element += "::battery"
		}
		fmt.Fprint(&csv, element)
		for j := range columns {
			fmt.Fprintf(&csv, ",%d.%d", i, j)
		}
		fmt.Fprintln(&csv)
	}
	return csv.Bytes()
}

func writeImportInputs(tb testing.TB, shape importShape, parts, attrs int) (string, string) {
	tb.Helper()
	dir := tb.TempDir()
	modelPath := filepath.Join(dir, "perf.sysml")
	csvPath := filepath.Join(dir, "values.csv")
	if err := os.WriteFile(modelPath, importModel(shape, parts, attrs), 0o600); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(csvPath, importCSV(shape, parts, attrs), 0o600); err != nil {
		tb.Fatal(err)
	}
	return modelPath, csvPath
}

func loadImportSession(tb testing.TB, modelPath string) *repl.Session {
	tb.Helper()
	sess := repl.NewSession()
	report, err := sess.LoadPathsReport([]string{modelPath})
	if err != nil {
		tb.Fatal(err)
	}
	if report.Errors {
		tb.Fatalf("model did not analyse cleanly: %s", strings.Join(report.Found, "\n"))
	}
	return sess
}

// BenchmarkImportValues times REPL value imports: go test ./tests/perf -run '^$' -bench ImportValues -benchtime=1x.
func BenchmarkImportValues(b *testing.B) {
	for _, shape := range importShapes {
		for _, size := range importSizes {
			name := fmt.Sprintf("%s/parts=%d/attrs=%d", shape.name, size.parts, size.attrs)
			b.Run(name, func(b *testing.B) {
				b.StopTimer()
				modelPath, csvPath := writeImportInputs(b, shape, size.parts, size.attrs)
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					sess := loadImportSession(b, modelPath)
					b.StartTimer()
					verdict := sess.ImportData(csvPath, repl.ImportOptions{})
					b.StopTimer()
					if verdict.Status != repl.VerdictHolds {
						b.Fatalf("import did not hold:\n%s", strings.Join(verdict.Lines, "\n"))
					}
				}
				b.ReportMetric(float64(size.parts*size.attrs), "values")
			})
		}
	}
}

func TestImportValuesWorkload(t *testing.T) {
	for _, shape := range importShapes {
		t.Run(shape.name, func(t *testing.T) {
			modelPath, csvPath := writeImportInputs(t, shape, 100, 3)
			sess := loadImportSession(t, modelPath)
			verdict := sess.ImportData(csvPath, repl.ImportOptions{})
			if verdict.Status != repl.VerdictHolds {
				t.Fatalf("import did not hold:\n%s", strings.Join(verdict.Lines, "\n"))
			}
			text := sess.Text()
			if shape.nested {
				for _, want := range []string{"part :>> battery {", "attribute :>> capacity = 0.0;"} {
					if !strings.Contains(text, want) {
						t.Errorf("imported text does not contain %q:\n%s", want, text)
					}
				}
			} else if !strings.Contains(text, "attribute :>> mass = 0.0;") {
				t.Errorf("imported text does not contain %q:\n%s", "attribute :>> mass = 0.0;", text)
			}
		})
	}
}
