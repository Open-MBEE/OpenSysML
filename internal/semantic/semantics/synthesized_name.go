package semantics

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// SynthesizedNameFQN is the qualified name of the metadata definition in the
// bundled library that marks a name a migration made up for an element its
// source left anonymous.
const SynthesizedNameFQN = "MigrationMetadata::SynthesizedName"

// NameSynthesized reports whether sym's name was made up by a migration: a
// SynthesizedName annotation applies to it, stated on it or about it.
func (m *Model) NameSynthesized(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || m.resolver == nil {
		return false
	}
	for _, a := range m.annotationsOf(sym) {
		if a.typ != nil && m.fqnOf(a.typ) == SynthesizedNameFQN {
			return true
		}
	}
	return false
}

// IsSynthesizedNameAnnotation reports whether sym is a metadata usage typed by
// SynthesizedName: a record of the migration, not model content.
func (m *Model) IsSynthesizedNameAnnotation(sym *symbols.Symbol) bool {
	return m.annotationTypeFQN(sym) == SynthesizedNameFQN
}
