package repl

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// satisfyText names a satisfaction assertion the way the prompt names every other
// element, quoting each inner name the notation quotes.
func satisfyText(a *runtime.SatisfyAssertion) string {
	if a == nil {
		return ""
	}
	var b strings.Builder
	if a.Negated {
		b.WriteString("not ")
	}
	b.WriteString("satisfy ")
	switch {
	case a.RequirementRef != "":
		b.WriteString(notationName(a.RequirementRef))
	case a.Symbol != nil && a.Symbol.Name != "":
		b.WriteString(notationName(a.Symbol.Name))
	default:
		b.WriteString("?")
	}
	if a.SubjectRef != "" {
		b.WriteString(" by ")
		if a.SubjectChain != nil {
			b.WriteString(chainNotation(a.SubjectChain))
		} else {
			b.WriteString(notationName(a.SubjectRef))
		}
	}
	return b.String()
}

// chainNotation spells a feature chain as the notation writes it: each member
// quoted on its own from the syntax, so a dot inside a quoted name stays inside
// its quotes, joined by the dots between members.
func chainNotation(node ast.Node) string {
	switch n := node.(type) {
	case *ast.FeatureChainExpr:
		return chainNotation(n.Operand) + "." + notationName(qualifiedText(n.Member))
	case *ast.FeatureReference:
		return notationName(qualifiedText(n.Name))
	case *ast.QualifiedName:
		return notationName(qualifiedText(n))
	}
	return ""
}
