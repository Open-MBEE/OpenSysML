package symbols

import "testing"

// aboutDoc declares one `about` metadata usage next to declarations that must
// not be collected: an inline metadata usage and a prefix annotation.
const aboutDoc = `package P {
	metadata def Safety;
	part def Belt;
	part radio { metadata inline : Safety; }
	#Safety part def Bag;
	metadata s : Safety about Belt;
}`

func TestFreezeCachesAboutUsages(t *testing.T) {
	idx := frozenBase(t, map[string]string{"about.sysml": aboutDoc})

	usages, cached := idx.FrozenAboutUsages("about.sysml")
	if !cached {
		t.Fatalf("frozen index must cover its own document")
	}
	if len(usages) != 1 || usages[0].Name != "s" {
		t.Fatalf("FrozenAboutUsages = %v, want the one `about` usage s", usages)
	}
	if usages, cached := idx.FrozenAboutUsages(libDoc); !cached || len(usages) != 0 {
		t.Fatalf("a frozen document without `about` usages must be covered and empty; got %v, %v", usages, cached)
	}
}

func TestFrozenAboutUsagesOverAnOverlay(t *testing.T) {
	base := frozenBase(t, map[string]string{"about.sysml": aboutDoc})
	over := overlayOver(t, base, map[string]string{"user.sysml": aboutDoc})

	if usages, cached := over.FrozenAboutUsages("about.sysml"); !cached || len(usages) != 1 {
		t.Fatalf("a base document must be covered through the overlay; got %v, %v", usages, cached)
	}
	if _, cached := over.FrozenAboutUsages("user.sysml"); cached {
		t.Fatalf("the overlay's own document is writable, so it must not be covered")
	}

	// Re-adding a base document shadows the frozen copy, so the cache no
	// longer speaks for it.
	addDoc(t, over, "about.sysml", "package P { part def Belt; }")
	if _, cached := over.FrozenAboutUsages("about.sysml"); cached {
		t.Fatalf("a shadowed base document must not be covered")
	}
}

func TestFrozenAboutUsagesOfARemovedBaseDocument(t *testing.T) {
	base := frozenBase(t, map[string]string{"about.sysml": aboutDoc})
	over := overlayOver(t, base, nil)

	over.RemoveDocument("about.sysml")
	if usages, cached := over.FrozenAboutUsages("about.sysml"); cached || len(usages) != 0 {
		t.Fatalf("a removed base document must not answer from the cache; got %v, %v", usages, cached)
	}
}

func TestFrozenAboutUsagesOnAWritableIndex(t *testing.T) {
	idx := buildIndex(t, map[string]string{"about.sysml": aboutDoc})
	if _, cached := idx.FrozenAboutUsages("about.sysml"); cached {
		t.Fatalf("an unfrozen index has no cache to answer from")
	}
}
