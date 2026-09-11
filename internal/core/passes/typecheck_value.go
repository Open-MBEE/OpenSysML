package passes

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// msgBoundValueType reports a bound value whose types are unrelated to the
// feature's; its arguments are the value's type names and the feature's.
const msgBoundValueType = "cannot bind a value of type %s to a feature typed by %s"

// checkValueConformance checks a bound value against the declaring feature's
// type and multiplicity for the cases the scalar lattice does not cover: values
// typed by a user-declared type, enumeration literals, and collections.
//
// Like the scalar rules, every check is one-sided — it reports only when both
// the expected and the actual property are known — so a partially typed model
// never produces a false positive.
func (ec *exprChecker) checkValueConformance(valueScope, declScope *symbols.Scope, d featureDecl, value ast.Node) {
	wants := ec.declaredTypeSymbols(declScope, d.relationships)
	if len(wants) == 0 {
		return
	}
	// A scalar value bound to a scalar-typed feature is the lattice rules' to
	// report — in lattice terms (Natural vs Integer), the more precise message.
	scalar := ec.anyScalar(wants)
	// A collection literal binds elementwise, so each element is checked
	// against the feature's type rather than the sequence as a whole.
	for _, value := range valueElements(value) {
		if feature := ec.valueFeature(valueScope, value); feature != nil {
			gots := ec.featureValueTypes(feature)
			if _, indexed := value.(*ast.IndexExpr); indexed {
				// The value is the selected element, not the indexed feature.
				gots = ec.model.ExprResultTypes(valueScope, value)
			}
			if len(gots) == 0 || (scalar && ec.anyScalar(gots)) {
				continue
			}
			// A binding equates the two features, so one conforming pairing in
			// either direction suffices; only unrelated types are rejected. The
			// feature is judged as a whole: a variant is typed by its variation too.
			if !ec.boundTypesConform(feature, gots, wants) {
				ec.errorf(value.Span(), msgBoundValueType, typeNames(gots), typeNames(wants))
			}
			continue
		}
		if elements, collection := ec.model.CollectionElements(valueScope, value); collection {
			// Every element a collection value may hold binds, not the Anything the
			// library declares; a scalar one written out is the lattice rules' to report.
			if gots := ec.unboundElementTypes(elements, wants, scalar); len(gots) > 0 {
				ec.errorf(value.Span(), msgBoundValueType, typeNames(gots), typeNames(wants))
			}
			continue
		}
		if result := ec.invocationResultParameter(valueScope, value); result != nil {
			gots := ec.featureValueTypes(result)
			if len(gots) > 0 && !(scalar && ec.anyScalar(gots)) && !ec.boundTypesConform(result, gots, wants) {
				ec.errorf(value.Span(), msgBoundValueType, typeNames(gots), typeNames(wants))
			}
			continue
		}
		if scalar {
			if len(wants) == 1 && wants[0].Kind == symbols.SymbolEnumerationDef {
				ec.checkEnumeratedValue(valueScope, wants[0], value)
			}
			continue
		}
		// The feature's type has no scalar ancestor, so no literal value can
		// conform to it. Only literals and bodies are judged here: any other
		// expression may produce an instance of the type.
		if prim := literalPrimType(value); prim != semantics.PrimUnknown {
			ec.errorf(value.Span(), "cannot bind %s value to a feature typed by %s", prim, typeNames(wants))
			continue
		}
		if _, ok := value.(*ast.BodyExpr); !ok {
			continue
		}
		got := ec.model.ExprResultType(valueScope, value)
		if got != nil && !ec.boundTypesConform(nil, []*symbols.Symbol{got}, wants) {
			ec.errorf(value.Span(), "cannot bind %s value to a feature typed by %s", semantics.PrimExpression, typeNames(wants))
		}
	}
}

// checkEnumeratedValue refuses a constant bound to an enumeration-typed feature that equals
// none of its enumerated values, the enumeration's only instances (SysML v2 §8.3.7).
func (ec *exprChecker) checkEnumeratedValue(scope *symbols.Scope, enum *symbols.Symbol, value ast.Node) {
	if ec.valueFeature(scope, value) != nil {
		return
	}
	got, ok := ec.constElement(scope, value)
	if !ok {
		return
	}
	literals := ec.model.EnumeratedValuesOf(enum)
	if len(literals) == 0 {
		ec.errorf(value.Span(), "cannot bind %s to a feature typed by %s, which enumerates no values", got.text, enum.Name)
		return
	}
	// A value of another kind than the enumerated ones is the lattice rules' to report.
	var enumerated []string
	sameKind := false
	for _, literal := range literals {
		declared := semantics.LiteralValue(literal)
		if declared == nil {
			continue // identified by itself, so equal to no constant
		}
		want, ok := ec.constElement(literal.OwnerScope, declared)
		if !ok {
			return
		}
		if want.equal(got) {
			return
		}
		sameKind = sameKind || want.comparableKind() == got.comparableKind()
		enumerated = append(enumerated, enumeratedText(enum, literal, want))
	}
	if len(enumerated) == 0 {
		ec.errorf(value.Span(), "cannot bind %s to a feature typed by %s, whose values are only its literals", got.text, enum.Name)
		return
	}
	if !sameKind {
		return
	}
	ec.errorf(value.Span(), "cannot bind %s to a feature typed by %s, whose values are %s",
		got.text, enum.Name, strings.Join(enumerated, ", "))
}

// enumeratedText names one enumerated value, `Level::high = 3`, or just `60.0` when unnamed.
func enumeratedText(enum, literal *symbols.Symbol, want constElement) string {
	if literal.Name == "" {
		return want.valueText()
	}
	return enum.Name + "::" + literal.Name + " = " + want.valueText()
}

// literalPrimType returns the scalar type of a literal value, or PrimUnknown
// for anything that is not one.
func literalPrimType(value ast.Node) semantics.PrimType {
	switch value.(type) {
	case *ast.LiteralBool:
		return semantics.PrimBoolean
	case *ast.LiteralString:
		return semantics.PrimString
	case *ast.LiteralInteger:
		return semantics.PrimNatural
	case *ast.LiteralReal:
		return semantics.PrimRational
	}
	return semantics.PrimUnknown
}

// checkValueCount checks a bound value's element count against the multiplicity
// governing the feature.
func (ec *exprChecker) checkValueCount(valueScope, declScope *symbols.Scope, d featureDecl, value ast.Node) {
	held, known := ec.heldCount(valueScope, value)
	if !known {
		return
	}
	r, ok := ec.effectiveRange(declScope, d, 0)
	if !ok {
		return
	}
	if msg := r.HeldViolation(held); msg != "" {
		ec.errorf(value.Span(), "%s", msg)
	}
}

// checkValueUniqueness reports a literal writing one const-decidable value twice to
// a unique, sequence-held feature (KerML 7.3.4.4); other equality is the runtime's.
func (ec *exprChecker) checkValueUniqueness(valueScope, declScope *symbols.Scope, d featureDecl, value ast.Node) {
	if _, ok := value.(*ast.SequenceExpr); !ok {
		return
	}
	feature := declaredSymbol(declScope, d.node)
	if feature == nil || !ec.model.IsUnique(feature) || ec.model.HoldsSet(feature, ownerType(feature)) {
		return
	}
	var seen []constElement
	for i, element := range valueElements(value) {
		el, ok := ec.constElement(valueScope, element)
		if !ok {
			continue
		}
		el.position = i + 1
		for _, prior := range seen {
			if prior.equal(el) {
				ec.errorf(element.Span(), "%s", semantics.UniquenessViolation(el.text, prior.position, el.position))
				return
			}
		}
		seen = append(seen, el)
	}
}

// constElement is one element of a collection literal whose value is decided
// statically: a scalar constant, a string literal or an enumeration literal.
type constElement struct {
	scalar   semantics.Value
	str      string
	literal  *symbols.Symbol
	text     string
	position int
}

// equal holds when the two elements are one value, as the runtime's equality judges.
func (e constElement) equal(o constElement) bool {
	switch {
	case e.literal != nil || o.literal != nil:
		return e.literal == o.literal
	case e.scalar.Kind != semantics.ValInvalid || o.scalar.Kind != semantics.ValInvalid:
		eq, ok := semantics.EvalBinary(ast.OpEq, e.scalar, o.scalar)
		return ok && eq.Kind == semantics.ValBool && eq.Bool
	}
	return e.str == o.str
}

// comparableKind partitions elements into those equality can relate: numbers with
// numbers, booleans with booleans, strings with strings, literals with literals.
func (e constElement) comparableKind() string {
	switch {
	case e.literal != nil:
		return "literal"
	case e.scalar.Kind == semantics.ValBool:
		return "boolean"
	case e.scalar.Kind != semantics.ValInvalid:
		return "number"
	}
	return "string"
}

// valueText is the element's value as written, without its kind.
func (e constElement) valueText() string {
	switch {
	case e.literal != nil:
		return e.literal.Name
	case e.scalar.Kind != semantics.ValInvalid:
		return semantics.FormatConst(e.scalar)
	}
	return strconv.Quote(e.str)
}

// constElement decides an element's value statically, or reports it cannot.
func (ec *exprChecker) constElement(scope *symbols.Scope, element ast.Node) (constElement, bool) {
	if s, ok := element.(*ast.LiteralString); ok {
		text := lexer.StringValue(s.Value)
		return constElement{str: text, text: strconv.Quote(text) + " (string)"}, true
	}
	if v, ok := ec.model.Eval(element); ok && v.Kind != semantics.ValInfinity {
		return constElement{scalar: v, text: semantics.FormatConst(v) + " (" + constKind(v) + ")"}, true
	}
	if sym := ec.valueFeature(scope, element); sym != nil {
		if enum := ec.owningEnumeration(sym); enum != nil {
			return constElement{literal: sym, text: enum.Name + "::" + sym.Name + " (enumeration literal)"}, true
		}
	}
	return constElement{}, false
}

// constKind names a constant's kind as the runtime describes one.
func constKind(v semantics.Value) string {
	switch v.Kind {
	case semantics.ValInt:
		return "an Integer"
	case semantics.ValReal:
		return "a Real"
	default:
		return "a Boolean"
	}
}

// ownerType is the symbol whose body declares a feature, or nil.
func ownerType(feature *symbols.Symbol) *symbols.Symbol {
	if feature == nil || feature.OwnerScope == nil {
		return nil
	}
	return feature.OwnerScope.Owner()
}

// declaredSymbol is the symbol scope registers for a declaration, or nil.
func declaredSymbol(scope *symbols.Scope, decl ast.Node) *symbols.Symbol {
	if scope == nil || decl == nil {
		return nil
	}
	for _, sym := range scope.AllMembers() {
		if sym.Decl == decl {
			return sym
		}
	}
	return nil
}

// maxRedefinitionDepth bounds the redefinition chain the effective multiplicity
// is looked up along, so a cyclic chain terminates.
const maxRedefinitionDepth = 32

// effectiveRange returns the multiplicity governing a feature: the one it
// declares, or the one it inherits from the feature it redefines
// (KerML 1.0 §7.3.4.5).
func (ec *exprChecker) effectiveRange(scope *symbols.Scope, d featureDecl, depth int) (semantics.Range, bool) {
	if r, ok := ec.model.RangeOf(d.multiplicity); ok {
		return r, true
	}
	if depth >= maxRedefinitionDepth {
		return semantics.Range{}, false
	}
	for _, rel := range d.relationships {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		target, ok := ec.resolver.ResolveRedefinitionTarget(scope, d.node, rel.Target)
		if !ok || target == nil {
			continue
		}
		td, isFeature := featureDeclOf(target.Decl)
		if !isFeature || td.node == d.node {
			continue
		}
		if r, ok := ec.effectiveRange(target.OwnerScope, td, depth+1); ok {
			return r, true
		}
	}
	return semantics.Range{}, false
}

// heldCount is how many values a bound expression produces, where statically bounded: one per
// literal, the sum over a collection literal, what a collection operation holds; else unknown.
func (ec *exprChecker) heldCount(scope *symbols.Scope, value ast.Node) (semantics.Range, bool) {
	if value == nil {
		return semantics.Range{}, false
	}
	if _, ok := value.(*ast.NullExpr); ok {
		return semantics.CountRange(0), true
	}
	// Binding flattens a collection into the values its elements produce, so a
	// nested literal contributes its own elements rather than one value.
	if seq, ok := value.(*ast.SequenceExpr); ok {
		total := semantics.CountRange(0)
		for _, element := range seq.Elements {
			held, ok := ec.heldCount(scope, element)
			if !ok {
				return semantics.Range{}, false
			}
			total = total.Plus(held)
		}
		return total, true
	}
	if literalPrimType(value) != semantics.PrimUnknown {
		return semantics.CountRange(1), true
	}
	return ec.model.CollectionValues(scope, value)
}

// valueElements returns the values a bound expression contributes: the elements
// of a collection literal, nested ones flattened, or the expression itself.
func valueElements(value ast.Node) []ast.Node {
	if value == nil {
		return nil
	}
	switch n := value.(type) {
	case *ast.NullExpr:
		return nil
	case *ast.SequenceExpr:
		var elements []ast.Node
		for _, element := range n.Elements {
			elements = append(elements, valueElements(element)...)
		}
		return elements
	}
	return []ast.Node{value}
}

// argumentElements is the values an argument binds: those written — the argument, or the elements
// of a collection literal — and, apart, each element a collection value among them holds.
func (ec *exprChecker) argumentElements(scope *symbols.Scope, value ast.Node) (written []ast.Node, held []semantics.CollectionElement) {
	for _, element := range valueElements(value) {
		if elements, collection := ec.model.CollectionElements(scope, element); collection {
			held = append(held, elements...)
			continue
		}
		written = append(written, element)
	}
	return written, held
}

// heldSpan is where a held element is written, or the argument when a result parameter produces it.
func heldSpan(el semantics.CollectionElement, argument ast.Node) source.Span {
	if el.Node != nil {
		return el.Node.Span()
	}
	return argument.Span()
}

// heldPrim is the scalar type of a held element, read silently — the collection value has been
// typed and reported once already; a result parameter's by its type.
func (ec *exprChecker) heldPrim(el semantics.CollectionElement) semantics.PrimType {
	if el.Node != nil {
		return ec.silent().infer(el.Scope, el.Node)
	}
	for _, t := range el.Types {
		if prim := ec.model.PrimTypeOf(t); prim != semantics.PrimUnknown {
			return prim
		}
	}
	return semantics.PrimUnknown
}

// silent is a checker typing as ec does, under the same chains and performances, reporting nothing.
func (ec *exprChecker) silent() *exprChecker {
	return &exprChecker{resolver: ec.resolver, model: ec.model, lang: ec.lang, chaining: ec.chaining, performed: ec.performed}
}

// declaredTypeSymbol returns the symbol a usage is typed by, or nil.
func (ec *exprChecker) declaredTypeSymbol(scope *symbols.Scope, rels []*ast.Relationship) *symbols.Symbol {
	if types := ec.declaredTypeSymbols(scope, rels); len(types) > 0 {
		return types[0]
	}
	return nil
}

// declaredTypeSymbols returns every resolved type a usage `x : A, B` is typed by.
func (ec *exprChecker) declaredTypeSymbols(scope *symbols.Scope, rels []*ast.Relationship) []*symbols.Symbol {
	var types []*symbols.Symbol
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if sym := ec.resolveTarget(scope, rel.Target); sym != nil {
			types = append(types, sym)
		}
	}
	return types
}

// anyScalar reports whether one of the types has a scalar ancestor.
func (ec *exprChecker) anyScalar(types []*symbols.Symbol) bool {
	for _, t := range types {
		if ec.model.PrimTypeOf(t) != semantics.PrimUnknown {
			return true
		}
	}
	return false
}

// boundTypesConform reports whether a bound feature, typed by gots, may be bound to
// a feature typed by wants: the feature itself conforms to a want, or some got and
// some want conform one way or the other (KerML 8.3.4.3, one compatible pairing).
func (ec *exprChecker) boundTypesConform(feature *symbols.Symbol, gots, wants []*symbols.Symbol) bool {
	for _, want := range wants {
		if ec.model.Conforms(feature, want) {
			return true
		}
		for _, got := range gots {
			if ec.model.Conforms(got, want) || ec.model.Conforms(want, got) {
				return true
			}
		}
	}
	return false
}

// unboundElementTypes is the types of those elements of a collection value none of whose
// types binds to a feature typed by wants; an untyped element binds, and a scalar expression
// bound to a scalar feature is left to the lattice rules.
func (ec *exprChecker) unboundElementTypes(elements []semantics.CollectionElement, wants []*symbols.Symbol, scalar bool) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, element := range elements {
		gots := element.Types
		if len(gots) == 0 || ec.boundTypesConform(nil, gots, wants) {
			continue
		}
		if scalar && element.Node != nil && ec.anyScalar(gots) {
			continue
		}
		for _, got := range gots {
			if !slices.Contains(out, got) {
				out = append(out, got)
			}
		}
	}
	return out
}

// typeNames joins the names of types as a declaration lists them.
func typeNames(types []*symbols.Symbol) string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

// valueTypeSymbol returns the type of a bound value as a symbol, or nil when it
// is not a feature the checker can type (a literal, an expression, an unresolved
// name). A feature chain is typed by its last feature (KerML 8.3.3.3).
func (ec *exprChecker) valueTypeSymbol(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	if feature := ec.valueFeature(scope, value); feature != nil {
		return ec.featureValueType(feature)
	}
	return nil
}

// valueFeature returns the usage a bound value names outright, aliases followed;
// nil for a computed value, an unresolved name or a definition.
func (ec *exprChecker) valueFeature(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	if !namesFeature(value) {
		return nil
	}
	sym, resolved := ec.resolver.ResolveTarget(scope, value)
	if !resolved || sym == nil {
		return nil
	}
	if sym.Kind == symbols.SymbolAlias {
		if target, ok := ec.resolver.ResolveAliasTarget(sym); ok && target != nil {
			sym = target
		}
	}
	if _, isUsage := sym.Decl.(*ast.Usage); !isUsage {
		// A definition used as a value is a type name, not an instance of one;
		// the checker has no rule for that.
		return nil
	}
	return sym
}

// featureValueType returns the declared type of a usage, or the enumeration
// owning an enumeration literal; nil when it declares none.
func (ec *exprChecker) featureValueType(sym *symbols.Symbol) *symbols.Symbol {
	if types := ec.featureValueTypes(sym); len(types) > 0 {
		return types[0]
	}
	return nil
}

// featureValueTypes returns every declared type of a usage, or the enumeration
// owning an enumeration literal; none when it declares none.
func (ec *exprChecker) featureValueTypes(sym *symbols.Symbol) []*symbols.Symbol {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	// The type name resolves from the scope the usage was declared in, as
	// generalization targets do elsewhere (semantics.DirectSupertypes, the
	// subsetting conformance checks). Resolving from the scope the usage owns
	// would let its own members shadow the type it names.
	if typed := ec.declaredTypeSymbols(sym.OwnerScope, u.Relationships); len(typed) > 0 {
		return typed
	}
	// An enumeration literal declares no type: it is a member of the
	// enumeration that owns it, which is what it conforms to.
	if u.Kind == ast.UsageEnumeration {
		if enum := ec.owningEnumeration(sym); enum != nil {
			return []*symbols.Symbol{enum}
		}
	}
	return nil
}

// invocationResultTypeSymbol returns the type of the result parameter of the
// behavior an invocation names, which is the type of the value it produces; for
// a collection value (`xs.{…}`, a collection function call) the one type the elements it holds
// have — none when it holds nothing.
func (ec *exprChecker) invocationResultTypeSymbol(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	if types, collection := ec.model.CollectionHeldTypes(scope, value); collection {
		if len(types) == 1 {
			return types[0]
		}
		return nil
	}
	result := ec.invocationResultParameter(scope, value)
	if result == nil {
		return nil
	}
	u, isUsage := result.Decl.(*ast.Usage)
	if !isUsage {
		return nil
	}
	return ec.declaredTypeSymbol(result.OwnerScope, u.Relationships)
}

// invocationResultParameter returns the result parameter of the behavior an
// invocation names; nil for any other value or an unresolved invocation.
func (ec *exprChecker) invocationResultParameter(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	inv, ok := value.(*ast.InvocationExpr)
	if !ok {
		return nil
	}
	var sym *symbols.Symbol
	if chain := ChainCallee(inv); chain != nil {
		sym, _ = ec.resolver.ResolveTarget(scope, chain)
	} else if inv.Type != nil {
		// The arguments type silently, but under the chains being typed: one whose
		// body reads the feature being valued would otherwise type it again.
		silent := ec.silent()
		sym = silent.selectInvocation(scope, inv, silent.argumentTypes(scope, inv), ec.performs(inv)).Selected
	}
	if sym == nil || !ec.isInvocationBehavior(sym, map[*symbols.Symbol]bool{}) {
		return nil
	}
	return ec.model.ResultParameterOf(sym)
}

// constructedTypeSymbol returns the type a constructor `new T(…)` instantiates,
// which is the type of the instance it produces, or nil for any other value.
func (ec *exprChecker) constructedTypeSymbol(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	ctor, ok := value.(*ast.ConstructorExpr)
	if !ok || ctor.Type == nil {
		return nil
	}
	return ec.resolveTarget(scope, ctor.Type)
}

// owningEnumeration returns the enumeration definition a literal belongs to, or
// nil when the literal is not owned by one.
func (ec *exprChecker) owningEnumeration(sym *symbols.Symbol) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || owner.Kind != symbols.SymbolEnumerationDef {
		return nil
	}
	return owner
}

// namesFeature reports whether a value expression names a feature outright — a
// qualified name, a feature chain or an element `a#(i)` — rather than computing one.
func namesFeature(value ast.Node) bool {
	switch v := value.(type) {
	case *ast.FeatureReference:
		return v.Name != nil
	case *ast.QualifiedName:
		return true
	case *ast.FeatureChainExpr:
		return v.Member != nil && namesFeature(v.Operand)
	case *ast.IndexExpr:
		return !v.Bracket && namesFeature(v.Operand)
	}
	return false
}
