package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Relationship kinds RelatedElements traverses. A lineage kind follows the
// declared relationships of the element itself; the others follow edges other
// declarations state about it (a connector usage, a satisfy/verify assertion,
// a requirement derivation, a refinement dependency).
const (
	relationshipSpecialization = "specialization"
	relationshipSubsetting     = "subsetting"
	relationshipRedefinition   = "redefinition"
	relationshipTyping         = "typing"
	relationshipConnection     = "connection"
	relationshipAllocation     = "allocation"
	relationshipSatisfaction   = "satisfaction"
	relationshipVerification   = "verification"
	relationshipDerivation     = "derivation"
	relationshipRefinement     = "refinement"
)

// Traversal directions: outgoing follows an edge from its source to its
// targets, incoming follows it in reverse.
const (
	directionOutgoing = "outgoing"
	directionIncoming = "incoming"
)

// lineageKinds maps the lineage relationship kinds to the AST relationship
// they follow.
var lineageKinds = map[string]ast.RelationshipKind{
	relationshipSpecialization: ast.RelSpecializes,
	relationshipSubsetting:     ast.RelSubsets,
	relationshipRedefinition:   ast.RelRedefines,
	relationshipTyping:         ast.RelTyping,
}

// relationshipEdges holds the resolved edges of one relationship kind, in
// document order then declaration order, keyed by semantic identity.
type relationshipEdges struct {
	outgoing map[symbols.ElementKey][]*symbols.Symbol
	incoming map[symbols.ElementKey][]*symbols.Symbol
}

// RelationshipTables memoizes one model's per-kind edge tables across the
// executions whose Context shares them. Not safe for concurrent use.
type RelationshipTables struct {
	index   *symbols.Index
	model   *semantics.Model
	entries map[string]*relationshipEdges
}

// NewRelationshipTables returns empty tables for a Context to carry.
func NewRelationshipTables() *RelationshipTables {
	return &RelationshipTables{entries: make(map[string]*relationshipEdges)}
}

// lookup returns the cached tables of one kind, discarding every entry built
// against another index or model first.
func (t *RelationshipTables) lookup(kind string, context Context) (*relationshipEdges, bool) {
	if t.index != context.Index || t.model != context.Model {
		t.index, t.model = context.Index, context.Model
		t.entries = make(map[string]*relationshipEdges)
	}
	edges, ok := t.entries[kind]
	return edges, ok
}

// relationshipWalk is the validated relationshipKind, direction and maxDepth
// arguments of an operation traversing relationships.
type relationshipWalk struct {
	kind      string
	direction string
	maxDepth  depthLimit
}

func (e *executor) evaluateRelated(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	walk, err := e.relationshipArguments(expression)
	if err != nil {
		return sequence{}, err
	}
	seeds := make([]*symbols.Symbol, len(source.values))
	for i, value := range source.values {
		seeds[i], _ = value.Element()
	}
	var result sequence
	err = e.traverseRelated(expression, walk, seeds, func(neighbor *symbols.Symbol) bool {
		result.values = append(result.values, ElementValue(neighbor))
		return true
	})
	if err != nil {
		return sequence{}, err
	}
	return result, nil
}

// relationshipArguments reads and validates the relationshipKind, direction and
// maxDepth arguments, reporting an unsupported kind or direction as a typed error.
func (e *executor) relationshipArguments(expression queryplan.Expression) (relationshipWalk, error) {
	kind, err := e.stringArgument(expression, "relationshipKind")
	if err != nil {
		return relationshipWalk{}, err
	}
	direction, err := e.stringArgument(expression, "direction")
	if err != nil {
		return relationshipWalk{}, err
	}
	maxDepth, err := e.depthArgument(expression)
	if err != nil {
		return relationshipWalk{}, err
	}
	if !supportedRelationship(kind) {
		return relationshipWalk{}, &Error{
			Kind:      ErrorUnknownRelationship,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Actual:    kind,
			Origin:    expression.Origin(),
		}
	}
	if direction != directionOutgoing && direction != directionIncoming {
		return relationshipWalk{}, e.operatorError(expression, direction)
	}
	return relationshipWalk{kind: kind, direction: direction, maxDepth: maxDepth}, nil
}

// traverseRelated walks the relationship breadth-first from the seeds to maxDepth, charging
// the visit budget and calling visit once per newly reached element until it returns false.
func (e *executor) traverseRelated(
	expression queryplan.Expression,
	walk relationshipWalk,
	seeds []*symbols.Symbol,
	visit func(*symbols.Symbol) bool,
) error {
	type pending struct {
		sym   *symbols.Symbol
		depth int64
	}
	queue := make([]pending, 0, len(seeds))
	seen := make(map[symbols.ElementKey]struct{})
	for _, sym := range seeds {
		seen[symbols.KeyOf(sym)] = struct{}{}
		queue = append(queue, pending{sym: sym})
	}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if walk.maxDepth.reached(next.depth) {
			continue
		}
		for _, neighbor := range e.relatedNeighbors(walk.kind, walk.direction, next.sym) {
			key := symbols.KeyOf(neighbor)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			if !e.consumeVisit() {
				return e.budgetError(expression)
			}
			seen[key] = struct{}{}
			if !visit(neighbor) {
				return nil
			}
			queue = append(queue, pending{sym: neighbor, depth: next.depth + 1})
		}
	}
	return nil
}

func supportedRelationship(kind string) bool {
	if _, ok := lineageKinds[kind]; ok {
		return true
	}
	switch kind {
	case relationshipConnection, relationshipAllocation,
		relationshipSatisfaction, relationshipVerification,
		relationshipDerivation, relationshipRefinement:
		return true
	}
	return false
}

// relatedNeighbors returns the elements one edge of the given kind away from
// sym in the given direction, in declaration order. Outgoing lineage reads
// sym's own declared relationships; every other combination reads the edge
// tables built from the workspace's declarations.
func (e *executor) relatedNeighbors(kind, direction string, sym *symbols.Symbol) []*symbols.Symbol {
	if relKind, lineage := lineageKinds[kind]; lineage && direction == directionOutgoing {
		return e.lineageTargets(sym, relKind)
	}
	edges := e.relationshipEdges(kind)
	table := edges.outgoing
	if direction == directionIncoming {
		table = edges.incoming
	}
	return table[symbols.KeyOf(sym)]
}

// lineageTargets resolves the targets of sym's declared relationships of the
// given kind, in declaration order.
func (e *executor) lineageTargets(sym *symbols.Symbol, kind ast.RelationshipKind) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != kind || rel.Target == nil {
			continue
		}
		if target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, rel.Target); ok && target != nil {
			out = append(out, target)
		}
	}
	return out
}

// relationshipEdges returns one kind's edge tables, built on first use by scanning
// the workspace's documents in name order. The scan is memoized and uncharged;
// only the elements a traversal reaches pay the visit budget.
func (e *executor) relationshipEdges(kind string) *relationshipEdges {
	if cached, ok := e.related.lookup(kind, e.context); ok {
		return cached
	}
	edges := &relationshipEdges{
		outgoing: make(map[symbols.ElementKey][]*symbols.Symbol),
		incoming: make(map[symbols.ElementKey][]*symbols.Symbol),
	}
	for _, document := range e.context.Index.WorkspaceDocuments() {
		e.scanScope(edges, kind, e.context.Index.DocumentRoot(document))
	}
	e.related.entries[kind] = edges
	return edges
}

// scanScope records the edges of one relationship kind that the declarations
// in scope and its nested scopes state.
func (e *executor) scanScope(edges *relationshipEdges, kind string, scope *symbols.Scope) {
	if scope == nil {
		return
	}
	for _, member := range scope.AllMembers() {
		e.scanSymbol(edges, kind, member)
	}
	for _, child := range scope.Children() {
		e.scanScope(edges, kind, child)
	}
}

// scanSymbol records the edges the given symbol's declaration states: the
// resolved targets of a lineage relationship, the resolved end features of a
// connector usage, the subject and requirement of a satisfaction assertion,
// the requirements of a derivation, or the ends of a refinement dependency.
func (e *executor) scanSymbol(edges *relationshipEdges, kind string, sym *symbols.Symbol) {
	if relKind, lineage := lineageKinds[kind]; lineage {
		for _, target := range e.lineageTargets(sym, relKind) {
			addEdge(edges, sym, target)
		}
		return
	}
	switch kind {
	case relationshipConnection, relationshipAllocation:
		e.scanConnector(edges, kind, sym)
	case relationshipSatisfaction, relationshipVerification:
		e.scanSatisfaction(edges, kind, sym)
	case relationshipDerivation:
		e.scanDerivation(edges, sym)
	case relationshipRefinement:
		e.scanRefinement(edges, sym)
	}
}

// scanConnector records the edges a connector usage states: from the feature
// its first end attaches to, to the feature of each later end, in declaration
// order.
func (e *executor) scanConnector(edges *relationshipEdges, kind string, sym *symbols.Symbol) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !connectorRelationship(usage.Kind, kind) || !e.context.Model.IsConnectorUsage(sym) {
		return
	}
	var ends []*symbols.Symbol
	for _, attachment := range e.context.Model.ConnectorEndAttachments(sym) {
		if attachment.Attachment == nil {
			continue
		}
		target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, attachment.Attachment)
		if !ok || target == nil {
			continue
		}
		ends = append(ends, target)
	}
	if len(ends) < 2 {
		return
	}
	for _, target := range ends[1:] {
		addEdge(edges, ends[0], target)
	}
}

// connectorRelationship reports whether a connector usage of the given AST
// kind carries edges of the named relationship kind. Connection covers the
// connection, connector, and interface usages; allocation stands alone.
func connectorRelationship(usage ast.UsageKind, kind string) bool {
	switch kind {
	case relationshipConnection:
		return usage == ast.UsageConnection || usage == ast.UsageConnector || usage == ast.UsageInterface
	case relationshipAllocation:
		return usage == ast.UsageAllocation
	}
	return false
}

// scanSatisfaction records the edge a satisfy or verify assertion states: from
// the subject its `by` clause names — else the element stating the assertion —
// to the requirement it references, or to the assertion itself when it
// declares its requirement (`satisfy requirement r by v { ... }`) — and then
// also to the requirement definition the declaration is typed by, which is
// the requirement a v1 model states the satisfaction of.
func (e *executor) scanSatisfaction(edges *relationshipEdges, kind string, sym *symbols.Symbol) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageSatisfy {
		return
	}
	if (usage.Keyword == "verify") != (kind == relationshipVerification) {
		return
	}
	var requirement, definition, subject *symbols.Symbol
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		target, ok := e.context.Resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets:
			// A declaration form's subsettings refine the declared requirement;
			// only the reference form names the requirement this way.
			if !usage.DeclaresRequirement {
				requirement = target
			}
		case ast.RelTyping:
			if usage.DeclaresRequirement && target.Kind == symbols.SymbolRequirementDef {
				definition = target
			}
		case ast.RelSubject:
			subject = target
		}
	}
	if usage.DeclaresRequirement {
		requirement = sym
	}
	if subject == nil && sym.OwnerScope != nil {
		subject = sym.OwnerScope.Owner()
		// A verify lives in the objective of a verification case; the case is
		// the verifier.
		if kind == relationshipVerification && subject != nil && isObjectiveUsage(subject.Decl) && subject.OwnerScope != nil {
			subject = subject.OwnerScope.Owner()
		}
	}
	if subject == nil || requirement == nil {
		return
	}
	addEdge(edges, subject, requirement)
	if definition != nil {
		addEdge(edges, subject, definition)
	}
}

func isObjectiveUsage(decl ast.Node) bool {
	usage, ok := decl.(*ast.Usage)
	return ok && usage.Kind == ast.UsageObjective
}

// addEdge records one source-to-target edge in both directions.
func addEdge(edges *relationshipEdges, source, target *symbols.Symbol) {
	sourceKey, targetKey := symbols.KeyOf(source), symbols.KeyOf(target)
	edges.outgoing[sourceKey] = append(edges.outgoing[sourceKey], target)
	edges.incoming[targetKey] = append(edges.incoming[targetKey], source)
}
