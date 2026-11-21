package queryexec

import "testing"

// TestPrefixMetadataIsOwnedThroughAnIdentifiedMembership: a `#M` prefix on a definition, a usage
// or a package reads the OwningMembership the conversion writes; a dependency's reads its Annotation.
func TestPrefixMetadataIsOwnedThroughAnIdentifiedMembership(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		metadata def M;
		#M part def n;
		#M part u;
		#M package Q;
		part def A;
		#M dependency d from n to A;
	}
}`)
	ids := ownedElementIDs(t, fixture, "P")
	const member = "/Package/OwningMembership/"
	for id, want := range map[string]string{
		"Observatory__P__n___400_om": member + "PartDefinition/OwningMembership",
		"Observatory__P__n___400":    member + "PartDefinition/OwningMembership/MetadataUsage",
		"Observatory__P__u___400_om": member + "PartUsage/OwningMembership",
		"Observatory__P__u___400":    member + "PartUsage/OwningMembership/MetadataUsage",
		"Observatory__P__Q___400_om": member + "Package/OwningMembership",
		"Observatory__P__Q___400":    member + "Package/OwningMembership/MetadataUsage",
		"Observatory__P__d___400_an": member + "Dependency/Annotation",
		"Observatory__P__d___400":    member + "Dependency/Annotation/MetadataUsage",
	} {
		if got := ids[id]; got != want {
			t.Errorf("elementId %q identifies %q, want %s", id, got, want)
		}
	}
}
