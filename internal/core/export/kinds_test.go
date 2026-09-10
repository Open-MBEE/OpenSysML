package export

import (
	"slices"
	"testing"
)

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
