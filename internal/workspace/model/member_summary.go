package model

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Member is one named member of a definition or usage, as an editor summarizes it.
type Member struct {
	Symbol *symbols.Symbol
	// Name is the name the member is known under: its effective name.
	Name string
	// Direction is a feature's direction as the summarized type sees it,
	// reversed through a conjugation.
	Direction ast.FeatureDirection
	// Types are the types a feature is written with, directly or through the
	// features it specializes.
	Types []*symbols.Symbol
	// Multiplicity is the multiplicity a feature declares, as `[0..*]`; "" for none.
	Multiplicity string
	// Origin is the type the member is inherited from; nil for an owned one.
	Origin *symbols.Symbol
}

// MemberSummaryOf returns the named members a definition or usage owns, in declaration order,
// then, when inherited is set, those it inherits; an alias stands for the element it names.
func (w *Workspace) MemberSummaryOf(sym *symbols.Symbol, inherited bool) []Member {
	if sym == nil || sym.DocName == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []Member
	w.queryLocked(sym.DocName, func(resolver *resolve.Resolver, sem *semantics.Model) {
		if target, ok := resolver.ResolveAliasTarget(sym); ok {
			sym = target
		}
		if !summarized(sym) {
			return
		}
		for _, member := range sem.MembersOf(sym) {
			if !summarized(member) {
				continue
			}
			origin := member.Owner()
			if origin == sym {
				origin = nil
			} else if !inherited || origin == nil || !resolver.InheritedMemberVisible(sym, member) {
				continue
			}
			out = append(out, Member{
				Symbol:       member,
				Name:         sem.EffectiveNameOf(member),
				Direction:    sem.EffectiveDirection(sym, member),
				Types:        sem.DeclaredFeatureTypes(member),
				Multiplicity: declaredMultiplicityText(member),
				Origin:       origin,
			})
		}
	})
	return out
}

// summarized reports whether sym is a definition or a feature: what a summary is of and lists,
// a subject or a required constraint with the usages, an import or a comment not.
func summarized(sym *symbols.Symbol) bool {
	return !sym.Implied() && (sym.DeclaresDefinition() || sym.Kind.IsDefinition() || sym.IsFeature())
}

// declaredMultiplicityText spells the multiplicity a feature declares: its
// bounds as written, or as evaluated for a declaration read from a record.
func declaredMultiplicityText(sym *symbols.Symbol) string {
	if sym.Recorded() {
		facts := sym.Facts.Multiplicity
		if facts == nil || !sym.DeclaresUsage() {
			return ""
		}
		lower, upper := boundFactsText(facts.Lower), boundFactsText(facts.Upper)
		if lower == upper {
			return "[" + lower + "]"
		}
		return "[" + lower + ".." + upper + "]"
	}
	mult := semantics.UsageMultiplicityOf(sym)
	if mult == nil {
		return ""
	}
	lower := boundText(mult.Lower)
	if !mult.IsRange {
		return "[" + lower + "]"
	}
	return "[" + lower + ".." + boundText(mult.Upper) + "]"
}

func boundText(node ast.Node) string {
	if text := (ast.ExprPrinter{}).Text(node); text != "" {
		return text
	}
	return "?"
}

func boundFactsText(bound symbols.BoundFacts) string {
	switch {
	case bound.Infinite:
		return "*"
	case bound.Known:
		return strconv.FormatInt(bound.Value, 10)
	}
	return "?"
}
