package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ResolveTarget resolves a relationship target to the feature it names. A
// target is either a qualified name (`references takePicture`) or a feature
// chain (`perform providePower.generateTorque`), and the chain's segments are
// members of the previous segment rather than of the enclosing scope
// (SysML 7.6.6).
//
// Unlike resolveFeatureChain, this reports nothing: the target of a declared
// relationship is already resolved — and diagnosed — while walking the
// document, and this is the query side of that result.
func (r *Resolver) ResolveTarget(scope *symbols.Scope, target ast.Node) (*symbols.Symbol, bool) {
	return r.resolveTarget(scope, target, nil)
}

// resolveTarget is ResolveTarget with an optional reference filter, which
// applies to the leading segment of the target only: the rest of a feature
// chain is looked up in the preceding segment, not in the enclosing scope.
func (r *Resolver) resolveTarget(scope *symbols.Scope, target ast.Node, hide *refFilter) (*symbols.Symbol, bool) {
	switch t := target.(type) {
	case nil:
		return nil, false
	case *ast.QualifiedName:
		return r.resolveQualified(scope, t, hide)
	case *ast.FeatureReference:
		if t.Name == nil {
			return nil, false
		}
		return r.resolveQualified(scope, t.Name, hide)
	case *ast.IndexExpr:
		// `a#(1)` names one element of the sequence a, of a's own type.
		if t.Bracket {
			return nil, false
		}
		return r.resolveTarget(scope, t.Operand, hide)
	case *ast.FeatureChainExpr:
		owner, ok := r.resolveTarget(scope, t.Operand, hide.forPrefix())
		if !ok || t.Member == nil {
			return nil, false
		}
		return r.memberChain(owner, t.Member, t)
	default:
		return nil, false
	}
}

// refFilter hides, during one reference-subsetting resolution, the bindings the
// referring declaration itself contributes. An unnamed feature takes its name
// from the feature it references (KerML Feature::effectiveName), so
// `perform 'provide power';` binds `'provide power'` in the very scope its
// target is looked up in; the target names the referenced feature, never a
// reference to it, so such borrowed bindings are invisible to it while a
// declaration of that name — local, inherited or imported — is a valid target.
//
// namingTarget hides the one feature a reference named, whoever is resolving
// it: the semantic model reads a redefinition without knowing it is a naming
// feature, and must still reach the redefined feature.
//
// hideBorrowedName applies only while resolving a reference subsetting's own
// target: a sibling that borrowed the same name is not the referenced feature
// (`perform a; perform a;` both perform a). Any other reference sees borrowed
// names as the owner's members they are.
type refFilter struct {
	decl ast.Node
	// redefiner is the feature decl declares; while its own target is resolved,
	// the redefinitions its owning type declares mask nothing there.
	redefiner    *symbols.Symbol
	namingTarget ast.Node
	targetName   string
	// featuredBy hides the members a scope features: a connector end's
	// participant is not a feature of the connector (KerML 8.3.4.5).
	featuredBy       *symbols.Scope
	hideBorrowedName bool
	skipNamingTarget bool
	skipBorrowedName bool
	// redefining marks a redefinition's own target, which the owner's other
	// redefinitions do not mask (KerML 8.3.3.3.6); a subsetting's target they do.
	redefining bool
}

// hides reports whether sym is a binding a reference target must not see.
func (f *refFilter) hides(sym *symbols.Symbol) bool {
	if f == nil || sym == nil {
		return false
	}
	if f.decl != nil && sym.Decl == f.decl {
		return true
	}
	if f.featuredBy != nil && sym.OwnerScope == f.featuredBy {
		return true
	}
	if f.namingTarget == nil {
		return false
	}
	if sym.NamingTarget == f.namingTarget {
		return true
	}
	if f.hideBorrowedName && namedByReference(sym) && sym.Name == f.targetName {
		return true
	}
	return false
}

// namedByReference reports whether sym borrowed its name from a reference
// subsetting rather than taking it from the feature it redefines: a redefining
// feature is itself the feature of that name here (KerML 7.3.4.5), so it is a
// valid target, while a borrowed binding never is.
func namedByReference(sym *symbols.Symbol) bool {
	return sym.Naming == symbols.NamedByReference
}

// contributedOnly reports whether a scope owner's own declarations are hidden,
// leaving only what it inherits or reference-subsets.
func (f *refFilter) contributedOnly() bool {
	return f != nil && f.decl != nil
}

// resolvesRedefinition reports whether f serves a redefinition's own target,
// which the owner's other redefinitions do not mask (KerML 8.3.3.3.6).
func (f *refFilter) resolvesRedefinition() bool {
	return f != nil && f.redefining
}

// hiding returns f extended to hide whatever target named a feature. A nil f
// yields a filter that hides only that.
func (f *refFilter) hiding(target ast.Node) *refFilter {
	if target == nil {
		return f
	}
	out := refFilter{namingTarget: target}
	if f != nil {
		out.decl = f.decl
		out.redefiner = f.redefiner
		out.featuredBy = f.featuredBy
		out.skipNamingTarget = f.skipNamingTarget
		out.skipBorrowedName = f.skipBorrowedName
		out.redefining = f.redefining
	}
	if out.skipNamingTarget {
		out.namingTarget = nil
		out.targetName = ""
		out.hideBorrowedName = false
	} else if out.decl != nil && !out.skipBorrowedName {
		if qn := ast.AsQualifiedName(target); qn != nil && len(qn.Parts) > 0 {
			out.targetName = qn.Parts[len(qn.Parts)-1].Text
			out.hideBorrowedName = true
		}
	}
	return &out
}

// forLeadingSegment returns f for the first segment of a qualified name: a
// subsetting's owner is a namespace that segment may name, while a redefinition
// still skips the redefining feature itself.
func (f *refFilter) forLeadingSegment() *refFilter {
	if f == nil || f.decl == nil || f.redefining {
		return f
	}
	out := *f
	out.decl = nil
	return &out
}

// forTail returns f for the later segments of a qualified name, members of what
// the segment before reached: a redefinition still skips the redefining feature
// itself wherever the walk meets it (KerML 8.2.3.5.2); nothing else is hidden.
func (f *refFilter) forTail() *refFilter {
	if f == nil || !f.redefining || f.decl == nil {
		return nil
	}
	return &refFilter{decl: f.decl, redefiner: f.redefiner, redefining: true}
}

// without returns syms less those f hides.
func (f *refFilter) without(syms []*symbols.Symbol) []*symbols.Symbol {
	if f == nil {
		return syms
	}
	kept := make([]*symbols.Symbol, 0, len(syms))
	for _, sym := range syms {
		if !f.hides(sym) {
			kept = append(kept, sym)
		}
	}
	return kept
}

func (f *refFilter) forPrefix() *refFilter {
	if f == nil {
		return nil
	}
	out := *f
	out.skipNamingTarget = true
	out.namingTarget = nil
	out.targetName = ""
	out.hideBorrowedName = false
	return &out
}

// lookupLocal is Scope.LookupLocal with the hidden bindings removed. A nil
// filter hides nothing.
func (f *refFilter) lookupLocal(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	if scope == nil {
		return nil, false
	}
	if f == nil {
		return scope.LookupLocal(name)
	}
	visible := make([]*symbols.Symbol, 0, 1)
	for _, sym := range scope.LookupLocalAll(name) {
		if !f.hides(sym) {
			visible = append(visible, sym)
		}
	}
	if len(visible) == 0 {
		return nil, false
	}
	return symbols.PreferDeclared(visible)[0], true
}

// referenceFilter is the filter a reference subsetting owned by decl resolves
// its target under; a chain target's prefix is looked up unhidden (see forPrefix).
func referenceFilter(decl, target ast.Node) *refFilter {
	hide := &refFilter{decl: decl}
	if _, ok := target.(*ast.FeatureChainExpr); ok {
		hide = hide.forPrefix()
	}
	return hide
}

// ResolveReferenceTarget resolves the target of a reference subsetting owned by
// decl, which is declared in scope.
func (r *Resolver) ResolveReferenceTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool) {
	return r.resolveTarget(scope, target, referenceFilter(decl, target))
}

// ResolveRedefinitionTarget resolves the target of a redefinition owned by decl:
// the feature decl's owner inherits, which decl's own redefinition does not
// mask (KerML 8.3.3.3.6), whatever name decl borrowed from it.
func (r *Resolver) ResolveRedefinitionTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool) {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	switch target := target.(type) {
	case *ast.QualifiedName:
		return r.ResolveReference(Reference{Scope: scope, QN: target, Referrer: decl, Redefines: true})
	case *ast.FeatureChainExpr:
		return r.resolveRedefinedChain(scope, target, decl)
	}
	return r.resolveTarget(scope, target, referenceFilter(decl, target))
}

// Reference describes one occurrence of a name to resolve on its own, outside a
// document walk: which scope it is written in, the declaration that refers to it
// when it is a reference subsetting's target, and the feature chain it is the
// member segment of. Used by the editor layer, which resolves a single name
// under the cursor and must reach the same symbol the document walk did.
type Reference struct {
	Scope *symbols.Scope
	QN    *ast.QualifiedName
	// Referrer owns the reference subsetting QN is the target of, if any.
	Referrer ast.Node
	// Chain is set when QN is the member of a feature chain, whose segments are
	// members of the operand rather than of Scope (SysML 7.6.6).
	Chain *ast.FeatureChainExpr
	// Constructed is set when QN labels a constructor argument: a simple name is
	// a feature of the instantiated type Constructed names, a qualified one is
	// resolved in Scope.
	Constructed *ast.QualifiedName
	// Redefines is set when QN is the target of a redefinition, which names a
	// feature of Scope's generals rather than a member of Scope itself.
	Redefines bool
	// Condition is set when QN is a name inside an element-filter condition, which
	// its own namespace's filters do not restrict (see InCondition).
	Condition bool
	// Invocation is set when QN is the name an invocation calls, which overload
	// selection may resolve to one of several declarations (see ResolveInvocationName).
	Invocation *ast.InvocationExpr
	// Performed is set when Invocation is the value of an action usage, which
	// runs the action it names rather than evaluating a calc of that name.
	Performed bool
	// Endpoint is set when QN is a transition end, which names a vertex of the
	// enclosing machine ahead of anything else it reaches (see ResolveEndpoint).
	Endpoint bool
	// Import is the import QN is the target of, if any; an `import all` reaches
	// private members the visibility rule would otherwise hide.
	Import *ast.Import
	// Subsetting is the declaration QN subsets, if any; its spelling decides
	// whether it is read as a redefinition, as a sibling redefining the name, or
	// as the declaration's own name.
	Subsetting ast.Node
	// Member is the declaration whose text QN is written in, when known.
	Member ast.Node
	// Head is set when QN is written in a head relationship of a declaration
	// with a scope of its own, where the target may resolve ahead of Scope.
	Head *HeadRelationship
}

// HeadRelationship places a reference in a declaration's head relationship: its
// target resolves in Scope, the declaration's own, when it opens with a member there.
type HeadRelationship struct {
	Scope *symbols.Scope
	Kind  ast.RelationshipKind
	// Member is the chain member the reference is a segment of; nil when the
	// reference is the name decided on.
	Member *ast.QualifiedName
}

// Spelled returns ref as it would be collected had qn been written in its
// place: the same occurrence, read the way the document walk reads that spelling.
func (ref Reference) Spelled(qn *ast.QualifiedName) Reference {
	ref.QN = qn
	if ref.Subsetting != nil {
		if namesDecl(qn, ref.Subsetting) {
			ref.Referrer, ref.Redefines = nil, false
		} else {
			ref.Referrer, ref.Redefines = ref.Subsetting, true
		}
	}
	return ref
}

// ProbeReference resolves ref as a trial reading: what a name spelled differently
// at the same place would denote. Its diagnostics are suppressed.
func (r *Resolver) ProbeReference(ref Reference) (*symbols.Symbol, bool) {
	var (
		sym *symbols.Symbol
		ok  bool
	)
	r.aside(func() { sym, ok = r.ResolveReference(ref) })
	return sym, ok
}

// ResolveReference resolves a single name occurrence, honoring both the
// reference-subsetting and feature-chain rules.
func (r *Resolver) ResolveReference(ref Reference) (*symbols.Symbol, bool) {
	if ref.QN == nil {
		return nil, false
	}
	if ref.Head != nil {
		ref.Scope, ref.Head = r.headScope(ref), nil
	}
	var hide *refFilter
	if ref.Referrer != nil {
		hide = &refFilter{decl: ref.Referrer}
	}
	if ref.Condition {
		ref.Condition = false
		var (
			sym *symbols.Symbol
			ok  bool
		)
		r.InCondition(func() { sym, ok = r.ResolveReference(ref) })
		return sym, ok
	}
	if ref.Chain != nil {
		var (
			owner *symbols.Symbol
			ok    bool
		)
		if ref.Redefines {
			owner, ok = r.redefinedChainOperand(ref.Scope, ref.Chain, ref.Referrer)
		} else {
			owner, ok = r.resolveTarget(ref.Scope, ref.Chain.Operand, hide)
		}
		if !ok {
			return nil, false
		}
		// A qualified segment the owner has no member for reads outward, as
		// resolveFeatureChain does.
		if len(ref.QN.Parts) > 1 {
			if _, member := r.chainMember(owner, ref.QN.Parts[0].Text, ref.Chain); !member {
				var outward *symbols.Symbol
				if r.probe(ref.QN, func() bool {
					outward, ok = r.resolveQualified(ref.Scope, ref.QN, hide)
					return ok
				}) {
					return outward, true
				}
			}
		}
		return r.memberChain(owner, ref.QN, ref.Chain)
	}
	if ref.Constructed != nil {
		if ref.QN != nil && len(ref.QN.Parts) > 1 {
			return r.resolveQualified(ref.Scope, ref.QN, hide)
		}
		owner, ok := r.resolveQualified(ref.Scope, ref.Constructed, hide)
		if !ok {
			return nil, false
		}
		return r.memberChain(owner, ref.QN, nil)
	}
	if ref.Redefines {
		// A subsetting reaches a sibling redefinition first, as resolveRelationships does.
		if ref.Subsetting == nil || !r.resolveOwnSibling(ref.Scope, ref.QN, ref.Subsetting) {
			r.resolveRedefinition(ref.Scope, ref.QN, ref.Referrer, ref.Subsetting == nil)
		}
		return r.PartSymbol(ref.QN, len(ref.QN.Parts)-1)
	}
	if ref.Endpoint {
		return r.ResolveEndpoint(ref.Scope, ref.QN)
	}
	if ref.Import != nil {
		return r.resolveImportName(ref.Scope, ref.QN, ref.Import)
	}
	if ref.Invocation != nil {
		return r.ResolveInvocationName(ref.Scope, ref.QN)
	}
	return r.resolveQualified(ref.Scope, ref.QN, hide)
}

// headScope is the scope ref, written in a head relationship, resolves in as the
// document walk reads it; spelled differently, the same occurrence may switch.
func (r *Resolver) headScope(ref Reference) *symbols.Scope {
	name, chain := ref.QN, ref.Chain != nil
	if ref.Head.Member != nil {
		name, chain = ref.Head.Member, true
	}
	if r.resolvesInHeader(ref.Head.Scope, name, chain, ref.Head.Kind) {
		return ref.Head.Scope
	}
	return ref.Scope
}

// leadingName returns the qualified name a relationship target starts with: the
// only part of it looked up in the enclosing scope.
func leadingName(target ast.Node) ast.Node {
	for {
		chain, ok := target.(*ast.FeatureChainExpr)
		if !ok {
			break
		}
		target = chain.Operand
	}
	if qname := ast.AsQualifiedName(target); qname != nil {
		return qname
	}
	return nil
}

// memberChain walks qn's segments as members of owner, reading each segment as
// the document walk does (see chainMember).
func (r *Resolver) memberChain(owner *symbols.Symbol, qn *ast.QualifiedName, chain ast.Node) (*symbols.Symbol, bool) {
	cur := owner
	for i, part := range qn.Parts {
		next, ok := r.chainMember(cur, part.Text, chain)
		if !ok {
			next, ok = r.implicitlyNamedMember(cur.Scope, part.Text, nil)
		}
		if !ok {
			return nil, false
		}
		cur = r.resolvedPart(qn, i, next)
	}
	return cur, true
}
