package examples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestParserFeaturesDemos(t *testing.T) {
	demoFiles := []string{
		"parser_features_demo_relationships.kerml",
		"parser_features_demo_modifiers.kerml",
		"parser_features_demo_binding.kerml",
		"parser_features_demo_connectors.sysml",
		"parser_features_demo_defaults.kerml",
	}

	for _, filename := range demoFiles {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(".", filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", filename, err)
			}

			p := parser.New(source.New(filename, data))
			file := p.ParseFile()

			if file == nil {
				t.Fatalf("Parser returned nil file for %s", filename)
			}

			if len(p.Diagnostics) > 0 {
				t.Errorf("Demo %s has %d parse errors:", filename, len(p.Diagnostics))
				for _, d := range p.Diagnostics {
					t.Errorf("  %s", d.Message)
					// Print context
					if d.Span.Offset >= 0 && d.Span.Offset < len(data) {
						start := d.Span.Offset - 30
						if start < 0 {
							start = 0
						}
						end := d.Span.Offset + 50
						if end > len(data) {
							end = len(data)
						}
						t.Errorf("    Context: %q", string(data[start:end]))
					}
				}
			} else {
				t.Logf("✓ %s parsed successfully", filename)
			}
		})
	}
}
