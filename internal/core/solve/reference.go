package solve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// reference translates a name or a feature chain: an enumeration literal or a
// variant becomes a value of its finite sort, and a scalar-valued feature becomes
// a variable standing for the values it may take.
func (t *translator) reference(node ast.Node, scope *symbols.Scope) (*Term, error) {
	segments, ok := pathSegments(node)
	if !ok || len(segments) == 0 {
		return nil, t.refuse(node, "reference", "it names nothing this translation can ground")
	}
	chain, err := t.resolvePath(node, scope, segments)
	if err != nil {
		return nil, err
	}
	target := chain[len(chain)-1]
	if sort, value, ok := t.valueOf(target); ok {
		return ValueTerm(sort, value), nil
	}
	return t.variableOf(node, scope, chain, segments)
}

// pathSegments flattens a name or a feature chain into the names it steps
// through, reporting false for anything else.
func pathSegments(node ast.Node) ([]string, bool) {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return nameSegments(n.Name)
	case *ast.QualifiedName:
		return nameSegments(n)
	case *ast.FeatureChainExpr:
		base, ok := pathSegments(n.Operand)
		if !ok {
			return nil, false
		}
		rest, ok := nameSegments(n.Member)
		if !ok {
			return nil, false
		}
		return append(base, rest...), true
	}
	return nil, false
}

// nameSegments returns the names a qualified name steps through.
func nameSegments(qn *ast.QualifiedName) ([]string, bool) {
	if qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		out = append(out, part.Text)
	}
	return out, true
}

// msgReferencePrefix names the reference a refusal is about.
const msgReferencePrefix = "reference `"

// resolvePath resolves the names a reference steps through, the same way the
// evaluator resolves them: the first as an effective feature of the element the
// conditions are stated by, else in scope, each further one as a member or a
// variant of what the previous named.
func (t *translator) resolvePath(node ast.Node, scope *symbols.Scope, segments []string) ([]*symbols.Symbol, error) {
	if scope == nil {
		return nil, t.refuse(node, msgReferencePrefix+joinPath(segments)+"`", "it is written in no scope")
	}
	resolver := t.ctx.Resolver()
	var chain []*symbols.Symbol
	// A member the objective being translated owns or inherits shadows the case's
	// and is read through it, so two objectives' `best` are two values.
	sym, ok := t.objectiveLocal(scope, segments[0])
	if !ok {
		if sym, ok = t.model.LookupMember(t.within, segments[0]); ok {
			chain = append(chain, t.within)
		} else if sym, ok = t.features[segments[0]]; !ok {
			sym, ok = resolver.LookupName(scope, segments[0])
		}
	}
	if !ok {
		// A qualified name may name a package member or a library element.
		if whole, found := resolver.ResolveQualified(scope, ast.QualifiedNameOf(segments...)); found {
			return []*symbols.Symbol{whole}, nil
		}
		return nil, t.refuse(node, msgReferencePrefix+joinPath(segments)+"`", "it resolves to nothing")
	}
	chain = append(chain, sym)
	for _, segment := range segments[1:] {
		next, ok := t.model.LookupMember(sym, segment)
		if !ok {
			next, ok = t.model.VariantOf(sym, segment)
		}
		if !ok {
			return nil, t.refuse(node, msgReferencePrefix+joinPath(segments)+"`",
				"`"+segment+"` names no member of `"+sym.Name+"`")
		}
		sym = next
		chain = append(chain, sym)
	}
	return chain, nil
}

// objectiveLocal resolves name to a declaration nested inside the objective being
// translated, or one of its types, that is not a member of the objective itself:
// a condition's own parameter or attribute, which the objective's members do not shadow.
func (t *translator) objectiveLocal(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	if t.within == nil {
		return nil, false
	}
	sym, ok := t.ctx.Resolver().LookupName(scope, name)
	if !ok {
		return nil, false
	}
	if member, own := t.model.LookupMember(t.within, name); own && member == sym {
		return nil, false
	}
	for s := sym.OwnerScope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == t.within || (owner != nil && t.model.Conforms(t.within, owner)) {
			return sym, true
		}
	}
	return nil, false
}

// joinPath renders a reference as it was written.
func joinPath(segments []string) string {
	out := ""
	for i, segment := range segments {
		if i > 0 {
			out += "."
		}
		out += segment
	}
	return out
}

// valueOf returns the finite sort and the value a reference to an enumeration
// literal or a variant stands for. An enumerated value is a variant of its
// enumeration, whose sort is the enumeration's rather than a variation point's.
func (t *translator) valueOf(sym *symbols.Symbol) (Sort, string, bool) {
	if enum := semantics.EnumerationOwning(sym); enum != nil {
		return t.datatype(enum, t.model.LiteralsOf(enum), false), t.fqn(sym), true
	}
	if variation := t.model.VariationPointOwning(sym); variation != nil {
		return t.datatype(variation, t.model.VariantsOf(variation), true), t.fqn(sym), true
	}
	return Sort{}, "", false
}

// datatype declares, once per definition, the finite sort its values range over;
// variation marks the sort of a variation point rather than of an enumeration.
func (t *translator) datatype(origin *symbols.Symbol, values []*symbols.Symbol, variation bool) Sort {
	name := t.fqn(origin)
	if sort, ok := t.sorts[name]; ok {
		return sort
	}
	sort := Sort{
		Kind:      SortDatatype,
		Name:      name,
		Origin:    t.fqn(origin),
		Variation: variation,
	}
	for _, value := range values {
		sort.Values = append(sort.Values, t.fqn(value))
	}
	t.sorts[name] = sort
	return sort
}

// variationDeclaring is the variation point whose variants these are: a usage
// redefining a variation selects among the variants the redefined one declares,
// so both read the same finite sort a variant's name stands for.
func (t *translator) variationDeclaring(sym *symbols.Symbol, variants []*symbols.Symbol) *symbols.Symbol {
	for _, variant := range variants {
		if owner := t.model.VariationPointOwning(variant); owner != nil {
			return owner
		}
	}
	return sym
}

// variableOf returns the variable a reference to a feature reads, declaring it
// the first time the feature is read. A feature whose type determines no scalar
// sort, or that holds more than one value, refuses.
func (t *translator) variableOf(
	node ast.Node,
	scope *symbols.Scope,
	chain []*symbols.Symbol,
	segments []string,
) (*Term, error) {
	target := chain[len(chain)-1]
	written := "feature `" + joinPath(segments) + "`"
	if !featureDecl(target) {
		return nil, t.refuse(node, written, "it names a definition rather than a value")
	}
	if target.Kind == symbols.SymbolCalcUsage {
		return nil, t.refuse(node, written, "a calc's result is computed rather than encoded")
	}
	// Every step a chain reads through must hold one value, since a variable
	// stands for one: `a.pressure` reads a pressure per `a` when `a` holds many.
	for _, step := range chain {
		if !featureDecl(step) {
			continue
		}
		if !singleValued(t.model.EffectiveMultiplicityOf(step)) {
			return nil, t.refuse(node, written, "it may hold more than one value")
		}
	}
	sort, dimension, err := t.sortOf(node, scope, target, written)
	if err != nil {
		return nil, err
	}
	return VarTerm(t.declare(variableName(t, chain), sort, target, dimension)), nil
}

// singleValued reports whether a multiplicity admits exactly one value, which is
// what a variable stands for.
func singleValued(r semantics.Range) bool {
	if !r.Upper.Known || r.Upper.Infinite {
		return false
	}
	return r.Upper.Value <= 1
}

// sortOf decides the sort a feature's values range over, from the type facts the
// semantic model holds rather than from any value written for it.
func (t *translator) sortOf(
	node ast.Node,
	scope *symbols.Scope,
	sym *symbols.Symbol,
	written string,
) (Sort, string, error) {
	if t.model.IsVariationFeature(sym) {
		variants := t.model.VariantsOf(sym)
		if len(variants) == 0 {
			return Sort{}, "", t.refuse(node, written, "the variation point offers no variant")
		}
		return t.datatype(t.variationDeclaring(sym, variants), variants, true), "", nil
	}
	if enum := t.enumerationTypeOf(sym); enum != nil {
		literals := t.model.LiteralsOf(enum)
		if len(literals) == 0 {
			return Sort{}, "", t.refuse(node, written, "its enumeration declares no literal")
		}
		return t.datatype(enum, literals, false), "", nil
	}
	switch t.model.PrimTypeOf(sym) {
	case semantics.PrimBoolean:
		return Bool, "", nil
	case semantics.PrimString:
		return String, "", nil
	case semantics.PrimNatural, semantics.PrimInteger:
		return Int, "", nil
	case semantics.PrimRational, semantics.PrimReal, semantics.PrimNumber:
		return Real, t.dimensionText(scope, node), nil
	case semantics.PrimComplex:
		return Sort{}, "", t.refuse(node, written, "a complex number has no sort here")
	}
	if dim, ok := t.model.DimensionOfExpr(scope, node); ok {
		return Real, dimensionUnits(dim), nil
	}
	return Sort{}, "", t.refuse(node, written, "its type determines no scalar sort")
}

// enumerationTypeOf returns the enumeration definition typing sym, or nil.
func (t *translator) enumerationTypeOf(sym *symbols.Symbol) *symbols.Symbol {
	for _, super := range t.model.AllSupertypes(sym) {
		if super.Kind == symbols.SymbolEnumerationDef {
			return super
		}
	}
	return nil
}

// dimensionText names the dimension a reference's magnitudes are expressed in,
// empty for a plain number.
func (t *translator) dimensionText(scope *symbols.Scope, node ast.Node) string {
	dim, ok := t.model.DimensionOfExpr(scope, node)
	if !ok {
		return ""
	}
	return dimensionUnits(dim)
}

// dimensionUnits renders a dimension's base units, empty for a dimensionless one.
func dimensionUnits(dim semantics.Dimension) string {
	if dim.Term.Dimensionless() {
		return ""
	}
	return dim.String()
}

// declare returns the variable of the name given, declaring it and any bound its
// declaration puts on its values the first time it is read.
func (t *translator) declare(name string, sort Sort, sym *symbols.Symbol, dimension string) *Var {
	if v, ok := t.vars[name]; ok {
		return v
	}
	v := &Var{Name: name, Sort: sort, Symbol: sym, Dimension: dimension}
	if sym != nil {
		v.File = sym.DocName
		v.Span = sym.DeclSpan
		v.Location = t.ctx.SourceLocation(v.File, v.Span)
	}
	t.vars[name] = v
	if t.model.PrimTypeOf(sym) == semantics.PrimNatural {
		t.domains = append(t.domains, Assertion{
			Term: Binary(OpGe, Bool, VarTerm(v), IntTerm(0)),
			From: Provenance{
				Kind:      "declaration",
				Element:   name,
				Condition: "a Natural is not negative",
				Role:      RoleDomain,
				Declared:  sym,
				File:      v.File,
				Span:      v.Span,
				Location:  v.Location,
			},
		})
	}
	return v
}

// variableName names the variable a reference reads: the feature's own qualified
// name, extended by the features a chain steps through, since two objects hold
// two values.
func variableName(t *translator, chain []*symbols.Symbol) string {
	first := len(chain) - 1
	for i, sym := range chain {
		if featureDecl(sym) {
			first = i
			break
		}
	}
	name := t.fqn(chain[first])
	for _, sym := range chain[first+1:] {
		name += "." + sym.Name
	}
	return name
}

// featureDecl reports whether sym declares a feature holding a value: a usage, or
// the subject a requirement declares. A definition or a package holds none.
func featureDecl(sym *symbols.Symbol) bool {
	switch sym.Decl.(type) {
	case *ast.Usage, *ast.SubjectMember:
		return true
	}
	return false
}

// fqn names a symbol as its qualified name, falling back to its own name for one
// no scope chain qualifies.
func (t *translator) fqn(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if name := symbols.FQNOf(sym); name != "" {
		return name
	}
	return sym.Name
}
