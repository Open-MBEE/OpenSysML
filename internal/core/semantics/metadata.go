package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// semanticMetadataFQN is the KerML metaclass a metadata definition specializes
// to declare that annotating an element implicitly specializes its baseType
// ([KerML, 9.2.16], SysML v2 §7.27.3).
const semanticMetadataFQN = "Metaobjects::SemanticMetadata"

// baseTypeFeature is the SemanticMetadata feature holding the type annotated
// elements implicitly specialize.
const baseTypeFeature = "baseType"

// semanticMetadataBases returns the types sym implicitly specializes because of
// the semantic metadata annotating it. SysML v2 §7.27.4 makes a user-defined
// keyword (`#cause part p`) a metadata annotation, and §7.27.3 gives the
// implicit specialization it contributes:
//   - a usage annotated with a usage baseType subsets that baseType;
//   - a definition annotated with a definition baseType subclassifies it;
//   - a definition annotated with a usage baseType subclassifies the types of
//     that baseType.
//
// Any other combination contributes nothing. The second result is false when an
// annotation named a type that could not be resolved yet, so the answer is
// provisional and must not be memoized.
func (m *Model) semanticMetadataBases(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	switch sym.Decl.(type) {
	case *ast.Definition, *ast.Usage, *ast.SubjectMember, *ast.AssumeMember, *ast.RequireMember:
	default:
		return nil, true
	}
	isDef := !sym.IsFeature()

	var out []*symbols.Symbol
	complete := true
	for _, a := range MetadataAnnotationsOf(sym.Decl) {
		p := a.Node
		if p.Type == nil {
			continue
		}
		def, ok := m.resolveAnnotationType(sym, a)
		if !ok || def == nil {
			complete = false
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(def); aliasOK {
			def = resolved
		} else {
			continue
		}
		if !m.isSemanticMetadata(def) {
			continue
		}
		base := m.baseTypeOf(def, sym)
		if base == nil {
			continue
		}
		switch {
		case isDef && base.IsFeature():
			// A definition cannot subclassify a feature: it subclassifies
			// what the feature is typed by.
			out = append(out, m.DirectSupertypes(base)...)
		case isDef || base.IsFeature():
			// Definition with a definition baseType, or usage with a usage
			// baseType. A usage with a definition baseType adds nothing.
			out = append(out, base)
		}
	}
	return out, complete
}

// MetadataAnnotation is a metadata feature annotating an element, written either
// as a prefix (`#A part p`) or as a member of the element's body (`@A;`).
type MetadataAnnotation struct {
	Node   *ast.PrefixMetadata
	Prefix bool
}

// resolveAnnotationType resolves the metadata type an annotation names. The
// annotated element owns the annotation, prefix or member (KerML 8.2.4.2
// PrefixMetadataMember), so the type is looked up in its own scope first.
func (m *Model) resolveAnnotationType(sym *symbols.Symbol, a MetadataAnnotation) (*symbols.Symbol, bool) {
	scopes := []*symbols.Scope{AnnotationScope(sym)}
	if scopes[0] != sym.OwnerScope {
		scopes = append(scopes, sym.OwnerScope)
	}
	for _, scope := range scopes {
		if def, ok := m.resolver.ResolveQualified(scope, a.Node.Type); ok && def != nil {
			return def, true
		}
	}
	return nil, false
}

// AnnotationScope is where an annotation on sym names its type: sym's own scope,
// or the enclosing one when sym has none.
func AnnotationScope(sym *symbols.Symbol) *symbols.Scope {
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// MetadataAnnotationsOf returns the metadata features annotating a declaration,
// in declaration order. `@A about x` annotates other elements, so it is not one.
func MetadataAnnotationsOf(decl ast.Node) []MetadataAnnotation {
	return metadataAnnotationsAbout(decl, false)
}

// MetadataAnnotationsAboutOthers returns the metadata features written on a
// declaration that annotate other elements (`@A about x`), in declaration order.
func MetadataAnnotationsAboutOthers(decl ast.Node) []MetadataAnnotation {
	return metadataAnnotationsAbout(decl, true)
}

// metadataAnnotationsAbout keeps the annotations written on decl with (about)
// or without an `about`.
func metadataAnnotationsAbout(decl ast.Node, about bool) []MetadataAnnotation {
	var out []MetadataAnnotation
	for _, a := range MetadataAnnotationsWritten(decl) {
		if (len(a.Node.About) > 0) == about {
			out = append(out, a)
		}
	}
	return out
}

// MetadataAnnotationsWritten returns every metadata feature written on decl, as
// prefixes then as members of its body, in declaration order.
func MetadataAnnotationsWritten(decl ast.Node) []MetadataAnnotation {
	prefixes, members, ok := ast.DeclaredMetadata(decl)
	if !ok {
		return nil
	}
	var out []MetadataAnnotation
	for _, p := range prefixes {
		if p != nil {
			out = append(out, MetadataAnnotation{Node: p, Prefix: true})
		}
	}
	for _, member := range members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		if p, ok := member.(*ast.PrefixMetadata); ok {
			out = append(out, MetadataAnnotation{Node: p})
		}
	}
	return out
}

// isSemanticMetadata reports whether def is a direct or indirect specialization
// of the KerML SemanticMetadata metaclass.
func (m *Model) isSemanticMetadata(def *symbols.Symbol) bool {
	if m.resolver.Index() == nil {
		return false
	}
	for _, meta := range m.resolver.Index().LookupQualified(semanticMetadataFQN) {
		if meta == nil {
			continue
		}
		if def == meta {
			return true
		}
		for _, sup := range m.AllSupertypes(def) {
			if sup == meta {
				return true
			}
		}
	}
	return false
}

// baseTypeOf returns the type bound to def's baseType feature for annotated,
// resolving the meta-cast operand of `:>> baseType = causes meta SysML::Usage`
// (§7.27.3). The binding is model-level evaluated, so a conditional binding is
// decided against the element being annotated. A definition binding no baseType
// of its own inherits the binding of the nearest supertypes that do; a binding
// replaces those of the supertypes of its binder, and unrelated binders are
// ordered as inherited members are, breadth-first in declaration order. A binder
// whose value does not resolve still replaces what it inherits. Returns nil when
// the binding in force names no type.
func (m *Model) baseTypeOf(def, annotated *symbols.Symbol) *symbols.Symbol {
	if def == nil {
		return nil
	}
	if base, bound := m.ownBaseTypeOf(def, annotated); bound {
		return base
	}
	binders := m.baseTypeBinders(def)
	for _, binder := range binders {
		if !m.baseTypeRebound(binder, binders) {
			base, _ := m.ownBaseTypeOf(binder, annotated)
			return base
		}
	}
	return nil
}

// baseTypeBinders lists the nearest supertypes of def whose own body binds
// baseType, breadth-first in declaration order, not looking past a binder. A
// declaration reached through several scope trees is visited once.
func (m *Model) baseTypeBinders(def *symbols.Symbol) []*symbols.Symbol {
	seen := map[symbols.ElementKey]bool{symbols.KeyOf(def): true}
	var binders []*symbols.Symbol
	for frontier := m.DirectSupertypes(def); len(frontier) > 0; {
		var next []*symbols.Symbol
		for _, super := range frontier {
			key := symbols.KeyOf(super)
			if seen[key] {
				continue
			}
			seen[key] = true
			if m.bindsBaseType(super) {
				binders = append(binders, super)
				continue
			}
			next = append(next, m.DirectSupertypes(super)...)
		}
		frontier = next
	}
	return binders
}

// baseTypeRebound reports whether another of binders specializes binder, so its
// binding replaces binder's.
func (m *Model) baseTypeRebound(binder *symbols.Symbol, binders []*symbols.Symbol) bool {
	for _, other := range binders {
		if !symbols.SameElement(other, binder) && m.Conforms(other, binder) {
			return true
		}
	}
	return false
}

// bindsBaseType reports whether def's own body binds baseType.
func (m *Model) bindsBaseType(def *symbols.Symbol) bool {
	return len(baseTypeBindings(def)) > 0
}

// baseTypeBindings lists the usages in def's own body that redefine and bind baseType.
// The binding is an anonymous member, so it is reached through the AST, not the scope.
func baseTypeBindings(def *symbols.Symbol) []*ast.Usage {
	decl, ok := def.Decl.(*ast.Definition)
	if !ok || def.Scope == nil {
		return nil
	}
	var out []*ast.Usage
	for _, member := range decl.Members {
		if mem, ok := member.(*ast.Membership); ok {
			member = mem.Member
		}
		if usage, ok := member.(*ast.Usage); ok && redefinesBaseType(usage) && usage.Value != nil {
			out = append(out, usage)
		}
	}
	return out
}

// ownBaseTypeOf returns the type def's own body binds baseType to for annotated,
// and whether the body binds baseType at all.
func (m *Model) ownBaseTypeOf(def, annotated *symbols.Symbol) (*symbols.Symbol, bool) {
	bindings := baseTypeBindings(def)
	for _, usage := range bindings {
		name := metaCastOperand(m.baseTypeBinding(def, annotated, usage.Value))
		if name == nil {
			continue
		}
		if base, ok := m.resolver.ResolveQualified(def.Scope, name); ok {
			if resolved, aliasOK := m.resolver.ResolveAliasTarget(base); aliasOK {
				return resolved, true
			}
		}
	}
	return nil, len(bindings) > 0
}

// baseTypeBinding reduces a baseType binding to the branch that applies to the
// annotated element: the binding is model-level evaluated (KerML §7.4.9), so
// `if annotatedElement istype S ? SS meta KerML::Type else CC meta KerML::Class`
// names one type per annotation. Returns nil when no branch can be decided.
func (m *Model) baseTypeBinding(def, annotated *symbols.Symbol, value ast.Node) ast.Node {
	op, ok := value.(*ast.OperatorExpr)
	if !ok || op.Operator != ast.OpConditional || len(op.Operands) != 3 {
		return value
	}
	cond, decided := m.evalAnnotatedElementTest(def, annotated, op.Operands[0])
	if !decided {
		return nil
	}
	if cond {
		return m.baseTypeBinding(def, annotated, op.Operands[1])
	}
	return m.baseTypeBinding(def, annotated, op.Operands[2])
}

// evalAnnotatedElementTest evaluates `annotatedElement istype T` (or `hastype`)
// against the metaclass of the annotated element, the value annotatedElement
// holds for this annotation ([KerML, 8.3.4.9]).
func (m *Model) evalAnnotatedElementTest(def, annotated *symbols.Symbol, cond ast.Node) (bool, bool) {
	op, ok := cond.(*ast.OperatorExpr)
	if !ok || len(op.Operands) == 0 {
		return false, false
	}
	if op.Operator != ast.OpIsType && op.Operator != ast.OpHasType {
		return false, false
	}
	if !readsAnnotatedElement(op.Operands[0]) {
		return false, false
	}
	qn := op.TypeRef
	if qn == nil && len(op.Operands) > 1 {
		if fr, isRef := op.Operands[1].(*ast.FeatureReference); isRef {
			qn = fr.Name
		}
	}
	target, resolved := m.resolveExprTarget(def.Scope, qn)
	meta := m.metaclassOf(annotated)
	if !resolved || meta == nil {
		return false, false
	}
	if op.Operator == ast.OpHasType {
		return meta == target, true
	}
	return m.Conforms(meta, target), true
}

// readsAnnotatedElement reports whether expr reads the annotatedElement feature.
func readsAnnotatedElement(expr ast.Node) bool {
	fr, ok := expr.(*ast.FeatureReference)
	if !ok || fr.Name == nil || len(fr.Name.Parts) == 0 {
		return false
	}
	return fr.Name.Parts[len(fr.Name.Parts)-1].Text == annotatedElementName
}

// redefinesBaseType reports whether usage redefines SemanticMetadata::baseType.
func redefinesBaseType(usage *ast.Usage) bool {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		if qn.Parts[len(qn.Parts)-1].Text == baseTypeFeature {
			return true
		}
	}
	return false
}

// metaCastOperand returns the qualified name of the type a baseType binding
// denotes: the left operand of a `meta` cast, or the reference itself.
func metaCastOperand(value ast.Node) *ast.QualifiedName {
	if op, ok := value.(*ast.OperatorExpr); ok && op.Operator == ast.OpMeta && len(op.Operands) > 0 {
		value = op.Operands[0]
	}
	if fr, ok := value.(*ast.FeatureReference); ok {
		return fr.Name
	}
	if qn, ok := value.(*ast.QualifiedName); ok {
		return qn
	}
	return nil
}
