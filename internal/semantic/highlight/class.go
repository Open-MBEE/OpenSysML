// Package highlight classifies the source text of a document into semantic
// tokens: the lexer's keywords, comments and literals, the names the symbol
// table declares, and the references the resolver answers. Its vocabulary is
// semtok's — the one the Language Server Protocol standardizes — so an editor
// legend is the list semtok.Classes and semtok.Modifiers return, in order.
package highlight

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/semtok"
)

// classify reports how a declared symbol is highlighted: the class of its name,
// whether it declares a definition, and the modifiers its declaration carries.
func classify(sym *symbols.Symbol) (semtok.Class, semtok.Modifier) {
	class, isDef := classOfKind(sym.Kind)
	mods := semtok.ModDeclaration
	if isDef {
		mods |= semtok.ModDefinition
	}
	if class == semtok.ClassEnumMember {
		mods |= semtok.ModReadonly
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		// A directed or result feature is a parameter of the behavior owning it.
		if decl.Direction != ast.DirNone || decl.IsResult {
			class = semtok.ClassParameter
		}
		if decl.IsConstant || decl.IsDerived {
			mods |= semtok.ModReadonly
		}
		if decl.IsAbstract {
			mods |= semtok.ModAbstract
		}
	case *ast.Definition:
		if decl.IsConstant {
			mods |= semtok.ModReadonly
		}
		if decl.IsAbstract {
			mods |= semtok.ModAbstract
		}
	}
	return class, mods
}

// referenceClass reports how a name referring to sym is highlighted: the class
// of what it denotes, without the modifiers that belong to a declaration.
func referenceClass(sym *symbols.Symbol) (semtok.Class, semtok.Modifier) {
	class, mods := classify(sym)
	return class, mods & semtok.ModReadonly
}

// classOfKind maps a symbol kind to its token class and reports whether the kind
// is a definition rather than a usage or a namespace.
func classOfKind(k symbols.SymbolKind) (semtok.Class, bool) {
	switch k {
	case symbols.SymbolPackage, symbols.SymbolNamespace, symbols.SymbolDependency,
		symbols.SymbolRelationship:
		return semtok.ClassNamespace, false
	case symbols.SymbolAlias:
		return semtok.ClassNamespace, false
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return semtok.ClassComment, false

	// Definitions: a value-like definition is a struct, a behavior-like one a
	// function, and everything structural a class.
	case symbols.SymbolAttributeDef:
		return semtok.ClassStruct, true
	case symbols.SymbolEnumerationDef:
		return semtok.ClassEnum, true
	case symbols.SymbolPortDef, symbols.SymbolInterfaceDef:
		return semtok.ClassInterface, true
	case symbols.SymbolActionDef, symbols.SymbolStateDef, symbols.SymbolCalcDef,
		symbols.SymbolConstraintDef, symbols.SymbolRequirementDef, symbols.SymbolCaseDef,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolVerificationCaseDef, symbols.SymbolUseCaseDef,
		symbols.SymbolViewpointDef, symbols.SymbolConcernDef:
		return semtok.ClassFunction, true
	case symbols.SymbolPartDef, symbols.SymbolItemDef, symbols.SymbolOccurrenceDef,
		symbols.SymbolIndividualDef, symbols.SymbolMetadataDef, symbols.SymbolMetaclass,
		symbols.SymbolConnectionDef, symbols.SymbolFlowDef, symbols.SymbolAllocationDef,
		symbols.SymbolViewDef, symbols.SymbolRenderingDef, symbols.SymbolKerMLType:
		return semtok.ClassClass, true

	// Usages: an attribute is a property, an enumeration literal an enum member,
	// a behavioral usage a method, and everything else a variable.
	case symbols.SymbolAttributeUsage, symbols.SymbolReferenceUsage:
		return semtok.ClassProperty, false
	case symbols.SymbolEnumerationUsage:
		return semtok.ClassEnumMember, false
	case symbols.SymbolActionUsage, symbols.SymbolStateUsage, symbols.SymbolCalcUsage,
		symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage, symbols.SymbolCaseUsage,
		symbols.SymbolAnalysisCaseUsage, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolUseCaseUsage, symbols.SymbolViewpointUsage, symbols.SymbolConcernUsage:
		return semtok.ClassMethod, false
	default:
		return semtok.ClassVariable, false
	}
}
