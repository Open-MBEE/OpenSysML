package model

import (
	"testing"
)

func TestMetaclassIndexing(t *testing.T) {
	ws := NewWorkspace()

	// Check what Metaobjects::SemanticMetadata resolves to
	symbols := ws.index.LookupQualified("Metaobjects::SemanticMetadata")

	t.Logf("Found %d symbols for Metaobjects::SemanticMetadata", len(symbols))
	for i, sym := range symbols {
		t.Logf("  Symbol %d: kind=%s, name=%s",
			i, sym.Kind, sym.Name)
	}
}
