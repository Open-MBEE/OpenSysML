package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementIDs reports the elementId a conversion of a model writes for each of
// its elements, in the qualified id form: a declared or normative id as it is,
// any other the encoding of the element's qualified name. It reads the same
// identity side tables the writer mints subjects from, so the two cannot
// disagree, and decides scope qualification over the whole model as
// ModelToRDFWith does.
type ElementIDs struct {
	facts   map[string]*identityFacts
	library *identityFacts
}

// NewElementIDs builds the identity tables of the named documents of one
// resolved model. A document whose identity the writer would refuse (an
// ElementId it cannot carry) has no table, and its elements report no id.
func NewElementIDs(documents []string, res *resolve.Resolver, model *semantics.Model) *ElementIDs {
	ids := &ElementIDs{facts: make(map[string]*identityFacts, len(documents))}
	scopes := map[string]bool{}
	for _, name := range documents {
		facts, err := documentIdentity(name, res, model, IDQualifiedName)
		if err != nil {
			continue
		}
		ids.facts[name] = facts
		if ids.library == nil {
			ids.library = facts
		}
		for key := range facts.scopes {
			scopes[key] = true
		}
	}
	if len(scopes) > 1 {
		for _, facts := range ids.facts {
			facts.qualified = true
		}
	}
	return ids
}

// Of is the elementId written for sym, whose qualified or positional name is
// fqn: the one its document's table gives it, or a standard library element's
// normative id; false where the model's documents give it none.
func (ids *ElementIDs) Of(sym *symbols.Symbol, fqn string) (string, bool) {
	if ids == nil || sym == nil || sym.Decl == nil || fqn == "" {
		return "", false
	}
	if facts, ok := ids.facts[sym.DocName]; ok {
		return rdf.LocalName(facts.subjectForNode(sym.Decl, fqn).Value), true
	}
	if ids.library == nil {
		return "", false
	}
	libraryFQN, ok := ids.library.libraryElement(sym)
	if !ok {
		return "", false
	}
	return rdf.LocalName(ids.library.subjectForNode(sym.Decl, libraryFQN).Value), true
}
