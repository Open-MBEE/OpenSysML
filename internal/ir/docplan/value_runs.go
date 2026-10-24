package docplan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
)

// markupValue plans a value run: its target is resolved as a reference target
// is, then read as a span column reads a feature of the chain's root.
func (c *compiler) markupValue(owner, rep *symbols.Symbol, body string, run markup.Run, origin symbols.Origin) (Run, error) {
	line := 1 + strings.Count(body[:run.Span.Start], "\n")
	notation := fmt.Sprintf("%q", markup.RefScheme+markup.ValuePrefix+run.Destination)
	invalid := func(actual string) error {
		return &Error{
			Kind:     ErrorInvalidValueTarget,
			Document: c.document,
			Content:  symbols.FQNOf(owner),
			Actual:   actual,
			Line:     line,
			Origin:   origin,
		}
	}
	value, ok := parser.ParseOneExpression(rep.DocName, run.Destination)
	if !ok {
		return Run{}, invalid(notation)
	}
	target, _, err := c.resolveTargetValue(owner, rep.OwnerScope, value, ErrorInvalidValueTarget, ErrorUnknownValueTarget, origin)
	if err != nil {
		var planning *Error
		if errors.As(err, &planning) {
			planning.Line = line
			if planning.Actual == "" {
				planning.Actual = notation
			}
		}
		return Run{}, err
	}
	if !carriesValue(target) {
		return Run{}, invalid(fmt.Sprintf("%s, %s", symbols.FQNOf(target), elementKindWords(target)))
	}
	root, path := c.valuePath(rep.OwnerScope, value, target)
	return Run{kind: RunValue, text: run.Text, refSym: target, valueRoot: root, valuePath: path, bound: true, origin: origin}, nil
}

// carriesValue reports whether sym is a feature whose value is a datum rather
// than an occurrence: an attribute, calculation or enumeration usage. Parts,
// ports and the like are elements, which `sysml:` references by name.
func carriesValue(sym *symbols.Symbol) bool {
	switch {
	case sym.Kind.IsAttributeLike(), sym.Kind == symbols.SymbolCalcUsage, sym.Kind == symbols.SymbolEnumerationUsage:
		return true
	case sym.Kind == symbols.SymbolUnknown:
		return sym.DeclaresUsage()
	}
	return false
}

// elementKindWords names an element's kind for a diagnostic: "a part usage".
func elementKindWords(sym *symbols.Symbol) string {
	family := sym.Kind.Family()
	switch {
	case sym.Kind.IsDefinition():
		return "a " + family + " definition"
	case sym.Kind.IsFeature():
		return "a " + family + " usage"
	}
	return "a " + family
}

// valuePath splits the target into the element read and the member path read
// below it: a chain reads from its root, a name from the feature's owner.
func (c *compiler) valuePath(scope *symbols.Scope, value ast.Node, target *symbols.Symbol) (*symbols.Symbol, string) {
	node := value
	if reference, ok := node.(*ast.FeatureReference); ok {
		node = reference.Name
	}
	if chain, ok := node.(*ast.FeatureChainExpr); ok {
		var segments []string
		for inner := chain; ; {
			segments = append([]string{inner.Member.Text()}, segments...)
			next, ok := inner.Operand.(*ast.FeatureChainExpr)
			if !ok {
				break
			}
			inner = next
		}
		if root := c.chainRoot(scope, chain); root != nil {
			return root, strings.Join(segments, ".")
		}
	}
	return target.Owner(), target.Name
}
