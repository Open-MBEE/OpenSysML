package symbolfacts

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementIdentity is the id a client keeps for any element sym, a metaobject's
// on the wire: Identity when sym has one, else the element form's `@id`.
func (sc *Context) ElementIdentity(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if identity := sc.Identity(sym); identity != "" {
		return identity
	}
	return sc.formID(sym)
}

// formID is the `@id` the element form gives sym: read for a declaration, and
// composed from its owner's for an element the semantic side tables synthesize.
func (sc *Context) formID(sym *symbols.Symbol) string {
	if sym == nil || sc.ElementIDs == nil {
		return ""
	}
	reader := sc.ElementIDs()
	if reader == nil {
		return ""
	}
	switch {
	case sym.Expr != nil:
		id, _ := export.SynthesizedElementID(sym, sc.formID)
		return id
	case sym.Chain != nil:
		id, _, _ := reader.ReadExpressionPart(sym.DocName, sym.Decl, "chain")
		return id
	case sym.Implicit != nil:
		if sym.Implicit.Node == nil {
			return ""
		}
		id, _ := reader.ReadRelationship(sym.DocName, sym.Implicit.Node)
		return id
	}
	id, _ := reader.Read(sym.DocName, sym.Decl, sc.Identity(sym))
	return id
}

// ResolveElement is the symbols of sem an ElementIdentity names: qualified or
// positional as Lookup reads it, and a form `@id` under the nearest such owner.
func (sc *Context) ResolveElement(id string, sem *semantics.Model) []*symbols.Symbol {
	named := sc.Lookup(id)
	owner, depth, ok := formIDOwner(id)
	if !ok {
		return named
	}
	// A declaration spelled like a form id is kept only when the id is its own.
	var found []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, sym := range named {
		if sc.ElementIdentity(sym) == id {
			found = append(found, sym)
			seen[sym] = true
		}
	}
	for owner != "" {
		if owners := sc.Lookup(owner); len(owners) > 0 {
			for _, root := range owners {
				if depth == 0 && !seen[root] {
					found = append(found, root)
					seen[root] = true
					continue
				}
				found = sc.ownedElementNamed(sem, root, id, depth, seen, found)
			}
			return found
		}
		cut := strings.LastIndex(owner, "::")
		if cut < 0 {
			break
		}
		owner, depth = owner[:cut], depth+1
	}
	return found
}

// formIDOwner decodes a form `@id` into the qualified name it is written under
// and how many elements deep below that the id can lie (one per expression position).
func formIDOwner(id string) (owner string, depth int, ok bool) {
	if owner, positions, ok := rdf.DecodeExpressionNodeID(id); ok {
		return owner, 2 * len(positions), true
	}
	owner, ok = rdf.DecodeElementID(id)
	return owner, 0, ok
}

// ownedElementNamed collects the elements owned at most depth levels below
// root, root included, whose ElementIdentity is id.
func (sc *Context) ownedElementNamed(sem *semantics.Model, root *symbols.Symbol, id string, depth int,
	seen map[*symbols.Symbol]bool, found []*symbols.Symbol) []*symbols.Symbol {
	if root == nil || seen[root] {
		return found
	}
	seen[root] = true
	if sc.ElementIdentity(root) == id {
		found = append(found, root)
	}
	if depth == 0 || sem == nil {
		return found
	}
	for _, feature := range []string{"ownedElement", "ownedRelationship"} {
		owned, _ := sem.ReflectiveElements(root, feature)
		for _, element := range owned {
			found = sc.ownedElementNamed(sem, element, id, depth-1, seen, found)
		}
	}
	return found
}
