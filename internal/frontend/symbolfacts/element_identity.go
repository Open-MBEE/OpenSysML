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
	// LibraryName is the qualified name of the standard-library element the form references
	// under id, the normative id no name can be decoded from.
	LibraryName(id string) (name string, ok bool)
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
	named := sc.lookupDeclared(id)
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
		if owners := sc.lookupDeclared(owner); len(owners) > 0 {
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

// ownedElementNamed collects the elements owned at most depth levels below root, root included, whose
// ElementIdentity or form `@id` is id: the form files a trigger's named payload under its accept.
func (sc *Context) ownedElementNamed(sem *semantics.Model, root *symbols.Symbol, id string, depth int,
	seen map[*symbols.Symbol]bool, found []*symbols.Symbol) []*symbols.Symbol {
	return sc.ownedElements(sem, root, depth, seen, found, func(sym *symbols.Symbol) bool {
		return sc.ElementIdentity(sym) == id || sc.FormID(sym) == id
	})
}

// ownedElements collects the elements owned at most depth levels below root, root included, that keep accepts.
func (sc *Context) ownedElements(sem *semantics.Model, root *symbols.Symbol, depth int,
	seen map[*symbols.Symbol]bool, found []*symbols.Symbol, keep func(*symbols.Symbol) bool) []*symbols.Symbol {
	if root == nil || seen[root] {
		return found
	}
	seen[root] = true
	if keep(root) {
		found = append(found, root)
	}
	if depth == 0 || sem == nil {
		return found
	}
	for _, feature := range []string{"ownedElement", "ownedRelationship"} {
		owned, _ := sem.ReflectiveElements(root, feature)
		for _, element := range owned {
			found = sc.ownedElements(sem, element, depth-1, seen, found, keep)
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
	if syms := sc.unnamedEnds(id); len(syms) > 0 {
		return syms
	}
	return sc.escapedRoots(id)
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
	// The owner is stated by the id it is reported under, which escapedID may have encoded.
	for _, connector := range append(sc.Lookup(owner), sc.escapedRoots(owner)...) {
		ends, _ := sc.Semantics.OwnedConnectorEnds(connector)
		for _, end := range ends {
			if sc.Identity(end) == "" && sc.composedEndID(end) == id {
				found = append(found, end)
			}
		}
	}
	return found
}

// formElements is the elements the element form writes or references under the `@id` id: a
// library element by name, else found below the nearest owner the id's decoded name states.
func (sc *Context) formElements(id string) []*symbols.Symbol {
	form := sc.elementForm()
	if form == nil {
		return nil
	}
	if name, ok := form.LibraryName(id); ok {
		// A model declaration named like the library element is not what its normative id names.
		var library []*symbols.Symbol
		for _, sym := range LookupNamed(sc.Index, name) {
			if sc.Index.Library(sym) {
				library = append(library, sym)
			}
		}
		return library
	}
	owner, depth, ok := form.Owner(id)
	if !ok {
		return nil
	}
	is := func(sym *symbols.Symbol) bool { return form.ID(sc, sym) == id }
	for owner != "" {
		if owners := LookupNamed(sc.Index, owner); len(owners) > 0 {
			var found []*symbols.Symbol
			seen := map[*symbols.Symbol]bool{}
			for _, root := range owners {
				found = sc.ownedElements(sc.Semantics, root, depth, seen, found, is)
			}
			return found
		}
		cut := strings.LastIndex(owner, "::")
		if cut < 0 {
			break
		}
		owner, depth = owner[:cut], depth+1
	}
	return nil
}

// generatedSpells reports whether an element other than sym has the generated `@id` identity
// spells, which only a root declaration's can be: an `@id` holds no `::`.
func (sc *Context) generatedSpells(identity string, sym *symbols.Symbol) bool {
	if strings.Contains(identity, "::") || !strings.Contains(identity, "_") {
		return false
	}
	other := func(found *symbols.Symbol) bool { return found != sym }
	if sc.elementForm() == nil {
		return slices.ContainsFunc(sc.composedEnds(identity), other) || slices.ContainsFunc(sc.escapedRoots(identity), other)
	}
	return slices.ContainsFunc(sc.formElements(identity), other)
}

// escapedID is the `@id` the element form gives sym, declared as identity: the form's own, or
// the encoding it writes for a root declaration where the host converts no form.
func (sc *Context) escapedID(sym *symbols.Symbol, identity string) string {
	if sc.elementForm() == nil {
		return rdf.EncodeElementID(identity)
	}
	return sc.FormID(sym)
}

// escapedRoots are the root declarations escapedID reports under id on a host converting no form.
func (sc *Context) escapedRoots(id string) []*symbols.Symbol {
	name, ok := rdf.DecodeElementID(id)
	// Only an encoded id names another declaration; one that decodes to itself is no escape.
	if !ok || name == id || sc.elementForm() != nil {
		return nil
	}
	var found []*symbols.Symbol
	for _, sym := range LookupNamed(sc.Index, name) {
		if sc.Identity(sym) == id {
			found = append(found, sym)
		}
	}
	return found
}
