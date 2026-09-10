// Package semantics provides the derived semantic model that validation
// depth-C constraint checks rely on: a specialization/typing graph (with cycle
// detection), and — in later increments — inherited-member resolution,
// multiplicity extraction, and a bounded model-level expression evaluator.
//
// All results are memoized in side tables keyed by *symbols.Symbol, consistent
// with the project rule that semantic information lives outside the immutable
// AST. A Model is built per resolution session over an existing symbol index
// and name resolver.
package semantics

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Model is the derived semantic model over a symbol index. It memoizes the
// specialization graph computed from resolved def/usage relationships.
type Model struct {
	resolver *resolve.Resolver

	directSupers map[*symbols.Symbol][]*symbols.Symbol
	allSupers    map[*symbols.Symbol][]*symbols.Symbol
	// provisionalSupers holds the symbols whose last DirectSupertypes answer was
	// incomplete, so neither it nor a closure over it may be memoized.
	provisionalSupers map[*symbols.Symbol]bool
	// computingSupers holds the resolver depth of each DirectSupertypes call on
	// the stack, so the re-entrancy guard answers nil for its symbol.
	computingSupers map[*symbols.Symbol]int
	// valuing holds the features whose typing value is being judged, so a value
	// that leads back to its own feature is not followed again.
	valuing       map[*symbols.Symbol]bool
	referenced    map[*symbols.Symbol]*symbols.Symbol
	resolvingRef  map[*symbols.Symbol]bool
	memberSources map[*symbols.Symbol][]*symbols.Symbol
	contributed   map[*symbols.Symbol][]*symbols.Symbol // memoized contributors
	primTypes     map[*symbols.Symbol]PrimType
	scalars       map[*symbols.Symbol]PrimType // stdlib scalar symbols, resolved once
	params        map[*symbols.Symbol]behaviorParameters
	invocations   map[invocationKey]*InvocationSelection
	arguments     ArgumentTyper // the checker's argument typing, nil when no checker runs
	sourceText    source.Lookup // notation of the loaded documents, nil when unavailable
	// typingArgs holds the calls whose arguments are being typed, so an argument
	// whose type leads back to its own call is not typed again.
	typingArgs map[*ast.InvocationExpr]bool
	composed   map[composedKey][]*symbols.Symbol
	ends       map[*symbols.Symbol][]connectorEnd

	superEdgeCache map[*symbols.Symbol][]superEdge      // generalization edges with conjugation
	conjSupers     map[*symbols.Symbol][]conjugatedType // supertypes with conjugation parity

	unitTerms    map[*symbols.Symbol]UnitTerm // measurement units reduced to base units
	reducingUnit map[*symbols.Symbol]bool     // units being reduced, to detect a cycle

	dimensions   map[*symbols.Symbol]dimensionResult // units to the dimension they measure in
	dimensioning map[*symbols.Symbol]bool            // units whose dimension is being derived, to detect a cycle
	libSymbols   map[string]*symbols.Symbol          // library elements resolved by qualified name
	baseUnits    map[*symbols.Symbol]*symbols.Symbol // base quantities to SI::si's base units, nil until read

	// Element-filter evaluation: conditions compiled once per expression, their
	// verdicts memoized per candidate, and the metadata annotating each candidate
	// collected once. A filter is evaluated on every import enumeration, so none
	// of this may be recomputed per candidate (KerML 8.2.4; see filter.go).
	filterPreds    map[ast.Node]*symbols.FilterPredicate
	filterVerdicts map[filterKey]filterVerdict
	filterTypes    map[string]*symbols.Symbol
	annotations    map[*symbols.Symbol][]annotation
	aboutAnnots    map[*symbols.Symbol][]annotation
	// aboutByDecl keys the same annotations by the declaration of the element
	// annotated, reaching them from a re-indexed twin of that symbol.
	aboutByDecl map[ast.Node][]annotation
	// aboutOrder lists aboutAnnots' targets in first-annotation order.
	aboutOrder []*symbols.Symbol

	// Redefinition masking (see masking.go): the features each declaration
	// redefines, and the elements each type does not inherit because of them.
	redefined map[*symbols.Symbol][]*symbols.Symbol
	redefMask map[*symbols.Symbol]map[*symbols.Symbol]bool
	// redefMaskInherited is the same mask counting inherited redefinitions only.
	redefMaskInherited         map[*symbols.Symbol]map[*symbols.Symbol]bool
	redefClosure               map[*symbols.Symbol]map[*symbols.Symbol]bool
	computingRedefClosure      map[*symbols.Symbol]bool
	computingRedefinedFeatures int
	// ctorSlots memoizes each type's constructible features (see shape.go).
	ctorSlots map[*symbols.Symbol]constructorSlots
	// members and shapes memoize MembersOf and ShapeFeatures once the member
	// sources they read are complete (see members.go).
	members map[memberKey][]*symbols.Symbol
	shapes  map[*symbols.Symbol][]ShapeFeature
	// bodyApplications is the operation each body expression is passed to, indexed per
	// document root on first query (see bodyparam.go).
	bodyApplications map[*ast.BodyExpr]bodyApplication
	bodyIndexed      map[*symbols.Scope]bool
}

// NewModel creates a semantic model backed by the given name resolver. The
// resolver must already be associated with the index whose symbols will be
// queried. The model attaches itself to the resolver so name resolution sees
// inherited members, which a redefinition target may only be reachable through.
func NewModel(resolver *resolve.Resolver) *Model {
	m := &Model{
		resolver:     resolver,
		directSupers: make(map[*symbols.Symbol][]*symbols.Symbol),
		allSupers:    make(map[*symbols.Symbol][]*symbols.Symbol),

		provisionalSupers: make(map[*symbols.Symbol]bool),
		computingSupers:   make(map[*symbols.Symbol]int),
		valuing:           make(map[*symbols.Symbol]bool),
		referenced:        make(map[*symbols.Symbol]*symbols.Symbol),
		resolvingRef:      make(map[*symbols.Symbol]bool),
		memberSources:     make(map[*symbols.Symbol][]*symbols.Symbol),
		contributed:       make(map[*symbols.Symbol][]*symbols.Symbol),
		primTypes:         make(map[*symbols.Symbol]PrimType),
		params:            make(map[*symbols.Symbol]behaviorParameters),
		invocations:       make(map[invocationKey]*InvocationSelection),
		typingArgs:        make(map[*ast.InvocationExpr]bool),
		composed:          make(map[composedKey][]*symbols.Symbol),
		ends:              make(map[*symbols.Symbol][]connectorEnd),

		superEdgeCache: make(map[*symbols.Symbol][]superEdge),
		conjSupers:     make(map[*symbols.Symbol][]conjugatedType),
		unitTerms:      make(map[*symbols.Symbol]UnitTerm),
		reducingUnit:   make(map[*symbols.Symbol]bool),

		dimensions:   make(map[*symbols.Symbol]dimensionResult),
		dimensioning: make(map[*symbols.Symbol]bool),
		libSymbols:   make(map[string]*symbols.Symbol),

		filterPreds:    make(map[ast.Node]*symbols.FilterPredicate),
		filterVerdicts: make(map[filterKey]filterVerdict),
		filterTypes:    make(map[string]*symbols.Symbol),
		annotations:    make(map[*symbols.Symbol][]annotation),

		redefined:             make(map[*symbols.Symbol][]*symbols.Symbol),
		redefMask:             make(map[*symbols.Symbol]map[*symbols.Symbol]bool),
		redefMaskInherited:    make(map[*symbols.Symbol]map[*symbols.Symbol]bool),
		redefClosure:          make(map[*symbols.Symbol]map[*symbols.Symbol]bool),
		computingRedefClosure: make(map[*symbols.Symbol]bool),
		ctorSlots:             make(map[*symbols.Symbol]constructorSlots),
		members:               make(map[memberKey][]*symbols.Symbol),
		shapes:                make(map[*symbols.Symbol][]ShapeFeature),
		bodyApplications:      make(map[*ast.BodyExpr]bodyApplication),
		bodyIndexed:           make(map[*symbols.Scope]bool),
	}
	if resolver != nil {
		resolver.SetModel(m)
	}
	return m
}

// GeneralizationKind reports whether a relationship kind forms a conformance
// ("is-a" / "conforms-to") edge for the specialization graph: specialization on
// definitions, and subsetting/redefinition/typing on usages.
//
// Reference subsetting (`references`) is excluded even though KerML 8.3.3.3.9
// makes it a kind of Subsetting: it contributes members through MemberSources
// instead, so that a referencing feature does not silently acquire the
// referenced feature's type for conformance and implicit-typing purposes.
// crosses is a feature-value edge, not generalization, and is excluded too.
func GeneralizationKind(k ast.RelationshipKind) bool {
	switch k {
	case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
		return true
	default:
		return false
	}
}

// RelationshipsOf returns the declared relationships of a symbol's def/usage
// declaration, or nil for symbols that are not def/usage.
func RelationshipsOf(sym *symbols.Symbol) []*ast.Relationship {
	if oc, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		return oc.Relationships
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Relationships
	case *ast.Usage:
		return d.Relationships
	case *ast.ConnectorEnd:
		return d.Relationships
	case *ast.CrossFeatureMember:
		return d.Relationships
	case *ast.BodyExpr:
		// A body parameter is not a node of its own, so its symbol declares the
		// body and names the parameter its typing is written on.
		return bodyParamRelationships(d, sym.Name)
	case *ast.SubjectMember:
		return subjectRelationships(d)
	case *ast.AssumeMember:
		return d.Relationships
	case *ast.RequireMember:
		return d.Relationships
	default:
		return nil
	}
}

// subjectRelationships returns a subject parameter's relationships, the typing
// it writes as `subject s : T` first.
func subjectRelationships(subj *ast.SubjectMember) []*ast.Relationship {
	if subj.TypeRef == nil {
		return subj.Relationships
	}
	out := make([]*ast.Relationship, 0, len(subj.Relationships)+1)
	out = append(out, &ast.Relationship{Kind: ast.RelTyping, Target: subj.TypeRef})
	return append(out, subj.Relationships...)
}

// bodyParamRelationships returns the relationships of the body parameter named
// name, its typing included.
func bodyParamRelationships(body *ast.BodyExpr, name string) []*ast.Relationship {
	for i := range body.Params {
		p := &body.Params[i]
		if p.Name != name {
			continue
		}
		if p.Type == nil {
			return p.Relationships
		}
		out := make([]*ast.Relationship, 0, len(p.Relationships)+1)
		out = append(out, &ast.Relationship{Kind: ast.RelTyping, Target: p.Type})
		return append(out, p.Relationships...)
	}
	return nil
}

// DirectSupertypes returns the immediate supertype symbols of sym: the resolved
// targets of its generalization relationships. Unresolved or non-def/usage
// targets are skipped. The result is memoized and deterministic (declaration
// order, duplicates removed).
func (m *Model) DirectSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.directSupers[sym]; ok {
		// The seed answers a re-entrant query with nothing, cutting that query short.
		if depth := m.computingSupers[sym]; depth != 0 {
			m.resolver.CutShort(depth)
		}
		return cached
	}
	// Guard against re-entrancy on cyclic graphs: seed with an empty slice.
	m.directSupers[sym] = nil
	m.computingSupers[sym] = m.resolver.Enter()
	defer delete(m.computingSupers, sym)
	if sym.Facts != nil && sym.Facts.Supers != nil {
		out := m.recordedSupertypes(sym)
		m.directSupers[sym] = out
		m.resolver.Leave()
		return out
	}

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		// Unwrap FeatureReference if needed
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			// A chain target (`subsets b.f`) generalizes to the chain's final feature.
			if fc, isChain := targetNode.(*ast.FeatureChainExpr); isChain {
				target, ok := m.resolver.ResolveTarget(sym.OwnerScope, fc)
				if ok && target != nil && target != sym && !seen[target] {
					seen[target] = true
					out = append(out, target)
				}
			}
			continue
		}
		target, ok := m.generalizationTarget(sym, rel.Kind, qn)
		if !ok || target == nil {
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(target); aliasOK {
			target = resolved
		} else {
			continue
		}
		// Same-named subsettings and redefinitions target the inherited feature,
		// not the binding that resolves first in the owner's scope.
		if len(qn.Parts) == 1 && (rel.Kind == ast.RelRedefines ||
			(rel.Kind == ast.RelSubsets && !subsetsSibling(sym, target))) {
			if redefined := m.inheritedFeature(sym, qn); redefined != nil {
				target = redefined
			} else if target == sym {
				continue
			}
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}

	// Flow usages (message/flow keywords) need implicit typing from stdlib Message/Flow
	// even when explicitly typed "of Type". This allows accessing stdlib members like
	// sourceEvent/targetEvent in messages typed by payload item defs.
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageFlow {
		// Look up stdlib Message flow def directly from index (FQN: Flows::Message)
		messageDefs := m.resolver.Index().LookupQualified("Flows::Message")
		if len(messageDefs) > 0 && messageDefs[0] != nil {
			messageDef := messageDefs[0]
			if !seen[messageDef] {
				seen[messageDef] = true
				out = append(out, messageDef)
			}
		}
	}

	// Action usages containing send statements need implicit SendAction typing
	// to provide access to sentMessage member
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
		if hasSendStatement(usage) {
			sendActionDefs := m.resolver.Index().LookupQualified("Actions::SendAction")
			if len(sendActionDefs) > 0 && sendActionDefs[0] != nil {
				sendActionDef := sendActionDefs[0]
				if !seen[sendActionDef] {
					seen[sendActionDef] = true
					out = append(out, sendActionDef)
				}
			}
		}
	}

	// A send written as an action node is a SendActionUsage of its own, so it is
	// typed by SendAction whether or not a usage declares it.
	if _, ok := sym.Decl.(*ast.SendStatement); ok {
		for _, sendDef := range m.resolver.Index().LookupQualified("Actions::SendAction") {
			if sendDef == nil || sendDef == sym || seen[sendDef] {
				continue
			}
			seen[sendDef] = true
			out = append(out, sendDef)
			break
		}
	}

	// An action usage with an accept payload is an AcceptActionUsage, implicitly
	// typed by AcceptAction, which supplies `receiver` and `acceptedMessage`
	// (SysML v2 §7.16.5, §8.3.17).
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageAction && hasAcceptPayload(usage) {
		for _, acceptDef := range m.resolver.Index().LookupQualified("Actions::AcceptAction") {
			if acceptDef == nil || acceptDef == sym || seen[acceptDef] {
				continue
			}
			seen[acceptDef] = true
			out = append(out, acceptDef)
			break
		}
	}

	// A variant specializes the variation it is a variant of, so it carries the
	// variation's type and features and restates only what it chooses
	// (SysML v2 §7.20).
	if variation := m.VariationPointOwning(sym); variation != nil && variation != sym && !seen[variation] {
		seen[variation] = true
		out = append(out, variation)
	}

	// Semantic metadata annotating this element — a `#keyword` prefix — adds the
	// implicit specialization of its baseType (SysML v2 §7.27.3, §7.27.4).
	fromMetadata := false
	metadataBases, metadataComplete := m.semanticMetadataBases(sym)
	for _, base := range metadataBases {
		if base == nil || base == sym || seen[base] {
			continue
		}
		fromMetadata = true
		seen[base] = true
		out = append(out, base)
	}

	// A parameter of a behavior or step implicitly redefines the corresponding
	// parameter of each behavior or step its owner specializes, and so takes
	// that parameter's type when it declares none (see redefinition.go).
	for _, redefined := range m.ImplicitParameterRedefinitions(sym) {
		if seen[redefined] {
			continue
		}
		seen[redefined] = true
		out = append(out, redefined)
	}

	// An untyped parameter of a body a collection function applies is bound to each
	// element of the collection, so it takes the elements' types (see bodyparam.go).
	for _, typ := range m.BodyParameterElementTypes(sym) {
		if typ == nil || typ == sym || seen[typ] {
			continue
		}
		seen[typ] = true
		out = append(out, typ)
	}

	// An end of a connector implicitly redefines the end at its own position of
	// each connector its owner specializes, and so takes that end's type when it
	// declares none (see connector.go).
	for _, redefined := range m.implicitEndRedefinitions(sym) {
		if seen[redefined] {
			continue
		}
		seen[redefined] = true
		out = append(out, redefined)
	}

	// The cross feature an end owns is implicitly typed by the end's types and
	// subsets the cross feature of each end the end redefines (see crossing.go).
	for _, general := range m.implicitCrossFeatureGenerals(sym) {
		if general == sym || seen[general] {
			continue
		}
		seen[general] = true
		out = append(out, general)
	}

	// The subject, objective, actor or stakeholder of a case, requirement or their usages
	// implicitly redefines the same role of each general, so it takes that role's type
	// when it declares none (see roles.go).
	for _, redefined := range m.ImplicitRoleRedefinitions(sym) {
		if redefined == nil || redefined == sym || seen[redefined] {
			continue
		}
		seen[redefined] = true
		out = append(out, redefined)
	}

	// A declaration keeps its kind's bases whatever else it declares; implicitBases
	// suppresses one only when a declared chain already reaches it. A metadata
	// keyword supplies the kind itself, so its baseType stands in.
	if !fromMetadata {
		for _, base := range m.implicitBases(sym) {
			if !seen[base] {
				seen[base] = true
				out = append(out, base)
			}
		}
	}

	// An answer derived while a guard cut a query short saw fewer members than
	// the finished model has, so it is recomputed on the next query, not memoized.
	if !m.resolver.Leave() || !metadataComplete {
		delete(m.directSupers, sym)
		m.provisionalSupers[sym] = true
		return out
	}

	delete(m.provisionalSupers, sym)
	m.directSupers[sym] = out
	return out
}

// recordedSupertypes resolves the supertype edges installed for a library
// symbol, which the same derivation over its declaration produced when they were
// recorded. An edge naming nothing in this index is dropped, as an unresolved
// declared target is.
func (m *Model) recordedSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, fqn := range sym.Facts.Supers {
		for _, target := range m.resolver.Index().LookupQualified(fqn) {
			if target == nil {
				continue
			}
			resolved, aliasOK := m.resolver.ResolveAliasTarget(target)
			if !aliasOK || resolved == sym || seen[resolved] {
				break
			}
			seen[resolved] = true
			out = append(out, resolved)
			break
		}
	}
	return out
}

// SupertypesProvisional reports whether sym's supertypes were last derived while
// a generalization target or metadata type had not resolved yet, so the answer
// may still change and must not be recorded as a fact.
func (m *Model) SupertypesProvisional(sym *symbols.Symbol) bool {
	return m.provisionalSupers[sym]
}

// supersUnstable reports whether sym's supertype answer may still change: it was
// provisional, or its own computation is on the stack and the re-entrancy guard
// is answering nil for it.
func (m *Model) supersUnstable(sym *symbols.Symbol) bool {
	return m.provisionalSupers[sym] || m.computingSupers[sym] != 0
}

// generalizationTarget resolves a relationship target as the document walk
// reads it: a redefinition or subsetting names what its owner inherits, past
// the masks the owner's own redefinitions cause and the name sym borrows.
func (m *Model) generalizationTarget(sym *symbols.Symbol, kind ast.RelationshipKind, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	ref := resolve.Reference{Scope: sym.OwnerScope, QN: qn}
	switch {
	case kind == ast.RelRedefines:
		ref.Referrer, ref.Redefines = sym.Decl, true
	case kind == ast.RelSubsets:
		ref = resolve.Reference{Scope: sym.OwnerScope, Subsetting: sym.Decl}.Spelled(qn)
	case kind.ReferenceSubsets():
		ref.Referrer = sym.Decl
	}
	return m.resolver.ResolveReference(ref)
}

// relationshipTarget resolves the element rel names, as generalizationTarget
// reads it, following an alias to what it names; a chain target (`subsets
// b.f`) is its final feature.
func (m *Model) relationshipTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	if m.resolver == nil || rel == nil {
		return nil
	}
	node := rel.Target
	if fr, ok := node.(*ast.FeatureReference); ok {
		node = fr.Name
	}
	var (
		target *symbols.Symbol
		ok     bool
	)
	switch node := node.(type) {
	case *ast.FeatureChainExpr:
		if rel.Kind.ReferenceSubsets() {
			target, ok = m.resolver.ResolveReferenceTarget(sym.OwnerScope, sym.Decl, node)
		} else {
			target, ok = m.resolver.ResolveTarget(sym.OwnerScope, node)
		}
	case *ast.QualifiedName:
		target, ok = m.generalizationTarget(sym, rel.Kind, node)
	}
	if !ok || target == nil {
		return nil
	}
	if resolved, aliasOK := m.resolver.ResolveAliasTarget(target); aliasOK {
		return resolved
	}
	return nil
}

// subsetsSibling reports whether sym's subsetting resolved to another member of
// its own type, which shadows the inherited feature of that name (KerML 7.3.4.5).
func subsetsSibling(sym, target *symbols.Symbol) bool {
	return target != sym && sym.OwnerScope != nil && target.OwnerScope == sym.OwnerScope
}

// inheritedFeature returns the feature that sym's owner inherits under the name
// qn denotes, skipping the owner's own members so a redefinition does not find
// itself. Only a single-segment name can denote an inherited feature this way;
// a qualified one names its owner explicitly.
func (m *Model) inheritedFeature(sym *symbols.Symbol, qn *ast.QualifiedName) *symbols.Symbol {
	if len(qn.Parts) != 1 {
		return nil
	}
	return m.inheritedFeatureNamed(sym, qn.Parts[0].Text)
}

// inheritedFeatureNamed is inheritedFeature for an already-extracted name.
func (m *Model) inheritedFeatureNamed(sym *symbols.Symbol, name string) *symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return nil
	}
	var candidates []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, sup := range m.AllSupertypes(owner) {
		if found, ok := m.LookupMember(sup, name); ok && found != sym {
			if !seen[found] {
				seen[found] = true
				candidates = append(candidates, found)
			}
		}
	}
	for _, candidate := range candidates {
		moreSpecificCandidateExists := false
		for _, other := range candidates {
			if other != candidate && m.Conforms(other, candidate) {
				moreSpecificCandidateExists = true
				break
			}
		}
		if !moreSpecificCandidateExists {
			// Unrelated ties use the breadth-first declaration order as a deterministic choice.
			return candidate
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

// AllSupertypes returns the transitive closure of DirectSupertypes, excluding
// sym itself, in a deterministic order (breadth-first over declaration order).
// It is safe on cyclic graphs. The result is memoized.
func (m *Model) AllSupertypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	if cached, ok := m.allSupers[sym]; ok {
		return cached
	}

	var order []*symbols.Symbol
	visited := make(map[*symbols.Symbol]bool)
	queue := append([]*symbols.Symbol(nil), m.DirectSupertypes(sym)...)
	provisional := m.supersUnstable(sym)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == sym || visited[cur] {
			continue
		}
		visited[cur] = true
		order = append(order, cur)
		queue = append(queue, m.DirectSupertypes(cur)...)
		provisional = provisional || m.supersUnstable(cur)
	}
	// A closure over a provisional answer is provisional too.
	if provisional {
		delete(m.allSupers, sym)
		return order
	}
	m.allSupers[sym] = order
	return order
}

// Conforms reports whether a conforms to b: a == b, b is a (transitive)
// supertype of a, or a is the union of types that all conform to b. Symbols
// are compared as elements, so one declaration reached through two scope
// trees (a document's and the index's) conforms to itself.
func (m *Model) Conforms(a, b *symbols.Symbol) bool {
	return m.conforms(a, b, nil)
}

// IsDataType reports whether sym is Base::DataValue or conforms to it, so its
// values are data (scalars, enumerations, attribute values) and never elements.
func (m *Model) IsDataType(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return false
	}
	for _, dataValue := range m.resolver.Index().LookupQualified(dataValueFQN) {
		if dataValue != nil && m.Conforms(sym, dataValue) {
			return true
		}
	}
	return false
}

// LiteralConforms reports whether sym is an enumeration literal whose
// enumeration conforms to expected: a literal is a value of its enumeration.
func (m *Model) LiteralConforms(sym, expected *symbols.Symbol) bool {
	enum := EnumerationOwning(sym)
	return enum != nil && m.Conforms(enum, expected)
}

func (m *Model) conforms(a, b *symbols.Symbol, unioning map[*symbols.Symbol]bool) bool {
	if a == nil || b == nil {
		return false
	}
	if symbols.SameElement(a, b) || IsAnything(b) {
		return true
	}
	for _, s := range m.AllSupertypes(a) {
		if symbols.SameElement(s, b) {
			return true
		}
	}
	return m.unionConforms(a, b, unioning)
}

// unionConforms reports whether a is declared as the union of types that all
// conform to b. A union's instances are exactly those of its unioning types
// (KerML 1.0 §8.3.3), so `classifier MyWheel unions MyWheel1, MyWheel2`
// conforms to every type both of them conform to. unioning guards a cycle.
func (m *Model) unionConforms(a, b *symbols.Symbol, unioning map[*symbols.Symbol]bool) bool {
	unions := m.UnioningTypes(a)
	if len(unions) == 0 || unioning[a] {
		return false
	}
	if unioning == nil {
		unioning = make(map[*symbols.Symbol]bool)
	}
	unioning[a] = true
	defer delete(unioning, a)
	for _, u := range unions {
		if !m.conforms(u, b, unioning) {
			return false
		}
	}
	return true
}

// FeatureTypes returns a feature's effective types: those it declares, else those
// of the features it redefines or subsets (KerML §8.3.3.3), else its kind's base.
func (m *Model) FeatureTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || !sym.IsFeature() {
		return nil
	}
	if types := m.featureTypes(sym, make(map[*symbols.Symbol]bool)); len(types) > 0 {
		return types
	}
	return m.implicitBases(sym)
}

// FeatureTypeSet returns a feature's types as KerML §8.3.3.3 derives Feature::type:
// the declared types of the feature and of every feature it subsets, redefines or
// references, keeping only the most specific; a feature reaching none has the
// types of its kind's base feature, or that base itself when it is a definition.
func (m *Model) FeatureTypeSet(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || !sym.IsFeature() {
		return nil
	}
	var types []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	var visit func(f *symbols.Symbol)
	visit = func(f *symbols.Symbol) {
		if f == nil || seen[f] {
			return
		}
		seen[f] = true
		bases := m.implicitBases(f)
		for _, super := range m.DirectSupertypes(f) {
			switch {
			case containsElement(bases, super):
			case super.IsFeature():
				visit(super)
			case !containsElement(types, super):
				types = append(types, super)
			}
		}
		visit(m.ReferencedFeature(f))
	}
	visit(sym)
	if len(types) > 0 {
		return m.mostSpecificTypes(types)
	}
	fqn, ok := m.FeatureBaseFQN(sym)
	if !ok || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(fqn) {
		if base == nil || base == sym {
			continue
		}
		if base.IsFeature() {
			return m.FeatureTypeSet(base)
		}
		return []*symbols.Symbol{base}
	}
	return nil
}

// mostSpecificTypes drops every type another one in the list specializes.
func (m *Model) mostSpecificTypes(types []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, t := range types {
		redundant := false
		for _, other := range types {
			if !symbols.SameElement(other, t) && m.Conforms(other, t) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, t)
		}
	}
	return out
}

func containsElement(list []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, s := range list {
		if symbols.SameElement(s, sym) {
			return true
		}
	}
	return false
}

// DeclaredFeatureTypes is FeatureTypes without the kind's base: the types a
// feature is written with, directly or through the features it specializes.
func (m *Model) DeclaredFeatureTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || !sym.IsFeature() {
		return nil
	}
	return m.featureTypes(sym, make(map[*symbols.Symbol]bool))
}

func (m *Model) featureTypes(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) []*symbols.Symbol {
	if visiting[sym] {
		return nil
	}
	visiting[sym] = true
	bases := m.implicitBases(sym)
	var types, features []*symbols.Symbol
	for _, super := range m.DirectSupertypes(sym) {
		switch {
		case slices.Contains(bases, super):
		case super.IsFeature():
			features = append(features, super)
		default:
			types = append(types, super)
		}
	}
	if len(types) > 0 {
		return types
	}
	seen := make(map[*symbols.Symbol]bool)
	for _, feature := range features {
		for _, t := range m.featureTypes(feature, visiting) {
			if !seen[t] {
				seen[t] = true
				types = append(types, t)
			}
		}
	}
	return types
}

// composedKey names one kind of type composition of one type, the key its
// resolved operands are memoized under.
type composedKey struct {
	sym  *symbols.Symbol
	kind ast.RelationshipKind
}

// UnioningTypes returns the resolved targets of sym's `unions` relationships:
// the types sym is declared to be the union of (KerML 1.0 §8.3.3). Unioning is
// not a generalization edge — a union is constrained by its members rather than
// inheriting from them — so it is resolved on its own. The result is memoized.
func (m *Model) UnioningTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.composedOperands(sym, ast.RelUnions)
}

// IntersectingTypes returns the targets of sym's `intersects` relationships: the
// types whose common values are sym's (KerML 1.0 §8.3.3), memoized.
func (m *Model) IntersectingTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.composedOperands(sym, ast.RelIntersects)
}

// DifferencingTypes returns the targets of sym's `differences` relationships: the
// values of the first that are none of the rest are sym's (KerML 1.0 §8.3.3).
func (m *Model) DifferencingTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.composedOperands(sym, ast.RelDifferences)
}

// composedOperands resolves the targets of sym's relationships of one composition
// kind, in declaration order and without repetition. The result is memoized.
func (m *Model) composedOperands(
	sym *symbols.Symbol, kind ast.RelationshipKind,
) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	key := composedKey{sym: sym, kind: kind}
	if cached, ok := m.composed[key]; ok {
		return cached
	}
	m.composed[key] = nil

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || rel.Kind != kind {
			continue
		}
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		target, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || target == nil {
			continue
		}
		if resolved, aliasOK := m.resolver.ResolveAliasTarget(target); aliasOK {
			target = resolved
		} else {
			continue
		}
		if target == sym || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}

	m.composed[key] = out
	return out
}

// IsCollection reports whether sym is a Kernel Data Type Library collection shape, which the
// runtime reads as its elements; a type elsewhere specializing one (a quantity value) is its own kind.
func IsCollection(sym *symbols.Symbol) bool {
	return sym != nil && strings.HasPrefix(symbols.FQNOf(sym), "Collections::")
}

// IsElementType reports whether sym is KerML::Root::Element, the metaclass every model
// element is an instance of, so a parameter it types takes the element any argument names.
func IsElementType(sym *symbols.Symbol) bool {
	return sym != nil && symbols.FQNOf(sym) == "KerML::Root::Element"
}

// IsAnything reports whether sym is Base::Anything, the classifier every type
// specializes (KerML 8.3.2.1), whether or not the chain to it is declared.
func IsAnything(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Name == "Base::Anything" ||
		(sym.Name == "Anything" && sym.OwnerScope != nil && sym.OwnerScope.Owner() != nil &&
			sym.OwnerScope.Owner().Name == "Base"))
}

// HasSpecializationCycle reports whether sym participates in a specialization
// cycle: sym is reachable from itself through one or more generalization edges
// (including a direct self-specialization). AllSupertypes excludes its own
// starting node, so sym is detected via a back-edge from one of its supertypes.
func (m *Model) HasSpecializationCycle(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	for _, s := range m.DirectSupertypes(sym) {
		if s == sym {
			return true // direct self-specialization
		}
		for _, up := range m.AllSupertypes(s) {
			if up == sym {
				return true
			}
		}
	}
	return false
}

// hasSendStatement checks if an action usage contains a send statement in its body
func hasSendStatement(usage *ast.Usage) bool {
	for _, member := range usage.Members {
		if _, ok := member.(*ast.SendStatement); ok {
			return true
		}
	}
	return false
}

// hasAcceptPayload reports whether an action usage declares an accept payload
// parameter, which is what makes it an AcceptActionUsage (SysML v2 §8.3.17).
func hasAcceptPayload(usage *ast.Usage) bool {
	for _, member := range usage.Members {
		if payload, ok := unwrapUsage(member); ok && payload.IsAccept {
			return true
		}
	}
	return false
}
