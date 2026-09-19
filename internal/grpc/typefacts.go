package grpc

import (
	"sync"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// fqnScalarQuantityValue is the library type every quantity value specializes.
const fqnScalarQuantityValue = "Quantities::ScalarQuantityValue"

// SymbolContext holds what converting a symbol to proto needs besides the
// symbol: its index, and the resolver and semantic model that answer what its
// declared names refer to. Their memoization is why it is shared per model.
//
// The resolver and semantic model memoize into plain maps, so a shared context
// must be used by one goroutine at a time; Lock serializes concurrent
// conversions of the same model.
type SymbolContext struct {
	mu sync.Mutex

	Index     *symbols.Index
	Resolver  *resolve.Resolver
	Semantics *semantics.Model
}

// Lock takes exclusive use of the context's resolver and semantic model, and
// returns the function releasing it.
func (sc *SymbolContext) Lock() func() {
	sc.mu.Lock()
	// Resolution during conversion is speculative: a name that does not resolve
	// is reported as unresolved type facts, not as a model diagnostic. Dropping
	// what it appended keeps the shared resolver's diagnostics from growing with
	// every request.
	diags := len(sc.Resolver.Diagnostics)
	return func() {
		if len(sc.Resolver.Diagnostics) > diags {
			sc.Resolver.Diagnostics = sc.Resolver.Diagnostics[:diags]
		}
		sc.mu.Unlock()
	}
}

// NewSymbolContext builds a conversion context over a symbol index.
func NewSymbolContext(idx *symbols.Index) *SymbolContext {
	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	resolver.SetModel(sem)
	return &SymbolContext{Index: idx, Resolver: resolver, Semantics: sem}
}

// relationshipKindName maps a generalization relationship to the name the wire
// reports it under. A relationship that is not a generalization has none.
func relationshipKindName(k ast.RelationshipKind) string {
	switch k {
	case ast.RelSpecializes:
		return "specializes"
	case ast.RelSubsets:
		return "subsets"
	case ast.RelRedefines:
		return "redefines"
	case ast.RelTyping:
		return "typing"
	default:
		return ""
	}
}

// relationshipName returns the qualified name a relationship targets,
// unwrapping the feature reference a usage's typing may be written as.
func relationshipName(rel *ast.Relationship) *ast.QualifiedName {
	if rel == nil || rel.Target == nil {
		return nil
	}
	target := rel.Target
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, ok := target.(*ast.QualifiedName)
	if !ok {
		return nil
	}
	return qn
}

// specializationsOf reports every generalization edge a symbol declares, in
// declaration order, with each target resolved.
func (sc *SymbolContext) specializationsOf(sym *symbols.Symbol) []*pb.Specialization {
	var out []*pb.Specialization
	for _, rel := range semantics.RelationshipsOf(sym) {
		kind := relationshipKindName(rel.Kind)
		if kind == "" {
			continue
		}
		qn := relationshipName(rel)
		if qn == nil {
			continue
		}
		spec := &pb.Specialization{
			Kind:     kind,
			Declared: qn.Text(),
		}
		if target := sc.resolveFrom(sym, qn); target != nil {
			spec.TargetId = sc.Index.GetFQN(target)
			spec.TargetKind = target.Kind.String()
		}
		out = append(out, spec)
	}
	return out
}

// resolveFrom resolves a qualified name written in sym's declaration,
// following an alias to what it names.
func (sc *SymbolContext) resolveFrom(sym *symbols.Symbol, qn *ast.QualifiedName) *symbols.Symbol {
	target, ok := sc.Resolver.ResolveQualified(sym.OwnerScope, qn)
	if !ok || target == nil {
		return nil
	}
	if alias, ok := sc.Resolver.ResolveAliasTarget(target); ok {
		target = alias
	}
	return target
}

// multiplicityOf reports the multiplicity a symbol declares, or nil when it
// declares none. An unevaluable bound is reported empty rather than guessed at.
func (sc *SymbolContext) multiplicityOf(sym *symbols.Symbol) *pb.MultiplicityInfo {
	rng, ok := sc.Semantics.MultiplicityOf(sym)
	if !ok {
		return nil
	}
	return &pb.MultiplicityInfo{
		Lower: protoconv.BoundText(rng.Lower),
		Upper: protoconv.BoundText(rng.Upper),
	}
}

// typeInfoOf derives the static type facts of a def or usage. Anything the
// model cannot derive is left empty rather than guessed at.
func (sc *SymbolContext) typeInfoOf(sym *symbols.Symbol) *pb.TypeInfo {
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		return sc.usageTypeInfo(sym, decl)
	case *ast.Definition:
		return sc.definitionTypeInfo(sym)
	default:
		return nil
	}
}

// usageTypeInfo derives the type facts of a usage. An untyped usage still has a
// primitive when its default value determines one, reported as inferred.
func (sc *SymbolContext) usageTypeInfo(sym *symbols.Symbol, decl *ast.Usage) *pb.TypeInfo {
	info := &pb.TypeInfo{}

	// Declared type: the first typing relationship, which is the usage's own
	// type. Subsetting and redefinition are reported as specializations.
	for _, rel := range decl.Relationships {
		if rel.Kind != ast.RelTyping {
			continue
		}
		qn := relationshipName(rel)
		if qn == nil {
			continue
		}
		info.Declared = qn.Text()
		if target := sc.resolveFrom(sym, qn); target != nil {
			info.ResolvedId = sc.Index.GetFQN(target)
			info.ResolvedKind = target.Kind.String()
		}
		break
	}

	// PrimTypeOf walks the generalization graph, so it covers a type inherited
	// through subsetting or redefinition as well as a declared one.
	if prim := sc.Semantics.PrimTypeOf(sym); prim != semantics.PrimUnknown {
		info.Primitive = prim.String()
		info.PrimitiveSource = "declared"
	} else if prim, ok := primOfValue(sc.Semantics, decl.Value); ok {
		info.Primitive = prim
		info.PrimitiveSource = "value"
	}

	sc.markQuantity(sym, info, decl.Value)
	return info
}

// definitionTypeInfo derives the type facts of a definition: the library scalar
// it classifies as, and whether it is a quantity value type.
func (sc *SymbolContext) definitionTypeInfo(sym *symbols.Symbol) *pb.TypeInfo {
	info := &pb.TypeInfo{}
	if prim := sc.Semantics.PrimTypeOf(sym); prim != semantics.PrimUnknown {
		info.Primitive = prim.String()
		info.PrimitiveSource = "declared"
	}
	sc.markQuantity(sym, info, nil)
	return info
}

// markQuantity records that an element's values carry a measurement unit: its
// type is a quantity value type, or its default value names a unit.
func (sc *SymbolContext) markQuantity(sym *symbols.Symbol, info *pb.TypeInfo, value ast.Node) {
	if idx, ok := value.(*ast.IndexExpr); ok && idx.Bracket {
		if _, err := sc.Semantics.UnitTermOfExpr(sym.OwnerScope, idx.Index); err == nil {
			info.Quantity = true
			info.Unit = semantics.UnitExprText(idx.Index)
		}
	}
	if info.Quantity {
		return
	}
	quantityValue := sc.libSymbol(fqnScalarQuantityValue)
	if quantityValue == nil {
		return
	}
	// Typing is a generalization edge, so a usage conforms to its type's
	// supertypes: the same check covers a def and a usage.
	if sc.Semantics.Conforms(sym, quantityValue) {
		info.Quantity = true
	}
}

// libSymbol looks up a library element by qualified name, uniquely or not at
// all: an ambiguous name is no evidence of the library's element.
func (sc *SymbolContext) libSymbol(fqn string) *symbols.Symbol {
	matches := sc.Index.LookupQualified(fqn)
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

// primOfValue classifies a default value expression. Only a constant-evaluable
// value or a string literal determines a type; anything else stays underivable.
func primOfValue(sem *semantics.Model, value ast.Node) (string, bool) {
	if value == nil {
		return "", false
	}
	if _, isStr := value.(*ast.LiteralString); isStr {
		return semantics.PrimString.String(), true
	}
	val, ok := sem.Eval(value)
	if !ok {
		return "", false
	}
	switch val.Kind {
	case semantics.ValInt:
		return semantics.PrimInteger.String(), true
	case semantics.ValReal:
		return semantics.PrimReal.String(), true
	case semantics.ValBool:
		return semantics.PrimBoolean.String(), true
	default:
		return "", false
	}
}
