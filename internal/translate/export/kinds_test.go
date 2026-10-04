package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestIndividualMetaclassesHaveDeterministicReverseKinds(t *testing.T) {
	if got := definitionMetaclass[ast.DefIndividual]; got != "OccurrenceDefinition" {
		t.Errorf("individual definition metaclass = %q, want OccurrenceDefinition", got)
	}
	if got := metaclassDefinition["OccurrenceDefinition"]; got != ast.DefOccurrence {
		t.Errorf("OccurrenceDefinition reverse kind = %v, want occurrence", got)
	}
	if got := metaclassDefinition["IndividualDefinition"]; got != ast.DefIndividual {
		t.Errorf("legacy IndividualDefinition reverse kind = %v, want individual", got)
	}
	if got := usageMetaclass[ast.UsageIndividual]; got != "OccurrenceUsage" {
		t.Errorf("individual usage metaclass = %q, want OccurrenceUsage", got)
	}
	if got := metaclassUsage["OccurrenceUsage"]; got != ast.UsageOccurrence {
		t.Errorf("OccurrenceUsage reverse kind = %v, want occurrence", got)
	}
	if got := metaclassUsage["IndividualUsage"]; got != ast.UsageIndividual {
		t.Errorf("legacy IndividualUsage reverse kind = %v, want individual", got)
	}
}

// Both directions walk relationshipOrder, so a relationship kind the mapping
// names a property for but the order omits would be dropped rather than written.
func TestRelationshipOrderCoversEveryMappedKind(t *testing.T) {
	for kind := range relationshipProperty {
		if !slices.Contains(relationshipOrder, kind) {
			t.Errorf("relationship kind %v has a property but no place in relationshipOrder", kind)
		}
	}
	seen := make(map[string]bool, len(relationshipOrder))
	for _, kind := range relationshipOrder {
		property, ok := relationshipProperty[kind]
		if !ok {
			t.Errorf("relationshipOrder lists kind %v, which has no property", kind)
		}
		if seen[property] {
			t.Errorf("relationshipOrder lists %q twice", property)
		}
		seen[property] = true
	}
}
