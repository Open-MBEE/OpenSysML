package semantics

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// UnitPower is a named measurement unit raised to an exponent: one factor of a
// unit product, `N*m` being the powers N¹ and m¹.
type UnitPower struct {
	Unit         *symbols.Symbol // the unit the name resolves to, nil where it resolves to none
	Name         string          // the unit as the model wrote it, quoted where the notation must (`'A/m'`)
	Exponent     float64
	DimensionOne bool      // the unit reduces to no base unit: an angle, a ratio, a count
	Reduces      *UnitTerm // what a unit resolving to none reduces to, where known; nil otherwise
}

// UnitProduct is a unit as a sorted product of powers of the units the model named,
// the canonical display form (`N*m`, `m**2`); UnitTerm, not this, decides conversion.
type UnitProduct struct {
	Powers []UnitPower
}

// NamedUnitProduct is the product of one named unit to the first power, spelt
// as one name: text that is no name is quoted so the product reads back as itself.
func NamedUnitProduct(unit *symbols.Symbol, name string, dimensionOne bool) UnitProduct {
	return UnitProduct{Powers: []UnitPower{{Unit: unit, Name: unitNameSpelling(name), Exponent: 1, DimensionOne: dimensionOne}}}
}

// OpaqueUnitProduct is the product of one unit that resolves to no declaration,
// known by its text and what it reduces to; the reduction tells two so spelt apart.
func OpaqueUnitProduct(name string, reduces UnitTerm) UnitProduct {
	p := NamedUnitProduct(nil, name, reduces.Dimensionless())
	p.Powers[0].Reduces = &reduces
	return p
}

// unitNameSpelling quotes a name the notation cannot read as one name (`metres per
// second`, `1000·metre`), escaping what a quoted name must (`'it\'s'`).
func unitNameSpelling(name string) string {
	if name == "" || atomicName(name) {
		return name
	}
	return lexer.UnrestrictedNameText(name)
}

// IsEmpty reports whether the product names no unit, as a bare number does.
func (p UnitProduct) IsEmpty() bool { return len(p.Powers) == 0 }

// NamesDimensionOne reports whether the product is a named dimension-one unit
// (`rad`, `°`, `sr`) rather than nothing or a ratio (`°/rad`) that cancels to a number.
func (p UnitProduct) NamesDimensionOne() bool {
	if p.IsEmpty() {
		return false
	}
	sign := p.Powers[0].Exponent > 0
	for _, f := range p.Powers {
		if !f.DimensionOne || (f.Exponent > 0) != sign {
			return false
		}
	}
	return true
}

// Clone returns a product sharing no storage with p, its powers' reductions included.
func (p UnitProduct) Clone() UnitProduct {
	out := UnitProduct{Powers: slices.Clone(p.Powers)}
	for i, f := range out.Powers {
		if f.Reduces != nil {
			reduces := f.Reduces.Clone()
			out.Powers[i].Reduces = &reduces
		}
	}
	return out
}

// ShortSpelling respells each declared unit by its short name (`km/h` for `SI::km/SI::h`),
// so the spelling does not depend on the scope that named it; an undeclared one keeps its text.
func (p UnitProduct) ShortSpelling() UnitProduct {
	out := p.Clone()
	for i, f := range out.Powers {
		if f.Unit != nil {
			out.Powers[i].Name = unitNameSpelling(unitShortName(f.Unit))
		}
	}
	return out
}

// Times returns the product of two unit products.
func (p UnitProduct) Times(q UnitProduct) UnitProduct { return combineProducts(p, q, 1) }

// DividedBy returns the quotient of two unit products.
func (p UnitProduct) DividedBy(q UnitProduct) UnitProduct { return combineProducts(p, q, -1) }

// Pow raises every power of the product to exp.
func (p UnitProduct) Pow(exp float64) UnitProduct {
	out := UnitProduct{}
	for _, f := range p.Powers {
		f.Exponent *= exp
		out.Powers = append(out.Powers, f)
	}
	return normalizeProduct(out)
}

// Root divides every power by n, reporting false where a power does not divide
// into a whole exponent: `m**2` has a square root `m`, `rad` and `km*m` have none.
func (p UnitProduct) Root(n float64) (UnitProduct, bool) {
	for _, f := range p.Powers {
		if math.Mod(f.Exponent, n) != 0 {
			return UnitProduct{}, false
		}
	}
	return p.Pow(1 / n), true
}

// String renders the product as the notation reads it back: positive powers joined
// by `*`, negative ones grouped after one `/`, an exponent other than one as `**n`.
func (p UnitProduct) String() string {
	var num, den []string
	for _, f := range p.Powers {
		switch {
		case f.Exponent > 0:
			num = append(num, powerText(f.Name, f.Exponent))
		case f.Exponent < 0:
			den = append(den, powerText(f.Name, -f.Exponent))
		}
	}
	out := strings.Join(num, "*")
	if len(den) == 0 {
		if out == "" {
			return "1"
		}
		return out
	}
	if out == "" {
		out = "1"
	}
	if len(den) == 1 {
		return out + "/" + den[0]
	}
	return out + "/(" + strings.Join(den, "*") + ")"
}

// powerText renders one unit raised to a positive exponent, grouping a name
// that is itself an expression so `(km/h)**2` is not read as `km/(h**2)`.
func powerText(name string, exp float64) string {
	if !atomicName(name) && strings.ContainsAny(name, "*/^·⋅") {
		name = "(" + name + ")"
	}
	if exp == 1 {
		return name
	}
	return fmt.Sprintf("%s**%g", name, exp)
}

// atomicName reports whether name is one qualified name as the notation spells it,
// each segment a basic name or quoted, so its operator characters are all in quotes.
func atomicName(name string) bool {
	for name != "" {
		rest, ok := afterNameSegment(name)
		if !ok {
			return false
		}
		if rest == "" {
			return true
		}
		if !strings.HasPrefix(rest, "::") {
			return false
		}
		name = rest[2:]
	}
	return false
}

// afterNameSegment strips one leading name segment — a basic name spelling no keyword,
// or a quoted one (which may hold `::` and escapes) — reporting false where name starts with neither.
func afterNameSegment(name string) (string, bool) {
	if strings.HasPrefix(name, "'") {
		for i := 1; i < len(name); i++ {
			switch name[i] {
			case '\\':
				i++
			case '\'':
				return name[i+1:], i > 1
			}
		}
		return "", false
	}
	segment, rest := name, ""
	if at := strings.Index(name, "::"); at >= 0 {
		segment, rest = name[:at], name[at:]
	}
	return rest, lexer.IsIdentifier(segment) && !lexer.IsKeyword(segment)
}

// combineProducts multiplies two products, with the exponents of the second
// signed by sign so that division shares multiplication's accumulation.
func combineProducts(a, b UnitProduct, sign float64) UnitProduct {
	out := UnitProduct{}
	out.Powers = append(out.Powers, a.Powers...)
	for _, f := range b.Powers {
		f.Exponent *= sign
		out.Powers = append(out.Powers, f)
	}
	return normalizeProduct(out)
}

// normalizeProduct merges repeated units (by symbol, or by name and reduction
// where both are unresolved), drops cancelled powers and orders the rest by name.
func normalizeProduct(p UnitProduct) UnitProduct {
	merged := make([]UnitPower, 0, len(p.Powers))
	for _, f := range p.Powers {
		at := slices.IndexFunc(merged, func(g UnitPower) bool { return sameUnit(f, g) })
		if at < 0 {
			merged = append(merged, f)
			continue
		}
		merged[at].Exponent += f.Exponent
		merged[at].Name = shorterSpelling(merged[at].Name, f.Name)
		if merged[at].Reduces == nil {
			merged[at].Reduces = f.Reduces
		}
	}
	kept := merged[:0]
	for _, f := range merged {
		if f.Exponent != 0 {
			kept = append(kept, f)
		}
	}
	slices.SortStableFunc(kept, func(a, b UnitPower) int { return strings.Compare(a.Name, b.Name) })
	return UnitProduct{Powers: kept}
}

// shorterSpelling picks, of two spellings of one unit, the one with fewer
// qualifiers, then fewer characters, then the lesser text: `m` over `SI::m`.
func shorterSpelling(a, b string) string {
	if na, nb := strings.Count(a, "::"), strings.Count(b, "::"); na != nb {
		if na < nb {
			return a
		}
		return b
	}
	if len(a) != len(b) {
		if len(a) < len(b) {
			return a
		}
		return b
	}
	return min(a, b)
}

// sameUnit: two resolved powers are one unit by symbol; two unresolved ones by
// text, unless both are known to reduce differently; a resolved and an
// unresolved power are never the same unit.
func sameUnit(f, g UnitPower) bool {
	if f.Unit != nil || g.Unit != nil {
		return f.Unit == g.Unit
	}
	if f.Name != g.Name {
		return false
	}
	return f.Reduces == nil || g.Reduces == nil || f.Reduces.Same(*g.Reduces)
}

// Equal reports whether two products are the same units to the same exponents.
func (p UnitProduct) Equal(q UnitProduct) bool {
	if len(p.Powers) != len(q.Powers) {
		return false
	}
	for _, f := range p.Powers {
		if !slices.ContainsFunc(q.Powers, func(g UnitPower) bool { return sameUnit(f, g) && f.Exponent == g.Exponent }) {
			return false
		}
	}
	return true
}

// UnitLookup resolves a unit name written in a unit expression to its
// declaration, reporting false for a name it does not know.
type UnitLookup func(qn *ast.QualifiedName) (*symbols.Symbol, bool)

// UnitProductOfExpr reads a unit expression as the product of the named units it
// multiplies, divides and raises; it accepts and rejects exactly what UnitTermOfExpr does.
func (m *Model) UnitProductOfExpr(scope *symbols.Scope, node ast.Node) (UnitProduct, error) {
	return m.UnitProductOfExprBy(node, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if m.resolver == nil {
			return nil, false
		}
		sym, ok := m.resolver.ResolveQualified(scope, qn)
		if !ok || sym == nil {
			return nil, false
		}
		if alias, ok := m.resolver.ResolveAliasTarget(sym); ok {
			return alias, true
		}
		return sym, true
	})
}

// UnitOfExpr reads a unit expression as a quantity carries it: its text, the named units
// it is a product of, and its reduction to base units.
func (m *Model) UnitOfExpr(scope *symbols.Scope, node ast.Node) (Unit, error) {
	term, err := m.UnitTermOfExpr(scope, node)
	if err != nil {
		return Unit{}, err
	}
	product, err := m.UnitProductOfExpr(scope, node)
	if err != nil {
		return Unit{}, err
	}
	return Unit{Text: UnitExprText(node), Product: product, Term: term}, nil
}

// UnitProductOfExprBy reads a unit expression, identifying each named unit by
// what lookup resolves it to; a name lookup does not know is kept by its text.
func (m *Model) UnitProductOfExprBy(node ast.Node, lookup UnitLookup) (UnitProduct, error) {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return m.unitProductOfName(n.Name, lookup)
	case *ast.QualifiedName:
		return m.unitProductOfName(n, lookup)
	case *ast.OperatorExpr:
		return m.unitProductOfOperator(n, lookup)
	case *ast.LiteralInteger:
		if val, ok := m.Eval(n); ok && val.Kind == ValInt && val.Int == 1 {
			return UnitProduct{}, nil
		}
	}
	return UnitProduct{}, fmt.Errorf("%w: %T", ErrUnitExpr, node)
}

// unitProductOfName is the named unit qn writes, identified by the declaration
// it resolves to where it resolves.
func (m *Model) unitProductOfName(qn *ast.QualifiedName, lookup UnitLookup) (UnitProduct, error) {
	if qn == nil {
		return UnitProduct{}, ErrUnitExpr
	}
	var unit *symbols.Symbol
	dimensionOne := false
	if sym, ok := lookup(qn); ok && sym != nil {
		unit = sym
		if term, err := m.UnitTermOf(unit); err == nil {
			dimensionOne = term.Dimensionless()
		}
	}
	return NamedUnitProduct(unit, UnitNameText(qn), dimensionOne), nil
}

// unitProductOfOperator reads a product, quotient or power of units.
func (m *Model) unitProductOfOperator(n *ast.OperatorExpr, lookup UnitLookup) (UnitProduct, error) {
	if len(n.Operands) != 2 {
		return UnitProduct{}, fmt.Errorf("%w: %v takes 2 operands, got %d", ErrUnitExpr, n.Operator, len(n.Operands))
	}
	left, err := m.UnitProductOfExprBy(n.Operands[0], lookup)
	if err != nil {
		return UnitProduct{}, err
	}
	switch n.Operator {
	case ast.OpMul, ast.OpDiv:
		right, err := m.UnitProductOfExprBy(n.Operands[1], lookup)
		if err != nil {
			return UnitProduct{}, err
		}
		if n.Operator == ast.OpMul {
			return left.Times(right), nil
		}
		return left.DividedBy(right), nil
	case ast.OpPow:
		exp, ok := m.Eval(n.Operands[1])
		if !ok || !exp.IsNumeric() {
			return UnitProduct{}, fmt.Errorf("%w: unit exponent is not a constant number", ErrUnitExpr)
		}
		return left.Pow(exp.AsReal()), nil
	default:
		return UnitProduct{}, fmt.Errorf("%w: operator %v", ErrUnitExpr, n.Operator)
	}
}
