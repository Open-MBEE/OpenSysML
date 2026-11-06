package symbolfacts

import (
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// fqnScalarQuantityValue is the library type every quantity value specializes.
const fqnScalarQuantityValue = "Quantities::ScalarQuantityValue"

// Context holds what converting a symbol to proto needs besides the
// symbol: its index, and the resolver and semantic model that answer what its
// declared names refer to. Their memoization is why it is shared per model.
//
// The resolver and semantic model memoize into plain maps, so a shared context
// must be used by one goroutine at a time; Lock serializes concurrent
// conversions of the same model.
type Context struct {
	mu sync.Mutex

	Index     *symbols.Index
	Resolver  *resolve.Resolver
	Semantics *semantics.Model
}

// NewContext builds a conversion context over a symbol index.
func NewContext(idx *symbols.Index) *Context {
	resolver := resolve.New(idx)
	sem := passes.NewTypedModel(resolver)
	resolver.SetModel(sem)
	return &Context{Index: idx, Resolver: resolver, Semantics: sem}
}

// Lock takes exclusive use of the context's resolver and semantic model, and
// returns the function releasing it.
func (sc *Context) Lock() func() {
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
	ConnectorEnds             []*ConnectorEnd
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

// Specialization describes a declared generalization relationship. ChainIds
// are the FQNs of the chain's features when the target is written as a feature
// chain, TargetId then being the last of them.
type Specialization struct {
	Kind       string
	Declared   string
	TargetId   string
	TargetKind string
	ChainIds   []string
}

// ConnectorEnd is one end of a connector and the feature it connects, as the
// end's ReferenceSubsetting references it (KerML 8.3.4.5.3): the feature named,
// or for an end written as a feature chain, the chain whose features ChainIds
// name in order, FeatureId being the last of them.
type ConnectorEnd struct {
	EndId       string
	Name        string
	Declared    string
	FeatureId   string
	FeatureKind string
	ChainIds    []string
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
		Id:       idx.GetFQN(sym), // Fully qualified name
		Name:     sym.Name,
		Kind:     sym.Kind.String(),
		Metadata: make(map[string]string),
	}
	// Extract metadata from AST node
	extractMetadata(sym, info.Metadata)
	// Add visibility to metadata
	info.Metadata["visibility"] = visibilityToString(sym.Visibility)
	if sym.Scope != nil {
		// Collect child IDs
		info.ChildIds = collectChildIDs(sym.Scope, idx)
	}
	// Static type facts: the resolved type, the declared multiplicity and every
	// generalization edge. These are what a client needs to reconstruct the
	// element's type without re-deriving it from the metadata strings.
	info.TypeInfo = sc.typeInfoOf(sym)
	info.Multiplicity = sc.multiplicityOf(sym)
	info.Specializations = sc.specializationsOf(sym)
	// The attributes the element has, own and inherited, with their resolved
	// types and constant default values.
	info.Attributes, info.WithheldLibraryAttributes = sc.attributesOf(sym)
	// The features a connector's ends connect, chains included.
	info.ConnectorEnds = sc.connectorEndsOf(sym)
	return info
}

// Root is one document's root namespace: the index's own root symbol for
// it, else the document's root scope described as one, which is what a document
// declaring no root symbol has.
func Root(sc *Context, rootScope *symbols.Scope) *Info {
	for _, sym := range sc.Index.LookupQualified("") { // Root has empty name
		if sym.Scope == rootScope {
			return Of(sym, sc)
		}
	}
	if rootScope == nil {
		return nil
	}
	info := &Info{
		Kind:     "RootNamespace",
		Metadata: make(map[string]string),
	}
	info.ChildIds = collectChildIDs(rootScope, sc.Index)
	return info
}

// collectChildIDs extracts child symbol FQNs from a scope
func collectChildIDs(scope *symbols.Scope, idx *symbols.Index) []string {
	var ids []string
	for _, sym := range scope.AllMembers() {
		ids = append(ids, idx.GetFQN(sym))
	}
	return ids
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
	// A connector end references the feature it connects (SysML v2 8.2.2.13.1),
	// whether written `::> x` or implied by `connect x to y`.
	if end := sc.endReferenceOf(sym); end != nil {
		out = append(out, &Specialization{
			Kind: "references", Declared: end.Declared, TargetId: end.FeatureId, TargetKind: end.FeatureKind, ChainIds: end.ChainIds,
		})
	}
	return out
}

// connectorEndsOf reports a connector's effective ends in order — its own, then
// the ones it inherits — and the features they connect; an end with no symbol
// of its own (a recorded end without a name) and a flow, whose ends are its
// type's, report what the connector's `connect`/`from`/`to` paths name at that
// position instead. Nil for anything else.
func (sc *Context) connectorEndsOf(sym *symbols.Symbol) []*ConnectorEnd {
	m := sc.Semantics
	if m == nil {
		return nil
	}
	ends := m.EndFeatures(sym)
	paths := m.ConnectorEndPaths(sym)
	featurePaths := m.ConnectorEndFeaturePaths(sym)
	var out []*ConnectorEnd
	for i := 0; i < len(ends) || i < len(paths) || i < len(featurePaths); i++ {
		info := &ConnectorEnd{}
		var end *symbols.Symbol
		if i < len(ends) {
			end = ends[i]
		}
		owned := end != nil && end.OwnerScope != nil && end.OwnerScope.OwningElement() == sym
		if end != nil {
			info.EndId = sc.Index.GetFQN(end)
			info.Name = end.Name
			if decl, ok := end.Decl.(*ast.ConnectorEnd); ok {
				info.Declared = writtenTarget(decl.ReferencedTarget())
				if info.Declared == "" {
					info.Declared = writtenTarget(semantics.ImpliedEndReference(decl))
				}
			}
		}
		// An end of the connector's own says what it connects. An inherited
		// end connects what the connector's own `connect`, `from` or `to`
		// clause names at that position — nothing, when that name does not
		// resolve — and what the end itself references only when the
		// connector has no clause for the position.
		if owned {
			sc.connectedFeature(info, m.EndReferencedFeature(end))
		}
		if info.Name == "" && i < len(paths) {
			info.Name = paths[i].Name
		}
		if info.FeatureId == "" {
			switch {
			case i < len(featurePaths):
				sc.connectedPath(info, featurePaths[i])
			case i < len(paths):
				sc.connectedPath(info, paths[i].Features)
			case end != nil && !owned:
				sc.connectedFeature(info, m.EndReferencedFeature(end))
			}
		}
		out = append(out, info)
	}
	return out
}

// endReferenceOf is the end's own entry of its connector's ends, or nil when
// sym is no connector end.
func (sc *Context) endReferenceOf(sym *symbols.Symbol) *ConnectorEnd {
	if _, ok := sym.Decl.(*ast.ConnectorEnd); !ok || sym.OwnerScope == nil {
		return nil
	}
	connector := sym.OwnerScope.OwningElement()
	if connector == nil {
		return nil
	}
	id := sc.Index.GetFQN(sym)
	for _, end := range sc.connectorEndsOf(connector) {
		if end.EndId == id {
			return end
		}
	}
	return nil
}

// connectedFeature fills in the feature an end references: the features of a
// chain Feature in order, or the one feature named.
func (sc *Context) connectedFeature(info *ConnectorEnd, feature *symbols.Symbol) {
	if feature == nil {
		return
	}
	if feature.Chain != nil {
		sc.connectedPath(info, feature.Chain.Features)
		return
	}
	info.FeatureId = sc.Index.GetFQN(feature)
	info.FeatureKind = feature.Kind.String()
}

// connectedPath fills in a path of features: the last is the feature reached,
// and a path of several is a chain.
func (sc *Context) connectedPath(info *ConnectorEnd, path []*symbols.Symbol) {
	if len(path) == 0 {
		return
	}
	last := path[len(path)-1]
	info.FeatureId = sc.Index.GetFQN(last)
	info.FeatureKind = last.Kind.String()
	if len(path) > 1 {
		info.ChainIds = make([]string, len(path))
		for i, feature := range path {
			info.ChainIds[i] = sc.Index.GetFQN(feature)
		}
	}
	if info.Declared == "" {
		names := make([]string, len(path))
		for i, feature := range path {
			names[i] = feature.Name
		}
		info.Declared = strings.Join(names, ".")
	}
}

// writtenTarget is a connector end's target as written: a qualified name, its
// chained segments after `.`, or the empty string for anything else.
func writtenTarget(target ast.Node) string {
	if chain, ok := target.(*ast.FeatureChainExpr); ok && chain.Member != nil {
		operand, member := writtenTarget(chain.Operand), writtenTarget(chain.Member)
		if operand == "" || member == "" {
			return ""
		}
		return operand + "." + member
	}
	qn := ast.AsQualifiedName(target)
	if qn == nil {
		return ""
	}
	var b strings.Builder
	for i, part := range qn.Parts {
		if i > 0 {
			if part.Chained {
				b.WriteByte('.')
			} else {
				b.WriteString("::")
			}
		}
		b.WriteString(part.Text)
	}
	return b.String()
}

// resolveFrom resolves a qualified name written in sym's declaration,
// following an alias to what it names.
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

// multiplicityOf reports the multiplicity a symbol declares, or nil when it
// declares none. An unevaluable bound is reported empty rather than guessed at.
func (sc *Context) multiplicityOf(sym *symbols.Symbol) *Multiplicity {
	rng, ok := sc.Semantics.MultiplicityOf(sym)
	if !ok {
		return nil
	}
	return &Multiplicity{Lower: BoundText(rng.Lower), Upper: BoundText(rng.Upper)}
}

// typeInfoOf derives the static type facts of a def or usage. Anything the
// model cannot derive is left empty rather than guessed at.
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

// usageTypeInfo derives the type facts of a usage. An untyped usage still has a
// primitive when its default value determines one, reported as inferred.
func (sc *Context) usageTypeInfo(sym *symbols.Symbol, decl *ast.Usage) *TypeInfo {
	info := &TypeInfo{}

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
func (sc *Context) definitionTypeInfo(sym *symbols.Symbol) *TypeInfo {
	info := &TypeInfo{}
	if prim := sc.Semantics.PrimTypeOf(sym); prim != semantics.PrimUnknown {
		info.Primitive = prim.String()
		info.PrimitiveSource = "declared"
	}
	sc.markQuantity(sym, info, nil)
	return info
}

// markQuantity records that an element's values carry a measurement unit: its
// type is a quantity value type, or its default value names a unit.
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
func (sc *Context) libSymbol(fqn string) *symbols.Symbol {
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
	case semantics.ValRational:
		return semantics.PrimRational.String(), true
	case semantics.ValReal:
		return semantics.PrimReal.String(), true
	case semantics.ValBool:
		return semantics.PrimBoolean.String(), true
	default:
		return "", false
	}
}
