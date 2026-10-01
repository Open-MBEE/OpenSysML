package symbolfacts

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

const fqnScalarQuantityValue = "Quantities::ScalarQuantityValue"

// Context holds the index, resolver and semantic model used to derive symbol facts.
type Context struct {
	mu sync.Mutex

	Index     *symbols.Index
	Resolver  *resolve.Resolver
	Semantics *semantics.Model
}

// NewContext builds a fact-derivation context over a symbol index.
func NewContext(idx *symbols.Index) *Context {
	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	resolver.SetModel(sem)
	return &Context{Index: idx, Resolver: resolver, Semantics: sem}
}

// Lock takes exclusive use of the context's resolver and semantic model.
func (sc *Context) Lock() func() {
	sc.mu.Lock()
	diags := len(sc.Resolver.Diagnostics)
	return func() {
		if len(sc.Resolver.Diagnostics) > diags {
			sc.Resolver.Diagnostics = sc.Resolver.Diagnostics[:diags]
		}
		sc.mu.Unlock()
	}
}

// Info contains protobuf-independent facts about a symbol.
type Info struct {
	Id                        string
	Name                      string
	Kind                      string
	Metadata                  map[string]string
	ChildIds                  []string
	TypeInfo                  *TypeInfo
	Multiplicity              *Multiplicity
	Specializations           []*Specialization
	Attributes                []*Attribute
	WithheldLibraryAttributes int32
}

// TypeInfo describes a symbol's declared, resolved and primitive type.
type TypeInfo struct {
	Declared        string
	ResolvedId      string
	ResolvedKind    string
	Primitive       string
	PrimitiveSource string
	Quantity        bool
	Unit            string
}

// Multiplicity contains the lower and upper bounds of a symbol's multiplicity.
type Multiplicity struct {
	Lower string
	Upper string
}

// Specialization describes a declared generalization relationship.
type Specialization struct {
	Kind       string
	Declared   string
	TargetId   string
	TargetKind string
}

// Attribute contains a symbol's attribute declaration facts.
type Attribute struct {
	Name  string
	Type  string
	Unit  string
	Value *Value
}

// Value contains either a string value or an evaluated semantic constant.
type Value struct {
	String *string
	Const  *semantics.Value
}

// Of derives the plain-Go facts associated with a symbol.
func Of(sym *symbols.Symbol, sc *Context) *Info {
	defer sc.Lock()()
	idx := sc.Index
	info := &Info{
		Id:       idx.GetFQN(sym),
		Name:     sym.Name,
		Kind:     sym.Kind.String(),
		Metadata: make(map[string]string),
	}
	extractMetadata(sym, info.Metadata)
	info.Metadata["visibility"] = visibilityToString(sym.Visibility)
	if sym.Scope != nil {
		for _, child := range sym.Scope.AllMembers() {
			info.ChildIds = append(info.ChildIds, idx.GetFQN(child))
		}
	}
	info.TypeInfo = sc.typeInfoOf(sym)
	info.Multiplicity = sc.multiplicityOf(sym)
	info.Specializations = sc.specializationsOf(sym)
	info.Attributes, info.WithheldLibraryAttributes = sc.attributesOf(sym)
	return info
}

// Root derives facts for a document's root namespace.
func Root(sc *Context, rootScope *symbols.Scope) *Info {
	if rootScope == nil {
		return nil
	}
	for _, sym := range sc.Index.LookupQualified("") {
		if sym.Scope == rootScope {
			return Of(sym, sc)
		}
	}
	info := &Info{
		Kind:     "RootNamespace",
		Metadata: make(map[string]string),
	}
	for _, sym := range rootScope.AllMembers() {
		info.ChildIds = append(info.ChildIds, sc.Index.GetFQN(sym))
	}
	return info
}

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

func (sc *Context) specializationsOf(sym *symbols.Symbol) []*Specialization {
	var out []*Specialization
	for _, rel := range semantics.RelationshipsOf(sym) {
		kind := relationshipKindName(rel.Kind)
		if kind == "" {
			continue
		}
		qn := relationshipName(rel)
		if qn == nil {
			continue
		}
		spec := &Specialization{Kind: kind, Declared: qn.Text()}
		if target := sc.resolveFrom(sym, qn); target != nil {
			spec.TargetId = sc.Index.GetFQN(target)
			spec.TargetKind = target.Kind.String()
		}
		out = append(out, spec)
	}
	return out
}

func (sc *Context) resolveFrom(sym *symbols.Symbol, qn *ast.QualifiedName) *symbols.Symbol {
	target, ok := sc.Resolver.ResolveQualified(sym.OwnerScope, qn)
	if !ok || target == nil {
		return nil
	}
	if alias, ok := sc.Resolver.ResolveAliasTarget(target); ok {
		target = alias
	}
	return target
}

func (sc *Context) multiplicityOf(sym *symbols.Symbol) *Multiplicity {
	rng, ok := sc.Semantics.MultiplicityOf(sym)
	if !ok {
		return nil
	}
	return &Multiplicity{Lower: BoundText(rng.Lower), Upper: BoundText(rng.Upper)}
}

func (sc *Context) typeInfoOf(sym *symbols.Symbol) *TypeInfo {
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		return sc.usageTypeInfo(sym, decl)
	case *ast.Definition:
		return sc.definitionTypeInfo(sym)
	default:
		return nil
	}
}

func (sc *Context) usageTypeInfo(sym *symbols.Symbol, decl *ast.Usage) *TypeInfo {
	info := &TypeInfo{}
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

func (sc *Context) definitionTypeInfo(sym *symbols.Symbol) *TypeInfo {
	info := &TypeInfo{}
	if prim := sc.Semantics.PrimTypeOf(sym); prim != semantics.PrimUnknown {
		info.Primitive = prim.String()
		info.PrimitiveSource = "declared"
	}
	sc.markQuantity(sym, info, nil)
	return info
}

func (sc *Context) markQuantity(sym *symbols.Symbol, info *TypeInfo, value ast.Node) {
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
	if quantityValue != nil && sc.Semantics.Conforms(sym, quantityValue) {
		info.Quantity = true
	}
}

func (sc *Context) libSymbol(fqn string) *symbols.Symbol {
	matches := sc.Index.LookupQualified(fqn)
	if len(matches) != 1 {
		return nil
	}
	return matches[0]
}

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
