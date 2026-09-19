package grpc

import (
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// attributesOf reports the attributes an element declares and inherits, a
// closer declaration masking an inherited one of the same name, each with its
// resolved type, constant default value and unit. An attribute declared in
// standard-library content is withheld and counted: a client asks about its
// model, not about the metamodel frame every element sits in.
func (sc *SymbolContext) attributesOf(sym *symbols.Symbol) ([]*pb.AttributeInfo, int32) {
	out := []*pb.AttributeInfo{}
	var withheld int32
	seen := make(map[string]bool)
	for _, member := range sc.Semantics.MembersOf(sym) {
		if !isAttributeSymbol(member) || member.Name == "" || seen[member.Name] {
			continue
		}
		seen[member.Name] = true
		if sc.libraryDeclared(member) && member.OwnerScope != sym.Scope {
			withheld++
			continue
		}
		out = append(out, sc.attributeInfoOf(member))
	}
	return out, withheld
}

// libraryDeclared reports whether the index marks sym as standard-library
// content, which is provenance rather than a name or FQN prefix test.
func (sc *SymbolContext) libraryDeclared(sym *symbols.Symbol) bool {
	return sc.Index != nil && sc.Index.Library(sym)
}

// isAttributeSymbol reports whether a member is an attribute rather than some
// other kind of feature.
func isAttributeSymbol(sym *symbols.Symbol) bool {
	return sym != nil &&
		(sym.Kind == symbols.SymbolAttributeDef || sym.Kind == symbols.SymbolAttributeUsage)
}

// attributeInfoOf derives one attribute's facts.
func (sc *SymbolContext) attributeInfoOf(sym *symbols.Symbol) *pb.AttributeInfo {
	info := &pb.AttributeInfo{Name: sym.Name}

	if t := sc.typeInfoOf(sym); t != nil {
		info.Type = attributeTypeText(t)
		info.Unit = t.GetUnit()
	}
	if info.Type == "" {
		info.Type = sc.inheritedTypeText(sym, 0)
	}

	value, unit := sc.attributeValue(sym)
	// Only a declaration writing no value at all takes one from what it
	// redefines; one writing a non-constant value has none, not the base's.
	if !writesValue(sym) {
		value, unit = sc.inheritedValue(sym, 0)
	}
	info.Value = value
	if unit != "" {
		info.Unit = unit
	}
	return info
}

// maxGeneralizationDepth bounds how far a redefinition chain is followed for a
// fact the redefining declaration leaves out.
const maxGeneralizationDepth = 8

// generalizationTargets resolves what an element specializes, subsets or
// redefines, in declaration order.
func (sc *SymbolContext) generalizationTargets(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		switch rel.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
		default:
			continue
		}
		qn := relationshipName(rel)
		if qn == nil {
			continue
		}
		if target := sc.resolveFrom(sym, qn); target != nil {
			out = append(out, target)
		}
	}
	return out
}

// inheritedTypeText names the type a redefining or subsetting attribute takes
// from what it redefines, for a declaration that writes no type of its own.
func (sc *SymbolContext) inheritedTypeText(sym *symbols.Symbol, depth int) string {
	if depth >= maxGeneralizationDepth {
		return ""
	}
	for _, target := range sc.generalizationTargets(sym) {
		if t := sc.typeInfoOf(target); t != nil {
			if text := attributeTypeText(t); text != "" {
				return text
			}
		}
		if text := sc.inheritedTypeText(target, depth+1); text != "" {
			return text
		}
	}
	return ""
}

// inheritedValue reports the default a subsetting attribute takes from what it
// subsets, for a declaration that writes no value of its own.
func (sc *SymbolContext) inheritedValue(sym *symbols.Symbol, depth int) (*pb.Value, string) {
	if depth >= maxGeneralizationDepth {
		return nil, ""
	}
	for _, target := range sc.generalizationTargets(sym) {
		if value, unit := sc.attributeValue(target); value != nil {
			return value, unit
		}
		// A target stating a non-constant default has none to pass on, and does
		// not fall back to what it itself redefines.
		if writesValue(target) {
			continue
		}
		if value, unit := sc.inheritedValue(target, depth+1); value != nil {
			return value, unit
		}
	}
	return nil, ""
}

// attributeTypeText names an attribute's type: what the declaration resolved
// to, else what it wrote, else the library scalar the element reduces to.
func attributeTypeText(t *pb.TypeInfo) string {
	switch {
	case t.GetResolvedId() != "":
		return t.GetResolvedId()
	case t.GetDeclared() != "":
		return t.GetDeclared()
	default:
		return t.GetPrimitive()
	}
}

// attributeValue evaluates an attribute's default value, reporting the unit it
// was written in when it is a quantity. A value that is not a model-level
// constant — a feature reference, a call — has none rather than a guess.
func (sc *SymbolContext) attributeValue(sym *symbols.Symbol) (*pb.Value, string) {
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok || decl.Value == nil {
		return nil, ""
	}

	node, unit := decl.Value, ""
	if idx, isQuantity := node.(*ast.IndexExpr); isQuantity && idx.Bracket {
		if _, err := sc.Semantics.UnitTermOfExpr(sym.OwnerScope, idx.Index); err == nil {
			node, unit = idx.Operand, semantics.UnitExprText(idx.Index)
		}
	}

	if str, isStr := node.(*ast.LiteralString); isStr {
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: unquote(str.Value)}}, unit
	}
	val, ok := sc.Semantics.Eval(node)
	if !ok {
		return nil, unit
	}
	return protoconv.ValueToProto(runtime.Value{Kind: runtime.ValConst, Const: val}, sc.Index), unit
}

// writesValue reports whether an attribute's declaration states a default of
// its own, constant or not.
func writesValue(sym *symbols.Symbol) bool {
	decl, ok := sym.Decl.(*ast.Usage)
	return ok && decl.Value != nil
}

// unquote reads the text a string literal spells, so a reported attribute value
// is the same string evaluating the literal answers.
func unquote(s string) string {
	return source.StringValue(s)
}
