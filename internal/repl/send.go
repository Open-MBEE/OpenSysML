package repl

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

const sendUsage = "usage: %send <signal>[(<parameter>=<expression>, ...)] [to <object>]"

// sendRequest is a %send line taken apart: the signal, its arguments as
// written, and the object named after `to`, empty when none was.
type sendRequest struct {
	signal string
	args   []string
	target string
}

// parseSendLine takes apart what follows %send.
func parseSendLine(text string) (sendRequest, error) {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "(") || text == "to" || strings.HasPrefix(text, "to ") {
		return sendRequest{}, errors.New(sendUsage)
	}
	var req sendRequest
	end := indexOutsideName(text, "( \t")
	if end < 0 {
		end = len(text)
	}
	req.signal, text = text[:end], strings.TrimSpace(text[end:])

	if strings.HasPrefix(text, "(") {
		closing := closingParen(text)
		if closing < 0 {
			return sendRequest{}, fmt.Errorf("the argument list of %s is not closed: %s", req.signal, sendUsage)
		}
		for _, group := range splitTopLevel(text[1:closing]) {
			if arg := strings.Join(group, " "); arg != "" {
				req.args = append(req.args, arg)
			}
		}
		text = strings.TrimSpace(text[closing+1:])
	}
	if text == "" {
		return req, nil
	}
	target, ok := strings.CutPrefix(text, "to")
	if !ok || (target != "" && !strings.HasPrefix(target, " ") && !strings.HasPrefix(target, "\t")) {
		return sendRequest{}, fmt.Errorf("unexpected %q after the signal: %s", text, sendUsage)
	}
	req.target = strings.TrimSpace(target)
	if req.target == "" || indexOutsideName(req.target, " \t") >= 0 {
		return sendRequest{}, fmt.Errorf("`to` names one object: %s", sendUsage)
	}
	return req, nil
}

// quoteTracker follows the lexer's quoting through a prompt argument: a string
// or quoted name is opaque to a scanner, its escaped characters included.
type quoteTracker struct {
	quote   rune
	escaped bool
}

// inside consumes r, reporting whether it is part of a string or quoted name.
func (q *quoteTracker) inside(r rune) bool {
	switch {
	case q.escaped:
		q.escaped = false
	case q.quote != 0:
		if r == '\\' {
			q.escaped = true
		} else if r == q.quote {
			q.quote = 0
		}
	case r == '"' || r == '\'':
		q.quote = r
	default:
		return false
	}
	return true
}

// closingParen indexes the parenthesis closing the one text opens with, -1 when
// none does; parentheses inside a string or quoted name do not count.
func closingParen(text string) int {
	depth, q := 0, quoteTracker{}
	for i, r := range text {
		switch {
		case q.inside(r):
		case r == '(':
			depth++
		case r == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// signalTarget is where a %send delivers: the object (nil for a machine no
// object performs), the machines that may accept it, and its name in the report.
type signalTarget struct {
	object   *runtime.Instance
	machines []*runtime.StateExecutor
	label    string
}

// doSend injects a signal into an object's machine through the runtime's own
// message bus, so the debugger delivers it as it would one an action sent.
func (s *Session) doSend(text string) ([]string, bool, error) {
	lines, err := s.sendSignal(text)
	if err != nil {
		if errors.Is(err, errRuntimeInit) {
			return nil, false, err
		}
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// sendSignal resolves the signal and its destination, refuses what no machine
// there would accept, and posts the rest. The arguments are parsed before the
// destination is reached, so a malformed one materializes nothing.
func (s *Session) sendSignal(text string) ([]string, error) {
	req, err := parseSendLine(text)
	if err != nil {
		return nil, err
	}
	args, err := parseArguments(req.args)
	if err != nil {
		return nil, err
	}
	if err := checkArgumentNames(args); err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
	}
	target, err := s.signalTarget(req.target)
	if err != nil {
		return nil, err
	}
	msg, typed, err := s.signalMessage(ctx, req.signal, args, target)
	if err != nil {
		return nil, err
	}
	accepting, err := acceptingMachines(target.machines, msg)
	if err != nil {
		return nil, err
	}
	if len(accepting) == 0 {
		return nil, fmt.Errorf("%s accepts no signal %s now: %s", target.label, msg.SignalType, machineStates(target.machines))
	}
	decisions, err := decideMachines(accepting, msg)
	if err != nil {
		return nil, err
	}
	if len(decisions) == 0 {
		return nil, fmt.Errorf("%s would fire no transition on %s now, so it was not sent: %s", target.label, signalText(msg), guardsHolding(accepting, msg))
	}
	ctx.PostMessage(msg)

	out := []string{fmt.Sprintf("✓ Sent %s to %s", signalText(msg), target.label)}
	if !typed {
		out = append(out, fmt.Sprintf("  No declaration types %s, so the signal is matched by name alone", msg.SignalType))
	}
	for _, d := range decisions {
		out = append(out, "  "+d.text())
	}
	if s.stateExec != nil {
		out = append(out, s.dispatchHint(accepting, decisions)...)
	}
	return out, nil
}

// dispatchHint says how the debugged machine relates to the signal just sent:
// that a step of it dispatches it, or that the machine leaves it to a sibling
// because its own guards would drop it.
func (s *Session) dispatchHint(accepting []*runtime.StateExecutor, decisions []machineDecision) []string {
	exec := s.stateExec.executor
	if slices.ContainsFunc(decisions, func(d machineDecision) bool { return d.machine == exec }) {
		return []string{"", "Use %step or %advance <time> to dispatch it"}
	}
	if slices.Contains(accepting, exec) {
		return []string{fmt.Sprintf("  %s would fire nothing on it, so a step of it leaves it to the machine above", machineStates([]*runtime.StateExecutor{exec}))}
	}
	return nil
}

// machineDecision is what one machine decided for a message: what its dispatch
// would do with it now.
type machineDecision struct {
	machine  *runtime.StateExecutor
	decision runtime.Decision
}

// text says what the machine would do with the message when it is dispatched.
func (d machineDecision) text() string {
	where := machineStates([]*runtime.StateExecutor{d.machine})
	if d.decision.Deferred {
		return fmt.Sprintf("Deferred by %s, to be dispatched once it leaves", where)
	}
	resumes := ""
	if len(d.decision.Resumes) > 0 {
		resumes = fmt.Sprintf("the %s goes on from its accept", strings.Join(d.decision.Resumes, " and the "))
	}
	if len(d.decision.Fires) == 0 {
		return fmt.Sprintf("Accepted by %s: %s", where, resumes)
	}
	if resumes != "" {
		resumes = ", and " + resumes
	}
	return fmt.Sprintf("Accepted by %s: %s fires on it%s", where, strings.Join(d.decision.Fires, " and "), resumes)
}

// decideMachines decides the message with each machine as its dispatch would,
// keeping the machines that would fire a transition on it, defer it, or let a do
// behavior go on with it.
func decideMachines(machines []*runtime.StateExecutor, msg runtime.Message) ([]machineDecision, error) {
	var out []machineDecision
	for _, m := range machines {
		decision, err := m.Decide(msg)
		if err != nil {
			return nil, fmt.Errorf("state machine %q cannot decide %s: %w", machineName(m), signalText(msg), err)
		}
		if decision.Enabled() {
			out = append(out, machineDecision{machine: m, decision: decision})
		}
	}
	return out, nil
}

// guardsHolding explains a message the machines accept but would fire nothing
// on: every transition it triggers is held back by its guard.
func guardsHolding(machines []*runtime.StateExecutor, msg runtime.Message) string {
	return fmt.Sprintf("%s, and the guard of every transition %s triggers is false", machineStates(machines), msg.SignalType)
}

// signalTarget resolves the object a %send names, or the object the debugged
// machine is performed by when it names none.
func (s *Session) signalTarget(name string) (signalTarget, error) {
	if name == "" {
		if s.stateExec == nil {
			return signalTarget{}, fmt.Errorf("%w, so there is no object to send to: name one with `to <object>`", s.noStateSessionErr())
		}
		exec := s.stateExec.executor
		self := exec.Performer()
		if self == nil {
			label := fmt.Sprintf("state machine %q", s.stateExec.name)
			return signalTarget{machines: []*runtime.StateExecutor{exec}, label: label}, nil
		}
		label := objectMention(self, s.stateExec.selfFQN)
		return signalTarget{object: self, machines: s.machinesOf(self), label: label}, nil
	}

	inst, ref, err := s.resolveObject(name)
	if err != nil {
		return signalTarget{}, err
	}
	label := objectMention(inst, ref)
	machines := s.machinesOf(inst)
	if len(machines) == 0 {
		return signalTarget{}, fmt.Errorf("%s runs no state machine, so nothing there accepts a signal (%%state <machine> <object> starts one)", label)
	}
	return signalTarget{object: inst, machines: machines, label: label}, nil
}

// machinesOf lists the machines an object runs: those it exhibits, and the one
// a %state session performs on its behalf.
func (s *Session) machinesOf(inst *runtime.Instance) []*runtime.StateExecutor {
	var machines []*runtime.StateExecutor
	for _, b := range inst.Behaviors() {
		if b.State != nil {
			machines = append(machines, b.State)
		}
	}
	if s.stateExec != nil {
		exec := s.stateExec.executor
		if exec.Performer() == inst && !slices.Contains(machines, exec) {
			machines = append(machines, exec)
		}
	}
	return machines
}

// signalMessage builds the message to post, typed by the definition the name
// resolves to; a bare name no declaration types is matched by name, as `accept go` is.
func (s *Session) signalMessage(ctx *runtime.Context, signal string, args []argument, target signalTarget) (runtime.Message, bool, error) {
	sym, _, lerr := s.lookupSymbolOfKinds(signal, runtime.SignalDefinitionKinds...)
	if lerr != nil {
		if !errors.Is(lerr, runtime.ErrUnresolvedReference) {
			return runtime.Message{}, false, lerr
		}
		named := runtime.NamedSignalMessage(nameText(signal), target.object)
		if len(args) > 0 {
			return runtime.Message{}, false, lerr
		}
		accepting, err := acceptingMachines(target.machines, named)
		if err != nil {
			return runtime.Message{}, false, err
		}
		if len(accepting) == 0 {
			return runtime.Message{}, false, lerr
		}
		return named, false, nil
	}
	if !runtime.IsSignalDefinition(sym) {
		return runtime.Message{}, false, fmt.Errorf("%s is a %s, not a signal definition", signal, sym.Notation())
	}
	bound, err := s.evalArguments(ctx, args)
	if err != nil {
		return runtime.Message{}, false, err
	}
	msg, err := ctx.SignalMessage(sym, bound, target.object)
	if err != nil {
		return runtime.Message{}, false, err
	}
	return msg, true, nil
}

// checkArgumentNames refuses an argument list that binds one parameter twice,
// which a map of bindings would otherwise silently reduce to the last.
func checkArgumentNames(args []argument) error {
	seen := make(map[string]bool, len(args))
	for _, arg := range args {
		if seen[arg.param] {
			return fmt.Errorf("argument %s is given twice", arg.param)
		}
		seen[arg.param] = true
	}
	return nil
}

// acceptingMachines keeps the machines whose active configuration accepts msg;
// a machine whose accept port fails to resolve is the error.
func acceptingMachines(machines []*runtime.StateExecutor, msg runtime.Message) ([]*runtime.StateExecutor, error) {
	var out []*runtime.StateExecutor
	for _, m := range machines {
		accepted, err := m.AcceptsMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("state machine %q cannot accept %s: %w", machineName(m), signalText(msg), err)
		}
		if accepted {
			out = append(out, m)
		}
	}
	return out, nil
}

// droppedSignalNote says what became of a signal the last step dispatched and no
// transition fired on; "" when one did, or when the step dispatched no signal.
func droppedSignalNote(exec *runtime.StateExecutor) string {
	d, ok := exec.LastDispatch()
	if !ok {
		return ""
	}
	return droppedDispatchNote(d)
}

// droppedDispatchNote is droppedSignalNote for one dispatch.
func droppedDispatchNote(d runtime.Dispatch) string {
	msg, isSignal := d.Event.Payload.(runtime.Message)
	if !isSignal || d.Fired {
		return ""
	}
	if d.Deferred {
		return msg.SignalType + " was deferred by the active state, to be dispatched again once it leaves"
	}
	return msg.SignalType + " was consumed by no transition: since it was sent, the state or the data its guards read had changed"
}

// machineStates names each machine with the state it is in.
func machineStates(machines []*runtime.StateExecutor) string {
	parts := make([]string, 0, len(machines))
	for _, m := range machines {
		parts = append(parts, fmt.Sprintf("state machine %q in state %s", machineName(m), currentStateName(m)))
	}
	return strings.Join(parts, ", ")
}

// machineName names a machine the way the notation declares it.
func machineName(exec *runtime.StateExecutor) string {
	if sym := exec.StateMachineSymbol(); sym != nil && sym.Name != "" {
		return sym.Name
	}
	return "<anonymous>"
}

// signalText writes a message as the send that posts it: the signal with its
// payload, parameter by parameter.
func signalText(msg runtime.Message) string {
	if len(msg.Payload) == 0 {
		if msg.Value != nil {
			return msg.SignalType + "(" + runtime.FormatValue(*msg.Value) + ")"
		}
		return msg.SignalType
	}
	names := make([]string, 0, len(msg.Payload))
	for name := range msg.Payload {
		names = append(names, name)
	}
	sort.Strings(names)
	args := make([]string, 0, len(names))
	for _, name := range names {
		args = append(args, name+"="+runtime.FormatValue(msg.Payload[name]))
	}
	return msg.SignalType + "(" + strings.Join(args, ", ") + ")"
}

// eventText writes a queued event as the signal or call that raised it.
func eventText(event runtime.Event) string {
	switch payload := event.Payload.(type) {
	case runtime.Message:
		return signalText(payload)
	case runtime.Call:
		return payload.Operation + "()"
	}
	return event.Type.String()
}
