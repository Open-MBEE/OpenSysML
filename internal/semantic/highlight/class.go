// Package highlight classifies the source text of a document into semantic
// tokens: the lexer's keywords, comments and literals, the names the symbol
// table declares, and the references the resolver answers. Its vocabulary is
// the one the Language Server Protocol standardizes, so an editor legend is the
// list Classes and Modifiers return, in order.
package highlight

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Class is the kind of a semantic token. Its String is the LSP token type name.
type Class int

// The classes this package emits, in legend order.
const (
	ClassNamespace Class = iota
	ClassClass
	ClassEnum
	ClassInterface
	ClassStruct
	ClassParameter
	ClassVariable
	ClassProperty
	ClassEnumMember
	ClassFunction
	ClassMethod
	ClassKeyword
	ClassComment
	ClassString
	ClassNumber
)

var classNames = []string{
	ClassNamespace:  "namespace",
	ClassClass:      "class",
	ClassEnum:       "enum",
	ClassInterface:  "interface",
	ClassStruct:     "struct",
	ClassParameter:  "parameter",
	ClassVariable:   "variable",
	ClassProperty:   "property",
	ClassEnumMember: "enumMember",
	ClassFunction:   "function",
	ClassMethod:     "method",
	ClassKeyword:    "keyword",
	ClassComment:    "comment",
	ClassString:     "string",
	ClassNumber:     "number",
}

// String returns the LSP token type name of the class.
func (c Class) String() string {
	if int(c) < 0 || int(c) >= len(classNames) {
		return "unknown"
	}
	return classNames[c]
}

// Classes returns every class in legend order, so a class's index in the result
// is the token type index an encoded token carries.
func Classes() []Class {
	out := make([]Class, len(classNames))
	for i := range classNames {
		out[i] = Class(i)
	}
	return out
}

// Modifier is a bitset of token modifiers. Bit i is the modifier at index i of
// Modifiers.
type Modifier uint32

// The modifiers this package emits, in legend order.
const (
	ModDeclaration Modifier = 1 << iota
	ModDefinition
	ModReadonly
	ModAbstract
)

var modifierNames = []struct {
	mod  Modifier
	name string
}{
	{ModDeclaration, "declaration"},
	{ModDefinition, "definition"},
	{ModReadonly, "readonly"},
	{ModAbstract, "abstract"},
}

// Modifiers returns every modifier in legend order.
func Modifiers() []Modifier {
	out := make([]Modifier, len(modifierNames))
	for i, m := range modifierNames {
		out[i] = m.mod
	}
	return out
}

// String returns the LSP modifier name of a single modifier bit.
func (m Modifier) String() string {
	for _, entry := range modifierNames {
		if entry.mod == m {
			return entry.name
		}
	}
	return "unknown"
}

// Token is one classified span of source text.
type Token struct {
	Span      source.Span
	Class     Class
	Modifiers Modifier
}

// classify reports how a declared symbol is highlighted: the class of its name,
// whether it declares a definition, and the modifiers its declaration carries.
func classify(sym *symbols.Symbol) (Class, Modifier) {
	class, isDef := classOfKind(sym.Kind)
	mods := ModDeclaration
	if isDef {
		mods |= ModDefinition
	}
	if class == ClassEnumMember {
		mods |= ModReadonly
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		// A directed or result feature is a parameter of the behavior owning it.
		if decl.Direction != ast.DirNone || decl.IsResult {
			class = ClassParameter
		}
		if decl.IsConstant || decl.IsDerived {
			mods |= ModReadonly
		}
		if decl.IsAbstract {
			mods |= ModAbstract
		}
	case *ast.Definition:
		if decl.IsConstant {
			mods |= ModReadonly
		}
		if decl.IsAbstract {
			mods |= ModAbstract
		}
	}
	return class, mods
}

// referenceClass reports how a name referring to sym is highlighted: the class
// of what it denotes, without the modifiers that belong to a declaration.
func referenceClass(sym *symbols.Symbol) (Class, Modifier) {
	class, mods := classify(sym)
	return class, mods & ModReadonly
}

// classOfKind maps a symbol kind to its token class and reports whether the kind
// is a definition rather than a usage or a namespace.
func classOfKind(k symbols.SymbolKind) (Class, bool) {
	switch k {
	case symbols.SymbolPackage, symbols.SymbolNamespace, symbols.SymbolDependency,
		symbols.SymbolRelationship:
		return ClassNamespace, false
	case symbols.SymbolAlias:
		return ClassNamespace, false
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return ClassComment, false

	// Definitions: a value-like definition is a struct, a behavior-like one a
	// function, and everything structural a class.
	case symbols.SymbolAttributeDef:
		return ClassStruct, true
	case symbols.SymbolEnumerationDef:
		return ClassEnum, true
	case symbols.SymbolPortDef, symbols.SymbolInterfaceDef:
		return ClassInterface, true
	case symbols.SymbolActionDef, symbols.SymbolStateDef, symbols.SymbolCalcDef,
		symbols.SymbolConstraintDef, symbols.SymbolRequirementDef, symbols.SymbolCaseDef,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolVerificationCaseDef, symbols.SymbolUseCaseDef,
		symbols.SymbolViewpointDef, symbols.SymbolConcernDef:
		return ClassFunction, true
	case symbols.SymbolPartDef, symbols.SymbolItemDef, symbols.SymbolOccurrenceDef,
		symbols.SymbolIndividualDef, symbols.SymbolMetadataDef, symbols.SymbolMetaclass,
		symbols.SymbolConnectionDef, symbols.SymbolFlowDef, symbols.SymbolAllocationDef,
		symbols.SymbolViewDef, symbols.SymbolRenderingDef, symbols.SymbolKerMLType:
		return ClassClass, true

	// Usages: an attribute is a property, an enumeration literal an enum member,
	// a behavioral usage a method, and everything else a variable.
	case symbols.SymbolAttributeUsage:
		return ClassProperty, false
	case symbols.SymbolEnumerationUsage:
		return ClassEnumMember, false
	case symbols.SymbolActionUsage, symbols.SymbolStateUsage, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage, symbols.SymbolCaseUsage,
		symbols.SymbolAnalysisCaseUsage, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolUseCaseUsage, symbols.SymbolViewpointUsage, symbols.SymbolConcernUsage:
		return ClassMethod, false
	default:
		return ClassVariable, false
	}
}
