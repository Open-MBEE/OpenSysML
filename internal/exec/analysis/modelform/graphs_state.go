package modelform

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	Body   *int           `json:"body,omitempty"`
	Guard  *ExprForm      `json:"guard,omitempty"`
	Target *int           `json:"target,omitempty"`
	Via    *int           `json:"via,omitempty"`
	Effect []BehaviorForm `json:"effect,omitempty"`
	Scope  string         `json:"scope,omitempty"`
	Decl   SpanForm       `json:"decl"`
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
	outer := x.exprs
	x.exprs = graph.Exprs
	defer func() { x.exprs = outer }()

	ids := newVertexIDs[program.VertexRef]()
	regions := newVertexIDs[program.RegionID]()
	state := func(id program.StateID) program.VertexRef {
		ref := program.StateVertex(id)
		if _, ok := graph.VertexSite(ref); !ok {
			return program.VertexRef{}
		}
		return ref
	}
	ids.add(state(graph.MachineID()))
	ids.add(state(graph.InitialID()))
	for _, id := range graph.StateIDs() {
		ids.add(state(id))
	}
	for _, id := range graph.PseudoIDs() {
		ids.add(program.PseudoVertex(id))
	}
	for _, r := range graph.TopRegionIDs() {
		regions.add(r)
	}
	for _, id := range graph.CompositeIDs() {
		ids.add(state(id))
		for _, r := range graph.RegionsOf(id) {
			regions.add(r)
		}
	}
	// The states, pseudostates and regions only a table of the graph names are
	// numbered after the declared ones, in an order their sites decide.
	var restVertices []program.VertexRef
	for i := 1; i < len(graph.Origins); i++ {
		restVertices = append(restVertices, state(program.StateID(i)))
	}
	for i := 1; i < len(graph.PseudoOrigins); i++ {
		restVertices = append(restVertices, program.PseudoVertex(program.PseudoID(i)))
	}
	var restRegions []program.RegionID
	for i := 1; i < len(graph.RegionOrigins); i++ {
		restRegions = append(restRegions, program.RegionID(i))
	}
	stateName := func(id program.StateID) string {
		site, _ := graph.VertexSite(program.StateVertex(id))
		return site.Name
	}
	vertexOwner := func(ref program.VertexRef) string {
		switch ref.Kind {
		case program.VertexState:
			return stateName(graph.ParentOf(ref.State()))
		case program.VertexPseudostate:
			return stateName(graph.PseudostateOwnerOf(ref.Pseudo()))
		}
		return ""
	}
	regionOwner := func(id program.RegionID) string { return stateName(graph.RegionOwnerOf(id)) }
	if err := ids.addSorted(restVertices, graph.VertexSite, vertexOwner); err != nil {
		return nil, fmt.Errorf("%s: %w", symbols.FQNOf(sym), err)
	}
	if err := regions.addSorted(restRegions, graph.RegionSite, regionOwner); err != nil {
		return nil, fmt.Errorf("%s: %w", symbols.FQNOf(sym), err)
	}
	// Vertices reached only as a transition's end or a behavior's owner are
	// numbered after those, in the order those reach them.
	owners := func(list []lower.StateBehavior) {
		for _, behavior := range list {
			ids.add(state(behavior.OwnerID))
		}
	}
	entries := func(list []*lower.EntryTransition) {
		for _, et := range list {
			ids.add(state(et.TargetID))
			ids.add(program.PseudoVertex(et.ViaID))
			owners(et.Effect)
		}
	}
	for i := 0; i < len(ids.order); i++ {
		ref := ids.order[i]
		for _, t := range graph.TransitionsOf(ref) {
			ids.add(t.SourceRef)
			ids.add(t.TargetRef)
			owners(t.Effect)
		}
		if ref.Kind != program.VertexState {
			continue
		}
		if b := graph.BehaviorsOf(ref.State()); b != nil {
			owners(b.Entry)
			owners(b.Do)
			owners(b.Exit)
		}
		entries(graph.EntryTransitionsOf(ref.State()))
	}
	entries(graph.EntryTransitionsOf(0))
	for _, r := range regions.order {
		entries(graph.RegionEntryTransitions(r))
	}
	vertices := len(ids.order)

	form := &StateForm{
		Name:    symbols.FQNOf(sym),
		Kind:    sym.Kind.String(),
		Scope:   scopeName(graph.Scope),
		Machine: ids.ref(state(graph.MachineID())),
		Initial: ids.ref(state(graph.InitialID())),
	}
	for _, attr := range graph.Attributes {
		form.Attributes = append(form.Attributes, x.attribute(graph.Scope, attr))
	}
	for id, ref := range ids.order {
		site, _ := graph.VertexSite(ref)
		v := StateVertexForm{ID: id, Kind: site.Kind, Name: site.Name, Span: x.span(graph.Scope, lower.Extent{Span: site.Span, Written: true})}
		scope := graph.Scope
		switch ref.Kind {
		case program.VertexState:
			n := ref.State()
			own := graph.StateScope(n)
			if own != nil {
				scope = own
			}
			v.Scope = scopeName(own)
			v.Hidden = graph.Hidden(n)
			v.HiddenRegion = regions.ref(graph.HiddenRegion(n))
			v.Parent = ids.ref(state(graph.ParentOf(n)))
			v.Region = regions.ref(graph.StateRegion(n))
			for _, r := range graph.RegionsOf(n) {
				v.Regions = append(v.Regions, regions.add(r))
			}
			for _, attr := range graph.AttributesOf(n) {
				v.Attributes = append(v.Attributes, x.attribute(scope, attr))
			}
			if b := graph.BehaviorsOf(n); b != nil {
				var err error
				if v.Entry, err = x.behaviors(graph, ids, scope, b.Entry); err != nil {
					return nil, err
				}
				if v.Do, err = x.behaviors(graph, ids, scope, b.Do); err != nil {
					return nil, err
				}
				if v.Exit, err = x.behaviors(graph, ids, scope, b.Exit); err != nil {
					return nil, err
				}
			}
		case program.VertexPseudostate:
			v.Owner = ids.ref(state(graph.PseudostateOwnerOf(ref.Pseudo())))
		}
		form.Vertices = append(form.Vertices, v)
		for _, t := range graph.TransitionsOf(ref) {
			tf, err := x.transition(graph, ids, scope, t)
			if err != nil {
				return nil, err
			}
			form.Transitions = append(form.Transitions, tf)
		}
	}
	top := graph.TopRegionIDs()
	for id, r := range regions.order {
		site, _ := graph.RegionSite(r)
		form.Regions = append(form.Regions, RegionForm{
			ID:      id,
			Name:    site.Name,
			Span:    x.span(graph.Scope, lower.Extent{Span: site.Span, Written: true}),
			Owner:   ids.ref(state(graph.RegionOwnerOf(r))),
			State:   ids.ref(state(graph.RegionStateOf(r))),
			Initial: ids.ref(state(graph.RegionInitialOf(r))),
			Top:     slices.Contains(top, r),
		})
	}
	if err := x.entryTransitions(form, graph, ids, regions); err != nil {
		return nil, err
	}
	if len(ids.order) != vertices {
		return nil, fmt.Errorf("%w: %s: a vertex is reached only while it is written", ErrGraphsOrder, symbols.FQNOf(sym))
	}
	form.Connections = x.connections(graph.Connections, graph.Scope)
	return form, nil
}

// entryTransitions writes the entry transitions body by body: the machine's own
// first, then those of each state vertex, then those of each region.
func (x *graphsExporter) entryTransitions(form *StateForm, graph *lower.StateGraph, ids *vertexIDs[program.VertexRef], regions *vertexIDs[program.RegionID]) error {
	write := func(body *int, list []*lower.EntryTransition) error {
		for _, et := range list {
			scope := orScope(et.Scope, graph.Scope)
			effect, err := x.behaviors(graph, ids, scope, et.Effect)
			if err != nil {
				return err
			}
			form.EntryTransitions = append(form.EntryTransitions, EntryTransitionForm{
				Body:   body,
				Guard:  x.expr(scope, et.GuardID),
				Target: ids.ref(program.StateVertex(et.TargetID)),
				Via:    ids.ref(program.PseudoVertex(et.ViaID)),
				Effect: effect,
				Scope:  scopeName(et.Scope),
				Decl:   x.span(scope, et.At()),
			})
		}
		return nil
	}
	if err := write(nil, graph.EntryTransitionsOf(0)); err != nil {
		return err
	}
	for id, ref := range ids.order {
		if ref.Kind != program.VertexState {
			continue
		}
		body := id
		if err := write(&body, graph.EntryTransitionsOf(ref.State())); err != nil {
			return err
		}
	}
	for id, r := range regions.order {
		body := id
		if err := write(&body, graph.RegionEntryTransitions(r)); err != nil {
			return err
		}
	}
	return nil
}

// behaviors writes the lowered behaviors of one kind in order.
func (x *graphsExporter) behaviors(graph *lower.StateGraph, ids *vertexIDs[program.VertexRef], enclosing *symbols.Scope, list []lower.StateBehavior) ([]BehaviorForm, error) {
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
		owner := program.StateVertex(b.OwnerID)
		if _, ok := graph.VertexSite(owner); !ok {
			owner = program.VertexRef{}
		}
		out = append(out, BehaviorForm{
			Name:  b.Name,
			Span:  x.span(scope, b.At()),
			Scope: scopeName(b.Scope),
			Owner: ids.ref(owner),
			Body:  body,
			Nodes: b.NodeNames(),
		})
	}
	return out, nil
}

// transition writes one transition with its trigger, guard and effect.
func (x *graphsExporter) transition(graph *lower.StateGraph, ids *vertexIDs[program.VertexRef], enclosing *symbols.Scope, t *lower.Transition) (TransitionForm, error) {
	scope := orScope(t.Scope, enclosing)
	bodyScope := orScope(t.BodyScope, scope)
	effect, err := x.behaviors(graph, ids, bodyScope, t.Effect)
	if err != nil {
		return TransitionForm{}, err
	}
	form := TransitionForm{
		Name:    t.Name,
		Source:  ids.add(t.SourceRef),
		Target:  ids.add(t.TargetRef),
		Trigger: x.trigger(scope, t),
		Guard:   x.expr(bodyScope, t.GuardID),
		Effect:  effect,
		Via:     t.Via,
		Scope:   scopeName(t.Scope),
		Decl:    x.span(scope, t.At()),
	}
	if t.BodyScope != nil && t.BodyScope != t.Scope {
		form.BodyScope = scopeName(t.BodyScope)
	}
	return form, nil
}
