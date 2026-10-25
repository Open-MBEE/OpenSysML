package docplan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
	if !c.carriesValue(target) {
		return Run{}, invalid(fmt.Sprintf("%s, %s", symbols.FQNOf(target), elementKindWords(target)))
	}
	root, path := c.valuePath(rep.OwnerScope, value, target)
	return Run{kind: RunValue, text: run.Text, refSym: target, valueRoot: root, valuePath: path, bound: true, origin: origin}, nil
}

// carriesValue reports whether sym is a feature whose value is a datum rather
// than an occurrence: an attribute, calculation or enumeration usage, or a
// reference usage typed by data types only. Parts, ports, references to them
// and the like are elements, which `sysml:` references by name.
func (c *compiler) carriesValue(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolAttributeUsage, symbols.SymbolCalcUsage, symbols.SymbolEnumerationUsage:
		return true
	case symbols.SymbolReferenceUsage:
		return c.referencesData(sym)
	case symbols.SymbolUnknown:
		return sym.DeclaresUsage()
	}
	return false
}

// referencesData reports whether a reference usage is typed by data types
// only, itself or through the features it redefines (`:>> mass = 120.0;`).
func (c *compiler) referencesData(sym *symbols.Symbol) bool {
	typed := false
	seen := map[*symbols.Symbol]bool{}
	for pending := []*symbols.Symbol{sym}; len(pending) > 0; pending = pending[1:] {
		feature := pending[0]
		if feature == nil || seen[feature] {
			continue
		}
		seen[feature] = true
		switch feature.Kind {
		case symbols.SymbolAttributeUsage, symbols.SymbolCalcUsage, symbols.SymbolEnumerationUsage:
			return true
		}
		for _, typ := range c.model.DeclaredTypes(feature) {
			if !c.model.IsDataType(typ) {
				return false
			}
			typed = true
		}
		pending = append(pending, c.model.RedefinedFeatures(feature)...)
	}
	return typed
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
// below it: a chain reads from its root, a name from the feature's owner. A
// chain member written `a::b` names two members in turn, so each part is one
// segment of the path, spelled as written.
func (c *compiler) valuePath(scope *symbols.Scope, value ast.Node, target *symbols.Symbol) (*symbols.Symbol, string) {
	node := value
	if reference, ok := node.(*ast.FeatureReference); ok {
		node = reference.Name
	}
	if chain, ok := node.(*ast.FeatureChainExpr); ok {
		var segments []string
		for inner := chain; ; {
			segments = append(memberSegments(inner.Member), segments...)
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

// memberSegments spells a chain member's parts, one path segment each, quoted
// as a member path needs.
func memberSegments(member *ast.QualifiedName) []string {
	if member == nil {
		return nil
	}
	segments := make([]string, len(member.Parts))
	for i, part := range member.Parts {
		segments[i] = source.NameText(part.Text)
	}
	return segments
}
