package semantics

import (
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ValuationFacts describe the value expression a feature declares (KerML 1.1
// Beta 2 §8.3.3.3.5 Feature::valuation, the FeatureValue's `value`, `isDefault`
// and `isInitial`), as a client reads them without the expression tree: the
// notation it was written in, what kind of expression it is, and the features
// it refers to.
type ValuationFacts struct {
	// Text is the expression as written, "" when the model keeps no notation.
	Text string
	// Metaclass is the expression's KerML metaclass ("FeatureChainExpression").
	Metaclass string
	// Kind is the metaclass in short: "literal", "null", "featureReference",
	// "featureChain", "operator", "invocation", "constructor", "index", …
	Kind string
	// IsDefault and IsInitial are the FeatureValue's flags (`default`, `:=`).
	IsDefault bool
	IsInitial bool
	// References are the features the expression reads, in source order.
	References []ValuationReference
	// Evaluated is the value the expression has at model level once the
	// features it reads are followed to the values they are bound to (EvalIn):
	// `usage_a.prop_a` with `:>> prop_a = 100.0` is 100.0. Nil when that does
	// not determine one — a feature with no value, a call, a quantity.
	Evaluated *Value
}

// ValuationReference is one feature an expression reads: a plain reference, or
// a chain whose Path lists the features chained, outermost first, ending at
// Target, the feature the reference denotes.
type ValuationReference struct {
	Text   string
	Path   []*symbols.Symbol
	Target *symbols.Symbol
}

// ValuationOf reads the facts of the value expression sym declares; false for a
// feature declaring none.
func (m *Model) ValuationOf(sym *symbols.Symbol) (*ValuationFacts, bool) {
	if m == nil || sym == nil {
		return nil, false
	}
	expr := m.ValueExpression(sym)
	if expr == nil {
		return nil, false
	}
	node, isDefault, isInitial, _ := featureValueOf(sym)
	metaclass := ""
	if mc := m.MetaclassOf(expr); mc != nil {
		metaclass = mc.Name
	}
	facts := &ValuationFacts{
		Text:      m.expressionText(expr),
		Metaclass: metaclass,
		Kind:      valuationKind(metaclass),
		IsDefault: isDefault,
		IsInitial: isInitial,
	}
	facts.References = m.appendValuationReferences(nil, expr)
	if node != nil {
		if value, ok := m.EvalIn(declScope(sym), node); ok {
			facts.Evaluated = &value
		}
	}
	return facts, true
}

// expressionText is the notation of the expression element expr, "" without a
// source lookup.
func (m *Model) expressionText(expr *symbols.Symbol) string {
	lookup := m.SourceText()
	if lookup == nil || expr == nil {
		return ""
	}
	return strings.TrimSpace(lookup(expr.DocName, expr.DeclSpan))
}

// valuationKind shortens an expression metaclass name: every literal is a
// "literal", the rest drop the "Expression" suffix and start lower-case.
func valuationKind(metaclass string) string {
	switch {
	case metaclass == "":
		return ""
	case strings.HasPrefix(metaclass, "Literal"):
		return "literal"
	case metaclass == "NullExpression":
		return "null"
	case metaclass == "Expression":
		return "body"
	}
	name := strings.TrimSuffix(metaclass, "Expression")
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// appendValuationReferences appends the features expr reads: a reference's
// referent, a chain's features in order, and the references of every operand
// of any other expression, in source order.
func (m *Model) appendValuationReferences(out []ValuationReference, expr *symbols.Symbol) []ValuationReference {
	if expr == nil {
		return out
	}
	switch expressionElementMetaclassName(expr) {
	case "FeatureReferenceExpression":
		if referents, ok := m.ReflectiveElements(expr, "referent"); ok && len(referents) == 1 && referents[0].IsFeature() {
			return append(out, ValuationReference{Text: m.expressionText(expr), Path: []*symbols.Symbol{referents[0]}, Target: referents[0]})
		}
		return out
	case "FeatureChainExpression":
		if operands := m.ExpressionArguments(expr); len(operands) == 1 && !expressionReads(operands[0]) {
			out = m.appendValuationReferences(out, operands[0])
		}
		if ref, ok := m.chainValuationReference(expr); ok {
			out = append(out, ref)
		}
		return out
	}
	for _, arg := range m.ExpressionArguments(expr) {
		out = m.appendValuationReferences(out, arg)
	}
	return out
}

// chainValuationReference is the reference a chain expression makes: the path
// its operand's own reference walks, extended by the chain's target — the
// chained features of a dotted member, else the member itself.
func (m *Model) chainValuationReference(expr *symbols.Symbol) (ValuationReference, bool) {
	var path []*symbols.Symbol
	if operands := m.ExpressionArguments(expr); len(operands) == 1 {
		if refs := m.appendValuationReferences(nil, operands[0]); len(refs) == 1 && expressionReads(operands[0]) {
			path = append(path, refs[0].Path...)
		}
	}
	if chain := m.ExpressionChainFeature(expr); chain != nil {
		path = append(path, m.ChainingFeatures(chain)...)
	} else if targets, ok := m.ReflectiveElements(expr, "targetFeature"); ok && len(targets) == 1 {
		path = append(path, targets[0])
	}
	if len(path) == 0 {
		return ValuationReference{}, false
	}
	return ValuationReference{Text: m.expressionText(expr), Path: path, Target: path[len(path)-1]}, true
}

// expressionReads reports whether expr is itself a reference or a chain, so its
// one reference is the feature its value is read from.
func expressionReads(expr *symbols.Symbol) bool {
	switch expressionElementMetaclassName(expr) {
	case "FeatureReferenceExpression", "FeatureChainExpression":
		return true
	}
	return false
}
