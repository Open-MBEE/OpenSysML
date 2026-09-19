package edit

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// RenameOccurrence is one reference segment written with the name being renamed.
type RenameOccurrence struct {
	// Ref is the reference the segment belongs to, as the document walk collected it.
	Ref resolve.Reference
	// Part indexes the segment in Ref.QN.
	Part int
}

// Span is the segment's bytes in its document.
func (o RenameOccurrence) Span() source.Span {
	return o.Ref.QN.Parts[o.Part].Span
}

// RenameConflict is why a rename is refused: what the new name would mean instead.
type RenameConflict struct {
	// Subject is the qualified name of the element being renamed.
	Subject string
	// NewName is the name refused.
	NewName string
	// Means is the qualified name NewName already means, or would at a reference.
	Means string
	// Site is the namespace of the captured reference; empty for a taken name.
	Site string
	// Occurrence indexes the captured reference among those checked; meaningful
	// only where Site is set.
	Occurrence int
	// Ambiguity is how many elements the reference would name at once, where the
	// rename would leave it ambiguous rather than reading another element.
	Ambiguity int
}

// Error describes the conflict, naming what the new name would mean.
func (c *RenameConflict) Error() string {
	if c.Site == "" {
		return fmt.Sprintf("%s cannot be renamed to %q: that name already means %s where"+
			" %s is declared, so the rename would make it ambiguous or silently rebind"+
			" what reads that name", c.Subject, c.NewName, c.Means, c.Subject)
	}
	if c.Ambiguity > 0 {
		return fmt.Sprintf("%s cannot be renamed to %q: the reference to it in %s"+
			" would name %d elements at once, so the rename would leave that reference ambiguous",
			c.Subject, c.NewName, c.Site, c.Ambiguity)
	}
	return fmt.Sprintf("%s cannot be renamed to %q: the reference to it in %s"+
		" would read %s instead, so the rename would change what that reference means",
		c.Subject, c.NewName, c.Site, c.Means)
}

// CheckRename reports the first conflict renaming sym's written name to newName would
// create: the declaration's own scope first, then each occurrence in order. sem
// selects what a respelled call would run, as the checker selects it.
func CheckRename(r *resolve.Resolver, sem *semantics.Model, sym *symbols.Symbol, name, newName string, occurrences []RenameOccurrence) *RenameConflict {
	if name == newName {
		return nil
	}
	subject := symbols.FQNOf(sym)
	if means, ok := taken(r, sym, newName); ok {
		return &RenameConflict{Subject: subject, NewName: newName, Means: means}
	}
	for i, occ := range occurrences {
		if c, ok := capturedAt(r, sem, sym, occ, newName); ok {
			c.Subject, c.NewName, c.Site, c.Occurrence = subject, newName, site(r, occ, name), i
			return &c
		}
	}
	return nil
}

// taken names what newName already means where sym is declared, sym's own
// bindings hidden: a sibling made ambiguous, or an outer/inherited name shadowed.
func taken(r *resolve.Resolver, sym *symbols.Symbol, newName string) (string, bool) {
	if sym.OwnerScope == nil {
		return "", false
	}
	other, ok := r.LookupNameExcluding(sym.OwnerScope, newName, sym.Decl)
	return otherThan(r, sym, other, ok)
}

// capturedAt trial-reads the reference spelled newName: an alias or a qualifier reaching
// another element captures, several elements leave it ambiguous, a call selects by arguments.
func capturedAt(r *resolve.Resolver, sem *semantics.Model, sym *symbols.Symbol, occ RenameOccurrence, newName string) (RenameConflict, bool) {
	qn := respelled(occ.Ref.QN, occ.Part, newName)
	rd := r.ProbeReading(occ.Ref.Spelled(qn))
	if n, ambiguous := rd.Ambiguity(); ambiguous {
		return RenameConflict{Ambiguity: n}, true
	}
	other, ok := rd.Symbol()
	if alias, aliased := rd.Alias(occ.Part); aliased {
		other, ok = alias, true
	} else if occ.Part < len(qn.Parts)-1 {
		other, ok = rd.Part(occ.Part)
	} else if occ.Ref.Invocation != nil {
		return calledInstead(r, sem, sym, occ, qn)
	}
	means, captured := otherThan(r, sym, other, ok)
	return RenameConflict{Means: means}, captured
}

// calledInstead selects, as the checker would, among the overloads qn denotes plus the
// renamed sym (or its alias target): another one chosen, or a tie, captures the call.
func calledInstead(r *resolve.Resolver, sem *semantics.Model, sym *symbols.Symbol, occ RenameOccurrence, qn *ast.QualifiedName) (RenameConflict, bool) {
	named := append(r.InvocationCandidates(occ.Ref.Scope, qn), sym)
	sel := sem.SelectCallAmong(occ.Ref.Scope, occ.Ref.Invocation, named, semantics.CallSite(occ.Ref))
	if sel.Ambiguous {
		return RenameConflict{Ambiguity: len(sel.Tied)}, true
	}
	runs, ok := r.ResolveAliasTarget(sym)
	if !ok {
		runs = sym
	}
	called := sel.Called()
	means, captured := otherThan(r, runs, called, called != nil)
	return RenameConflict{Means: means}, captured
}

// respelled is qn with segment i spelled name, on a fresh node: the resolver
// memoizes by node, so the trial must not overwrite the real reading.
func respelled(qn *ast.QualifiedName, i int, name string) *ast.QualifiedName {
	out := &ast.QualifiedName{NodeBase: qn.NodeBase, Global: qn.Global}
	out.Parts = append([]ast.NameSegment(nil), qn.Parts...)
	out.Parts[i].Text = name
	return out
}

// otherThan names a lookup's result when it is an element other than sym.
func otherThan(r *resolve.Resolver, sym, other *symbols.Symbol, ok bool) (string, bool) {
	if !ok || symbols.SameElement(other, sym) {
		return "", false
	}
	return symbols.FQNOf(other), true
}

// site names the namespace a reference is made in, or the name as written where
// that namespace has no FQN.
func site(r *resolve.Resolver, occ RenameOccurrence, name string) string {
	if fqn := r.ReferringNamespaceFQN(occ.Ref.Scope); fqn != "" {
		return fqn
	}
	return name
}
