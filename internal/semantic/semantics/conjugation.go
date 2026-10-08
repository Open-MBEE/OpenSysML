package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Port conjugation (SysML v2 §7.12.2, §7.12.3): `port p : ~P` is a usage of the
// conjugated definition of P, which has P's features with in/out reversed.

// PortFeature is a feature of a port, with the direction it has as seen through
// the port that was queried.
type PortFeature struct {
	Symbol    *symbols.Symbol
	Name      string
	Direction ast.FeatureDirection
}

// conjugatedType is a type a port takes features from, with the parity of the
// conjugations reaching it.
type conjugatedType struct {
	sym        *symbols.Symbol
	conjugated bool
}

// superEdge is one generalization edge, with whether it conjugates its target
// and whether it is the feature typing that states the declaration's type.
type superEdge struct {
	sym        *symbols.Symbol
	conjugated bool
	typing     bool
}

// ConjugateDirection returns the conjugate of a feature direction (§7.12.2):
// in and out are each other's conjugate, inout and none are their own.
func ConjugateDirection(d ast.FeatureDirection) ast.FeatureDirection {
	switch d {
	case ast.DirIn:
		return ast.DirOut
	case ast.DirOut:
		return ast.DirIn
	default:
		return d
	}
}

// IsConjugated reports whether sym's inherited features have reversed
// directions. Conjugation composes, so `~` of a conjugate is the original.
func (m *Model) IsConjugated(sym *symbols.Symbol) bool {
	parity := false
	visited := make(map[*symbols.Symbol]bool)
	for cur := sym; cur != nil && !visited[cur]; {
		visited[cur] = true
		edge, ok := m.typeEdge(cur)
		if !ok {
			break
		}
		parity = parity != edge.conjugated
		cur = edge.sym
	}
	return parity
}

// typeEdge returns the edge that gives sym its port type: the feature typing
// (`~` types a port, so a redefinition declared before it must not hide it),
// else the first generalization.
func (m *Model) typeEdge(sym *symbols.Symbol) (superEdge, bool) {
	edges := m.superEdges(sym)
	for _, edge := range edges {
		if edge.typing {
			return edge, true
		}
	}
	if len(edges) == 0 {
		return superEdge{}, false
	}
	return edges[0], true
}

// featureEdges returns sym's generalization edges with the one giving it its
// type first, so a feature of that type masks a same-named inherited one.
func (m *Model) featureEdges(sym *symbols.Symbol) []superEdge {
	edges := m.superEdges(sym)
	typed, ok := m.typeEdge(sym)
	if !ok || !typed.typing {
		return edges
	}
	out := make([]superEdge, 0, len(edges))
	out = append(out, typed)
	for _, edge := range edges {
		if edge.sym != typed.sym {
			out = append(out, edge)
		}
	}
	return out
}

// superEdges returns sym's generalization edges in declaration order, each with
// whether the relationship conjugates its target.
func (m *Model) superEdges(sym *symbols.Symbol) []superEdge {
	if sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.superEdgeCache[sym]; ok {
		return cached
	}
	journal(m, m.superEdgeCache, sym, sym.Decl)
	m.superEdgeCache[sym] = nil

	seen := make(map[*symbols.Symbol]bool)
	out := m.declaredSuperEdges(sym, seen)
	if sym.Recorded() {
		for _, rel := range sym.Facts.Relationships {
			if !GeneralizationKind(rel.Kind) {
				continue
			}
			resolved := m.recordedElement(rel.Target)
			if resolved == nil || resolved == sym || seen[resolved] {
				continue
			}
			seen[resolved] = true
			out = append(out, superEdge{sym: resolved, conjugated: rel.Conjugated, typing: rel.Kind == ast.RelTyping})
		}
	}
	// Supertypes known beyond declared relationships never conjugate.
	for _, sup := range m.DirectSupertypes(sym) {
		if !seen[sup] {
			seen[sup] = true
			out = append(out, superEdge{sym: sup})
		}
	}
	m.superEdgeCache[sym] = out
	return out
}

// declaredSuperEdges resolves the generalizations sym's declaration states,
// each target once, through its alias.
func (m *Model) declaredSuperEdges(sym *symbols.Symbol, seen map[*symbols.Symbol]bool) []superEdge {
	var out []superEdge
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, isQN := target.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		resolved, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok || resolved == nil || resolved == sym || seen[resolved] {
			continue
		}
		canonical, aliasOK := m.resolver.ResolveAliasTarget(resolved)
		if !aliasOK || canonical == sym || seen[canonical] {
			continue
		}
		seen[canonical] = true
		out = append(out, superEdge{sym: canonical, conjugated: rel.Conjugated, typing: rel.Kind == ast.RelTyping})
	}
	return out
}

// conjugatedSupertypes returns the types sym takes features from — sym first,
// then its supertypes breadth-first — with the conjugation parity of each.
func (m *Model) conjugatedSupertypes(sym *symbols.Symbol) []conjugatedType {
	if sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.conjSupers[sym]; ok {
		return cached
	}
	journal(m, m.conjSupers, sym, sym.Decl)
	m.conjSupers[sym] = nil

	out := []conjugatedType{{sym: sym}}
	visited := map[*symbols.Symbol]bool{sym: true}
	for i := 0; i < len(out); i++ {
		cur := out[i]
		for _, edge := range m.featureEdges(cur.sym) {
			if visited[edge.sym] {
				continue
			}
			visited[edge.sym] = true
			out = append(out, conjugatedType{
				sym:        edge.sym,
				conjugated: edge.conjugated != cur.conjugated,
			})
		}
	}
	m.conjSupers[sym] = out
	return out
}

// PortFeatures returns the features of the port sym, declared and inherited,
// with the direction each has as seen through sym. A closer declaration masks
// an inherited feature of the same name.
func (m *Model) PortFeatures(sym *symbols.Symbol) []PortFeature {
	if sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.portFeatures[sym]; ok {
		return cached
	}
	var out []PortFeature
	seenName := make(map[string]bool)
	for _, typ := range m.conjugatedSupertypes(sym) {
		if typ.sym == nil || typ.sym.Scope == nil {
			continue
		}
		names := make(map[string]bool)
		for _, feature := range ownedPortFeatures(typ.sym) {
			if feature.Name != "" && seenName[feature.Name] {
				continue
			}
			if typ.conjugated {
				feature.Direction = ConjugateDirection(feature.Direction)
			}
			out = append(out, feature)
			if feature.Name != "" {
				names[feature.Name] = true
			}
		}
		for name := range names {
			seenName[name] = true
		}
	}
	journal(m, m.portFeatures, sym, sym.Decl)
	m.portFeatures[sym] = out
	return out
}

// ownedPortFeatures returns the usages the port sym declares, with their
// declared directions, read from its record when its document is recorded.
func ownedPortFeatures(sym *symbols.Symbol) []PortFeature {
	var out []PortFeature
	if sym.Recorded() {
		seen := make(map[*symbols.Symbol]bool)
		sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
			if !seen[member] && member.DeclaresUsage() {
				seen[member] = true
				out = append(out, PortFeature{Symbol: member, Name: member.Name, Direction: member.Facts.Direction})
			}
			return true
		})
		return out
	}
	for _, member := range declMembers(sym) {
		usage, ok := unwrapUsage(member)
		if !ok {
			continue
		}
		out = append(out, PortFeature{Symbol: memberSymbol(sym.Scope, usage), Name: usage.Ident.Name, Direction: usage.Direction})
	}
	return out
}

// PortsConform reports whether every feature of port a matches one on port b
// (§7.12.2): conforming types, and conjugate or absent directions.
func (m *Model) PortsConform(a, b *symbols.Symbol) bool {
	if a == nil || b == nil {
		return false
	}
	return m.featuresMatchConjugate(m.PortFeatures(a), m.PortFeatures(b), nil)
}

// featuresMatchConjugate reports whether every named directed feature in
// features has a counterpart in others with a conforming type and the conjugate
// direction. Undirected features carry no flow, so they impose nothing.
func (m *Model) featuresMatchConjugate(features, others []PortFeature, paired map[string]bool) bool {
	for _, feature := range features {
		if feature.Name == "" {
			continue // an unnamed feature has nothing to be matched by name
		}
		if feature.Direction == ast.DirNone {
			continue // conjugation constrains directed features only (§7.12.2)
		}
		if paired[feature.Name] {
			continue // an interface flow already pairs it, whatever the other end calls it
		}
		match, ok := findPortFeature(others, feature.Name)
		if !ok {
			return false
		}
		if match.Direction != ConjugateDirection(feature.Direction) {
			return false
		}
		if !m.featureTypesConform(feature.Symbol, match.Symbol) {
			return false
		}
	}
	return true
}

// InterfaceEndPortMismatch returns the port types of the two ends of the
// interface sym when they do not match with conjugate directions (§7.12.2).
// Ends whose port type is not resolvable, and interfaces whose ends are not
// both declared, are not reported.
func (m *Model) InterfaceEndPortMismatch(sym *symbols.Symbol) (a, b *symbols.Symbol, mismatch bool) {
	if !interfaceLike(sym) {
		return nil, nil, false
	}
	ends := m.endsOf(sym)
	if len(ends) != 2 {
		return nil, nil, false
	}
	first, firstFeatures, ok := m.endPortFeatures(ends[0])
	if !ok {
		return nil, nil, false
	}
	second, secondFeatures, ok := m.endPortFeatures(ends[1])
	if !ok {
		return nil, nil, false
	}
	paired := m.interfaceFlowPairedFeatures(sym, ends)
	if m.featuresMatchConjugate(firstFeatures, secondFeatures, paired[ends[0]]) {
		return nil, nil, false
	}
	return first, second, true
}

// interfaceFlowPairedFeatures collects, per end, the features the flows of the
// interface sym pair with a compatible feature on the other end (§8.2.2.14):
// the flows it declares and those it inherits, `interface m : Mounting connect
// a to b` having Mounting's. Each flow end is read as the interface end its
// chain starts at and the port feature it reaches, so a flow between the two
// ends pairs them even when both are typed by the same port, and a flow
// within one end pairs nothing.
func (m *Model) interfaceFlowPairedFeatures(sym *symbols.Symbol, ends []*symbols.Symbol) map[*symbols.Symbol]map[string]bool {
	paired := make(map[*symbols.Symbol]map[string]bool)
	if sym == nil || len(ends) != 2 {
		return paired
	}
	features := make(map[*symbols.Symbol][]PortFeature, len(ends))
	for _, end := range ends {
		paired[end] = make(map[string]bool)
		_, features[end], _ = m.endPortFeatures(end)
	}
	for _, flow := range m.interfaceFlows(sym) {
		paths := m.ConnectorEndFeaturePaths(flow)
		if len(paths) != 2 {
			continue
		}
		from, source, fromOK := m.flowEndFeature(ends, features, paths[0])
		to, target, toOK := m.flowEndFeature(ends, features, paths[1])
		if !fromOK || !toOK || from == to ||
			!flowDirectionsConform(source.Direction, target.Direction) ||
			!m.featureTypesConform(source.Symbol, target.Symbol) {
			continue
		}
		paired[from][source.Name] = true
		paired[to][target.Name] = true
	}
	return paired
}

// flowEndFeature finds which of the interface ends a flow end's chain starts at
// (the end itself, or one redefining it, as `connect a to b` ends redefine the
// interface definition's by position) and the feature of its port it reaches.
func (m *Model) flowEndFeature(ends []*symbols.Symbol, features map[*symbols.Symbol][]PortFeature, path []*symbols.Symbol) (*symbols.Symbol, PortFeature, bool) {
	if len(path) < 2 {
		return nil, PortFeature{}, false
	}
	head, tip := path[0], path[len(path)-1]
	for _, end := range ends {
		if end != head && !slices.Contains(m.AllRedefinedFeatures(end), head) {
			continue
		}
		if feature, ok := portFeatureOf(features[end], tip); ok {
			return end, feature, true
		}
	}
	return nil, PortFeature{}, false
}

// interfaceFlows returns the flows the interface sym declares or inherits,
// named or not, in declaration order from sym outward.
func (m *Model) interfaceFlows(sym *symbols.Symbol) []*symbols.Symbol {
	var flows []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	collect := func(member *symbols.Symbol) bool {
		if kind, ok := member.UsageKind(); ok && kind == ast.UsageFlow && !seen[member] {
			seen[member] = true
			flows = append(flows, member)
		}
		return true
	}
	for _, owner := range append([]*symbols.Symbol{sym}, m.AllSupertypes(sym)...) {
		if owner.Scope == nil {
			continue
		}
		for _, member := range ownedMembersOf(owner) {
			collect(member)
		}
		owner.Scope.ForEachAnonymousMember(collect)
	}
	return flows
}

// portFeatureOf finds the port feature that is sym.
func portFeatureOf(features []PortFeature, sym *symbols.Symbol) (PortFeature, bool) {
	for _, feature := range features {
		if sym != nil && feature.Symbol == sym {
			return feature, true
		}
	}
	return PortFeature{}, false
}

// flowDirectionsConform reports whether a flow can leave source and enter target.
func flowDirectionsConform(source, target ast.FeatureDirection) bool {
	sourceOK := source == ast.DirOut || source == ast.DirInOut
	targetOK := target == ast.DirIn || target == ast.DirInOut
	return sourceOK && targetOK
}

// endPortFeatures returns the port definition typing the end feature sym and
// the features of that port as seen through the end, so a `~P` end reports
// reversed directions (§7.12.2).
func (m *Model) endPortFeatures(sym *symbols.Symbol) (*symbols.Symbol, []PortFeature, bool) {
	typ, conjugated := m.portTypeOf(sym)
	if typ == nil {
		return nil, nil, false
	}
	features := m.PortFeatures(typ)
	if !conjugated {
		return typ, features, true
	}
	out := make([]PortFeature, len(features))
	for i, feature := range features {
		feature.Direction = ConjugateDirection(feature.Direction)
		out[i] = feature
	}
	return typ, out, true
}

// interfaceLike reports whether sym declares an interface definition or usage.
func interfaceLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if kind, ok := sym.DefinitionKind(); ok {
		return kind == ast.DefInterface
	}
	kind, ok := sym.UsageKind()
	return ok && kind == ast.UsageInterface
}

// portTypeOf returns the port definition typing sym, by its own typing or else
// by that of the nearest model feature it redefines, and whether that typing
// conjugates it.
func (m *Model) portTypeOf(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if sym == nil {
		return nil, false
	}
	for _, cur := range append([]*symbols.Symbol{sym}, m.redefinedTransitively(sym)...) {
		if m.libraryTier(cur).Library() {
			continue
		}
		typ, conjugated, typed := m.declaredTyping(cur)
		if !typed {
			continue
		}
		if kind, isDef := typ.DefinitionKind(); !isDef || kind != ast.DefPort {
			return nil, false
		}
		return typ, conjugated
	}
	return nil, false
}

// declaredTyping returns the first type sym's declaration or record types it by
// that resolves, and whether that typing conjugates it.
func (m *Model) declaredTyping(sym *symbols.Symbol) (typ *symbols.Symbol, conjugated, ok bool) {
	if sym.Recorded() {
		for _, rel := range sym.Facts.Relationships {
			if rel.Kind != ast.RelTyping {
				continue
			}
			if target := m.recordedElement(rel.Target); target != nil {
				return target, rel.Conjugated, true
			}
		}
		return nil, false, false
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, isRef := target.(*ast.FeatureReference); isRef {
			target = fr.Name
		}
		qn, isQN := target.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		resolved, found := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !found || resolved == nil {
			continue
		}
		canonical, aliasOK := m.resolver.ResolveAliasTarget(resolved)
		if !aliasOK {
			continue
		}
		return canonical, rel.Conjugated, true
	}
	return nil, false, false
}

// findPortFeature returns the feature named name, if any.
func findPortFeature(features []PortFeature, name string) (PortFeature, bool) {
	for _, feature := range features {
		if feature.Name == name {
			return feature, true
		}
	}
	return PortFeature{}, false
}

// featureTypesConform reports whether two matched features' types conform in
// either direction. An unresolved type conforms; it is reported on its own.
func (m *Model) featureTypesConform(a, b *symbols.Symbol) bool {
	typeA := m.featureType(a)
	typeB := m.featureType(b)
	if typeA == nil || typeB == nil {
		return true
	}
	return m.Conforms(typeA, typeB) || m.Conforms(typeB, typeA)
}

// featureType returns the symbol a feature's typing relationship names, or nil.
func (m *Model) featureType(sym *symbols.Symbol) *symbols.Symbol {
	if types := m.DeclaredTypes(sym); len(types) > 0 {
		return types[0]
	}
	return nil
}

// DeclaredTypes are the types sym's own typing relationships name, in
// declaration order, each resolved in the scope that declares sym and followed
// through any alias; a typing that does not resolve to an element is left out.
func (m *Model) DeclaredTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || m.resolver == nil {
		return nil
	}
	if sym.Recorded() {
		return m.RecordedRelationshipTargets(sym, ast.RelTyping)
	}
	var types []*symbols.Symbol
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, isQN := target.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		resolved, ok := m.resolver.ResolveQualified(sym.OwnerScope, qn)
		if !ok {
			continue
		}
		if canonical, aliasOK := m.resolver.ResolveAliasTarget(resolved); aliasOK {
			types = append(types, canonical)
		}
	}
	return types
}

// ConnectedPortsMismatch returns the port definitions typing the end features
// a and b the connector joins when they are incompatible: neither conforms to
// the other, they specialize no common definition of the model's own, and
// neither port's directed features all match the other's with conjugate
// directions and conforming types (§7.12.2). An end not typed by a port
// definition is not reported, nor is a connector typed by an interface whose
// two ends are typed by ports: that interface decides what they pair.
func (m *Model) ConnectedPortsMismatch(connector, a, b *symbols.Symbol) (portA, portB *symbols.Symbol, mismatch bool) {
	portA, featuresA, ok := m.endPortFeatures(a)
	if !ok {
		return nil, nil, false
	}
	portB, featuresB, ok := m.endPortFeatures(b)
	if !ok {
		return nil, nil, false
	}
	if m.Conforms(portA, portB) || m.Conforms(portB, portA) || m.shareModelSupertype(portA, portB) {
		return nil, nil, false
	}
	if m.featuresMatchConjugate(featuresA, featuresB, nil) || m.featuresMatchConjugate(featuresB, featuresA, nil) {
		return nil, nil, false
	}
	if m.typedByPortEnds(connector) {
		return nil, nil, false
	}
	return portA, portB, true
}

// typedByPortEnds reports whether connector is typed by an interface whose two
// ends are both typed by port definitions: InterfaceEndPortMismatch judges that
// pairing, so the interface decides what its usages' ends pair.
func (m *Model) typedByPortEnds(connector *symbols.Symbol) bool {
	for _, typ := range m.DeclaredTypes(connector) {
		if !interfaceLike(typ) {
			continue
		}
		ends := m.endsOf(typ)
		if len(ends) == 2 && m.typedByPort(ends[0]) && m.typedByPort(ends[1]) {
			return true
		}
	}
	return false
}

// typedByPort reports whether end is typed by a port definition, by its own
// typing or else by that of the nearest model end it redefines; a library end
// types every connector end by a library base, which decides nothing.
func (m *Model) typedByPort(end *symbols.Symbol) bool {
	if end == nil {
		return false
	}
	for _, sym := range append([]*symbols.Symbol{end}, m.redefinedTransitively(end)...) {
		if m.libraryTier(sym).Library() {
			continue
		}
		if types := m.DeclaredTypes(sym); len(types) > 0 {
			return slices.ContainsFunc(types, func(typ *symbols.Symbol) bool {
				return typ != nil && typ.Kind == symbols.SymbolPortDef
			})
		}
	}
	return false
}

// shareModelSupertype reports whether a and b both specialize one definition
// the model declares; every port specializes the library's Ports::Port, which
// relates nothing.
func (m *Model) shareModelSupertype(a, b *symbols.Symbol) bool {
	supers := make(map[*symbols.Symbol]bool)
	for _, typ := range m.conjugatedSupertypes(a) {
		if typ.sym != a && !m.libraryTier(typ.sym).Library() {
			supers[typ.sym] = true
		}
	}
	for _, typ := range m.conjugatedSupertypes(b) {
		if typ.sym != b && supers[typ.sym] {
			return true
		}
	}
	return false
}
