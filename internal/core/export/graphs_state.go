package export

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// StateForm is one lowered StateGraph. Vertices — the machine, its states and
// pseudostates, the graph-only states the lowering synthesizes for orthogonal
// regions — are numbered by their position in Vertices; regions by theirs in
// Regions. Every other field refers to them by those numbers.
type StateForm struct {
	// Name is the qualified name of the state machine; Kind is its symbol kind.
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Error is the lowering's refusal of an exhibited machine, with no graph beside it.
	Error string `json:"error,omitempty"`
	// Scope is the scope the machine's body resolves names in.
	Scope      string          `json:"scope,omitempty"`
	Attributes []AttributeForm `json:"attributes,omitempty"`
	// Machine is the vertex standing for the machine itself; Initial the state
	// it starts in, absent when the entry transitions choose it.
	Machine  *int              `json:"machine,omitempty"`
	Initial  *int              `json:"initial,omitempty"`
	Vertices []StateVertexForm `json:"vertices"`
	Regions  []RegionForm      `json:"regions,omitempty"`
	// Transitions are by source vertex, then declaration order.
	Transitions []TransitionForm `json:"transitions,omitempty"`
	// EntryTransitions are the transitions out of an entry action, by the body
	// they are written in: a state, a region, or the machine's own body.
	EntryTransitions []EntryTransitionForm `json:"entryTransitions,omitempty"`
	Connections      []ConnectionForm      `json:"connections,omitempty"`
}

// StateVertexForm is a state, a pseudostate or the machine, with everything the
// lowering attached to it.
type StateVertexForm struct {
	ID   int      `json:"id"`
	Kind string   `json:"kind"`
	Name string   `json:"name,omitempty"`
	Span SpanForm `json:"span"`
	// Scope is the scope the state's own members resolve in.
	Scope string `json:"scope,omitempty"`
	// Hidden marks a graph-only state the lowering synthesized; HiddenRegion is
	// the region it stands for.
	Hidden       bool `json:"hidden,omitempty"`
	HiddenRegion *int `json:"hiddenRegion,omitempty"`
	// Parent is the composite state a substate is in; Region the region it is in.
	Parent *int `json:"parent,omitempty"`
	Region *int `json:"region,omitempty"`
	// Owner is the state a pseudostate belongs to; absent for the machine's own.
	Owner *int `json:"owner,omitempty"`
	// Regions are a composite state's regions, in order.
	Regions    []int           `json:"regions,omitempty"`
	Attributes []AttributeForm `json:"attributes,omitempty"`
	Entry      []BehaviorForm  `json:"entry,omitempty"`
	Do         []BehaviorForm  `json:"do,omitempty"`
	Exit       []BehaviorForm  `json:"exit,omitempty"`
	// Deferred are the triggers the state defers while active.
	Deferred []TriggerForm `json:"deferred,omitempty"`
}

// RegionForm is a region of a composite state.
type RegionForm struct {
	ID   int      `json:"id"`
	Name string   `json:"name,omitempty"`
	Span SpanForm `json:"span"`
	// Owner is the composite state the region is in; Top marks a region of the
	// machine's own body.
	Owner *int `json:"owner,omitempty"`
	Top   bool `json:"top,omitempty"`
	// State is the graph-only state standing for the region, holding the
	// attributes its body declares.
	State   *int `json:"state,omitempty"`
	Initial *int `json:"initial,omitempty"`
}

// TransitionForm is a transition; a nil Trigger is a completion transition.
type TransitionForm struct {
	Name    string         `json:"name,omitempty"`
	Source  int            `json:"source"`
	Target  int            `json:"target"`
	Trigger *TriggerForm   `json:"trigger,omitempty"`
	Guard   *ExprForm      `json:"guard,omitempty"`
	Effect  []BehaviorForm `json:"effect,omitempty"`
	Via     string         `json:"via,omitempty"`
	Scope   string         `json:"scope,omitempty"`
	// BodyScope is the scope the guard and effect resolve in, when it differs
	// from Scope: a call trigger's parameters are visible there alone.
	BodyScope string   `json:"bodyScope,omitempty"`
	Decl      SpanForm `json:"decl"`
}

// EntryTransitionForm is a transition out of the entry action of Body.
type EntryTransitionForm struct {
	// Body is the state or region the transition is written in; absent for the
	// machine's own body.
	Body   *int      `json:"body,omitempty"`
	Guard  *ExprForm `json:"guard,omitempty"`
	Target int       `json:"target"`
	Scope  string    `json:"scope,omitempty"`
	Decl   SpanForm  `json:"decl"`
}

// BehaviorForm is an entry, do, exit or effect behavior.
type BehaviorForm struct {
	Name  string   `json:"name,omitempty"`
	Span  SpanForm `json:"span"`
	Scope string   `json:"scope,omitempty"`
	// Owner is the state whose attributes the behavior reads and writes.
	Owner *int            `json:"owner,omitempty"`
	Body  []StatementForm `json:"body"`
	// Nodes are the action nodes the body's blocks declare, each a
	// subperformance of the behavior's, by name.
	Nodes []string `json:"nodes,omitempty"`
}

// stateForm writes a lowered state graph.
func (x *graphsExporter) stateForm(sym *symbols.Symbol, graph *lower.StateGraph) (*StateForm, error) {
	ids := newVertexIDs()
	regions := newVertexIDs()
	ids.add(graph.Machine)
	ids.add(graph.Initial)
	for _, s := range graph.States {
		ids.add(s)
	}
	for _, p := range graph.Pseudostates {
		ids.add(p)
	}
	for _, r := range graph.TopRegions {
		regions.add(r)
	}
	for _, s := range graph.CompositeStateOrder {
		ids.add(s)
		for _, r := range graph.CompositeStates[s] {
			regions.add(r)
		}
	}
	var restNodes, restRegions []ast.Node
	for node := range graph.Transitions {
		restNodes = append(restNodes, node)
	}
	for node := range graph.EntryTransitions {
		if !nilNode(node) {
			if _, ok := node.(*ast.StateRegion); ok {
				restRegions = append(restRegions, node)
			} else {
				restNodes = append(restNodes, node)
			}
		}
	}
	for s := range graph.StateScopes {
		restNodes = append(restNodes, s)
	}
	for s := range graph.Behaviors {
		restNodes = append(restNodes, s)
	}
	for s := range graph.HiddenStates {
		restNodes = append(restNodes, s)
	}
	for s := range graph.ParentState {
		restNodes = append(restNodes, s)
	}
	for s := range graph.Deferred {
		restNodes = append(restNodes, s)
	}
	for s := range graph.StateAttributes {
		restNodes = append(restNodes, s)
	}
	for p := range graph.PseudostateOwner {
		restNodes = append(restNodes, p)
	}
	for r := range graph.RegionState {
		restRegions = append(restRegions, r)
	}
	for r := range graph.RegionInitials {
		restRegions = append(restRegions, r)
	}
	for r := range graph.RegionOwner {
		restRegions = append(restRegions, r)
	}
	for s, r := range graph.RegionOf {
		restNodes = append(restNodes, s)
		restRegions = append(restRegions, r)
	}
	for s, r := range graph.HiddenRegionOf {
		restNodes = append(restNodes, s)
		restRegions = append(restRegions, r)
	}
	owner := func(node ast.Node) string {
		switch n := node.(type) {
		case *ast.StateNode:
			return vertexName(graph.ParentState[n])
		case *ast.PseudostateNode:
			return vertexName(graph.PseudostateOwner[n])
		case *ast.StateRegion:
			return vertexName(graph.RegionOwner[n])
		}
		return ""
	}
	if err := ids.addSorted(restNodes, owner); err != nil {
		return nil, fmt.Errorf("%s: %w", symbols.FQNOf(sym), err)
	}
	if err := regions.addSorted(restRegions, owner); err != nil {
		return nil, fmt.Errorf("%s: %w", symbols.FQNOf(sym), err)
	}
	// Vertices reached only as a target or an owner are numbered after the
	// declared ones, in the order the declared ones reach them.
	for i := 0; i < len(ids.order); i++ {
		node := ids.order[i]
		for _, t := range graph.Transitions[node] {
			ids.add(t.Source)
			ids.add(t.Target)
			for _, b := range t.Effect {
				ids.add(b.Owner)
			}
		}
		if s, ok := node.(*ast.StateNode); ok {
			if b := graph.Behaviors[s]; b != nil {
				for _, list := range [][]lower.StateBehavior{b.Entry, b.Do, b.Exit} {
					for _, behavior := range list {
						ids.add(behavior.Owner)
					}
				}
			}
		}
		for _, et := range graph.EntryTransitions[node] {
			ids.add(et.Target)
		}
	}
	for _, et := range graph.EntryTransitions[nil] {
		ids.add(et.Target)
	}
	for _, r := range regions.order {
		for _, et := range graph.EntryTransitions[r] {
			ids.add(et.Target)
		}
	}
	vertices := len(ids.order)

	form := &StateForm{
		Name:    symbols.FQNOf(sym),
		Kind:    sym.Kind.String(),
		Scope:   scopeName(graph.Scope),
		Machine: ids.ref(graph.Machine),
		Initial: ids.ref(graph.Initial),
	}
	for _, attr := range graph.Attributes {
		form.Attributes = append(form.Attributes, x.attribute(graph.Scope, attr))
	}
	for id, node := range ids.order {
		v := StateVertexForm{ID: id, Kind: vertexKind(node), Name: vertexName(node), Span: x.span(graph.Scope, node)}
		scope := graph.Scope
		switch n := node.(type) {
		case *ast.StateNode:
			if s := graph.StateScopes[n]; s != nil {
				scope = s
			}
			v.Scope = scopeName(graph.StateScopes[n])
			v.Hidden = graph.HiddenStates[n]
			v.HiddenRegion = regions.ref(graph.HiddenRegionOf[n])
			v.Parent = ids.ref(graph.ParentState[n])
			v.Region = regions.ref(graph.RegionOf[n])
			for _, r := range graph.CompositeStates[n] {
				v.Regions = append(v.Regions, regions.add(r))
			}
			for _, attr := range graph.StateAttributes[n] {
				v.Attributes = append(v.Attributes, x.attribute(scope, attr))
			}
			if b := graph.Behaviors[n]; b != nil {
				var err error
				if v.Entry, err = x.behaviors(ids, scope, b.Entry); err != nil {
					return nil, err
				}
				if v.Do, err = x.behaviors(ids, scope, b.Do); err != nil {
					return nil, err
				}
				if v.Exit, err = x.behaviors(ids, scope, b.Exit); err != nil {
					return nil, err
				}
			}
			for _, trigger := range graph.Deferred[n] {
				if t := x.trigger(scope, trigger); t != nil {
					v.Deferred = append(v.Deferred, *t)
				}
			}
		case *ast.PseudostateNode:
			v.Owner = ids.ref(graph.PseudostateOwner[n])
		}
		form.Vertices = append(form.Vertices, v)
		for _, t := range graph.Transitions[node] {
			tf, err := x.transition(ids, scope, t)
			if err != nil {
				return nil, err
			}
			form.Transitions = append(form.Transitions, tf)
		}
	}
	for id, node := range regions.order {
		r, ok := node.(*ast.StateRegion)
		if !ok {
			return nil, fmt.Errorf("%w: %s: %s is not a region", ErrGraphsOrder, symbols.FQNOf(sym), vertexKind(node))
		}
		rf := RegionForm{
			ID:      id,
			Name:    r.Name,
			Span:    x.span(graph.Scope, r),
			Owner:   ids.ref(graph.RegionOwner[r]),
			State:   ids.ref(graph.RegionState[r]),
			Initial: ids.ref(graph.RegionInitials[r]),
		}
		for _, top := range graph.TopRegions {
			if top == r {
				rf.Top = true
			}
		}
		form.Regions = append(form.Regions, rf)
	}
	x.entryTransitions(form, graph, ids, regions)
	if len(ids.order) != vertices {
		return nil, fmt.Errorf("%w: %s: a vertex is reached only while it is written", ErrGraphsOrder, symbols.FQNOf(sym))
	}
	form.Connections = connections(graph.Connections)
	return form, nil
}

// entryTransitions writes the entry transitions body by body: the machine's own
// first, then those of each state vertex, then those of each region.
func (x *graphsExporter) entryTransitions(form *StateForm, graph *lower.StateGraph, ids, regions *vertexIDs) {
	write := func(body *int, list []*lower.EntryTransition) {
		for _, et := range list {
			scope := orScope(et.Scope, graph.Scope)
			form.EntryTransitions = append(form.EntryTransitions, EntryTransitionForm{
				Body:   body,
				Guard:  x.expr(scope, et.Guard),
				Target: ids.add(et.Target),
				Scope:  scopeName(et.Scope),
				Decl:   x.span(scope, et.Decl),
			})
		}
	}
	write(nil, graph.EntryTransitions[nil])
	for id, node := range ids.order {
		if list, ok := graph.EntryTransitions[node]; ok {
			body := id
			write(&body, list)
		}
	}
	for id, node := range regions.order {
		if list, ok := graph.EntryTransitions[node]; ok {
			body := id
			write(&body, list)
		}
	}
}

// behaviors writes the lowered behaviors of one kind in order.
func (x *graphsExporter) behaviors(ids *vertexIDs, enclosing *symbols.Scope, list []lower.StateBehavior) ([]BehaviorForm, error) {
	var out []BehaviorForm
	for _, b := range list {
		scope := orScope(b.Scope, enclosing)
		body, err := x.statements(scope, b.Body)
		if err != nil {
			return nil, err
		}
		if body == nil {
			body = []StatementForm{}
		}
		form := BehaviorForm{
			Name:  b.Name,
			Span:  x.span(scope, b.Node),
			Scope: scopeName(b.Scope),
			Owner: ids.ref(b.Owner),
			Body:  body,
			Nodes: vertexNames(b.Nodes),
		}
		out = append(out, form)
	}
	return out, nil
}

// transition writes one transition with its trigger, guard and effect.
func (x *graphsExporter) transition(ids *vertexIDs, enclosing *symbols.Scope, t *lower.Transition) (TransitionForm, error) {
	scope := orScope(t.Scope, enclosing)
	bodyScope := orScope(t.BodyScope, scope)
	effect, err := x.behaviors(ids, bodyScope, t.Effect)
	if err != nil {
		return TransitionForm{}, err
	}
	form := TransitionForm{
		Name:    t.Name,
		Source:  ids.add(t.Source),
		Target:  ids.add(t.Target),
		Trigger: x.trigger(scope, t.Trigger),
		Guard:   x.expr(bodyScope, t.Guard),
		Effect:  effect,
		Via:     t.Via,
		Scope:   scopeName(t.Scope),
		Decl:    x.span(scope, t.Decl),
	}
	if t.BodyScope != nil && t.BodyScope != t.Scope {
		form.BodyScope = scopeName(t.BodyScope)
	}
	return form, nil
}
