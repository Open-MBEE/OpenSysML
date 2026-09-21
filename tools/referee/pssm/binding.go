package pssm

import (
	"fmt"
	"slices"
	"sort"
)

// Binding is the one event whose data every path to a behavior with parameters
// carries (PSSM 8.5.5 binds inputs from the triggering occurrence's data).
type Binding struct {
	Behavior *Behavior
	Event    *Event
	Triggers []*Transition
	// Data is the event's data the inputs bind to, in order: a call's input
	// parameters, or a scalar signal's payload typed by the signal.
	Data []Param
	// Outputs names the behavior's outputs as they return: the operation's
	// output parameters for a call event, the behavior's own names otherwise.
	Outputs []string
	// Direct is the accepting transition whose effect the behavior is; the
	// effect reads the accept's own parameters and nothing is carried for it.
	Direct *Transition
}

// Bindings resolves each behavior with parameters to the one event bound on
// the transitions before it, or records why no translation binds it.
type Bindings struct {
	// Bound maps each bound behavior to its binding.
	Bound map[*Behavior]*Binding
	// Refused lists each behavior no binding holds for, in document order.
	Refused []Refusal
	// Carry maps each triggered transition whose event data a bound behavior
	// reads after the transition's effect to that event.
	Carry map[*Transition]*Event
	// overloads numbers each operation another call trigger's shares a name with.
	overloads map[*Operation]int
}

// Refusal is a behavior with parameters no binding holds for, and why.
type Refusal struct {
	Behavior *Behavior
	// Where locates the behavior as the classifier names it: the state's path,
	// or the transition's name.
	Where  string
	Reason string
}

// refusal is the refusal recorded for a behavior, nil for a bound one.
func (b *Bindings) refusal(bh *Behavior) *Refusal {
	for i := range b.Refused {
		if b.Refused[i].Behavior == bh {
			return &b.Refused[i]
		}
	}
	return nil
}

// BindBehaviors analyzes the machine's behaviors with parameters.
func BindBehaviors(m *StateMachine) *Bindings {
	b := &binder{
		all:  allTransitions(m.Regions),
		out:  &Bindings{Bound: map[*Behavior]*Binding{}, Carry: map[*Transition]*Event{}},
		memo: map[*Transition]triggerSet{},
	}
	b.regions(m.Regions)
	b.out.numberOverloads(b.all)
	return b.out
}

type binder struct {
	all  []*Transition
	out  *Bindings
	memo map[*Transition]triggerSet
}

// triggerSet is the triggered transitions whose event reaches a transition,
// or the reason one path carries no event.
type triggerSet struct {
	triggers []*Transition
	unbound  string
}

func (b *binder) regions(regions []*Region) {
	for _, r := range regions {
		for _, v := range r.Vertices {
			if v.Kind == VertexState {
				b.state(v)
				b.regions(v.Regions)
			}
		}
		for _, t := range r.Transitions {
			if hasParams(t.Effect) {
				b.site(t.Effect, t.Name, b.triggersOf(t, nil), t)
			}
		}
	}
}

// state binds a state's entry and do activity from the transitions entering it;
// its exit runs before the leaving transition's effect, so nothing binds it.
func (b *binder) state(v *Vertex) {
	where := v.Path()
	if hasParams(v.Entry) {
		b.site(v.Entry, where, b.entering(v), nil)
	}
	if hasParams(v.Exit) {
		b.refuse(v.Exit, where, b.exiting(v))
	}
	if hasParams(v.Do) {
		if len(outputs(v.Do)) > 0 {
			b.refuse(v.Do, where, "a do activity's outputs return to nobody: the step that dispatched the call ends while it runs")
		} else {
			b.site(v.Do, where, b.entering(v), nil)
		}
	}
}

func hasParams(bh *Behavior) bool { return bh != nil && len(bh.Params) > 0 }

func (b *binder) refuse(bh *Behavior, where, reason string) {
	b.out.Refused = append(b.out.Refused, Refusal{Behavior: bh, Where: where, Reason: reason})
}

// site records the binding of one behavior from the triggers reaching it, or
// its refusal; at is the transition whose effect the behavior is, if any.
func (b *binder) site(bh *Behavior, where string, set triggerSet, at *Transition) {
	if set.unbound != "" {
		b.refuse(bh, where, set.unbound)
		return
	}
	var event *Event
	for _, t := range set.triggers {
		if len(t.Triggers) != 1 {
			b.refuse(bh, where, fmt.Sprintf("transition %s accepts several events; which one's data binds the behavior is decided per occurrence", t.Name))
			return
		}
		trig := t.Triggers[0]
		if trig.Event == nil {
			b.refuse(bh, where, fmt.Sprintf("transition %s has a trigger without an event", t.Name))
			return
		}
		if event == nil {
			event = trig.Event
		} else if !sameEvent(event, trig.Event) {
			first, second := describeApart(event, trig.Event)
			b.refuse(bh, where, fmt.Sprintf("bound from %s by one path and %s by another", first, second))
			return
		}
	}
	data, ok := eventData(event)
	if !ok {
		b.refuse(bh, where, fmt.Sprintf("%s carries no data the notation binds", event.Describe()))
		return
	}
	if reason := conforms(bh, event, data); reason != "" {
		b.refuse(bh, where, reason)
		return
	}
	outputs, reason := outputNames(bh, event)
	if reason != "" {
		b.refuse(bh, where, reason)
		return
	}
	binding := &Binding{Behavior: bh, Event: event, Triggers: set.triggers, Data: data, Outputs: outputs}
	if at != nil && len(at.Triggers) > 0 {
		binding.Direct = at
	}
	b.out.Bound[bh] = binding
	for _, t := range set.triggers {
		if t != binding.Direct {
			b.out.Carry[t] = event
		}
	}
}

// entering collects the triggers of every transition entering state v: one
// whose target is v or lies inside it, from outside v.
func (b *binder) entering(v *Vertex) triggerSet {
	var set triggerSet
	for _, t := range b.all {
		if t.Target == nil || t.Source == nil || !enters(v, t) {
			continue
		}
		set = set.union(b.triggersOf(t, nil))
	}
	if len(set.triggers) == 0 && set.unbound == "" {
		set.unbound = "no transition enters the state"
	}
	return set
}

// exiting is why state v's exit reads no event data: the exit precedes the
// leaving transition's effect, or no path to v carries an event at all.
func (b *binder) exiting(v *Vertex) string {
	unbound := ""
	for _, t := range b.all {
		if t.Target == nil || t.Source == nil || !exits(v, t) {
			continue
		}
		if set := b.triggersOf(t, nil); set.unbound != "" {
			if unbound == "" {
				unbound = set.unbound
			}
			continue
		}
		return fmt.Sprintf("the exit runs before the effect of transition %s, the first place the accepted data is readable; a state's exit action reads nothing of the transition leaving it", t.Name)
	}
	if unbound != "" {
		return unbound
	}
	return "no transition exits the state"
}

// triggersOf finds the triggered transitions whose event reaches t: t itself,
// or through its pseudostate source; a completion transition carries none.
func (b *binder) triggersOf(t *Transition, seen map[*Transition]bool) triggerSet {
	if len(t.Triggers) > 0 {
		return triggerSet{triggers: []*Transition{t}}
	}
	if seen == nil {
		if set, ok := b.memo[t]; ok {
			return set
		}
		set := b.triggersOf(t, map[*Transition]bool{})
		b.memo[t] = set
		return set
	}
	if seen[t] {
		return triggerSet{}
	}
	seen[t] = true
	src := t.Source
	var set triggerSet
	switch {
	case src == nil:
		set.unbound = fmt.Sprintf("transition %s has no source", t.Name)
	case src.Kind == VertexInitial:
		owner := (*Vertex)(nil)
		if src.Region != nil {
			owner = src.Region.owner
		}
		if owner == nil {
			set.unbound = "reached when the machine starts, which carries no event"
		} else {
			set = b.entering(owner)
		}
	case src.Kind.IsPseudostate():
		for _, in := range b.all {
			if in.Target == src {
				set = set.union(b.triggersOf(in, seen))
			}
		}
		if len(set.triggers) == 0 && set.unbound == "" {
			set.unbound = fmt.Sprintf("no transition reaches %s", src.Describe())
		}
	default:
		set.unbound = fmt.Sprintf("reached by the completion of %s, which carries no event", src.Describe())
	}
	return set
}

func (s triggerSet) union(o triggerSet) triggerSet {
	if s.unbound == "" {
		s.unbound = o.unbound
	}
	for _, t := range o.triggers {
		found := false
		for _, have := range s.triggers {
			if have == t {
				found = true
			}
		}
		if !found {
			s.triggers = append(s.triggers, t)
		}
	}
	return s
}

// parent is the state owning v's region; nil at the top of the machine.
func parent(v *Vertex) *Vertex {
	if v == nil || v.Region == nil {
		return nil
	}
	return v.Region.owner
}

// inside reports whether v is s or nested anywhere within s.
func inside(v, s *Vertex) bool {
	for ; v != nil; v = parent(v) {
		if v == s {
			return true
		}
	}
	return false
}

// mainEnd climbs from one end of a transition to the vertex just below the
// least common ancestor of both ends (UML 14.2.3.9.4).
func mainEnd(end, other *Vertex) *Vertex {
	for p := parent(end); p != nil && !inside(other, p); p = parent(end) {
		end = p
	}
	return end
}

// enters reports whether transition t enters state v: v lies on the path from
// the main target down to the target; a still active target is not re-entered.
func enters(v *Vertex, t *Transition) bool {
	// Into the state enclosing the source: its region completes (PSSM 8.5.8).
	if t.Source != t.Target && inside(t.Source, t.Target) {
		return false
	}
	return inside(t.Target, v) && inside(v, mainEnd(t.Target, t.Source))
}

// exits reports whether transition t exits state v when v is active: v is the
// main source or lies within it.
func exits(v *Vertex, t *Transition) bool {
	main := mainEnd(t.Source, t.Target)
	return main.Kind == VertexState && inside(v, main)
}

// describeApart names two distinct events for a diagnostic, same-named
// operations by their signatures so the reader can tell the overloads apart.
func describeApart(a, b *Event) (string, string) {
	x, y := a.Describe(), b.Describe()
	if x == y && a.Kind == EventCall {
		return a.Operation.Signature(), b.Operation.Signature()
	}
	return x, y
}

// sameEvent reports whether two triggers' events are one event: the same signal
// or the same operation by identity, so same-named overloads stay apart.
func sameEvent(a, b *Event) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case EventSignal:
		return a.Signal != nil && a.Signal == b.Signal
	case EventCall:
		return a.Operation != nil && a.Operation == b.Operation
	}
	return a.Type == b.Type
}

// eventData is the data an event carries, in order: a call event's input
// parameters, or a scalar signal's one payload attribute, typed by the signal.
func eventData(ev *Event) ([]Param, bool) {
	switch {
	case ev == nil:
		return nil, false
	case ev.Kind == EventSignal && ev.Signal != nil:
		sig := ev.Signal
		if len(sig.Attributes) != 1 || scalarTypes[sig.Attributes[0].Type] == "" {
			return nil, false
		}
		return []Param{{Name: sig.Attributes[0].Name, Type: sig.Name, Direction: "in"}}, true
	case ev.Kind == EventCall && ev.Operation != nil:
		return ev.Operation.Inputs(), true
	}
	return nil, false
}

// inputs are a behavior's in and inout parameters, outputs its out, inout and
// return parameters, each in declaration order.
func inputs(bh *Behavior) []Param {
	var in []Param
	for _, p := range bh.Params {
		if p.Direction == "in" || p.Direction == "inout" {
			in = append(in, p)
		}
	}
	return in
}

func outputs(bh *Behavior) []Param {
	var out []Param
	for _, p := range bh.Params {
		if p.Direction == "out" || p.Direction == "inout" || p.Direction == "return" {
			out = append(out, p)
		}
	}
	return out
}

// conforms checks the behavior's inputs against the event's data, by position
// and type (PSSM 8.5.5: an input-conforming signature).
func conforms(bh *Behavior, ev *Event, data []Param) string {
	ins := inputs(bh)
	if len(ins) != len(data) {
		return fmt.Sprintf("%d input parameters, %s carries %d values", len(ins), ev.Describe(), len(data))
	}
	for i, p := range ins {
		if p.Type != data[i].Type {
			return fmt.Sprintf("parameter %s is a %s but %s carries a %s there", p.Name, p.Type, ev.Describe(), data[i].Type)
		}
		if scalarTypes[p.Type] == "" && (ev.Kind != EventSignal || p.Type != ev.Signal.Name) {
			return fmt.Sprintf("parameter %s has type %s, which has no ScalarValues counterpart", p.Name, p.Type)
		}
	}
	return ""
}

// outputNames names the behavior's outputs as they return: the operation's
// output parameters by position and type for a call event, their own names
// otherwise.
func outputNames(bh *Behavior, ev *Event) ([]string, string) {
	outs := outputs(bh)
	var names []string
	if ev.Kind == EventCall {
		returned := ev.Operation.Outputs()
		if len(outs) != len(returned) {
			return nil, fmt.Sprintf("%d output parameters, %s returns %d values", len(outs), ev.Describe(), len(returned))
		}
		for i, p := range returned {
			if outs[i].Type != p.Type {
				return nil, fmt.Sprintf("parameter %s is a %s but %s returns a %s there", outs[i].Name, outs[i].Type, ev.Describe(), p.Type)
			}
			names = append(names, p.Name)
		}
	} else {
		for _, p := range outs {
			names = append(names, p.Name)
		}
	}
	for _, p := range outs {
		if scalarTypes[p.Type] == "" {
			return nil, fmt.Sprintf("parameter %s has type %s, which has no ScalarValues counterpart", p.Name, p.Type)
		}
	}
	return names, ""
}

// carriedAttr names the attribute a triggered transition stores one value of
// its event's data in, for the behaviors bound from it to read: the signal's
// name for a payload, the operation's and the input's for a call, a same-named
// operation's carrying its place among the machine's call triggers.
func (b *Bindings) carriedAttr(ev *Event, p Param) string {
	if ev.Kind == EventSignal {
		return "trigger_" + identifier(ev.Signal.Name)
	}
	name := identifier(ev.Operation.Name)
	if tag := b.overloads[ev.Operation]; tag > 0 {
		name = fmt.Sprintf("%s_%d", name, tag)
	}
	return "trigger_" + name + "_" + identifier(p.Name)
}

// carrierKey tells the events whose data is carried apart: a signal by name,
// an operation by identity, so same-named overloads keep their own carriers.
func carrierKey(ev *Event) string {
	if ev.Kind == EventCall && ev.Operation != nil {
		return ev.Operation.Signature() + " " + ev.Operation.ID
	}
	return ev.Describe()
}

// carriedEvents lists the events some triggered transition carries, one per
// signal or operation, in a fixed order.
func (b *Bindings) carriedEvents() []*Event {
	byKey := map[string]*Event{}
	for _, ev := range b.Carry {
		byKey[carrierKey(ev)] = ev
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*Event, len(keys))
	for i, k := range keys {
		out[i] = byKey[k]
	}
	return out
}

// numberOverloads gives each operation sharing its name with another call
// trigger's a number, its place among them in document order, 1 first.
func (b *Bindings) numberOverloads(all []*Transition) {
	b.overloads = map[*Operation]int{}
	byName := map[string][]*Operation{}
	for _, t := range all {
		for _, trig := range t.Triggers {
			ev := trig.Event
			if ev == nil || ev.Kind != EventCall || ev.Operation == nil {
				continue
			}
			if !slices.Contains(byName[ev.Operation.Name], ev.Operation) {
				byName[ev.Operation.Name] = append(byName[ev.Operation.Name], ev.Operation)
			}
		}
	}
	for _, ops := range byName {
		if len(ops) < 2 {
			continue
		}
		for i, op := range ops {
			b.overloads[op] = i + 1
		}
	}
}
