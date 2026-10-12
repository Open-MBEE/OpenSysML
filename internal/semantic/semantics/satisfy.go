package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// SatisfyEnds is what a satisfy usage relates, as written: the requirement it
// reference-subsets, or the one it declares itself, and its `by` subject.
type SatisfyEnds struct {
	Requirement         ast.Node
	DeclaresRequirement bool
	Satisfier           ast.Node
}

// SatisfyEndsOf reads the ends of a `satisfy` usage — satisfiedRequirement and
// satisfyingFeature — and reports false for any other element, a `verify`
// included. An end left unwritten is nil.
func SatisfyEndsOf(sym *symbols.Symbol) (SatisfyEnds, bool) {
	if sym == nil || sym.Kind != symbols.SymbolSatisfyRequirementUsage {
		return SatisfyEnds{}, false
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Kind != ast.UsageSatisfy || decl.IsVerifiedRequirement() {
		return SatisfyEnds{}, false
	}
	ends := SatisfyEnds{DeclaresRequirement: decl.DeclaresRequirement}
	if rel := decl.ReferenceSubsetting(); rel != nil && !decl.DeclaresRequirement {
		ends.Requirement = rel.Target
	}
	for _, rel := range decl.Relationships {
		if rel != nil && rel.Kind == ast.RelSubject {
			ends.Satisfier = rel.Target
			break
		}
	}
	return ends, true
}
