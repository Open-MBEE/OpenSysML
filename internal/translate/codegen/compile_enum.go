package codegen

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// enumLiteral is one literal of a compiled enumeration.
type enumLiteral struct {
	enum *Enum
	i    int
}

// enumOf is the compiled enumeration sym defines, or why it has none: only an
// enumeration whose literals are named, declare no value and are its own
// compiles, each literal then identified by itself.
func (c *Compiler) enumOf(sym *symbols.Symbol) (*Enum, string) {
	if e, ok := c.enums[sym]; ok {
		return e, ""
	}
	fqn := c.name(sym)
	if def, ok := sym.Decl.(*ast.Definition); ok {
		for _, r := range def.Relationships {
			if r != nil && r.Kind == ast.RelSpecializes {
				return nil, fmt.Sprintf("enumeration %s specializes another type, whose values its literals may be", fqn)
			}
		}
	}
	e := &Enum{Name: fqn, Short: sym.Name, Base: len(c.literalNames), ID: len(c.enumOrder)}
	lits := c.model.EnumeratedValuesOf(sym)
	for _, lit := range lits {
		switch {
		case lit.Name == "":
			return nil, fmt.Sprintf("enumeration %s has an unnamed literal", fqn)
		case semantics.EnumerationOwning(lit) != sym:
			return nil, fmt.Sprintf("enumeration %s inherits the literal %s", fqn, lit.Name)
		case semantics.LiteralValue(lit) != nil:
			return nil, fmt.Sprintf("enumeration %s: the literal %s declares a value", fqn, lit.Name)
		}
	}
	for _, lit := range lits {
		c.literals[lit] = enumLiteral{enum: e, i: len(e.Literals)}
		e.Literals = append(e.Literals, lit.Name)
	}
	for i := range e.Literals {
		c.literalNames = append(c.literalNames, e.Literal(i))
	}
	if c.enums == nil {
		c.enums = map[*symbols.Symbol]*Enum{}
	}
	c.enums[sym] = e
	c.enumOrder = append(c.enumOrder, e)
	return e, ""
}

// compileEnumLiteral is the literal sym names, ok false when sym is none.
func (fc *funcCompiler) compileEnumLiteral(sym *symbols.Symbol) (Expr, bool, error) {
	enum := semantics.EnumerationOwning(sym)
	if enum == nil {
		return nil, false, nil
	}
	if _, why := fc.c.enumOf(enum); why != "" {
		return nil, true, fc.unsupported(why)
	}
	lit, ok := fc.c.literals[sym]
	if !ok {
		return nil, true, fc.unsupported(fmt.Sprintf("the enumeration literal %s", sym.Name))
	}
	return EnumLit{T: EnumType(lit.enum), I: lit.i}, true, nil
}

// enumOperator is the binary operator op over operands l and r one of which is
// an enumeration literal, which no library function declares op for: a
// failure, worded as the interpreter's, once both are evaluated.
func (fc *funcCompiler) enumOperator(op ast.OperatorKind, l, r Expr) (Expr, bool, error) {
	lt, rt := l.Type(), r.Type()
	if !lt.IsEnum() && !rt.IsEnum() {
		return nil, false, nil
	}
	for _, t := range []Type{lt, rt} {
		if !t.Scalar() || t == TypeString {
			return nil, true, fc.unsupported(fmt.Sprintf("'%s' over %s and %s", op, lt, rt))
		}
	}
	parts := []string{fmt.Sprintf("type mismatch: operator '%s' is not defined for ", op), " and ", ""}
	t := lt
	switch op {
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		enum := lt
		if !enum.IsEnum() {
			enum = rt
		}
		parts[2] = fmt.Sprintf("; DataFunctions::'%s' is abstract and no library function declares '%s' for the enumeration %s, which is no ScalarValue", op, op, enum.Enum.Short)
		t = TypeBool
	}
	fc.c.collections = true
	return Refusal{Operands: []Expr{l, r}, Parts: parts, Describe: true, T: t}, true, nil
}
