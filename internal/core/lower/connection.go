package lower

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ConnectionOwner is what a lowered connection belongs to, which decides whose
// selection governs it when it is a variant's: a behavior activation, or the
// object performing the behavior.
type ConnectionOwner int

const (
	// OwnerBehavior marks a connection declared in the behavior's own body.
	OwnerBehavior ConnectionOwner = iota
	// OwnerObject marks a connection declared in the body of a type an object
	// performing the behavior is of.
	OwnerObject
)

// Connection is a lowered connector: the ends it joins, by the name each end
// resolves to. A `connect a to b` has two ends; a multi-end `connect` has more,
// and every end is reachable from every other.
//
// A connection a `variant interface` declares joins its ends only where that
// variant is the one selected, so it carries the variation it belongs to, its
// own name, and what owns it: routing must ask what that owner bound the
// variation to before delivering through it (SysML v2 §7.20).
//
// An end keeps the whole path it was written as (`sensor.out`, `p.q`), so a
// nested port is joined as itself and not as a same-named port elsewhere; Scope
// is where those paths resolve, which is what routing asks to learn the
// direction an end's flow features carry.
type Connection struct {
	Ends      []string
	Variation string          // variation point the connection is a variant of, empty when it is not
	Variant   string          // name of the variant declaring it, empty when it is not one
	Owner     ConnectionOwner // what declares it, whose selection governs a variant's connection
	Scope     *symbols.Scope  // scope the end paths resolve in, nil when unknown
}

// ToObjectConnections lowers the connectors declared in the body of a type an
// object is of. A `send … via p` of a behavior an object performs routes through
// the object's connections as well as through the behavior's own, and it is that
// object's selection that realizes a variant's connection among them.
func ToObjectConnections(decl ast.Node, scope *symbols.Scope) []Connection {
	var members []ast.Node
	switch n := decl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil
	}
	return lowerConnections(members, OwnerObject, scope)
}

// lowerConnections extracts the connectors declared among members, in
// declaration order. Usages that are connector-shaped but declare fewer than
// two ends join nothing and are dropped: the constraint pass reports them, and
// carrying them would only make routing check the same thing again.
func lowerConnections(members []ast.Node, owner ConnectionOwner, scope *symbols.Scope) []Connection {
	var out []Connection
	for _, member := range members {
		u, isUsage := unwrapMembership(member).(*ast.Usage)
		if !isUsage || !connectorKind(u.Kind) {
			continue
		}
		// A variation point declares no ends of its own: the connections are the
		// ones its variants declare, of which a selection realizes one.
		if u.IsVariation {
			out = append(out, lowerVariantConnections(u, owner, scope)...)
			continue
		}
		if conn, ok := lowerConnection(u, owner, scope); ok {
			out = append(out, conn)
		}
	}
	return out
}

// lowerConnection extracts the ends of one connector usage. A usage joining
// fewer than two ends joins nothing and is dropped.
func lowerConnection(u *ast.Usage, owner ConnectionOwner, scope *symbols.Scope) (Connection, bool) {
	ends := make([]string, 0, len(u.ConnectorEnds))
	for _, end := range u.ConnectorEnds {
		if name := endName(end); name != "" {
			ends = append(ends, name)
		}
	}
	if len(ends) < 2 {
		return Connection{}, false
	}
	return Connection{Ends: ends, Owner: owner, Scope: scope}, true
}

// lowerVariantConnections extracts the connections the variants of a variation
// connector declare, each tagged with the variation and variant it came from so
// routing can honor the selection.
func lowerVariantConnections(variation *ast.Usage, owner ConnectionOwner, scope *symbols.Scope) []Connection {
	name := variation.Ident.Name
	var out []Connection
	for _, member := range variation.Members {
		u, isUsage := unwrapMembership(member).(*ast.Usage)
		if !isUsage || !u.IsVariant || !connectorKind(u.Kind) {
			continue
		}
		conn, ok := lowerConnection(u, owner, scope)
		if !ok {
			continue
		}
		conn.Variation, conn.Variant = name, u.Ident.Name
		out = append(out, conn)
	}
	return out
}

// connectorKind reports whether a usage kind declares connector ends that route
// messages. Flows are excluded: they carry a payload along a declared direction
// rather than joining ends for message passing, and are lowered as data flows.
func connectorKind(k ast.UsageKind) bool {
	switch k {
	case ast.UsageConnection, ast.UsageConnector, ast.UsageInterface, ast.UsageAllocation:
		return true
	}
	return false
}

// endName names the feature an end attaches to, as the whole path it was written
// as: a chain (`sensor.out`) keeps every segment, since the port it names is
// that one and not another named `out`. An end that declares its own name
// (`bead references t.bead`) attaches to what it reference-subsets.
func endName(end *ast.ConnectorEnd) string {
	if end == nil {
		return ""
	}
	return FeaturePath(end.AttachedTarget())
}

// SendTarget renders a send's target and reports whether it is a feature chain
// (`alpha.inPort`) rather than a name in a namespace (`R`, `P::R`).
func SendTarget(node ast.Node) (string, bool) {
	if _, chain := node.(*ast.FeatureChainExpr); chain {
		return FeaturePath(node), true
	}
	if qname := ast.AsQualifiedName(node); qname != nil {
		parts := make([]string, 0, len(qname.Parts))
		for _, part := range qname.Parts {
			if part.Text == "" {
				return "", false
			}
			parts = append(parts, part.Text)
		}
		return strings.Join(parts, "::"), false
	}
	return ast.SimpleName(node), false
}

// FeaturePath renders the feature a node names as a dotted path, so a nested
// port keeps every segment it was written with. It returns "" for a node that
// names no feature.
func FeaturePath(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return ""
	case *ast.FeatureChainExpr:
		base, member := FeaturePath(n.Operand), ast.SimpleName(n.Member)
		if base == "" || member == "" {
			return ""
		}
		return base + "." + member
	}
	if qname := ast.AsQualifiedName(node); qname != nil {
		parts := make([]string, 0, len(qname.Parts))
		for _, part := range qname.Parts {
			if part.Text == "" {
				return ""
			}
			parts = append(parts, part.Text)
		}
		return strings.Join(parts, ".")
	}
	return ast.SimpleName(node)
}
