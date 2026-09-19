package examples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestParserFeaturesAdvancedConnectors tests advanced connector demo
func TestParserFeaturesAdvancedConnectors(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(".", "parser_features_demo_advanced_connectors.kerml"))
	if err != nil {
		t.Fatalf("Failed to read advanced connectors demo: %v", err)
	}

	p := parser.New(source.New("parser_features_demo_advanced_connectors.kerml", content))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Advanced connectors demo has %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}
}

// TestParserFeaturesActionSemantics tests action semantics demo
func TestParserFeaturesActionSemantics(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(".", "parser_features_demo_action_semantics.sysml"))
	if err != nil {
		t.Fatalf("Failed to read action semantics demo: %v", err)
	}

	p := parser.New(source.New("parser_features_demo_action_semantics.sysml", content))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Action semantics demo has %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}
}

// TestParserFeaturesAdvancedBodies tests advanced bodies demo
func TestParserFeaturesAdvancedBodies(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(".", "parser_features_demo_advanced_bodies.kerml"))
	if err != nil {
		t.Fatalf("Failed to read advanced bodies demo: %v", err)
	}

	p := parser.New(source.New("parser_features_demo_advanced_bodies.kerml", content))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Advanced bodies demo has %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}
}

// TestParserFeaturesMessagesEvents tests messages & events demo
func TestParserFeaturesMessagesEvents(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(".", "parser_features_demo_messages_events.sysml"))
	if err != nil {
		t.Fatalf("Failed to read messages & events demo: %v", err)
	}

	p := parser.New(source.New("parser_features_demo_messages_events.sysml", content))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Messages & events demo has %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}
}

// TestParserFeaturesDeclarations tests declarations demo
func TestParserFeaturesDeclarations(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(".", "parser_features_demo_declarations.kerml"))
	if err != nil {
		t.Fatalf("Failed to read declarations demo: %v", err)
	}

	p := parser.New(source.New("parser_features_demo_declarations.kerml", content))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Declarations demo has %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}
}

// TestParserFeaturesEdgeCases tests edge cases demo
func TestParserFeaturesEdgeCases(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(".", "parser_features_demo_edge_cases.kerml"))
	if err != nil {
		t.Fatalf("Failed to read edge cases demo: %v", err)
	}

	p := parser.New(source.New("parser_features_demo_edge_cases.kerml", content))
	_ = p.ParseFile()

	if len(p.Diagnostics) > 0 {
		t.Errorf("Edge cases demo has %d errors:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Errorf("  - %s", d.Message)
		}
	}
}

// TestAllParserFeaturesDemos runs all parser demos (original + new)
func TestAllParserFeaturesDemos(t *testing.T) {
	demos := []string{
		// Original demos (Sessions 2-3)
		"parser_features_demo_relationships.kerml",
		"parser_features_demo_modifiers.kerml",
		"parser_features_demo_binding.kerml",
		"parser_features_demo_connectors.sysml",
		"parser_features_demo_defaults.kerml",

		// New demos (Sessions 4-5)
		"parser_features_demo_advanced_connectors.kerml",
		"parser_features_demo_action_semantics.sysml",
		"parser_features_demo_advanced_bodies.kerml",
		"parser_features_demo_messages_events.sysml",
		"parser_features_demo_declarations.kerml",
		"parser_features_demo_edge_cases.kerml",
	}

	totalErrors := 0
	for _, demo := range demos {
		content, err := os.ReadFile(filepath.Join(".", demo))
		if err != nil {
			t.Errorf("Failed to read %s: %v", demo, err)
			continue
		}

		p := parser.New(source.New(demo, content))
		_ = p.ParseFile()

		if len(p.Diagnostics) > 0 {
			t.Errorf("%s has %d errors", demo, len(p.Diagnostics))
			totalErrors += len(p.Diagnostics)
			for _, d := range p.Diagnostics {
				t.Errorf("  - %s", d.Message)
			}
		}
	}

	if totalErrors == 0 {
		t.Logf("✓ All %d parser feature demos parse cleanly", len(demos))
	} else {
		t.Errorf("Total errors across all demos: %d", totalErrors)
	}
}
