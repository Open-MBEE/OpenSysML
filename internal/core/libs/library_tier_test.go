package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A bundled file is classified by the bundle directory it sits under, however
// the path is spelled; one outside the layout is library content of no tier.
func TestTierOf(t *testing.T) {
	for name, want := range map[string]symbols.LibraryTier{
		"Kernel Libraries/Kernel Semantic Library/Occurrences.kerml":   symbols.TierKernelSemantic,
		"Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml": symbols.TierKernelDataType,
		"Kernel Libraries/Kernel Function Library/BaseFunctions.kerml": symbols.TierKernelFunction,
		"Systems Library/Items.sysml":                                  symbols.TierSystems,
		"Domain Libraries/Geometry/ShapeItems.sysml":                   symbols.TierDomain,
		"OpenSysML Libraries/Units.sysml":                              symbols.TierOpenSysML,
		"Kernel Libraries/Other.kerml":                                 symbols.TierLibrary,
		"Systems Library":                                              symbols.TierLibrary,
		"custom/Extra.sysml":                                           symbols.TierLibrary,
	} {
		if got := TierOf(name); got != want {
			t.Errorf("TierOf(%q) = %s, want %s", name, got, want)
		}
	}
}

// Every declaration of the bundled library carries its file's tier, whether
// the index was loaded from source or decoded from the embedded snapshot.
func TestLoadedLibraryCarriesTiers(t *testing.T) {
	fresh := symbols.NewIndex()
	if err := NewLoader(EmbeddedSource(), nil).LoadAll(fresh); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	decoded, err := SnapshotIndex()
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	want := map[string]symbols.LibraryTier{
		"Occurrences::Occurrence::self":         symbols.TierKernelSemantic,
		"Occurrences::Occurrence::timeSlices":   symbols.TierKernelSemantic,
		"ScalarValues::Real":                    symbols.TierKernelDataType,
		"BaseFunctions::ToString":               symbols.TierKernelFunction,
		"Items::Item::isSolid":                  symbols.TierSystems,
		"Items::Item::voids":                    symbols.TierSystems,
		"Requirements::RequirementCheck::subj":  symbols.TierSystems,
		"ShapeItems::RectangularCuboid":         symbols.TierDomain,
		"ShapeItems::RectangularCuboid::length": symbols.TierDomain,
	}
	for name, idx := range map[string]*symbols.Index{"fresh": fresh, "decoded": decoded} {
		for fqn, tier := range want {
			sym := idx.Declaring(fqn)
			if sym == nil {
				t.Errorf("%s: %s declares nothing", name, fqn)
				continue
			}
			if got := idx.LibraryTier(sym); got != tier {
				t.Errorf("%s: LibraryTier(%s) = %s, want %s", name, fqn, got, tier)
			}
		}
	}
}

// Every bundled document is recorded with the digest of its text, so the library
// as a whole has one identity, the same whether loaded from source or decoded
// from the snapshot, and another once any document's text differs.
func TestLoadedLibraryHasAnIdentity(t *testing.T) {
	src := BundledSource()
	fresh := symbols.NewIndex()
	if err := NewLoader(src, nil).LoadAll(fresh); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	decoded, err := SnapshotIndex()
	if err != nil {
		t.Fatalf("SnapshotIndex: %v", err)
	}
	for _, name := range src.List() {
		text, err := src.Read(name)
		if err != nil {
			t.Fatalf("Read(%s): %v", name, err)
		}
		want := symbols.LibraryDocument{Tier: TierOf(name), Digest: symbols.TextDigest(text)}
		for label, idx := range map[string]*symbols.Index{"fresh": fresh, "decoded": decoded} {
			if got := idx.LibraryDocumentOf(name); got != want {
				t.Errorf("%s: LibraryDocumentOf(%s) = %+v, want %+v", label, name, got, want)
			}
		}
	}
	identity, known := fresh.LibraryIdentity()
	if !known || identity == "" {
		t.Fatalf("fresh index: LibraryIdentity() = %q, %t; want a known identity", identity, known)
	}
	if got, known := decoded.LibraryIdentity(); !known || got != identity {
		t.Errorf("decoded index: LibraryIdentity() = %q, %t; want %q, true", got, known, identity)
	}
	if got, known := symbols.NewOverlay(decoded).LibraryIdentity(); !known || got != identity {
		t.Errorf("overlay: LibraryIdentity() = %q, %t; want %q, true", got, known, identity)
	}

	edited := symbols.NewIndex()
	if err := NewLoader(editedSource{src, "Systems Library/Items.sysml"}, nil).LoadAll(edited); err != nil {
		t.Fatalf("LoadAll(edited): %v", err)
	}
	if got, known := edited.LibraryIdentity(); !known || got == identity {
		t.Errorf("edited library: LibraryIdentity() = %q, %t; want another known identity", got, known)
	}
}

// editedSource serves src with a comment appended to one file.
type editedSource struct {
	Source
	edit string
}

func (s editedSource) Read(name string) ([]byte, error) {
	text, err := s.Source.Read(name)
	if err == nil && name == s.edit {
		text = append(append([]byte{}, text...), "\n// edited\n"...)
	}
	return text, nil
}
