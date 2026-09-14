package smt

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// The constructors the finite sorts carry beside the flow's own nodes and edges.
const (
	// Absent is the node value of a slot no token occupies: one never filled,
	// or one whose token retired or was consumed.
	Absent = "Absent"
	// NoEdge is the edge value of a token that arrived over no succession: the
	// initial token, and a token a synchronization made.
	NoEdge = "none"
	// Stutter is the choice of a move in which no token acts: the flow is
	// complete, or every token left is held.
	Stutter = "stutter"
)

// Sorts are the finite datatype sorts one encoding declares.
type Sorts struct {
	// Node ranges over the flow's nodes and Absent.
	Node solve.Sort
	// Edge ranges over the flow's successions and NoEdge.
	Edge solve.Sort
	// Choice ranges over the token slots that may act in a move, and Stutter.
	Choice solve.Sort
	// Signal ranges over the signal types the sends post and NoSignal; its
	// values are empty for a flow without sends.
	Signal solve.Sort
}

// NoSignal is the signal value of an empty bus slot.
const NoSignal = "none"

// newSorts declares the sorts of a flow with the given token slots. Names are
// prefixed by the action so two encodings in one query stay distinct.
func newSorts(prefix string, f *Flow) Sorts {
	nodes := make([]string, 0, len(f.Labels)+1)
	nodes = append(nodes, f.Labels...)
	nodes = append(nodes, Absent)
	edges := make([]string, 0, len(f.Edges)+1)
	for i := range f.Edges {
		edges = append(edges, edgeLabel(f, i))
	}
	edges = append(edges, NoEdge)
	choices := make([]string, 0, f.Slots+1)
	for t := 0; t < f.Slots; t++ {
		choices = append(choices, slotLabel(t))
	}
	choices = append(choices, Stutter)
	sorts := Sorts{
		Node:   solve.Sort{Kind: solve.SortDatatype, Name: prefix + "::Node", Values: nodes, Origin: prefix},
		Edge:   solve.Sort{Kind: solve.SortDatatype, Name: prefix + "::Edge", Values: edges, Origin: prefix},
		Choice: solve.Sort{Kind: solve.SortDatatype, Name: prefix + "::Choice", Values: choices, Origin: prefix},
	}
	if f.Bus > 0 {
		signals := append(f.signalTypes(), NoSignal)
		sorts.Signal = solve.Sort{Kind: solve.SortDatatype, Name: prefix + "::Signal", Values: signals, Origin: prefix}
	}
	return sorts
}

// signalTypes lists, in first-met order, the signal type each send posts.
func (f *Flow) signalTypes() []string {
	var types []string
	seen := make(map[string]bool, len(f.Sends))
	for _, site := range f.Sends {
		name := sendSignal(site.Send)
		if !seen[name] {
			seen[name] = true
			types = append(types, name)
		}
	}
	return types
}

// sendSignal names the signal type a send posts: the type its message names,
// or the message text where it is a value.
func sendSignal(send lower.Send) string {
	switch m := send.Message.(type) {
	case *ast.QualifiedName:
		return ast.QualifiedText(m)
	case *ast.InvocationExpr:
		return ast.QualifiedText(m.Type)
	}
	return nodeLabel(send.Message)
}

// edgeLabel names succession i of the flow: source, target and its position
// among the source's successions, as a decision witness names a branch.
func edgeLabel(f *Flow, i int) string {
	edge := f.Edges[i]
	pos := 0
	for n, out := range f.Outgoing[edge.Source] {
		if out == i {
			pos = n + 1
			break
		}
	}
	return fmt.Sprintf("%s>%d->%s", f.label(edge.Source), pos, f.label(edge.Target))
}

// slotLabel names token slot t as a choice.
func slotLabel(t int) string { return fmt.Sprintf("slot%d", t) }

// Slot is one token slot at one state: where its token is, the succession it
// arrived over and the ID the interpreter gave it.
type Slot struct {
	At  *solve.Var
	Via *solve.Var
	ID  *solve.Var
	// Able holds when the slot's token may act from this state, as the
	// interpreter's step would offer it.
	Able *solve.Var
	// Parked holds when the token waits at an accept: for a message, or for
	// the clock to reach Due. Both are nil for a flow without accepts.
	Parked *solve.Var
	Due    *solve.Var
}

// BusSlot is one message slot of the bus at one state: whether a message sits
// in it, its signal type, its payload and the move that posted it.
type BusSlot struct {
	Present  *solve.Var
	Signal   *solve.Var
	Payload  *solve.Var
	PostedAt *solve.Var
}

// State is the symbolic state after move Move; move 0 is the initial state.
// Each variable is a copy of its own, named `<what>@<move>`.
type State struct {
	Move   int
	Slots  []Slot
	NextID *solve.Var
	// Values holds this state's copy of every feature the bodies read or
	// write, keyed by the name the translator gives the feature.
	Values map[string]*solve.Var
	// Loop is set, per body loop, when the move reaching this state ran the
	// loop past the unroll bound, so what follows is not known.
	Loop []*solve.Var
	// Failed is set when the move reaching this state met an error the
	// evaluator reports: a division by zero, a guard that is not Boolean.
	Failed *solve.Var
	// Overflow is set when a move reaching this state exceeded what the state
	// holds: a fork found no free slot, or a delivery found a pin's queue full.
	Overflow *solve.Var
	// Now is the clock in seconds; nil for a flow without timed accepts.
	Now *solve.Var
	// Bus holds the M message slots; BusOverflow is set when a send found none
	// free. Both are empty for a flow without sends.
	Bus         []BusSlot
	BusOverflow *solve.Var
}

// Named is one variable of the state vector under the name it has at every
// move, so a query over two copies of the relation pairs them by name.
type Named struct {
	Name string
	Var  *solve.Var
}

// Move is the choice made in move Index, from state Index-1 to state Index.
type Move struct {
	Index int
	// Choice is the slot whose token acts, or Stutter.
	Choice *solve.Var
	// Travel is the succession the acting token left over — the decision
	// witness where several guards held — or NoEdge for a move taking none.
	Travel *solve.Var
	// Held holds, per succession out of a decision node with a guard, when
	// the acting token read that guard and it held; nil for every other succession.
	Held []*solve.Var
}

// newState declares the variables of the state after move i.
func newState(sorts Sorts, f *Flow, i int) *State {
	s := &State{
		Move:   i,
		Slots:  make([]Slot, f.Slots),
		NextID: intVar(fmt.Sprintf("next@%d", i)),
		Values: make(map[string]*solve.Var),
		Loop:   make([]*solve.Var, len(f.Loops)),
		Failed: boolVar(fmt.Sprintf("failed@%d", i)),
	}
	for t := range s.Slots {
		s.Slots[t] = Slot{
			At:   sortedVar(fmt.Sprintf("at[%d]@%d", t, i), sorts.Node),
			Via:  sortedVar(fmt.Sprintf("via[%d]@%d", t, i), sorts.Edge),
			ID:   intVar(fmt.Sprintf("id[%d]@%d", t, i)),
			Able: boolVar(fmt.Sprintf("able[%d]@%d", t, i)),
		}
		if len(f.Accepts) > 0 {
			s.Slots[t].Parked = boolVar(fmt.Sprintf("parked[%d]@%d", t, i))
		}
		if f.Timed {
			s.Slots[t].Due = realVar(fmt.Sprintf("due[%d]@%d", t, i))
		}
	}
	for l := range s.Loop {
		s.Loop[l] = boolVar(fmt.Sprintf("loop[%d]@%d", l, i))
	}
	if f.Cyclic || f.Delivers {
		s.Overflow = boolVar(fmt.Sprintf("overflow@%d", i))
	}
	if f.Timed {
		s.Now = realVar(fmt.Sprintf("now@%d", i))
	}
	if f.Bus > 0 {
		s.Bus = make([]BusSlot, f.Bus)
		for b := range s.Bus {
			s.Bus[b] = BusSlot{
				Present:  boolVar(fmt.Sprintf("bus[%d].present@%d", b, i)),
				Signal:   sortedVar(fmt.Sprintf("bus[%d].signal@%d", b, i), sorts.Signal),
				Payload:  intVar(fmt.Sprintf("bus[%d].payload@%d", b, i)),
				PostedAt: intVar(fmt.Sprintf("bus[%d].posted@%d", b, i)),
			}
		}
		s.BusOverflow = boolVar(fmt.Sprintf("bus.overflow@%d", i))
	}
	return s
}

// newMove declares the choice variables of move i.
func newMove(sorts Sorts, f *Flow, i int) *Move {
	m := &Move{
		Index:  i,
		Choice: sortedVar(fmt.Sprintf("choice@%d", i), sorts.Choice),
		Travel: sortedVar(fmt.Sprintf("travel@%d", i), sorts.Edge),
		Held:   make([]*solve.Var, len(f.Edges)),
	}
	for _, node := range f.Nodes {
		if _, decision := node.(*ast.DecisionNode); !decision {
			continue
		}
		for _, edge := range f.Outgoing[node] {
			if f.Edges[edge].Guard != nil {
				m.Held[edge] = boolVar(fmt.Sprintf("held[%d]@%d", edge, i))
			}
		}
	}
	return m
}

// vars lists the move's variables in a stable order.
func (m *Move) vars() []*solve.Var {
	vars := []*solve.Var{m.Choice, m.Travel}
	for _, held := range m.Held {
		if held != nil {
			vars = append(vars, held)
		}
	}
	return vars
}

// value is this state's copy of the feature base stands for, declared on first use.
func (s *State) value(base *solve.Var) *solve.Var {
	if v, ok := s.Values[base.Name]; ok {
		return v
	}
	v := &solve.Var{
		Name:      fmt.Sprintf("%s@%d", base.Name, s.Move),
		Sort:      base.Sort,
		Symbol:    base.Symbol,
		Dimension: base.Dimension,
		Unit:      base.Unit,
		File:      base.File,
		Span:      base.Span,
		Location:  base.Location,
	}
	s.Values[base.Name] = v
	return v
}

// Vector lists the state's variables under their move-independent names: the
// slots, the clock, the bus, the flags and the feature copies. Two states of
// one encoding have vectors of the same names in the same order.
func (s *State) Vector(features []*solve.Var) []Named {
	vector := make([]Named, 0, 6*len(s.Slots)+4*len(s.Bus)+len(s.Loop)+len(features)+5)
	add := func(name string, v *solve.Var) {
		if v != nil {
			vector = append(vector, Named{Name: name, Var: v})
		}
	}
	for t, slot := range s.Slots {
		add(fmt.Sprintf("at[%d]", t), slot.At)
		add(fmt.Sprintf("via[%d]", t), slot.Via)
		add(fmt.Sprintf("id[%d]", t), slot.ID)
		add(fmt.Sprintf("able[%d]", t), slot.Able)
		add(fmt.Sprintf("parked[%d]", t), slot.Parked)
		add(fmt.Sprintf("due[%d]", t), slot.Due)
	}
	add("next", s.NextID)
	add("failed", s.Failed)
	add("overflow", s.Overflow)
	add("now", s.Now)
	for b, slot := range s.Bus {
		add(fmt.Sprintf("bus[%d].present", b), slot.Present)
		add(fmt.Sprintf("bus[%d].signal", b), slot.Signal)
		add(fmt.Sprintf("bus[%d].payload", b), slot.Payload)
		add(fmt.Sprintf("bus[%d].posted", b), slot.PostedAt)
	}
	add("bus.overflow", s.BusOverflow)
	for l, loop := range s.Loop {
		add(fmt.Sprintf("loop[%d]", l), loop)
	}
	for _, base := range features {
		add(base.Name, s.value(base))
	}
	return vector
}

// vars lists the state's variables in the vector's order.
func (s *State) vars(features []*solve.Var) []*solve.Var {
	vector := s.Vector(features)
	vars := make([]*solve.Var, len(vector))
	for i, named := range vector {
		vars[i] = named.Var
	}
	return vars
}

func intVar(name string) *solve.Var  { return &solve.Var{Name: name, Sort: solve.Int} }
func realVar(name string) *solve.Var { return &solve.Var{Name: name, Sort: solve.Real} }
func boolVar(name string) *solve.Var { return &solve.Var{Name: name, Sort: solve.Bool} }
func sortedVar(name string, sort solve.Sort) *solve.Var {
	return &solve.Var{Name: name, Sort: sort}
}

// nodeValue is the term of the node sort naming node i of the flow.
func nodeValue(sorts Sorts, f *Flow, i int) *solve.Term {
	return solve.ValueTerm(sorts.Node, f.Labels[i])
}

// edgeValue is the term of the edge sort naming succession i of the flow.
func edgeValue(sorts Sorts, f *Flow, i int) *solve.Term {
	return solve.ValueTerm(sorts.Edge, edgeLabel(f, i))
}
