package export_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

var apiJSONBenchmarkOutput []byte

func BenchmarkWriteAPIJSON(b *testing.B) {
	examplePath := os.Getenv("OPENSYSML_APIJSON_BENCH_EXAMPLE")
	if examplePath == "" {
		examplePath = filepath.Join("..", "..", "examples", "pilot-corpora", "sysml-examples", "Vehicle Example", "SysML v2 Spec Annex A SimpleVehicleModel.sysml")
	}
	models := []struct {
		name string
		path string
	}{
		{
			name: "ISQAtomicNuclear",
			path: filepath.Join("..", "..", "internal", "workspace", "libs", "stdlib", "Domain Libraries", "Quantities and Units", "ISQAtomicNuclear.sysml"),
		},
		{
			name: "SimpleVehicleModel",
			path: examplePath,
		},
		{
			name: "then_after_members",
			path: filepath.Join("testdata", "convert", "then_after_members.sysml"),
		},
	}
	writers := []struct {
		name  string
		write func(*rdf.Graph) ([]byte, error)
	}{
		{name: "standard", write: export.WriteAPIJSON},
		{name: "compact", write: func(graph *rdf.Graph) ([]byte, error) {
			return export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{})
		}},
	}
	for _, model := range models {
		source, err := os.ReadFile(model.path)
		if err != nil {
			b.Fatal(err)
		}
		graph, err := convert.SysMLToRDF(model.path, source)
		if err != nil {
			b.Fatalf("convert %s: %v", model.path, err)
		}
		for _, writer := range writers {
			b.Run(fmt.Sprintf("%s/%s", model.name, writer.name), func(b *testing.B) {
				initial, err := writer.write(graph)
				if err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(initial)))
				b.ReportMetric(float64(len(initial)), "output-B")
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					output, err := writer.write(graph)
					if err != nil {
						b.Fatal(err)
					}
					apiJSONBenchmarkOutput = output
				}
				b.ReportMetric(float64(len(initial)), "output-B")
			})
		}
	}
}
