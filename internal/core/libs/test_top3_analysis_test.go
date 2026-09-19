package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestTop3FilesAnalysis(t *testing.T) {
	files := []string{
		"Kernel Libraries/Kernel Semantic Library/Occurrences.kerml",
		"Kernel Libraries/Kernel Semantic Library/FeatureReferencingPerformances.kerml",
		"Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml",
	}

	srcLoader := &embedSource{}

	for _, name := range files {
		data, err := srcLoader.Read(name)
		if err != nil {
			t.Fatalf("Failed to load %s: %v", name, err)
		}

		src := source.New(name, data)
		p := parser.New(src)
		_ = p.ParseFile()

		t.Logf("\n=== %s (%d errors) ===", name, len(p.Diagnostics))
		for i, d := range p.Diagnostics {
			if i >= 10 {
				t.Logf("  ... and %d more", len(p.Diagnostics)-10)
				break
			}
			offset := d.Span.Offset
			context := ""
			char := ""
			if offset < len(data) {
				char = string(data[offset])
				start := offset - 20
				if start < 0 {
					start = 0
				}
				end := offset + 40
				if end > len(data) {
					end = len(data)
				}
				context = string(data[start:end])
			}
			t.Logf("  %d. offset=%d (char=%q): %s", i+1, offset, char, d.Message)
			t.Logf("     Context: %q", context)
		}
	}
}
