package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Implicit redefinition of connector ends by position (SysML v2 7.13.2, 7.14.2,
// KerML 7.4.6): an end of a connector, in lexical order, redefines the end at
// the same position of each connector its owner specializes, and so takes that
// end's type.

// connectorLike reports whether sym declares a connector — the only owning
// types whose end features are matched by position, and the only general types
// whose ends are implicitly redefined.
func connectorLike(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefConnection, ast.DefInterface, ast.DefAllocation, ast.DefFlow,
			ast.DefAssoc, ast.DefBinding:
			return true
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation,
			ast.UsageConnector, ast.UsageFlow, ast.UsageSuccession,
			ast.UsageAssoc, ast.UsageInteraction, ast.UsageBinding:
			return true
		}
	}
	return false
}

func (m *Model) isConnectorLike(sym *symbols.Symbol) bool {
	return connectorLike(sym) || sym != nil && sym.Decl == nil && m.IsBinaryConnector(sym)
}

// ownedEnds returns the end features sym owns, one entry per end in
// declaration order: first the ends of its `connect` clause, then the `end`
// features of its body. An end that declares no name of its own — `connect a
// to b` — still occupies its position, as does one whose symbol is not
// registered; both are reported as a nil entry.
func ownedEnds(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	if u, ok := sym.Decl.(*ast.Usage); ok {
		for _, end := range u.ConnectorEnds {
			if end == nil {
				continue
			}
			if _, declares := end.DeclaredName(); !declares {
				out = append(out, nil)
				continue
			}
			out = append(out, memberSymbol(sym.Scope, end))
		}
	}
	for _, member := range declMembers(sym) {
		usage, ok := unwrapUsage(member)
		if !ok || !usage.IsEnd {
			continue
		}
		// An end with no registered symbol still occupies its position.
		out = append(out, memberSymbol(sym.Scope, usage))
	}
	return out
}

// connectorEnd is one effective end of a connector: the end feature, or, for
// an end with no symbol (`connect a to b`), the connector and position declaring it.
type connectorEnd struct {
	feature *symbols.Symbol
	owner   *symbols.Symbol
	index   int
}

// identity keys an end so one declaration inherited along several paths is one end.
func (e connectorEnd) identity() connectorEnd {
	if e.feature != nil {
		return connectorEnd{feature: e.feature}
	}
	return e
}

// endsOf returns the effective ends of the connector sym as features, nil for
// an end that has none, in the order of effectiveEnds.
func (m *Model) endsOf(sym *symbols.Symbol) []*symbols.Symbol {
	return endFeatures(m.effectiveEnds(sym))
}

func endFeatures(ends []connectorEnd) []*symbols.Symbol {
	if len(ends) == 0 {
		return nil
	}
	out := make([]*symbols.Symbol, len(ends))
	for i, end := range ends {
		out[i] = end.feature
	}
	return out
}

// effectiveEnds returns sym's owned ends in declaration order, then the unredefined
// ends of its generals, each once however many paths inherit it. Memoized.
func (m *Model) effectiveEnds(sym *symbols.Symbol) []connectorEnd {
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.ends[sym]; ok {
		return cached
	}
	if sym == nil {
		return nil
	}
	// Guard against re-entrancy on cyclic specialization graphs.
	journal(m, m.ends, sym, sym.Decl)
	m.ends[sym] = nil

	if sym.Decl == nil && m.IsBinaryConnector(sym) {
		var out []connectorEnd
		for i, name := range binaryConnectorEndNames {
			end, ok := m.LookupMember(sym, name)
			if !ok {
				m.ends[sym] = nil
				return nil
			}
			out = append(out, connectorEnd{feature: end, owner: sym, index: i})
		}
		m.ends[sym] = out
		return out
	}

	owned := ownedEnds(sym)
	out := make([]connectorEnd, 0, len(owned))
	for i, end := range owned {
		out = append(out, connectorEnd{feature: end, owner: sym, index: i})
	}

	redefined := m.redefinedByEnds(owned)
	var inherited []connectorEnd
	for _, sup := range m.DirectSupertypes(sym) {
		if !m.isConnectorLike(sup) {
			continue
		}
		for i, end := range m.effectiveEnds(sup) {
			if !endRedefined(end.feature, i, owned, redefined) {
				inherited = append(inherited, end)
			}
		}
	}
	out = append(out, m.unmaskedEnds(inherited)...)

	m.ends[sym] = out
	return out
}

// unmaskedEnds drops ends already listed and ends another listed end redefines,
// so a feature inherited through several generals counts once.
func (m *Model) unmaskedEnds(ends []connectorEnd) []connectorEnd {
	masked := make(map[*symbols.Symbol]bool)
	for _, end := range ends {
		for _, redefined := range m.AllRedefinedFeatures(end.feature) {
			masked[redefined] = true
		}
	}
	seen := make(map[connectorEnd]bool)
	out := make([]connectorEnd, 0, len(ends))
	for _, end := range ends {
		id := end.identity()
		if masked[end.feature] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, end)
	}
	return out
}

// redefinedByEnds returns every feature the owned ends redefine, by clause or by position.
func (m *Model) redefinedByEnds(owned []*symbols.Symbol) map[*symbols.Symbol]bool {
	redefined := make(map[*symbols.Symbol]bool)
	for _, end := range owned {
		for _, target := range m.AllRedefinedFeatures(end) {
			redefined[target] = true
		}
	}
	return redefined
}

// endRedefined reports whether an owned end redefines the general end at position i;
// an end without a symbol on either side is matched by position alone.
func endRedefined(end *symbols.Symbol, i int, owned []*symbols.Symbol, redefined map[*symbols.Symbol]bool) bool {
	if end == nil {
		return i < len(owned)
	}
	if redefined[end] {
		return true
	}
	return i < len(owned) && owned[i] == nil
}

// namedEnds returns the ends of general that end's `:>>` clauses name, matched
// syntactically against each candidate's owner chain (resolving would re-enter DirectSupertypes).
func namedEnds(end *symbols.Symbol, general []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	if end == nil {
		return nil
	}
	for _, rel := range RelationshipsOf(end) {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		for _, candidate := range general {
			if candidate != nil && qualifiedNameNames(qn, candidate) {
				out = append(out, candidate)
			}
		}
	}
	return out
}

// qualifiedNameNames reports whether qn's segments, read from the last, match
// sym's name and then its owners' names; `$` pins the chain to the root.
func qualifiedNameNames(qn *ast.QualifiedName, sym *symbols.Symbol) bool {
	parts := qn.Parts
	if len(parts) == 0 || leafName(sym.Name) != parts[len(parts)-1].Text {
		return false
	}
	scope := sym.OwnerScope
	for i := len(parts) - 2; i >= 0; i-- {
		if parts[i].Text == "$" {
			return i == 0 && (scope == nil || scope.Owner() == nil)
		}
		if scope == nil || scope.Owner() == nil || scope.Owner().Name != parts[i].Text {
			return false
		}
		scope = scope.Owner().OwnerScope
	}
	return true
}

// ImplicitEndRedefinitions returns the ends of the connectors its owner
// specializes that sym redefines by occupying their position, whatever its
// `:>>` clauses name besides. The features it redefines are one feature with it,
// so an object holds one set of values under all their names.
func (m *Model) ImplicitEndRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	return m.implicitEndRedefinitions(sym)
}

// implicitEndRedefinitions returns the features sym implicitly redefines as an
// end of its owning connector: the end at the same position of each general
// connector (KerML 7.4.6, SysML v2 7.13.2). An explicit `:>>` adds to these
// rather than replacing them. It returns nothing for a feature that is not an end.
func (m *Model) implicitEndRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if !declaresEnd(sym) {
		return nil
	}
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if !m.isConnectorLike(owner) {
		return nil
	}
	position := -1
	for i, end := range ownedEnds(owner) {
		if end == sym {
			position = i
			break
		}
	}
	if position < 0 {
		return nil
	}

	var out []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(owner) {
		if !m.isConnectorLike(sup) {
			continue
		}
		supEnds := m.endsOf(sup)
		if position >= len(supEnds) {
			continue
		}
		if target := supEnds[position]; target != nil && target != sym {
			out = append(out, target)
		}
	}
	return out
}

// declaredEndCount counts sym's owned ends plus the unclaimed ends of its declared
// generals; it consults no implicit base, so implicit base selection may use it.
func (m *Model) declaredEndCount(sym *symbols.Symbol) int {
	owned := ownedEnds(sym)
	var inherited []connectorEnd
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		sup := m.relationshipTarget(sym, rel)
		if sup == nil || sup == sym || !m.isConnectorLike(sup) {
			continue
		}
		ends := m.effectiveEnds(sup)
		features := endFeatures(ends)
		for i, end := range ends {
			if !endClaimed(end.feature, i, owned, features) {
				inherited = append(inherited, end)
			}
		}
	}
	return len(owned) + len(m.unmaskedEnds(inherited))
}

// endClaimed reports whether an owned end takes over general end i: by position,
// or by the ends its redefinition clause names.
func endClaimed(end *symbols.Symbol, i int, owned, general []*symbols.Symbol) bool {
	if i < len(owned) {
		return true
	}
	if end == nil {
		return false
	}
	for _, o := range owned {
		if slices.Contains(namedEnds(o, general), end) {
			return true
		}
	}
	return false
}

// declaresEnd reports whether sym declares an end feature: an end of a
// `connect` clause, or a body feature declared with the `end` modifier.
func declaresEnd(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.ConnectorEnd:
		_, declares := d.DeclaredName()
		return declares
	case *ast.Usage:
		return d.IsEnd
	}
	return false
}

// UnmatchedConnectorEnds returns the ends the connector sym declares that
// redefine no end of a general connector, together with the general connector
// that has too few ends. A connector with no connector-like general — an
// untyped `connect a to b` — has none: there is nothing to match against.
func (m *Model) UnmatchedConnectorEnds(sym *symbols.Symbol) (*symbols.Symbol, []*symbols.Symbol) {
	if !m.isConnectorLike(sym) {
		return nil, nil
	}
	owned := ownedEnds(sym)
	if len(owned) == 0 {
		return nil, nil
	}
	for _, sup := range m.DirectSupertypes(sym) {
		if !m.isConnectorLike(sup) {
			continue
		}
		supEnds := m.endsOf(sup)
		if len(supEnds) == 0 {
			// The general's own ends are not enumerable (no parsed body, or they
			// are inherited from an unparsed library), so its arity is unknown.
			continue
		}
		var unmatched []*symbols.Symbol
		for i, end := range owned {
			if end == nil || i < len(supEnds) || len(namedEnds(end, supEnds)) > 0 {
				continue
			}
			unmatched = append(unmatched, end)
		}
		if len(unmatched) > 0 {
			return sup, unmatched
		}
	}
	return nil, nil
}

// ConnectorEndCount returns the number of effective ends of the connector sym.
func (m *Model) ConnectorEndCount(sym *symbols.Symbol) int {
	count := len(m.endsOf(sym))
	if count == 0 && sym != nil && sym.Decl == nil &&
		m.fqnOf(sym) == binaryConnectorBaseFQN {
		return len(binaryConnectorEndNames)
	}
	return count
}

// BinaryConnectorExcessEnds returns the ends past the second of a connector or
// association that conforms to Links::BinaryLink yet has more than two
// effective ends (KerML 1.1 validateAssociationBinarySpecialization,
// validateConnectorBinarySpecialization), with the effective end count. Each
// node is an owned end beyond position two, or the declaration itself when only
// inherited ends take the count past two.
func (m *Model) BinaryConnectorExcessEnds(sym *symbols.Symbol) ([]ast.Node, int) {
	if sym == nil || sym.Decl == nil || !connectorLike(sym) {
		return nil, 0
	}
	total := m.ConnectorEndCount(sym)
	if total <= 2 || !m.IsBinaryConnector(sym) {
		return nil, total
	}
	usage, _ := sym.Decl.(*ast.Usage)
	var excess []ast.Node
	for i, end := range ownedEnds(sym) {
		if i < 2 {
			continue
		}
		switch {
		case end != nil && end.Decl != nil:
			excess = append(excess, end.Decl)
		case clauseEndNode(usage, i) != nil:
			excess = append(excess, clauseEndNode(usage, i))
		}
	}
	if len(excess) == 0 {
		excess = []ast.Node{sym.Decl}
	}
	return excess, total
}

// RelatedFeatureCount returns how many features the ends of the connector sym
// reference (KerML 1.1 Connector::relatedFeature): the ends of its `connect`,
// `from`/`to` and binding clauses, its owned end features with a reference
// clause, and the inherited ends it does not redefine that reference one.
func (m *Model) RelatedFeatureCount(sym *symbols.Symbol) int {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !connectorLike(sym) {
		return 0
	}
	count := ownedRelatedFeatureCount(usage)
	for _, end := range m.effectiveEnds(sym) {
		if end.owner == sym {
			// Clause ends are counted above; only body ends with a reference remain.
			if end.feature == nil {
				continue
			}
			if d, ok := end.feature.Decl.(*ast.Usage); ok && referencesFeature(d.Relationships) {
				count++
			}
		} else if endReferencesFeature(end) {
			count++
		}
	}
	if len(ownedEnds(sym)) == 0 {
		count += m.clauseRelatedFeatureCount(sym, make(map[*symbols.Symbol]bool))
	}
	return count
}

// clauseRelatedFeatureCount counts the related features of generals whose ends occupy
// no position of their own (`flow f from a to b`), inherited whole by an endless sym.
func (m *Model) clauseRelatedFeatureCount(sym *symbols.Symbol, visited map[*symbols.Symbol]bool) int {
	visited[sym] = true
	count := 0
	for _, sup := range m.DirectSupertypes(sym) {
		usage, ok := sup.Decl.(*ast.Usage)
		if !ok || visited[sup] || !connectorLike(sup) || len(ownedEnds(sup)) > 0 {
			continue
		}
		count += ownedRelatedFeatureCount(usage) + m.clauseRelatedFeatureCount(sup, visited)
	}
	return count
}

// ownedRelatedFeatureCount counts the features the clauses of usage attach its
// ends to: `connect a to b`, `from a to b`, `bind a = b`.
func ownedRelatedFeatureCount(usage *ast.Usage) int {
	count := 0
	for _, end := range usage.ConnectorEnds {
		if end.AttachedTarget() != nil {
			count++
		}
	}
	// `succession then b` states only its target; its source is the member before it.
	if usage.Kind == ast.UsageSuccession && len(usage.ConnectorEnds) == 1 && count == 1 {
		count++
	}
	if usage.FlowEnds != nil {
		if usage.FlowEnds.From != nil {
			count++
		}
		if usage.FlowEnds.To != nil {
			count++
		}
	}
	return count
}

// endReferencesFeature reports whether an effective end references a feature, through
// its own reference clause or its owner's `connect`/binding clause when it has no symbol.
func endReferencesFeature(end connectorEnd) bool {
	if end.feature != nil {
		switch d := end.feature.Decl.(type) {
		case *ast.Usage:
			return referencesFeature(d.Relationships)
		case *ast.ConnectorEnd:
			return d.AttachedTarget() != nil
		}
		return false
	}
	usage, ok := end.owner.Decl.(*ast.Usage)
	return ok && clauseEndTarget(usage, end.index) != nil
}

// clauseEndNode returns the connector end the clause states at position i:
// `connect a to b`, `first a then b`, `bind a = b`.
func clauseEndNode(usage *ast.Usage, i int) ast.Node {
	if usage == nil || i < 0 || i >= len(usage.ConnectorEnds) || usage.ConnectorEnds[i] == nil {
		return nil
	}
	return usage.ConnectorEnds[i]
}

// clauseEndTarget returns the feature the clause end at position i attaches to.
func clauseEndTarget(usage *ast.Usage, i int) ast.Node {
	if end, ok := clauseEndNode(usage, i).(*ast.ConnectorEnd); ok {
		return end.AttachedTarget()
	}
	return nil
}

// referencesFeature reports whether rels carry a reference-subsetting clause.
func referencesFeature(rels []*ast.Relationship) bool {
	for _, rel := range rels {
		if rel != nil && rel.Kind == ast.RelReferences && rel.Target != nil {
			return true
		}
	}
	return false
}

// IsConnectorUsage reports whether sym declares a connector usage that joins
// features it names — a `connection`/`interface`/`allocation`/`connector` usage
// with a `connect … to …` clause. Such a usage is materialized from the
// features its ends attach to, unlike a usage that only holds objects of its
// own.
func (m *Model) IsConnectorUsage(sym *symbols.Symbol) bool {
	if sym == nil || !m.isConnectorLike(sym) {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	switch usage.Kind {
	case ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation, ast.UsageConnector:
		return len(usage.ConnectorEnds) > 0
	}
	return false
}

// ConnectorEndAttachment is one end of a connector usage as an object of that
// usage carries it: the name of the end feature the position occupies, and the
// node naming the feature the end attaches to. A connector end
// reference-subsets what it attaches to (KerML 1.0 §7.4.6, SysML v2 §7.13.2),
// so an object of the connector holds that very feature at the end rather than
// an object of the end's declared type.
type ConnectorEndAttachment struct {
	// Name is the effective name of the end feature this position occupies: the
	// name the end declares for itself, else the name of the end it implicitly
	// redefines in the connector it specializes, else the name a binary
	// connector's ends have in the library. It is empty when none of those
	// answers a name — an end of an untyped connector with an arity other than
	// two, whose ends are the participants of a link and are unnamed.
	Name string
	// Attachment is the expression naming the connected feature (`a.p`).
	Attachment ast.Node
	// End is the syntax of the end itself, which carries its source location.
	End *ast.ConnectorEnd
	// EndFeature is the end feature Name comes from, when a declaration in the
	// model declares it: the end's own symbol, or the end it redefines. It is nil
	// for a name taken from the library, whose declarations are indexed without
	// bodies.
	EndFeature *symbols.Symbol
}

// binaryConnectorEndNames are the ends of every binary connector: a connector
// usage with two ends specializes `Connections::BinaryConnection` (SysML v2
// §8.3.13), whose `source` and `target` redefine those of `Links::BinaryLink`,
// and each of the usage's ends implicitly redefines the one at its position. A
// binary `interface`, `allocation` or `flow` reaches the same pair through
// `BinaryInterface`, `Allocation` and `Flow`, which redefine them again.
// An indexed base retains these two effective ends even without its body.
var binaryConnectorEndNames = [2]string{"source", "target"}

const binaryConnectorBaseFQN = "Links::BinaryLink"

// BinaryConnectorBaseFQN is the library base every binary association and
// connector conforms to.
const BinaryConnectorBaseFQN = binaryConnectorBaseFQN

// IsBinaryConnector reports whether sym conforms to the library's binary-link
// base, including through cached/index-only specialization edges.
func (m *Model) IsBinaryConnector(sym *symbols.Symbol) bool {
	return m != nil && m.conformsByName(sym, binaryConnectorBaseFQN)
}

// ConnectorEndAttachments returns the ends of the connector usage sym in
// declaration order, one entry per end of its `connect` clause. It returns
// nothing for a symbol that is no connector usage.
func (m *Model) ConnectorEndAttachments(sym *symbols.Symbol) []ConnectorEndAttachment {
	if !m.IsConnectorUsage(sym) {
		return nil
	}
	usage := sym.Decl.(*ast.Usage)
	owned := ownedEnds(sym)

	out := make([]ConnectorEndAttachment, 0, len(usage.ConnectorEnds))
	for i, end := range usage.ConnectorEnds {
		if end == nil {
			continue
		}
		att := ConnectorEndAttachment{Attachment: end.AttachedTarget(), End: end}
		general := m.generalEndAt(sym, i)
		switch {
		case i < len(owned) && owned[i] != nil:
			att.Name, att.EndFeature = leafName(owned[i].Name), owned[i]
		case general != nil:
			att.Name, att.EndFeature = leafName(general.Name), general
		case len(usage.ConnectorEnds) == 2:
			att.Name = binaryConnectorEndNames[i]
		}
		out = append(out, att)
	}
	return out
}

// FlowEndAttachment is one declared from/to target of a flow usage.
type FlowEndAttachment struct {
	Attachment ast.Node
}

// FlowEndAttachments returns the declared from/to targets of a flow usage.
func (m *Model) FlowEndAttachments(sym *symbols.Symbol) []FlowEndAttachment {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageFlow || usage.Keyword == "message" || usage.FlowEnds == nil {
		return nil
	}
	out := make([]FlowEndAttachment, 0, 2)
	for _, target := range []ast.Node{usage.FlowEnds.From, usage.FlowEnds.To} {
		if target == nil {
			continue
		}
		out = append(out, FlowEndAttachment{Attachment: target})
	}
	return out
}

// generalEndAt returns the end at position i of the first connector sym
// specializes that has one there, which sym's end at that position redefines.
func (m *Model) generalEndAt(sym *symbols.Symbol, i int) *symbols.Symbol {
	for _, sup := range m.DirectSupertypes(sym) {
		if !m.isConnectorLike(sup) {
			continue
		}
		if ends := m.endsOf(sup); i < len(ends) && ends[i] != nil {
			return ends[i]
		}
	}
	return nil
}
