package semantics

import "github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

// SynthesizedNameFQN is the qualified name of the metadata definition in the
// bundled library that marks a name a migration made up for an element its
// source left anonymous.
const SynthesizedNameFQN = "MigrationMetadata::SynthesizedName"

// StandInFQN is the qualified name of the metadata definition in the bundled
// library that marks a member a migration made up which stands for no element
// of its source.
const StandInFQN = "MigrationMetadata::StandIn"

// AppliedStereotypeFQN is the qualified name of the metadata definition in the
// bundled library that records a source stereotype a migrated element's form
// does not carry.
const AppliedStereotypeFQN = "MigrationMetadata::AppliedStereotype"

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

// StandIn reports whether sym stands for no element of a migration's source: a
// StandIn annotation applies to it, stated on it or about it.
func (m *Model) StandIn(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || m.resolver == nil {
		return false
	}
	for _, a := range m.annotationsOf(sym) {
		if a.typ != nil && m.fqnOf(a.typ) == StandInFQN {
			return true
		}
	}
	return false
}

// IsMigrationAnnotation reports whether sym is a metadata usage typed by
// SynthesizedName, StandIn or AppliedStereotype: a record of the migration,
// not model content.
func (m *Model) IsMigrationAnnotation(sym *symbols.Symbol) bool {
	fqn := m.annotationTypeFQN(sym)
	return fqn == SynthesizedNameFQN || fqn == StandInFQN || fqn == AppliedStereotypeFQN
}
