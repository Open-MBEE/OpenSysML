package pssm

import (
	"fmt"
	"sort"
	"strings"
)

// Expressibility is whether a test's model can be spelled in SysML v2 textual
// notation, decided by the UML constructs its state machine uses, in the order of the
// alignment note's construct-to-notation table: a construct with no spelling
// makes the test not expressible whatever else it uses; otherwise `terminate`
// puts it in the gap, any of this project's extensions makes it an extension
// test, and it is standard otherwise.
type Expressibility int

// The four expressibility classes, in decision order.
const (
	// NotExpressible: the model uses a construct with no SysML v2 spelling.
	NotExpressible Expressibility = iota
	// TerminateGap: spellable, but reaches `terminate`, which the runtime refuses.
	TerminateGap
	// Extension: spellable with this project's extensions (defer, fork, join,
	// junction, choice, history).
	Extension
	// Standard: spellable in standard SysML v2 notation.
	Standard
)

var expressibilityNames = [...]string{
	NotExpressible: "not-expressible",
	TerminateGap:   "terminate-gap",
	Extension:      "extension",
	Standard:       "standard",
}

func (c Expressibility) String() string {
	if int(c) < len(expressibilityNames) {
		return expressibilityNames[c]
	}
	return fmt.Sprintf("Expressibility(%d)", int(c))
}

// Expressible reports whether a translated model can be built and run.
func (c Expressibility) Expressible() bool { return c == Standard || c == Extension }

// Construct is one UML construct the classifier looks for, named as the
// alignment note's table names it.
type Construct string

// The constructs that decide a class. Standard constructs are not recorded:
// they never move a test out of Standard.
const (
	// No spelling.
	ConstructEntryPoint          Construct = "entry point"
	ConstructExitPoint           Construct = "exit point"
	ConstructConnectionPoint     Construct = "connection point reference"
	ConstructLocalTransition     Construct = "local transition"
	ConstructInternalTransition  Construct = "internal transition"
	ConstructExtendedRegion      Construct = "extended region"
	ConstructRedefinedState      Construct = "redefined state"
	ConstructRedefinedTransition Construct = "redefined transition"
	ConstructRedefinedMachine    Construct = "redefined state machine"
	ConstructSubmachine          Construct = "submachine state"
	ConstructStandalone          Construct = "standalone state machine"
	ConstructUnknownVertex       Construct = "unknown pseudostate kind"
	ConstructNoMachine           Construct = "no state machine"
	// No translation: the model's behaviors read what the notation cannot bind.
	ConstructBehaviorParameter Construct = "behavior parameter"
	ConstructOperationResult   Construct = "operation result"
	ConstructTesterTrace       Construct = "tester trace"
	ConstructNoInitial         Construct = "orthogonal region without an initial state"
	// Recorded but not deciding: a pseudostate filed as a connection point that
	// is neither an entry nor an exit point and that no transition reaches.
	ConstructStrayConnectionPoint Construct = "stray connection point"
	// The runtime gap.
	ConstructTerminate Construct = "terminate"
	// This project's extensions.
	ConstructDefer          Construct = "defer"
	ConstructFork           Construct = "fork"
	ConstructJoin           Construct = "join"
	ConstructJunction       Construct = "junction"
	ConstructChoice         Construct = "choice"
	ConstructShallowHistory Construct = "shallow history"
	ConstructDeepHistory    Construct = "deep history"
)

// constructClass is the class each recorded construct pulls a test down to.
var constructClass = map[Construct]Expressibility{
	ConstructEntryPoint:           NotExpressible,
	ConstructExitPoint:            NotExpressible,
	ConstructConnectionPoint:      NotExpressible,
	ConstructLocalTransition:      NotExpressible,
	ConstructInternalTransition:   NotExpressible,
	ConstructExtendedRegion:       NotExpressible,
	ConstructRedefinedState:       NotExpressible,
	ConstructRedefinedTransition:  NotExpressible,
	ConstructRedefinedMachine:     NotExpressible,
	ConstructSubmachine:           NotExpressible,
	ConstructStandalone:           NotExpressible,
	ConstructUnknownVertex:        NotExpressible,
	ConstructNoMachine:            NotExpressible,
	ConstructBehaviorParameter:    NotExpressible,
	ConstructOperationResult:      NotExpressible,
	ConstructTesterTrace:          NotExpressible,
	ConstructNoInitial:            NotExpressible,
	ConstructTerminate:            TerminateGap,
	ConstructDefer:                Extension,
	ConstructFork:                 Extension,
	ConstructJoin:                 Extension,
	ConstructJunction:             Extension,
	ConstructChoice:               Extension,
	ConstructShallowHistory:       Extension,
	ConstructDeepHistory:          Extension,
	ConstructStrayConnectionPoint: Standard,
}

// Use is one occurrence of a deciding construct: which, and where.
type Use struct {
	Construct Construct
	// Where names the element using it, e.g. "T3" or "S1.S1.1".
	Where string
}

func (u Use) String() string {
	if u.Where == "" {
		return string(u.Construct)
	}
	return string(u.Construct) + " " + u.Where
}

// Classification is a test's class and the construct uses that decided it.
type Classification struct {
	Class Expressibility
	// Uses lists every recorded construct, those deciding the class first,
	// then by class and document order. A Standard test has at most strays.
	Uses []Use
}

// Reason spells the uses that decided the class, for a report.
func (c Classification) Reason() string {
	if c.Class == Standard {
		return "standard notation only"
	}
	var decisive []string
	for _, u := range c.Uses {
		if constructClass[u.Construct] == c.Class {
			decisive = append(decisive, u.String())
		}
	}
	return strings.Join(decisive, "; ")
}

// Classify decides a test's expressibility from the constructs its state
// machine uses.
func Classify(t *Test) Classification {
	var uses []Use
	add := func(c Construct, where string) { uses = append(uses, Use{c, where}) }
	if t.Machine == nil {
		add(ConstructNoMachine, "")
	} else {
		if t.Target != nil && t.Target.Standalone {
			add(ConstructStandalone, t.Machine.Name)
		}
		if t.Machine.Redefines != "" {
			add(ConstructRedefinedMachine, t.Machine.Name)
		}
		w := &walker{add: add, reached: reachedVertices(t.Machine.Regions)}
		w.connectionPoints(t.Machine.ConnectionPoints)
		w.regions(t.Machine.Regions)
		w.tester(t.Stimulation)
	}
	class := Standard
	for _, u := range uses {
		if c := constructClass[u.Construct]; c < class {
			class = c
		}
	}
	sort.SliceStable(uses, func(i, j int) bool {
		return constructClass[uses[i].Construct] < constructClass[uses[j].Construct]
	})
	return Classification{Class: class, Uses: uses}
}

// reachedVertices collects every vertex some transition in the regions, at
// any depth, has as source or target.
func reachedVertices(regions []*Region) map[*Vertex]bool {
	reached := map[*Vertex]bool{}
	var visit func([]*Region)
	visit = func(regions []*Region) {
		for _, r := range regions {
			for _, tr := range r.Transitions {
				reached[tr.Source] = true
				reached[tr.Target] = true
			}
			for _, v := range r.Vertices {
				visit(v.Regions)
			}
		}
	}
	visit(regions)
	return reached
}

type walker struct {
	add     func(Construct, string)
	reached map[*Vertex]bool
}

// connectionPoints records a machine's or state's connection points. Entry and
// exit points have no spelling; any other kind filed there is a UML violation
// the suite commits in places, counted as the pseudostate it is if a
// transition reaches it and recorded as stray otherwise.
func (w *walker) connectionPoints(cps []*Vertex) {
	for _, cp := range cps {
		switch {
		case cp.Kind == VertexEntryPoint || cp.Kind == VertexExitPoint || w.reached[cp]:
			w.vertex(cp)
		default:
			w.add(ConstructStrayConnectionPoint, cp.Describe())
		}
	}
}

func (w *walker) regions(regions []*Region) {
	for _, r := range regions {
		if r.ExtendedRegion != "" {
			w.add(ConstructExtendedRegion, r.Name)
		}
		w.initial(r)
		for _, v := range r.Vertices {
			w.vertex(v)
		}
		for _, tr := range r.Transitions {
			for _, trig := range tr.Triggers {
				if trig.Event != nil && trig.Event.Kind == EventCall && trig.Event.Operation != nil {
					w.operation(trig.Event.Operation, tr.Name)
				}
			}
			switch tr.Kind {
			case TransitionLocal:
				w.add(ConstructLocalTransition, tr.Name)
			case TransitionInternal:
				w.add(ConstructInternalTransition, tr.Name)
			}
			if tr.Redefines != "" {
				w.add(ConstructRedefinedTransition, tr.Name)
			}
		}
	}
}

// initial records an orthogonal region no initial pseudostate starts: the
// notation's explicit regions each need an `entry; then` the lowerer checks.
func (w *walker) initial(r *Region) {
	if r.owner == nil || len(r.owner.Regions) < 2 {
		return
	}
	for _, v := range r.Vertices {
		if v.Kind == VertexInitial {
			return
		}
	}
	w.add(ConstructNoInitial, r.owner.Path()+"/"+r.Name)
}

// behavior records a state behavior with parameters: the notation binds event
// data on the transition, never on an entry, exit or do action.
func (w *walker) behavior(b *Behavior, where string) {
	if b != nil && len(b.Params) > 0 {
		w.add(ConstructBehaviorParameter, where)
	}
}

// operation records a call trigger whose operation returns a value: the
// runtime's call events carry no result back to the caller.
func (w *walker) operation(op *Operation, where string) {
	for _, p := range op.Params {
		if p.Direction == "out" || p.Direction == "return" || p.Direction == "inout" {
			w.add(ConstructOperationResult, where)
			return
		}
	}
}

// tester records a tester that writes the trace itself: only the target's
// behaviors append to the model's log.
func (w *walker) tester(body *Body) {
	if body == nil {
		return
	}
	for _, st := range body.Statements {
		if st.Kind == StmtCall && st.Name == "trace" && isTarget(st.Receiver) {
			w.add(ConstructTesterTrace, st.String())
		}
	}
}

func (w *walker) vertex(v *Vertex) {
	where := v.Path()
	switch v.Kind {
	case VertexState, VertexFinal, VertexInitial:
	case VertexJunction:
		w.add(ConstructJunction, where)
	case VertexChoice:
		w.add(ConstructChoice, where)
	case VertexFork:
		w.add(ConstructFork, where)
	case VertexJoin:
		w.add(ConstructJoin, where)
	case VertexShallowHistory:
		w.add(ConstructShallowHistory, where)
	case VertexDeepHistory:
		w.add(ConstructDeepHistory, where)
	case VertexEntryPoint:
		w.add(ConstructEntryPoint, where)
	case VertexExitPoint:
		w.add(ConstructExitPoint, where)
	case VertexTerminate:
		w.add(ConstructTerminate, where)
	default:
		w.add(ConstructUnknownVertex, where)
	}
	if v.Kind != VertexState {
		return
	}
	w.behavior(v.Entry, where)
	w.behavior(v.Exit, where)
	w.behavior(v.Do, where)
	if len(v.Deferred) > 0 {
		w.add(ConstructDefer, where)
	}
	if v.Redefines != "" {
		w.add(ConstructRedefinedState, where)
	}
	if v.Submachine != "" {
		w.add(ConstructSubmachine, where)
	}
	if v.ConnectionPointReferences > 0 {
		w.add(ConstructConnectionPoint, where)
	}
	w.connectionPoints(v.ConnectionPoints)
	w.regions(v.Regions)
}
