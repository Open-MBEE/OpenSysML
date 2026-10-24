package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// reconstruction lists, by the diagram's kind, the owner's subtree a view exposes in
// place of rows the tool computed on opening and never stored (no stream, no used elements).
type reconstruction struct {
	rule *reconstructionRule
	// listed are the owner's elements the rule lists, in document order.
	listed []*sysmlv1.Element
}

// reconstructionRule says what a diagram kind's rows are computed from.
type reconstructionRule struct {
	// kind is the tool's diagram type, lower-cased.
	kind string
	// stored names what the tool computes and does not store: rows or cells.
	stored string
	// noun names a row's element in the singular.
	noun string
	// lists reports whether e, under the diagram's owner, is one of the rows.
	lists func(m *migration, e *sysmlv1.Element) bool
	// refs are the references a view exposing the row e writes from scope: the
	// element itself, or the connection written for it and its ends.
	refs func(m *migration, e, scope *sysmlv1.Element) []string
}

var reconstructionRules = []*reconstructionRule{
	{kind: "requirement table", stored: "rows for this table", noun: "requirement", lists: listsRequirement, refs: elementRefs},
	{kind: "refine requirement matrix", stored: "cells for this matrix", noun: "refinement", lists: listsRefinement, refs: refinementRefs},
}

func listsRequirement(m *migration, e *sysmlv1.Element) bool {
	cat, _ := m.classify(e)
	return cat == catRequirement && m.written(e)
}

func listsRefinement(_ *migration, e *sysmlv1.Element) bool {
	switch e.Type {
	case "Dependency", "Abstraction", "Realization":
		return has(e, "Refine")
	}
	return false
}

func elementRefs(m *migration, e, scope *sysmlv1.Element) []string {
	if ref := m.exposure(e, scope); ref != "" {
		return []string{ref}
	}
	return nil
}

// refinementRefs are the connection a «Refine» is written as and the usages it joins
// (the matrix's cell and both ends), or the dependency itself where no connection is.
func refinementRefs(m *migration, e, scope *sysmlv1.Element) []string {
	if refs := m.connRefs(e, scope); len(refs) > 0 {
		return refs
	}
	return elementRefs(m, e, scope)
}

// unstored reports whether the tool wrote none of what the diagram shows: a
// representation with neither a symbol stream nor a list of used elements.
func unstored(d *sysmlv1.Diagram) bool {
	return d.Represented() && len(d.Shown) == 0 && d.Stream == "" && !d.Drawn
}

// reconstructionRuleFor is the rule whose kind the diagram is of; nil when its
// kind is not one whose rows the tool computes on opening.
func reconstructionRuleFor(d *sysmlv1.Diagram) *reconstructionRule {
	kind := strings.ToLower(d.Kind)
	for _, r := range reconstructionRules {
		if strings.Contains(kind, r.kind) {
			return r
		}
	}
	return nil
}

// planReconstructions lists the owner's elements each placed view of an unstored diagram
// exposes by its kind's rule; a table definition carries the tool's scope, so none is planned beside it.
func (m *migration) planReconstructions() {
	for i := range m.model.Diagrams {
		d := &m.model.Diagrams[i]
		v := m.viewOf[d]
		if v == nil || !v.placed || len(v.tables) > 0 || d.Owner == nil || !unstored(d) {
			continue
		}
		rule := reconstructionRuleFor(d)
		if rule == nil {
			continue
		}
		rc := &reconstruction{rule: rule}
		where := "view '" + v.name + "' in " + qualifiedName(d.Owner) + " lists it for the " + rule.stored + " the tool stored none of"
		walkOwned(d.Owner, func(e *sysmlv1.Element) {
			if !rule.lists(m, e) {
				return
			}
			rc.listed = append(rc.listed, e)
			if nameableEdge(e) && len(m.connsOf(e)) == 0 {
				// A dependency the view lists is named by its writer, as a shown one is.
				m.shownEdges[e] = true
				return
			}
			if x := m.exposable(e); x != nil {
				m.segments(x)
				if x.Parent != v.host {
					m.expose(x, where)
				}
			}
		})
		v.rebuilt = rc
	}
}

// refs are the expose references written for the listed elements from scope, each once, and
// the count nothing written stands for; resolved when the view is written, once edges are named.
func (rc *reconstruction) refs(m *migration, scope *sysmlv1.Element) (refs []string, unwritten int) {
	seen := map[string]bool{}
	for _, e := range rc.listed {
		found := rc.rule.refs(m, e, scope)
		if len(found) == 0 {
			unwritten++
			continue
		}
		for _, ref := range found {
			if !seen[ref] {
				seen[ref] = true
				refs = append(refs, ref)
			}
		}
	}
	return refs, unwritten
}

// walkOwned visits every element under root in document order, root excluded.
func walkOwned(root *sysmlv1.Element, visit func(*sysmlv1.Element)) {
	for _, c := range root.Children {
		visit(c)
		walkOwned(c, visit)
	}
}

// note is the report's account of the reconstruction: that the tool stored
// nothing, and what the view exposes from the owning scope instead.
func (rc *reconstruction) note(m *migration, owner, scope *sysmlv1.Element) string {
	rule := rc.rule
	refs, unwritten := rc.refs(m, scope)
	note := "the tool stored no " + rule.stored + "; reconstructed from the owning scope: "
	exposed := len(rc.listed) - unwritten
	switch {
	case len(rc.listed) == 0:
		note += "no " + rule.noun + " is written under " + qualifiedName(owner) + ", so the view exposes nothing"
	case exposed == 0:
		note += "none of the " + plural(len(rc.listed), rule.noun) + " under " + qualifiedName(owner) + " is written, so the view exposes nothing"
	default:
		note += plural(exposed, rule.noun) + " exposed"
		if len(refs) > exposed {
			note += " with the usages they join"
		}
		if unwritten > 0 {
			note += "; " + plural(unwritten, rule.noun) + " not written and not exposed"
		}
	}
	return note
}
