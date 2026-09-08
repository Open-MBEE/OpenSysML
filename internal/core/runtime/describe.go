package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// notOfKind reports that sym declares something other than the kind asked of
// it: a usage error naming what it is, never a Go type name.
func notOfKind(sentinel error, sym *symbols.Symbol, kind string) error {
	return fmt.Errorf("%w: %s is %s, not %s %s definition or usage",
		sentinel, sym.Name, describeDecl(sym.Decl), articleFor(kind), kind)
}

// articleFor is the indefinite article a word reads with, so a kind beginning
// with a vowel is "an analysis" rather than "a analysis".
func articleFor(word string) string {
	if word == "" {
		return "a"
	}
	switch word[0] {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return "an"
	}
	return "a"
}

// describeOperand names an operand's type for an operator diagnostic, with its
// article, so a message reads "an Integer and a string".
func describeOperand(val Value) string {
	switch val.Kind {
	case ValConst, ValComplex:
		return describeValue(val)
	case ValString:
		return "a string"
	case ValNull:
		return "null"
	case ValInstance:
		return "an instance"
	case ValSequence:
		return "a sequence"
	case ValSet:
		return "a set"
	case ValExpr:
		return "an expression"
	case ValQuantity:
		return "a quantity"
	case ValVariant:
		return "a variant"
	case ValEnumLiteral:
		return "the enumeration literal " + val.LiteralText()
	case ValArray:
		return "an array"
	case ValVector:
		return "a vector"
	case ValVectorQuantity:
		return "a vector quantity"
	case ValTensorQuantity:
		return "a tensor quantity"
	case ValMeasurementRef:
		return "a measurement reference"
	case ValCoordinateFrame:
		if val.CoordinateFrame().IsScale() {
			return "a measurement scale"
		}
		return "a coordinate frame"
	case ValCoordinateTransformation:
		return "a coordinate transformation"
	case ValFunction:
		return "the function " + val.FunctionName()
	}
	return "a value"
}

// describeDecl names a declaration the way the notation does, e.g. "a part def"
// or "a calc usage", so a diagnostic about a symbol of the wrong kind never
// prints a Go type name.
func describeDecl(decl ast.Node) string {
	written := ast.Notation(decl)
	switch decl.(type) {
	case *ast.Definition:
		return articleFor(written) + " " + written
	case *ast.Usage, *ast.AssumeMember, *ast.RequireMember:
		return articleFor(written) + " " + written + " usage"
	case nil:
		return "nothing"
	}
	return "a declaration"
}
