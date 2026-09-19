package repl

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
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
// object performs), the behaviors that may accept it, and its name in the report.
type signalTarget struct {
	object    *runtime.Instance
	receivers []signalReceiver
	label     string
}

// signalReceiver is one behavior a %send may reach: a state machine, or a
// performed action a token of which may be parked at an accept for the signal.
type signalReceiver struct {
	machine *runtime.StateExecutor
	action  *runtime.ActionExecutor
	name    string
}

// machineReceiver is a state machine as a %send reaches it.
func machineReceiver(exec *runtime.StateExecutor) signalReceiver {
	return signalReceiver{machine: exec, name: machineName(exec)}
}

// actionReceiver is a performed action as a %send reaches it, under the name the
// object performs it by, or the action's own when it performs it by none.
func actionReceiver(exec *runtime.ActionExecutor, name string) signalReceiver {
	if name == "" {
		name = actionName(exec)
	}
	return signalReceiver{action: exec, name: name}
}

// kind says what the receiver is, as a report names it.
func (r signalReceiver) kind() string {
	if r.machine != nil {
		return "state machine"
	}
	return "performed action"
}

// status names the receiver with where it stands: the state a machine is in,
// the accepts an action is parked at or the state of its run.
func (r signalReceiver) status() string {
	if r.machine != nil {
		if r.machine.State() == runtime.StateTerminated {
			return fmt.Sprintf("state machine %q terminated", r.name)
		}
		return fmt.Sprintf("state machine %q in state %s", r.name, currentStateName(r.machine))
	}
	return fmt.Sprintf("performed action %q %s", r.name, actionStanding(r.action))
}

// accepts reports whether the receiver takes msg in its present configuration; a
// receiver whose accept port fails to resolve is the error.
func (r signalReceiver) accepts(msg runtime.Message) (bool, error) {
	var (
		accepted bool
		err      error
	)
	if r.machine != nil {
		accepted, err = r.machine.AcceptsMessage(msg)
	} else {
		accepted, err = r.action.AcceptsMessage(msg)
	}
	if err != nil {
		return false, fmt.Errorf("%s %q cannot accept %s: %w", r.kind(), r.name, signalText(msg), err)
	}
	return accepted, nil
}

// decide says what the receiver would do with msg once dispatched (what a machine
// fires, defers or resumes on it; the accepts an action is parked at for it), false for nothing.
func (r signalReceiver) decide(msg runtime.Message) (acceptance, bool, error) {
	if r.machine != nil {
		decision, err := r.machine.Decide(msg)
		if err != nil {
			return acceptance{}, false, fmt.Errorf("state machine %q cannot decide %s: %w", r.name, signalText(msg), err)
		}
		return acceptance{receiver: r, decision: decision}, decision.Enabled(), nil
	}
	taking, err := r.action.AcceptTaking(msg)
	if err != nil {
		return acceptance{}, false, fmt.Errorf("performed action %q cannot accept %s: %w", r.name, signalText(msg), err)
	}
	return acceptance{receiver: r, accepts: taking}, len(taking) > 0, nil
}

// doSend injects a signal into an object's behaviors through the runtime's own
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

// sendSignal resolves the signal and its destination, refuses what no behavior
// there would accept, and posts the rest; arguments are parsed before anything materializes.
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
	target, err := s.signalTarget(ctx, req.target)
	if err != nil {
		return nil, err
	}
	msg, typed, err := s.signalMessage(ctx, req.signal, args, target)
	if err != nil {
		return nil, err
	}
	accepting, err := acceptingReceivers(target.receivers, msg)
	if err != nil {
		return nil, err
	}
	if len(accepting) == 0 {
		return nil, fmt.Errorf("%s accepts no signal %s now: %s", target.label, msg.SignalType, receiverStatuses(target.receivers))
	}
	acceptances, err := decideReceivers(accepting, msg)
	if err != nil {
		return nil, err
	}
	if len(acceptances) == 0 {
		return nil, fmt.Errorf("%s would fire no transition on %s now, so it was not sent: %s", target.label, signalText(msg), guardsHolding(accepting, msg))
	}
	ctx.PostMessage(msg)

	out := []string{fmt.Sprintf("✓ Sent %s to %s", signalText(msg), target.label)}
	if !typed {
		out = append(out, fmt.Sprintf("  No declaration types %s, so the signal is matched by name alone", msg.SignalType))
	}
	for _, a := range acceptances {
		out = append(out, "  "+a.text())
	}
	out = append(out, s.dispatchHint(ctx, accepting, acceptances)...)
	return out, nil
}

// dispatchHint says what dispatches the signal just sent: a step of the debugged
// behavior taking it, an advance of a session driving the runtime it is in flight
// on, or a session to open when none does; a debugged machine whose guards would
// drop it is said to leave it to a sibling.
func (s *Session) dispatchHint(ctx *runtime.Context, accepting []signalReceiver, acceptances []acceptance) []string {
	debugged := s.debuggedReceivers(ctx)
	if slices.ContainsFunc(acceptances, func(a acceptance) bool { return slices.ContainsFunc(debugged, a.receiver.sameExecution) }) {
		return []string{"", "Use %step or %advance <time> to dispatch it"}
	}
	for _, r := range debugged {
		if r.machine != nil && slices.ContainsFunc(accepting, r.sameExecution) {
			return []string{fmt.Sprintf("  %s would fire nothing on it, so a step of it leaves it to the machine above", r.status())}
		}
	}
	if slices.Contains(distinctContexts(s.stateExec.contextOf(), s.actionExec.contextOf()), ctx) {
		return []string{"", "Use %advance <time> to dispatch it"}
	}
	if s.stateExec != nil || s.actionExec != nil {
		return []string{"", "The open session runs on the runtime of an earlier model, so open a %state or %action session anew, then %advance <time> dispatches it"}
	}
	return []string{"", "Open a %state or %action session, then %advance <time> dispatches it"}
}

// debuggedReceivers are the behaviors the open debugging sessions step on ctx's
// bus; a session a rebuild left on an earlier runtime hears nothing posted here.
func (s *Session) debuggedReceivers(ctx *runtime.Context) []signalReceiver {
	var out []signalReceiver
	if s.stateExec != nil && s.stateExec.contextOf() == ctx {
		out = append(out, machineReceiver(s.stateExec.executor))
	}
	if s.actionExec != nil && s.actionExec.contextOf() == ctx {
		out = append(out, actionReceiver(s.actionExec.executor, ""))
	}
	return out
}

// acceptance is what one receiver would do with a message once it is dispatched:
// what a machine's dispatch does with it, or the accepts an action is parked at for it.
type acceptance struct {
	receiver signalReceiver
	decision runtime.Decision
	accepts  []runtime.TakingAccept
}

// text says what the receiver would do with the message when it is dispatched; an
// action parked at several accepts for it names them all, as its step picks the taker.
func (a acceptance) text() string {
	if a.receiver.action != nil {
		accepts := make([]string, len(a.accepts))
		for i, accept := range a.accepts {
			accepts[i] = accept.String()
		}
		if len(accepts) == 1 {
			return fmt.Sprintf("Accepted by performed action %q waiting at %s", a.receiver.name, accepts[0])
		}
		return fmt.Sprintf("Accepted by performed action %q waiting at %s; the step dispatching it lets one of them take it", a.receiver.name, strings.Join(accepts, " and at "))
	}
	where := a.receiver.status()
	if a.decision.Deferred {
		return fmt.Sprintf("Deferred by %s, to be dispatched once it leaves", where)
	}
	resumes := ""
	if len(a.decision.Resumes) > 0 {
		resumes = fmt.Sprintf("the %s goes on from its accept", strings.Join(a.decision.Resumes, " and the "))
	}
	if len(a.decision.Fires) == 0 {
		return fmt.Sprintf("Accepted by %s: %s", where, resumes)
	}
	if resumes != "" {
		resumes = ", and " + resumes
	}
	return fmt.Sprintf("Accepted by %s: %s fires on it%s", where, strings.Join(a.decision.Fires, " and "), resumes)
}

// decideReceivers decides the message with each receiver as its dispatch would,
// keeping those that fire on it, defer it, or go on from an accept with it.
func decideReceivers(receivers []signalReceiver, msg runtime.Message) ([]acceptance, error) {
	var out []acceptance
	for _, r := range receivers {
		a, enabled, err := r.decide(msg)
		if err != nil {
			return nil, err
		}
		if enabled {
			out = append(out, a)
		}
	}
	return out, nil
}

// guardsHolding explains a message the machines accept but would fire nothing
// on: every transition it triggers is held back by its guard.
func guardsHolding(machines []signalReceiver, msg runtime.Message) string {
	return fmt.Sprintf("%s, and the guard of every transition %s triggers is false", receiverStatuses(machines), msg.SignalType)
}

// signalTarget resolves the object a %send names, or the object the debugged
// machine or action is performed by when it names none.
func (s *Session) signalTarget(ctx *runtime.Context, name string) (signalTarget, error) {
	if name == "" {
		return s.debuggedTarget(ctx)
	}

	inst, ref, err := s.resolveObject(name)
	if err != nil {
		return signalTarget{}, err
	}
	label := objectMention(inst, ref)
	receivers := s.receiversOf(ctx, inst)
	if len(receivers) == 0 {
		return signalTarget{}, fmt.Errorf("%s runs no state machine and performs no action, so nothing there accepts a signal (%%state <machine> <object> or %%action <action> <object> starts one)", label)
	}
	return signalTarget{object: inst, receivers: receivers, label: label}, nil
}

// debuggedTarget is where a %send naming no object delivers: the %state session's object (or its
// machine when none performs it, unless a rebuild left it on an earlier runtime), else the %action session's object.
func (s *Session) debuggedTarget(ctx *runtime.Context) (signalTarget, error) {
	if s.stateExec != nil {
		exec := s.stateExec.executor
		self := exec.Performer()
		if self == nil {
			if s.stateExec.contextOf() != ctx {
				return signalTarget{}, fmt.Errorf("the %%state session runs %q on the runtime of an earlier model, so nothing sent now reaches it: %%state %s starts it anew, or name an object with `to <object>`",
					s.stateExec.name, s.stateExec.name)
			}
			label := fmt.Sprintf("state machine %q", s.stateExec.name)
			return signalTarget{receivers: []signalReceiver{machineReceiver(exec)}, label: label}, nil
		}
		return signalTarget{object: self, receivers: s.receiversOf(ctx, self), label: objectMention(self, s.stateExec.selfFQN)}, nil
	}
	if s.actionExec != nil {
		self := s.actionExec.executor.Performer()
		if self == nil {
			return signalTarget{}, fmt.Errorf("the %%action session performs %q on behalf of no object, so there is no object to send to: name one with `to <object>` (%%action %s <object> performs it on one)",
				s.actionExec.name, s.actionExec.name)
		}
		return signalTarget{object: self, receivers: s.receiversOf(ctx, self), label: objectMention(self, s.actionExec.selfFQN)}, nil
	}
	return signalTarget{}, fmt.Errorf("%s, so there is no object to send to: name one with `to <object>` (a %%state or %%action session on an object supplies it)",
		noSessionText("debugging session", s.mostRecentlyEnded(), ""))
}

// receiversOf lists the behaviors an object runs, in declaration order, and the
// machine or action a debugging session performs on its behalf on ctx's bus.
func (s *Session) receiversOf(ctx *runtime.Context, inst *runtime.Instance) []signalReceiver {
	var receivers []signalReceiver
	for _, b := range inst.Behaviors() {
		switch {
		case b.State != nil:
			receivers = append(receivers, machineReceiver(b.State))
		case b.Action != nil:
			receivers = append(receivers, actionReceiver(b.Action, b.Name))
		}
	}
	for _, r := range s.debuggedReceivers(ctx) {
		if r.performer() == inst && !slices.ContainsFunc(receivers, r.sameExecution) {
			receivers = append(receivers, r)
		}
	}
	return receivers
}

// performer is the object the receiver runs on behalf of, nil for one no object performs.
func (r signalReceiver) performer() *runtime.Instance {
	if r.machine != nil {
		return r.machine.Performer()
	}
	return r.action.Performer()
}

// sameExecution reports whether other is the receiver's very execution, whatever it is named.
func (r signalReceiver) sameExecution(other signalReceiver) bool {
	return r.machine == other.machine && r.action == other.action
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
		accepting, err := acceptingReceivers(target.receivers, named)
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

// acceptingReceivers keeps the receivers whose present configuration accepts
// msg; one whose accept port fails to resolve is the error.
func acceptingReceivers(receivers []signalReceiver, msg runtime.Message) ([]signalReceiver, error) {
	var out []signalReceiver
	for _, r := range receivers {
		accepted, err := r.accepts(msg)
		if err != nil {
			return nil, err
		}
		if accepted {
			out = append(out, r)
		}
	}
	return out, nil
}

// dispatchedEventNote says what the last step's dispatch did beyond firing a
// transition: the do behaviors it let go on, or what became of a signal no
// transition fired on; "" when the step dispatched nothing else worth noting.
func dispatchedEventNote(exec *runtime.StateExecutor) string {
	d, ok := exec.LastDispatch()
	if !ok {
		return ""
	}
	if len(d.Resumed) > 0 {
		resumed := "letting the " + strings.Join(d.Resumed, " and the ") + " go on from its accept"
		if d.Fired {
			return " and " + resumed
		}
		return ", " + resumed
	}
	if note := droppedDispatchNote(d); note != "" {
		return ", but " + note
	}
	return ""
}

// droppedDispatchNote says what became of a signal a dispatch fired no transition
// on and let no do behavior go on with; "" for any other dispatch.
func droppedDispatchNote(d runtime.Dispatch) string {
	msg, isSignal := d.Event.Payload.(runtime.Message)
	if !isSignal || d.Fired || len(d.Resumed) > 0 {
		return ""
	}
	if d.Deferred {
		return msg.SignalType + " was deferred by the active state, to be dispatched again once it leaves"
	}
	return msg.SignalType + " was consumed by no transition: since it was sent, the state or the data its guards read had changed"
}

// receiverStatuses names each receiver with where it stands.
func receiverStatuses(receivers []signalReceiver) string {
	parts := make([]string, 0, len(receivers))
	for _, r := range receivers {
		parts = append(parts, r.status())
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

// actionName names an action the way the notation declares it.
func actionName(exec *runtime.ActionExecutor) string {
	if sym := exec.ActionSymbol(); sym != nil && sym.Name != "" {
		return sym.Name
	}
	return "<anonymous>"
}

// actionStanding says where a performed action stands: the accepts its tokens
// are parked at, or else the state of its run.
func actionStanding(exec *runtime.ActionExecutor) string {
	var waits []string
	for _, token := range exec.Tokens() {
		if token.Wait != nil && !token.Wait.Timed && token.Wait.Trigger == "" {
			waits = append(waits, fmt.Sprintf("accept %s of type %s", token.Wait.ParamName, orAnySignal(token.Wait.SignalType)))
		}
	}
	if len(waits) == 0 {
		return strings.ToLower(exec.State().String())
	}
	return "waiting at " + strings.Join(waits, " and ")
}

// orAnySignal names the type an accept awaits, "any" for an accept naming none.
func orAnySignal(signalType string) string {
	if signalType == "" {
		return "any"
	}
	return signalType
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
