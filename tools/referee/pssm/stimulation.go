package pssm

import (
	"fmt"
	"strings"
)

// Stimulus is one step of the tester's stimulation: a signal with its payload,
// an operation call with its arguments, or a trace of a value the tester computes.
type Stimulus struct {
	Signal string
	Call   string
	// Value is the scalar payload of a signal, nil for a plain signal.
	Value *Literal
	// Args are the call's arguments in parameter order.
	Args []Argument
	// Trace is the value the tester traces on the target: literals, library
	// behaviors and the results of the operation calls it embeds, each made
	// on the target as the value is evaluated.
	Trace *Expr
	// Calls are the calls Trace embeds, by the ID of the expression making
	// each, with their arguments bound as a call stimulus binds them.
	Calls map[string]Stimulus
}

// Argument is one argument of a queued operation call.
type Argument struct {
	Name  string
	Value *Literal
}

// String spells the stimulus for a report.
func (s Stimulus) String() string {
	if s.Trace != nil {
		return "trace(" + s.Trace.String() + ")"
	}
	if s.Call != "" {
		parts := make([]string, len(s.Args))
		for i, a := range s.Args {
			parts[i] = a.Value.String()
		}
		return s.Call + "(" + strings.Join(parts, ", ") + ")"
	}
	if s.Value != nil {
		return s.Signal + "(" + s.Value.String() + ")"
	}
	return s.Signal
}

// TranslateError reports a construct of a test the translation has no exact
// spelling for. It never drops the construct instead.
type TranslateError struct {
	Test   string
	Where  string
	Reason string
}

func (e *TranslateError) Error() string {
	if e.Where == "" {
		return fmt.Sprintf("%s: %s", e.Test, e.Reason)
	}
	return fmt.Sprintf("%s: %s: %s", e.Test, e.Where, e.Reason)
}

// Stimulation reads the tester's behavior as the driver's steps, in the
// tester's order: Start when the machine reacts to it, then each send, call or trace.
func Stimulation(s *Suite, t *Test) ([]Stimulus, error) {
	if t.Machine == nil {
		return nil, &TranslateError{Test: t.ID, Reason: "no state machine"}
	}
	fail := func(reason string) error { return &TranslateError{Test: t.ID, Where: "tester", Reason: reason} }
	var events []Stimulus
	if mentionsSignal(t, "Start") {
		events = append(events, Stimulus{Signal: "Start"})
	}
	if t.Stimulation == nil {
		return events, nil
	}
	if len(t.Stimulation.Unsupported) > 0 {
		return nil, fail("activity nodes with no translation: " + strings.Join(t.Stimulation.Unsupported, "; "))
	}
	var prev *Statement
	for i := range t.Stimulation.Statements {
		st := &t.Stimulation.Statements[i]
		var ev Stimulus
		var reason string
		switch {
		case st.Kind == StmtAccept:
			prev = st
			continue
		case st.Kind == StmtSend:
			ev, reason = sentStimulus(s, st)
		case st.Kind == StmtCall && st.Name == "trace" && isTarget(st.Receiver):
			ev, reason = traceStimulus(t.Target, prev, st)
		case st.Kind == StmtCall:
			if !isTarget(st.Receiver) {
				reason = fmt.Sprintf("%s calls an object other than the target", st)
			} else {
				ev, reason = callStimulus(t.Target, st.OperationID, st.Name, st.Args)
			}
		default:
			reason = fmt.Sprintf("%s has no translation as a queued event", st)
		}
		if reason != "" {
			return nil, fail(reason)
		}
		events = append(events, ev)
		prev = st
	}
	return events, nil
}

// sentStimulus reads a send to the target as a queued signal, with its scalar
// payload when it carries one.
func sentStimulus(s *Suite, st *Statement) (Stimulus, string) {
	if !isTarget(st.Receiver) {
		return Stimulus{}, fmt.Sprintf("%s addresses an object other than the target", st)
	}
	sig := s.Signals[st.Name]
	ev := Stimulus{Signal: st.Name}
	if len(st.Args) > 0 {
		if sig == nil || len(sig.Attributes) != 1 || len(st.Args) != 1 || st.Args[0].Kind != ExprLiteral {
			return Stimulus{}, fmt.Sprintf("%s carries a payload the translation cannot bind", st)
		}
		ev.Value = st.Args[0].Literal
	}
	return ev, ""
}

// callStimulus reads an operation call on the target as a queued call with its
// literal arguments bound to the in parameters of the operation the call action
// references by identity (UML 16.3.3.1), so same-named operations stay apart.
func callStimulus(target *Class, id, name string, args []Expr) (Stimulus, string) {
	op := target.Operation(id)
	call := name + "(" + exprList(args) + ")"
	if op == nil {
		return Stimulus{}, fmt.Sprintf("this.testable.%s names no operation of the target", call)
	}
	ins := op.Inputs()
	if len(ins) != len(args) {
		return Stimulus{}, fmt.Sprintf("this.testable.%s passes %d arguments to %d parameters", call, len(args), len(ins))
	}
	ev := Stimulus{Call: name}
	for i, a := range args {
		if a.Kind != ExprLiteral {
			return Stimulus{}, fmt.Sprintf("this.testable.%s passes an argument that is not a literal", call)
		}
		ev.Args = append(ev.Args, Argument{Name: ins[i].Name, Value: a.Literal})
	}
	return ev, ""
}

// traceStimulus reads a tester trace as the driver's step, or says why it
// cannot: a trace embedding no call must follow a call, when the machine is quiescent.
func traceStimulus(target *Class, prev, st *Statement) (Stimulus, string) {
	if len(st.Args) != 1 {
		return Stimulus{}, fmt.Sprintf("%s passes %d arguments to trace", st, len(st.Args))
	}
	tr := &traceReader{target: target, st: st}
	if reason := tr.value(&st.Args[0]); reason != "" {
		return Stimulus{}, reason
	}
	if len(tr.calls) == 0 && (prev == nil || prev.Kind != StmtCall || !isTarget(prev.Receiver)) {
		return Stimulus{}, fmt.Sprintf("%s traces while the machine may still be running", st)
	}
	return Stimulus{Trace: &st.Args[0], Calls: tr.calls}, ""
}

// traceReader checks the value a tester traces is one the driver evaluates.
type traceReader struct {
	target *Class
	st     *Statement
	calls  map[string]Stimulus
}

func (tr *traceReader) value(x *Expr) string {
	switch x.Kind {
	case ExprLiteral:
		return ""
	case ExprApply:
		if x.Library == nil {
			return fmt.Sprintf("%s applies %s, which is not a library behavior", tr.st, x.Name)
		}
		fn, ok := libraryBehaviors[x.Library.Qualified]
		if !ok {
			return fmt.Sprintf("%s applies %s, which the driver does not evaluate", tr.st, x.Library.Qualified)
		}
		if len(x.Args) != fn.arity {
			return fmt.Sprintf("%s applies %s to %d arguments, not %d", tr.st, x.Library.Qualified, len(x.Args), fn.arity)
		}
		for i := range x.Args {
			if reason := tr.value(&x.Args[i]); reason != "" {
				return reason
			}
		}
		return ""
	case ExprCall:
		if !isTarget(x.Object) {
			return fmt.Sprintf("%s calls an object other than the target", tr.st)
		}
		ev, reason := callStimulus(tr.target, x.OperationID, x.Name, x.Args)
		if reason != "" {
			return reason
		}
		if x.Result == "" {
			return fmt.Sprintf("%s reads a result %s does not return", tr.st, x.Name)
		}
		if x.ID == "" {
			return fmt.Sprintf("%s makes a call with no identity", tr.st)
		}
		if tr.calls == nil {
			tr.calls = map[string]Stimulus{}
		}
		tr.calls[x.ID] = ev
		return ""
	}
	return fmt.Sprintf("%s traces %s, which is not a literal, a library behavior or the call's result", tr.st, x)
}

// mentionsSignal reports whether the test's machine reacts to the signal: a
// transition or deferral triggered by it, or a behavior of the machine or the
// target that sends it to the machine or accepts it.
func mentionsSignal(t *Test, name string) bool {
	m := &mentionWalker{name: name}
	m.regions(t.Machine.Regions)
	if t.Target != nil {
		for _, op := range t.Target.Operations {
			m.behavior(op.Method)
		}
		for _, b := range t.Target.Behaviors {
			m.behavior(b)
		}
	}
	return m.found
}

type mentionWalker struct {
	name  string
	found bool
}

func (m *mentionWalker) regions(regions []*Region) {
	for _, r := range regions {
		for _, v := range r.Vertices {
			m.triggers(v.Deferred)
			m.behavior(v.Entry)
			m.behavior(v.Exit)
			m.behavior(v.Do)
			m.regions(v.Regions)
		}
		for _, tr := range r.Transitions {
			m.triggers(tr.Triggers)
			m.behavior(tr.Effect)
		}
	}
}

func (m *mentionWalker) triggers(triggers []*Trigger) {
	for _, trig := range triggers {
		if trig.Event != nil && trig.Event.Kind == EventSignal && trig.Event.Signal != nil && trig.Event.Signal.Name == m.name {
			m.found = true
		}
	}
}

func (m *mentionWalker) behavior(b *Behavior) {
	if b == nil || b.Body == nil {
		return
	}
	for _, st := range b.Body.Statements {
		switch st.Kind {
		case StmtSend:
			if st.Name == m.name && isSelf(st.Receiver) {
				m.found = true
			}
		case StmtAccept:
			for _, ev := range st.Events {
				if ev.Kind == EventSignal && ev.Signal != nil && ev.Signal.Name == m.name {
					m.found = true
				}
			}
		}
	}
}
