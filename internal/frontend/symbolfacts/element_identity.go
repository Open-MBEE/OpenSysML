package symbolfacts

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementForm is the element form's ids, which a host that links the exporter
// supplies; this package spells none itself but an unnamed connector end's (composedEndID).
type ElementForm interface {
	// ID is the form's `@id` of sym, "" where the form writes none.
	ID(sc *Context, sym *symbols.Symbol) string
	// Owner decodes an `@id` into the qualified name it is written under and how
	// many elements deep below that the element can lie; false for no `@id`.
	Owner(id string) (owner string, depth int, ok bool)
}

// ElementIdentity is the id a client keeps for any element sym, a metaobject's
// on the wire: Identity when sym has one, else the element form's `@id`.
func (sc *Context) ElementIdentity(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if identity := sc.Identity(sym); identity != "" {
		return identity
	}
	return sc.FormID(sym)
}

// FormID is the `@id` the element form gives sym, "" when the host converts no form.
func (sc *Context) FormID(sym *symbols.Symbol) string {
	if form := sc.elementForm(); form != nil && sym != nil {
		return form.ID(sc, sym)
	}
	return ""
}

func (sc *Context) elementForm() ElementForm {
	if sc.ElementForm == nil {
		return nil
	}
	return sc.ElementForm()
}

// ResolveElement is the symbols of sem an ElementIdentity names: qualified or
// positional as Lookup reads it, and a form `@id` under the nearest such owner.
func (sc *Context) ResolveElement(id string, sem *semantics.Model) []*symbols.Symbol {
	named := sc.Lookup(id)
	form := sc.elementForm()
	if form == nil {
		return named
	}
	owner, depth, ok := form.Owner(id)
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

// LookupElement resolves an id a Symbol is reported under: as Lookup reads it, else an unnamed
// connector end's element id (ownedIdentity). A declaration named like that id is what it answers.
func (sc *Context) LookupElement(id string) []*symbols.Symbol {
	if syms := sc.Lookup(id); len(syms) > 0 {
		return syms
	}
	return sc.unnamedEnds(id)
}

// unnamedEnds finds the unnamed connector ends id is the element id of.
func (sc *Context) unnamedEnds(id string) []*symbols.Symbol {
	if sc.elementForm() == nil {
		return sc.composedEnds(id)
	}
	var ends []*symbols.Symbol
	for _, sym := range sc.ResolveElement(id, sc.Semantics) {
		if sym.Kind == symbols.SymbolConnectorEnd {
			ends = append(ends, sym)
		}
	}
	return ends
}

// composedEndID is the element id the conversion mints for an unnamed end of a
// connector (`<connector id>_pend<i>`), composed for a host that converts no form.
func (sc *Context) composedEndID(end *symbols.Symbol) string {
	owner := end.Owner()
	ownerID := sc.Identity(owner)
	if ownerID == "" {
		return ""
	}
	ends, _ := sc.Semantics.OwnedConnectorEnds(owner)
	at := slices.Index(ends, end)
	if at < 0 {
		return ""
	}
	return rdf.ExpressionNodeID(rdf.EncodeElementID(ownerID), fmt.Sprintf("end%d", at))
}

// composedEnds are the unnamed connector ends composedEndID names id.
func (sc *Context) composedEnds(id string) []*symbols.Symbol {
	owner, positions, ok := rdf.DecodeExpressionNodeID(id)
	if !ok || len(positions) != 1 {
		return nil
	}
	var found []*symbols.Symbol
	for _, connector := range sc.Lookup(owner) {
		ends, _ := sc.Semantics.OwnedConnectorEnds(connector)
		for _, end := range ends {
			if sc.Identity(end) == "" && sc.composedEndID(end) == id {
				found = append(found, end)
			}
		}
	}
	return found
}
