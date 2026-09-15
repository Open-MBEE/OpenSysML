package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// The Kernel tiers frame every element; the Systems, Domain and OpenSysML tiers
// describe the objects a model asks for. Library content of no stated tier is
// held to the Kernel's standard.
func TestLibraryTierFrame(t *testing.T) {
	for tier, wantFrame := range map[LibraryTier]bool{
		TierLibrary:        true,
		TierKernelSemantic: true,
		TierKernelDataType: true,
		TierKernelFunction: true,
		TierSystems:        false,
		TierDomain:         false,
		TierOpenSysML:      false,
	} {
		if !tier.Library() {
			t.Errorf("%s.Library() = false, want true", tier)
		}
		if got := tier.Frame(); got != wantFrame {
			t.Errorf("%s.Frame() = %v, want %v", tier, got, wantFrame)
		}
	}
	if TierNone.Library() || TierNone.Frame() {
		t.Errorf("%s is library content, want the model's own", TierNone)
	}
	if got := LibraryTier(200).String(); got != "unknown library tier" {
		t.Errorf("an out-of-range tier spells %q", got)
	}
}

// A document marked with a tier gives the tier to what it declares, not to what
// it re-exports, and only while it is loaded; re-marking replaces the tier.
func TestLibraryTierProvenance(t *testing.T) {
	idx := NewIndex()
	addDoc(t, idx, "Systems Library/Items.sysml", "package Items { item def Item { attribute isSolid; } }")
	addDoc(t, idx, "Kernel Libraries/Kernel Semantic Library/Occurrences.kerml",
		"package Occurrences { class Occurrence { feature self; } }")
	addDoc(t, idx, "shared.sysml", "package Shared { public import Items::*; }")
	addDoc(t, idx, "user.sysml", "package Mine { part def Gadget; }")
	idx.ExpandWildcardImports()
	idx.MarkLibraryTier("Systems Library/Items.sysml", TierSystems)
	idx.MarkLibraryTier("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml", TierKernelSemantic)

	item := declaring(t, idx, "Items::Item")
	isSolid := declaring(t, idx, "Items::Item::isSolid")
	self := declaring(t, idx, "Occurrences::Occurrence::self")
	for sym, want := range map[*Symbol]LibraryTier{
		item:                              TierSystems,
		isSolid:                           TierSystems,
		self:                              TierKernelSemantic,
		declaring(t, idx, "Mine::Gadget"): TierNone,
		declaring(t, idx, "Shared"):       TierNone,
		nil:                               TierNone,
	} {
		if got := idx.LibraryTier(sym); got != want {
			t.Errorf("LibraryTier(%v) = %s, want %s", sym, got, want)
		}
	}
	if got := idx.DocumentLibraryTier("Systems Library/Items.sysml"); got != TierSystems {
		t.Errorf("DocumentLibraryTier(Items.sysml) = %s, want %s", got, TierSystems)
	}
	if got := idx.DocumentLibraryTier("user.sysml"); got != TierNone {
		t.Errorf("DocumentLibraryTier(user.sysml) = %s, want %s", got, TierNone)
	}

	idx.MarkLibraryTier("Systems Library/Items.sysml", TierDomain)
	if got := idx.LibraryTier(isSolid); got != TierDomain {
		t.Errorf("after re-marking, LibraryTier(isSolid) = %s, want %s", got, TierDomain)
	}
	idx.MarkLibraryTier("Systems Library/Items.sysml", TierNone)
	if idx.Library(isSolid) || idx.DocumentLibraryTier("Systems Library/Items.sysml") != TierNone {
		t.Errorf("marking with TierNone did not unmark the document")
	}

	idx.MarkLibraryTier("Systems Library/Items.sysml", TierSystems)
	idx.RemoveDocument("shared.sysml")
	if got := idx.LibraryTier(item); got != TierSystems {
		t.Errorf("Items::Item lost its tier when a re-exporting document was removed: %s", got)
	}
	idx.RemoveDocument("Systems Library/Items.sysml")
	if idx.Library(item) {
		t.Errorf("Items::Item is still library content after its document was removed")
	}
}

// The library's identity is a function of the language, tier and text of its
// documents: the same text under another name is the same library, another text,
// tier or language is not.
func TestLibraryIdentityFollowsTextNotName(t *testing.T) {
	const items = "package Items { item def Item; }"
	identityOfKind := func(name, text string, tier LibraryTier, kind source.Kind) string {
		t.Helper()
		idx := NewIndex()
		idx.AddDocumentWithKind(name, parser.New(source.New(name, []byte(text))).ParseFile(), kind)
		idx.MarkLibraryDocument(name, LibraryDocument{Tier: tier, Digest: TextDigest([]byte(text))})
		got, known := idx.LibraryIdentity()
		if !known || got == "" {
			t.Fatalf("LibraryIdentity() of %s = %q, %t; want a known identity", name, got, known)
		}
		return got
	}
	identity := func(name, text string, tier LibraryTier) string {
		t.Helper()
		return identityOfKind(name, text, tier, source.KindOf(name))
	}
	bundled := identity("Systems Library/Items.sysml", items, TierSystems)
	if got := identity("copy.sysml", items, TierSystems); got != bundled {
		t.Errorf("the same text under another name has another identity")
	}
	if got := identityOfKind("copy.kerml", items, TierSystems, source.KindSysML); got != bundled {
		t.Errorf("the same text under a name of another suffix, parsed as SysML, has another identity")
	}
	if got := identityOfKind("Systems Library/Items.sysml", items, TierSystems, source.KindKerML); got == bundled {
		t.Errorf("the same text parsed as KerML has the same identity")
	}
	if got := identity("Systems Library/Items.sysml", items+" // edited", TierSystems); got == bundled {
		t.Errorf("another text under the same name has the same identity")
	}
	if got := identity("Systems Library/Items.sysml", items, TierDomain); got == bundled {
		t.Errorf("the same text of another tier has the same identity")
	}

	idx := NewIndex()
	addDoc(t, idx, "Systems Library/Items.sysml", items)
	idx.MarkLibraryTier("Systems Library/Items.sysml", TierSystems)
	if got, known := idx.LibraryIdentity(); known {
		t.Errorf("LibraryIdentity() = %q, known, for a document of no text digest", got)
	}
}

// A snapshot restores each symbol's tier, and an overlay over the decoded base
// reads the same tiers as one over the original.
func TestSnapshotKeepsLibraryTiers(t *testing.T) {
	base := NewIndex()
	addDoc(t, base, "Systems Library/Items.sysml", "package Items { item def Item { attribute isSolid; } }")
	addDoc(t, base, "Kernel Libraries/Kernel Semantic Library/Occurrences.kerml",
		"package Occurrences { class Occurrence { feature self; } }")
	addDoc(t, base, "loose.kerml", "package Loose { class K; }")
	base.ExpandWildcardImports()
	base.MarkLibraryTier("Systems Library/Items.sysml", TierSystems)
	base.MarkLibraryTier("Kernel Libraries/Kernel Semantic Library/Occurrences.kerml", TierKernelSemantic)
	base.MarkLibrary("loose.kerml")
	base.Freeze()

	for _, idx := range []*Index{roundTrip(t, base), NewOverlay(roundTrip(t, base))} {
		for fqn, want := range map[string]LibraryTier{
			"Items::Item::isSolid":          TierSystems,
			"Occurrences::Occurrence::self": TierKernelSemantic,
			"Loose::K":                      TierLibrary,
		} {
			if got := idx.LibraryTier(declaring(t, idx, fqn)); got != want {
				t.Errorf("decoded LibraryTier(%s) = %s, want %s", fqn, got, want)
			}
		}
		for doc, want := range map[string]LibraryTier{
			"Systems Library/Items.sysml": TierSystems,
			"loose.kerml":                 TierLibrary,
		} {
			if got := idx.DocumentLibraryTier(doc); got != want {
				t.Errorf("decoded DocumentLibraryTier(%s) = %s, want %s", doc, got, want)
			}
		}
	}
}
