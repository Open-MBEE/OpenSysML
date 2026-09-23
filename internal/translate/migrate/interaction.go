package migrate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// scenario is an interaction resolved to the steps its v2 form performs in
// occurrence order: signal sends, operation calls, replies and combined fragments.
type scenario struct {
	m       *migration
	e       *sysmlv1.Element
	context *sysmlv1.Element
	self    string
	// order gives each fragment its position among the interaction's fragments.
	order map[*sysmlv1.Element]int
	lines map[*sysmlv1.Element]lifelineRef
	used  map[string]bool
	// placed holds each message some occurrence has already stepped.
	placed map[*sysmlv1.Element]bool
	steps  []*scenarioStep
	// calls lists the call steps resolved so far, which a reply answers.
	calls []*scenarioStep
	// others are the fragments that order nothing: executions, invariants, orderings.
	others []*sysmlv1.Element
	// waited holds the duration constraints written as waits before a step.
	waited map[*sysmlv1.Element]bool
	// names gives each stepped message its step's name; last is the latest one.
	names map[*sysmlv1.Element]string
	last  string
	// chain places each message step in the succession chain it is written in; chains counts them.
	chain  map[*sysmlv1.Element]chainPos
	chains int
	// pending holds the waits forked after a message, to be joined before the later message they span to.
	pending map[*sysmlv1.Element]pendingWait
	// outer gives each operand's body the body of the fragment it is nested in.
	outer map[*[]*scenarioStep]*[]*scenarioStep
	// nest names the actions the steps being written are nested in; hidden
	// counts the if and loop bodies among them, which no qualified name reaches.
	nest   []string
	hidden int
}

// chainPos is a step's position in a chain: the steps of one action body, seq operands inlined.
type chainPos struct {
	chain, pos int
	step       *scenarioStep
}

// pendingWait is a wait forked after the step from, for a constraint a later step joins it before.
type pendingWait struct {
	wait, base, from, note string
}

// lifelineRef is the object a lifeline stands for: the feature path that reads
// it from the scenario's self, and the classifier of the object at its end.
type lifelineRef struct {
	line *sysmlv1.Element
	// path names the object from the scenario's self; chain is the same path as a
	// feature chain a perform subsets, which names no `this`.
	path  string
	chain string
	typ   *sysmlv1.Element
}

type stepKind int

const (
	stepSend stepKind = iota
	stepCall
	stepReply
	stepCreate
	stepDelete
	stepAlt
	stepOpt
	stepLoop
	stepPar
	stepSeq
)

// scenarioStep is one thing the scenario does: a message, or a combined fragment
// whose operands are scenarios of their own.
type scenarioStep struct {
	kind stepKind
	// name is the step's written name, base its unquoted spelling, which a
	// fragment's operands extend.
	name     string
	base     string
	msg      *sysmlv1.Element
	frag     *sysmlv1.Element
	signal   *sysmlv1.Element
	op       *sysmlv1.Element
	sender   *lifelineRef
	receiver *lifelineRef
	// args are the bindings a send or call writes, note what the bindings leave out.
	args []string
	note string
	// call is the call step a reply answers; assigns the bindings of its results.
	call    *scenarioStep
	body    *[]*scenarioStep
	assigns []string
	// operands are a combined fragment's, guard the v2 expression of each one;
	// an else or unguarded operand has "". count repeats a loop a fixed number of times.
	operands []*scenarioOperand
	count    string
}

type scenarioOperand struct {
	e     *sysmlv1.Element
	guard string
	gnote string
	steps []*scenarioStep
}

// messagelessNote says why an interaction without messages has no steps; one of
// state invariants under time constraints is a recorded timing trace, not a behavior.

// The note fragments the writer repeats.
const (
	theMessage  = "the message "
	standsFor   = "stands for "
	noLifeline  = "is received on no lifeline"
	theValue    = "the value "
	theArgument = "the argument "
	notMigrated = " not migrated — "
)

func messagelessNote(e *sysmlv1.Element) string {
	invariants := 0
	for _, f := range e.Owned("fragment") {
		if f.Type == "StateInvariant" {
			invariants++
		}
	}
	if invariants == 0 {
		return "the interaction has no message"
	}
	note := fmt.Sprintf("the interaction has no message: it records %d state invariant(s)", invariants)
	if n := len(e.Owned("ownedRule")); n > 0 {
		note += fmt.Sprintf(" under %d time constraint(s), a timing trace", n)
	}
	return note + ", which no scenario step performs"
}

// interactionNote says why an interaction has no v2 form; "" when it becomes a scenario.
func (m *migration) interactionNote(e *sysmlv1.Element) string {
	_, note := m.scenario(e, "this")
	return note
}

// scenario resolves interaction e to the steps its v2 form performs, reading the
// context's features through self; a note says why it has none.
func (m *migration) scenario(e *sysmlv1.Element, self string) (*scenario, string) {
	if m.deciding[e] {
		return nil, "the interaction refers to itself through an interaction use"
	}
	m.deciding[e] = true
	defer delete(m.deciding, e)
	context := classifierOf(e)
	if context == nil {
		return nil, "the interaction belongs to no block whose parts its lifelines could stand for"
	}
	if !m.written(context) {
		return nil, "the interaction belongs to " + describe(context) + ", which has no v2 declaration to hold it"
	}
	if len(e.Owned("message")) == 0 {
		return nil, messagelessNote(e)
	}
	saved := m.self
	m.self = self
	defer func() { m.self = saved }()
	s := &scenario{
		m: m, e: e, context: context, self: self,
		order:   map[*sysmlv1.Element]int{},
		lines:   map[*sysmlv1.Element]lifelineRef{},
		used:    map[string]bool{"start": true, "done": true},
		placed:  map[*sysmlv1.Element]bool{},
		waited:  map[*sysmlv1.Element]bool{},
		names:   map[*sysmlv1.Element]string{},
		chain:   map[*sysmlv1.Element]chainPos{},
		pending: map[*sysmlv1.Element]pendingWait{},
		outer:   map[*[]*scenarioStep]*[]*scenarioStep{},
	}
	for i, f := range e.Owned("fragment") {
		s.order[f] = i
	}
	for _, line := range e.Owned("lifeline") {
		if _, note := s.lifeline(line); note != "" {
			return nil, "the lifeline " + describe(line) + " " + note
		}
	}
	steps, note := s.resolve(e.Owned("fragment"), e.Owned("message"), &s.steps)
	if note != "" {
		return nil, note
	}
	s.steps = steps
	return s, ""
}

// resolve turns the fragments of an operand (or the interaction) into steps, in
// fragment order; a message neither occurrence places is stepped where it is written.
func (s *scenario) resolve(fragments, messages []*sysmlv1.Element, body *[]*scenarioStep) ([]*scenarioStep, string) {
	var steps []*scenarioStep
	for _, f := range fragments {
		switch f.Type {
		case "MessageOccurrenceSpecification":
			msg := s.m.model.Ref(f, "message")
			if msg == nil {
				return nil, "the occurrence " + describe(f) + " belongs to no message" + suffixNote(s.m.dangling(f, "message"))
			}
			if s.placed[msg] || s.m.model.Ref(msg, "sendEvent") != nil && s.m.model.Ref(msg, "sendEvent") != f {
				continue
			}
			s.placed[msg] = true
			step, note := s.message(msg, body)
			if note != "" {
				return nil, theMessage + describe(msg) + " " + note
			}
			steps = append(steps, step)
		case "CombinedFragment":
			step, note := s.fragment(f, body)
			if note != "" {
				return nil, "the combined fragment " + describe(f) + " " + note
			}
			steps = append(steps, step)
		case "InteractionUse":
			return nil, "the interaction use " + describe(f) + " refers to another interaction, whose steps a scenario does not perform"
		case "ExecutionOccurrenceSpecification", "BehaviorExecutionSpecification", "ActionExecutionSpecification",
			"StateInvariant", "DestructionOccurrenceSpecification", "OccurrenceSpecification", "Continuation":
			s.others = append(s.others, f)
		default:
			return nil, "the fragment " + describe(f) + " is a " + f.Type + ", which has no v2 form"
		}
	}
	for _, msg := range messages {
		if s.placed[msg] {
			continue
		}
		s.placed[msg] = true
		step, note := s.message(msg, body)
		if note != "" {
			return nil, theMessage + describe(msg) + " " + note
		}
		steps = append(steps, step)
	}
	return steps, ""
}

// lifeline resolves the object a lifeline stands for: a part, port or reference
// reached through the context's part tree, or an in parameter of the interaction.
func (s *scenario) lifeline(line *sysmlv1.Element) (lifelineRef, string) {
	if ref, ok := s.lines[line]; ok {
		return ref, ""
	}
	rep := s.m.model.Ref(line, "represents")
	if rep == nil {
		return lifelineRef{}, "stands for nothing" + suffixNote(s.m.dangling(line, "represents"))
	}
	if sel := firstOwned(line, "selector"); sel != nil {
		return lifelineRef{}, "selects one of several " + s.m.nameOf(rep) + " by " + describeValue(sel) + ", which no feature path addresses"
	}
	var ref lifelineRef
	switch rep.Type {
	case "Parameter":
		if rep.Parent != s.e {
			return lifelineRef{}, "stands for the parameter " + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which the scenario has no access to"
		}
		if dir, _ := parameterDirection(rep); dir == "out" {
			return lifelineRef{}, "stands for the " + dir + " parameter " + describe(rep) + ", which holds no object when the scenario starts"
		}
		name := writeName(s.m.nameFor(rep))
		ref = lifelineRef{line: line, path: name, chain: name, typ: s.m.model.Ref(rep, "type")}
	case "Property", "Port":
		if !s.m.written(rep) {
			return lifelineRef{}, standsFor + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which has no v2 declaration"
		}
		paths := s.m.partPaths(s.context, rep)
		switch len(paths) {
		case 0:
			return lifelineRef{}, standsFor + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which no part of " + qualifiedName(s.context) + " reaches"
		case 1:
		default:
			return lifelineRef{}, standsFor + describe(rep) + ", which " + qualifiedName(s.context) + " reaches as both " + paths[0] + " and " + paths[1]
		}
		ref = lifelineRef{line: line, path: s.self + "." + paths[0], chain: paths[0], typ: s.m.model.Ref(rep, "type")}
		if s.self != "this" {
			ref.chain = ref.path
		}
	default:
		return lifelineRef{}, standsFor + describe(rep) + ", a " + rep.Type + " rather than a part or parameter"
	}
	s.lines[line] = ref
	return ref, ""
}

// partPaths lists the feature paths from classifier c to property p through the
// parts, ports and references c and their types own or inherit, nearest first; a
// type is not re-entered along its own path, while sibling parts of one type each count.
func (m *migration) partPaths(c, p *sysmlv1.Element) []string {
	var paths []string
	type node struct {
		c     *sysmlv1.Element
		path  []string
		along []*sysmlv1.Element
	}
	queue := []node{{c: c, along: []*sysmlv1.Element{c}}}
	for len(queue) > 0 && len(paths) < 2 {
		n := queue[0]
		queue = queue[1:]
		for _, f := range m.attributesOf(n.c) {
			if !m.written(f) {
				continue
			}
			path := append(append([]string{}, n.path...), writeName(m.nameOf(f)))
			if f == p {
				paths = append(paths, strings.Join(path, "."))
				continue
			}
			t := m.model.Ref(f, "type")
			if t == nil || slices.Contains(n.along, t) || !isBlockLike(t) {
				continue
			}
			queue = append(queue, node{c: t, path: path, along: append(append([]*sysmlv1.Element{}, n.along...), t)})
		}
	}
	return paths
}

// attributesOf lists the attributes of a classifier and its generals, nearest first.
func (m *migration) attributesOf(c *sysmlv1.Element) []*sysmlv1.Element {
	return m.signalAttributes(c)
}

func isBlockLike(t *sysmlv1.Element) bool {
	switch t.Type {
	case "Class", "Component", "Node", "Device", "ExecutionEnvironment", "Actor", "Interface":
		return true
	}
	return false
}

// message resolves one message to a step, or says why it has no v2 form.
func (s *scenario) message(msg *sysmlv1.Element, body *[]*scenarioStep) (*scenarioStep, string) {
	sort := msg.Attrs["messageSort"]
	if sort == "" {
		sort = "synchCall"
	}
	step := &scenarioStep{msg: msg, body: body}
	var note string
	if step.sender, note = s.end(msg, "sendEvent"); note != "" {
		return nil, "is sent from " + note
	}
	if step.receiver, note = s.end(msg, "receiveEvent"); note != "" {
		return nil, "is received on " + note
	}
	switch sort {
	case "asynchSignal":
		return s.send(step)
	case "synchCall", "asynchCall":
		return s.call(step, sort)
	case "reply":
		return s.reply(step)
	case "createMessage":
		step.kind = stepCreate
	case "deleteMessage":
		step.kind = stepDelete
	default:
		return nil, "is a " + sort + " message, which has no v2 form"
	}
	if step.receiver == nil {
		return nil, noLifeline
	}
	return step, ""
}

// end resolves the lifeline the occurrence in role of msg covers; nil when the
// message has no such occurrence (a found or lost message).
func (s *scenario) end(msg *sysmlv1.Element, role string) (*lifelineRef, string) {
	ev := s.m.model.Ref(msg, role)
	if ev == nil {
		return nil, ""
	}
	line := s.m.model.Ref(ev, "covered")
	if line == nil || line.Type != "Lifeline" {
		return nil, "no lifeline" + suffixNote(s.m.dangling(ev, "covered"))
	}
	ref, note := s.lifeline(line)
	if note != "" {
		return nil, "the lifeline " + describe(line) + ", which " + note
	}
	return &ref, ""
}

// send resolves a signal message: the signal it names is sent to the receiver's object.
func (s *scenario) send(step *scenarioStep) (*scenarioStep, string) {
	sig := s.m.model.Ref(step.msg, "signature")
	if sig == nil {
		return nil, "names no signal" + suffixNote(s.m.dangling(step.msg, "signature"))
	}
	if sig.Type != "Signal" || !s.m.written(sig) {
		return nil, "names " + describe(sig) + ", which is not a migrated signal"
	}
	if step.receiver == nil {
		return nil, noLifeline
	}
	step.kind = stepSend
	step.signal = sig
	var why string
	step.args, step.note, why = s.bindArguments(step.msg, s.m.signalAttributes(sig), "attribute", sig)
	if why != "" {
		return nil, why
	}
	step.base, step.name = s.stepName(step.msg, "send"+s.m.nameFor(sig))
	return step, ""
}

// call resolves an operation call: the receiver's type must have the operation,
// and every in parameter without a default must be bound by an argument.
func (s *scenario) call(step *scenarioStep, sort string) (*scenarioStep, string) {
	op := s.m.model.Ref(step.msg, "signature")
	if op == nil {
		return nil, "names no operation" + suffixNote(s.m.dangling(step.msg, "signature"))
	}
	if op.Type != "Operation" || !s.m.written(op) {
		return nil, "names " + describe(op) + ", which is not a migrated operation"
	}
	if step.receiver == nil {
		return nil, noLifeline
	}
	switch {
	case step.receiver.typ == nil:
		return nil, "calls " + op.Name + " on " + step.receiver.path + ", which has no type to hold the operation"
	case !s.m.hasFeature(step.receiver.typ, op):
		return nil, "calls " + op.Name + " on " + step.receiver.path + ", a " + qualifiedName(step.receiver.typ) + ", which has no such operation"
	}
	var ins []*sysmlv1.Element
	for _, p := range s.m.actionParameters(op) {
		if dir, _ := parameterDirection(p); dir == "in" || dir == "inout" {
			ins = append(ins, p)
		}
	}
	var note, why string
	step.args, note, why = s.bindArguments(step.msg, ins, "parameter", op)
	if why != "" {
		return nil, why
	}
	step.kind = stepCall
	step.op = op
	step.note = note
	if sort == "asynchCall" {
		step.note = joinNotes(step.note, "the asynchronous call is performed to completion before the next step: an action performs what it calls")
	}
	step.base, step.name = s.stepName(step.msg, "call"+s.m.nameFor(op))
	s.calls = append(s.calls, step)
	return step, ""
}

// reply resolves a reply: it answers the latest unanswered call of its operation between
// the same lifelines, and binds the call's results to the caller's attributes it names.
func (s *scenario) reply(step *scenarioStep) (*scenarioStep, string) {
	op := s.m.model.Ref(step.msg, "signature")
	if op == nil {
		return nil, "answers no operation" + suffixNote(s.m.dangling(step.msg, "signature"))
	}
	var call *scenarioStep
	for i := len(s.calls) - 1; i >= 0; i-- {
		c := s.calls[i]
		if c.op == op && sameLine(c.receiver, step.sender) && sameLine(c.sender, step.receiver) {
			call = c
			s.calls = slices.Delete(s.calls, i, i+1)
			break
		}
	}
	if call == nil {
		return nil, "answers no call of " + op.Name + " between its lifelines before it"
	}
	step.kind = stepReply
	step.call = call
	step.op = op
	var outs []*sysmlv1.Element
	for _, p := range s.m.actionParameters(op) {
		if dir, _ := parameterDirection(p); dir != "in" {
			outs = append(outs, p)
		}
	}
	replyArgs := step.msg.Owned("argument")
	for i, p := range s.pairArguments(replyArgs, outs) {
		arg := replyArgs[i]
		switch {
		case p == nil:
			step.note = joinNotes(step.note, theValue+describeValue(arg)+" has no out parameter of "+op.Name+" to stand for")
			continue
		case !s.within(step.body, call.body):
			step.note = joinNotes(step.note, "the result "+s.m.nameOf(p)+" is not bound: the reply is not in the fragment of the call it answers")
			continue
		}
		target, value := assignmentOf(arg)
		if target == "" {
			step.note = joinNotes(step.note, theValue+describeValue(arg)+" of "+s.m.nameOf(p)+" is the operation's own result, which the call computes")
			continue
		}
		attr := s.attributeNamed(step.receiver, target)
		if attr == nil {
			step.note = joinNotes(step.note, "the result "+s.m.nameOf(p)+" is not bound: "+step.receiver.path+" has no attribute "+target)
			continue
		}
		if value != "" {
			step.note = joinNotes(step.note, theValue+value+" the reply states for "+s.m.nameOf(p)+" is the operation's own result, which the call computes")
		}
		step.assigns = append(step.assigns, "assign "+step.receiver.path+"."+writeName(s.m.nameOf(attr))+" := "+call.name+"."+writeName(s.m.nameOf(p))+";")
	}
	if len(step.assigns) > 0 {
		step.base, step.name = s.stepName(step.msg, "reply"+s.m.nameFor(op))
	}
	return step, ""
}

func sameLine(a, b *lifelineRef) bool {
	return a != nil && b != nil && a.line == b.line
}

// within reports whether body is outer or a fragment operand nested in it, where outer's steps are in scope.
func (s *scenario) within(body, outer *[]*scenarioStep) bool {
	for ; body != nil; body = s.outer[body] {
		if body == outer {
			return true
		}
	}
	return false
}

// assignmentOf reads a reply argument written as `attribute = value` (an
// Expression with symbol "=" or an opaque body of that form): the attribute the
// caller stores the result in, and the value the reply states for it, if any.
func assignmentOf(arg *sysmlv1.Element) (target, value string) {
	text := ""
	switch arg.Type {
	case "Expression":
		operands := arg.Owned("operand")
		if arg.Attrs["symbol"] == "=" && len(operands) == 2 {
			return strings.TrimSpace(describeValue(operands[0])), strings.TrimSpace(describeValue(operands[1]))
		}
		return "", ""
	case "OpaqueExpression":
		text, _ = opaqueBody(arg)
	case "LiteralString":
		text = arg.Attrs["value"]
	default:
		return "", ""
	}
	before, after, ok := strings.Cut(text, "=")
	if !ok || strings.ContainsAny(before, "=<>!") || strings.HasPrefix(after, "=") {
		return "", ""
	}
	target = strings.TrimSpace(before)
	if _, isName := exprRefs(target); !isName || strings.ContainsAny(target, " .(") {
		return "", ""
	}
	return target, strings.TrimSpace(after)
}

// attributeNamed finds the attribute of the object a lifeline stands for by name.
func (s *scenario) attributeNamed(ref *lifelineRef, name string) *sysmlv1.Element {
	if ref == nil || ref.typ == nil {
		return nil
	}
	for _, f := range s.m.attributesOf(ref.typ) {
		if s.m.nameOf(f) == name && s.m.written(f) {
			return f
		}
	}
	return nil
}

// pairArguments pairs each argument with a target: a named argument with the target
// of its name, an unnamed one with the next target in order that no name claims.
func (s *scenario) pairArguments(args, targets []*sysmlv1.Element) []*sysmlv1.Element {
	paired := make([]*sysmlv1.Element, len(args))
	named := map[*sysmlv1.Element]bool{}
	for i, arg := range args {
		if arg.Name == "" {
			continue
		}
		for _, t := range targets {
			if s.m.nameOf(t) == arg.Name {
				paired[i], named[t] = t, true
				break
			}
		}
	}
	next := 0
	for i, arg := range args {
		if arg.Name != "" {
			continue
		}
		for next < len(targets) && named[targets[next]] {
			next++
		}
		if next < len(targets) {
			paired[i] = targets[next]
			next++
		}
	}
	return paired
}

// bindArguments writes a message's arguments as bindings of the targets, an owner's
// parameters (each with its direction) or attributes, by name or position; a target that
// must hold a value (no default, lower bound above 0) and that no argument binds is a refusal, why.
func (s *scenario) bindArguments(msg *sysmlv1.Element, targets []*sysmlv1.Element, kind string, owner *sysmlv1.Element) (args []string, note, why string) {
	bound := map[*sysmlv1.Element]bool{}
	msgArgs := msg.Owned("argument")
	for i, t := range s.pairArguments(msgArgs, targets) {
		arg := msgArgs[i]
		if t == nil {
			note = joinNotes(note, theArgument+describeValue(arg)+" has no "+kind+" of "+owner.Name+" to bind to and is dropped")
			continue
		}
		if bound[t] {
			note = joinNotes(note, theArgument+describeValue(arg)+" binds "+s.m.nameOf(t)+" a second time and is dropped")
			continue
		}
		expr, ok, vnote := s.m.typedBehaviorValue(arg, t, s.e)
		if !ok {
			if requiresValue(t) {
				return nil, "", "leaves the " + kind + " " + s.m.nameOf(t) + " of " + owner.Name + ", which must hold a value, unbound: the argument " + describeValue(arg) + " is not written: " + vnote
			}
			note = joinNotes(note, theArgument+describeValue(arg)+" for "+s.m.nameOf(t)+" is dropped: "+vnote)
			continue
		}
		bound[t] = true
		if kind == "parameter" {
			args = append(args, s.m.parameterBinding(t, s.m.nameOf(t), expr))
		} else {
			args = append(args, writeName(s.m.nameOf(t))+" = "+expr)
		}
		note = joinNotes(note, vnote)
	}
	for _, t := range targets {
		if !bound[t] && requiresValue(t) {
			return nil, "", "binds no argument to the " + kind + " " + s.m.nameOf(t) + " of " + owner.Name + ", which must hold a value"
		}
	}
	return args, note, ""
}

// stepName names a step after its message, or after what it does when the message is anonymous.
func (s *scenario) stepName(msg *sysmlv1.Element, fallback string) (base, written string) {
	name := s.m.nameOf(msg)
	if name == "" {
		name = lowerFirst(fallback)
	}
	base = freshIn(s.used, name)
	return base, writeName(base)
}

// fragment resolves a combined fragment: alt and opt to if, loop to while or for,
// par to fork and join, seq and strict to their operands in order.
func (s *scenario) fragment(f *sysmlv1.Element, body *[]*scenarioStep) (*scenarioStep, string) {
	kind := f.Attrs["interactionOperator"]
	if kind == "" {
		kind = "seq"
	}
	step := &scenarioStep{frag: f, body: body}
	operands := f.Owned("operand")
	if len(operands) == 0 {
		return nil, "has no operand"
	}
	switch kind {
	case "alt":
		step.kind = stepAlt
	case "opt":
		step.kind = stepOpt
	case "loop":
		step.kind = stepLoop
	case "par":
		step.kind = stepPar
	case "seq", "strict":
		step.kind = stepSeq
	default:
		return nil, "is a " + kind + " fragment, which has no v2 form"
	}
	if step.kind != stepAlt && step.kind != stepSeq && step.kind != stepPar && len(operands) > 1 {
		return nil, "is a " + kind + " fragment with " + strconv.Itoa(len(operands)) + " operands; it takes one"
	}
	// The operands of alt, opt, loop and par each resolve from the calls open before the fragment:
	// alternatives do not see each other, and concurrent operands are unordered between themselves.
	in := slices.Clone(s.calls)
	outs, ferr := s.fragmentOperands(step, kind, operands, in, body)
	if ferr != "" {
		return nil, ferr
	}
	s.joinCalls(step, in, outs)
	step.base = freshIn(s.used, kind)
	step.name = writeName(step.base)
	return step, ""
}

// fragmentOperands resolves each operand's steps and collects the calls each
// leaves open; isolated operands all start from the calls open before the fragment.
func (s *scenario) fragmentOperands(step *scenarioStep, kind string, operands []*sysmlv1.Element, in []*scenarioStep, body *[]*scenarioStep) (outs [][]*scenarioStep, err string) {
	isolated := step.kind != stepSeq
	for i, o := range operands {
		operand := &scenarioOperand{e: o}
		var steps []*scenarioStep
		operand.steps = steps
		s.outer[&operand.steps] = body
		guard := firstOwned(o, "guard")
		var note string
		if err := s.operandGuard(step, kind, operand, guard, i, len(operands)); err != "" {
			return nil, err
		}
		if isolated {
			s.calls = slices.Clone(in)
		}
		steps, note = s.resolve(o.Owned("fragment"), nil, &operand.steps)
		if note != "" {
			return nil, note
		}
		operand.steps = steps
		step.operands = append(step.operands, operand)
		outs = append(outs, s.calls)
	}
	return outs, ""
}

// operandGuard resolves an operand's guard by the fragment's kind, or why it cannot.
func (s *scenario) operandGuard(step *scenarioStep, kind string, operand *scenarioOperand, guard *sysmlv1.Element, i, n int) string {
	var note string
	switch step.kind {
	case stepAlt:
		operand.guard, operand.gnote, note = s.guard(guard)
		if note != "" {
			return "has an operand whose guard is not written: " + note
		}
		if operand.guard == "" && i != n-1 {
			return "has an operand without a guard before its last, so the operands after it would never run"
		}
	case stepOpt:
		operand.guard, operand.gnote, note = s.guard(guard)
		if note != "" {
			return "has a guard that is not written: " + note
		}
		if operand.guard == "" {
			return "has no guard, so whether its operand runs is unspecified"
		}
	case stepLoop:
		operand.guard, step.count, operand.gnote, note = s.loopBounds(guard)
		if note != "" {
			return note
		}
	case stepPar, stepSeq:
		if guard != nil && !trueGuard(guard) {
			return "has a guard on an operand of a " + kind + " fragment, which runs its operands regardless"
		}
	}
	return ""
}

// joinCalls closes the calls every operand path answers; a par's operands
// also open to replies the calls they made.
func (s *scenario) joinCalls(step *scenarioStep, in []*scenarioStep, outs [][]*scenarioStep) {
	switch step.kind {
	case stepAlt, stepOpt, stepLoop:
		// A path may skip the fragment unless an alt ends in an else; only a call still open on every path stays open.
		if last := step.operands[len(step.operands)-1]; step.kind != stepAlt || last.guard != "" {
			outs = append(outs, in)
		}
		s.calls = openOnEveryPath(in, outs)
	case stepPar:
		// Every operand runs and completes before the join: a call any of them answers is closed,
		// and the calls they make are open to replies after it.
		s.calls = openOnEveryPath(in, outs)
		for _, out := range outs {
			for _, c := range out {
				if !slices.Contains(in, c) {
					s.calls = append(s.calls, c)
				}
			}
		}
	}
}

// openOnEveryPath keeps the calls of in that every path in outs leaves unanswered.
func openOnEveryPath(in []*scenarioStep, outs [][]*scenarioStep) []*scenarioStep {
	var open []*scenarioStep
	for _, c := range in {
		everywhere := true
		for _, out := range outs {
			if !slices.Contains(out, c) {
				everywhere = false
				break
			}
		}
		if everywhere {
			open = append(open, c)
		}
	}
	return open
}

// guard translates an interaction constraint: "" for none, true or else, the v2
// expression otherwise; a note says why it cannot be written.
func (s *scenario) guard(g *sysmlv1.Element) (expr, note, refusal string) {
	if g == nil || trueGuard(g) {
		return "", "", ""
	}
	spec := firstOwned(g, "specification")
	if isElseGuard(spec) {
		return "", "", ""
	}
	expr, ok, note := s.m.behaviorValue(spec, s.e)
	if !ok {
		return "", "", "[" + describeValue(spec) + "] — " + note
	}
	return expr, note, ""
}

// trueGuard reports whether an interaction constraint constrains nothing.
func trueGuard(g *sysmlv1.Element) bool {
	spec := firstOwned(g, "specification")
	return spec == nil || spec.Type == "LiteralBoolean" && (spec.Attrs["value"] == "true" || spec.Attrs["value"] == "")
}

// loopBounds reads a loop's guard and bounds: a guard alone repeats while it
// holds; equal literal bounds without a guard repeat that many times.
func (s *scenario) loopBounds(g *sysmlv1.Element) (guard, count, note, refusal string) {
	var lo, hi string
	if g != nil {
		if v := firstOwned(g, "minint"); v != nil {
			lo = describeValue(v)
		}
		if v := firstOwned(g, "maxint"); v != nil {
			hi = describeValue(v)
		}
	}
	guard, note, refusal = s.guard(g)
	if refusal != "" {
		return "", "", "", "has a guard that is not written: " + refusal
	}
	switch {
	case guard != "" && (lo == "" || lo == "0") && (hi == "" || hi == "*"):
		return guard, "", note, ""
	case guard != "":
		return "", "", "", "repeats between " + lo + " and " + hi + " times while [" + guard + "] holds, which a while loop cannot count"
	case lo != "" && lo == hi && isNatural(lo):
		return "", lo, "", ""
	case lo == "" && hi == "":
		return "", "", "", "has neither a guard nor bounds, so how often it repeats is unspecified"
	}
	return "", "", "", "repeats between " + orAny(lo) + " and " + orAny(hi) + " times, which no fixed loop writes"
}

func orAny(s string) string {
	if s == "" {
		return "*"
	}
	return s
}

func suffixNote(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

// interactionBody writes an interaction as a scenario: an action whose steps are
// its messages and combined fragments in occurrence order.
func (m *migration) interactionBody(e *sysmlv1.Element) {
	m.unwrittenMembers(e, "lifeline", "message", "fragment", "generalOrdering", "ownedRule", "observation")
	s, note := m.scenario(e, "this")
	if note != "" {
		// classifyBehavior does not let this happen; keep the body honest anyway.
		m.w.lines(commentLines("interaction not migrated — " + note))
		return
	}
	s.write()
}

// write writes the scenario's steps and reports every element they stand for.
func (s *scenario) write() {
	saved := s.m.self
	s.m.self = s.self
	defer func() { s.m.self = saved }()
	for _, line := range s.e.Owned("lifeline") {
		ref := s.lines[line]
		s.m.add(line, Mapped, ref.path, "the lifeline stands for "+ref.path+", which the steps address")
	}
	s.writeSteps(s.steps)
	for _, f := range s.others {
		s.other(f)
	}
	for _, o := range s.e.Owned("generalOrdering") {
		s.m.add(o, Skipped, "", "the general ordering is not written: the steps run in fragment order")
	}
	for _, r := range s.e.Owned("ownedRule") {
		if s.waited[r] {
			continue
		}
		if r.Type == "DurationConstraint" {
			s.m.unmapped(r, "the duration constraint is on no message the scenario steps, so no step waits for it")
			continue
		}
		s.m.unmapped(r, "a "+r.Type+" on an interaction has no form in a scenario")
	}
	for _, o := range s.e.Owned("observation") {
		if s.waited[o] {
			continue
		}
		s.m.unmapped(o, "a "+o.Type+" has no v2 form")
	}
	n := countSteps(s.steps)
	steps := "steps"
	if n == 1 {
		steps = "step"
	}
	note := "written as a scenario of " + strconv.Itoa(n) + " " + steps +
		", one per message in occurrence order; the lifelines' own behavior is not part of it"
	if has(s.e, "TestCase") {
		note = joinNotes(note, s.verdictNote())
	}
	s.m.add(s.e, Approximated, s.m.v2Name(s.e), note)
}

// verdictNote says how the verification case written from a test case reaches a
// verdict: a return parameter is its verdict, and without one it stays inconclusive.
func (s *scenario) verdictNote() string {
	for _, p := range s.e.Owned("ownedParameter") {
		if p.Attrs["direction"] == "return" {
			return "its verdict is the return parameter " + s.m.nameFor(p) + ", which no step binds"
		}
	}
	return "the test case has no return parameter, so the verification case states no verdict and is inconclusive once its steps complete"
}

// countSteps counts the messages a body of steps and its fragments write.
func countSteps(steps []*scenarioStep) int {
	n := 0
	for _, s := range steps {
		if s.msg != nil {
			n++
		}
		for _, o := range s.operands {
			n += countSteps(o.steps)
		}
	}
	return n
}

// writeSteps writes a body of steps chained from start to done.
func (s *scenario) writeSteps(steps []*scenarioStep) {
	s.chains++
	s.indexChain(steps, s.chains, new(int))
	prev := "start"
	for _, step := range steps {
		if next := s.step(step, prev); next != "" {
			prev = next
		}
	}
	s.m.w.line(firstKw + prev + " then done;")
}

// indexChain positions the steps of a chain, the operands of its seq fragments inlined.
func (s *scenario) indexChain(steps []*scenarioStep, chain int, pos *int) {
	for _, step := range steps {
		if step.kind == stepSeq {
			for _, o := range step.operands {
				s.indexChain(o.steps, chain, pos)
			}
			continue
		}
		if step.msg != nil {
			s.chain[step.msg] = chainPos{chain: chain, pos: *pos, step: step}
		}
		*pos++
	}
}

// stepWaits reports whether a step writes a node the duration constraints on its message wait before.
func stepWaits(step *scenarioStep) bool {
	return step.kind == stepSend || step.kind == stepCall || step.kind == stepReply && len(step.assigns) > 0
}

// step writes one step after prev and returns the node the next step follows;
// "" when the step writes no node.
func (s *scenario) step(step *scenarioStep, prev string) string {
	m := s.m
	if stepWaits(step) {
		prev = s.waits(step, prev)
	}
	switch step.kind {
	case stepSend:
		m.w.line(actionKw + step.name + " send new " + m.ref(step.signal, s.e) + "(" + strings.Join(step.args, ", ") + ") to " + step.receiver.path + ";")
		s.messageDone(step, "written as a send to "+step.receiver.path)
	case stepCall:
		decl := "perform action " + step.name + " : " + m.ref(step.op, s.e) + " ::> " + step.receiver.chain + "." + writeName(m.operationUsage(step.op))
		if len(step.args) == 0 {
			m.w.line(decl + ";")
		} else {
			m.w.line(decl + " { " + strings.Join(step.args, "; ") + "; }")
		}
		s.messageDone(step, "written as a call of "+m.nameOf(step.op)+" on "+step.receiver.path)
	case stepReply:
		if len(step.assigns) == 0 {
			m.wroteNoMember(step.msg)
			s.messageDone(step, "the reply is the completion of the call "+step.call.name+", which the next step follows")
			return ""
		}
		m.w.block(actionKw+step.name, func() {
			for _, a := range step.assigns {
				m.w.line(a)
			}
		})
		s.messageDone(step, "written as the assignment of the call "+step.call.name+"'s results to "+step.receiver.path)
	case stepCreate, stepDelete:
		what := "creates"
		if step.kind == stepDelete {
			what = "deletes"
		}
		m.w.lines(commentLines("not migrated: Message " + describe(step.msg) + " — the message " + what + " the object " + step.receiver.path + ", which exists for as long as its owner does"))
		m.add(step.msg, Unmapped, "", theMessage+what+" "+step.receiver.path+", a part that exists for as long as its owner does; the steps after it address it as it is")
		s.occurrencesDone(step, "")
		return ""
	case stepAlt, stepOpt:
		m.w.block(actionKw+step.name, func() { s.nested(step.base, func() { s.branches(step, 0) }) })
		s.fragmentDone(step, "if")
	case stepLoop:
		o := step.operands[0]
		head := "while " + o.guard
		if step.count != "" {
			head = "for " + freshIn(s.used, "i") + " in 1.." + step.count
		}
		m.w.block(actionKw+step.name, func() {
			s.nested(step.base, func() { m.w.block(head, func() { s.hiding(func() { s.operand(step, o, 0) }) }) })
		})
		s.fragmentDone(step, strings.Fields(head)[0])
	case stepPar:
		join := writeName(freshIn(s.used, step.base+"End"))
		m.w.line("fork " + step.name + ";")
		m.w.line(firstKw + prev + thenKw + step.name + ";")
		for i, o := range step.operands {
			name := s.operand(step, o, i)
			m.w.line(firstKw + step.name + thenKw + name + ";")
			m.w.line(firstKw + name + thenKw + join + ";")
		}
		m.w.line("join " + join + ";")
		s.fragmentDone(step, "fork")
		return join
	case stepSeq:
		prevInner := prev
		for _, o := range step.operands {
			for _, inner := range o.steps {
				if next := s.step(inner, prevInner); next != "" {
					prevInner = next
				}
			}
			m.add(o.e, Mapped, "", "the operand's steps run in order")
		}
		m.add(step.frag, Mapped, "", "the "+step.frag.Attrs["interactionOperator"]+" fragment's operands run in order")
		if prevInner == prev {
			return ""
		}
		return prevInner
	}
	m.w.line(firstKw + prev + thenKw + step.name + ";")
	if step.msg != nil {
		return s.startWaits(step)
	}
	return step.name
}

// waits chains the duration constraints on a message step as waits after prev and returns the
// node the step follows: a constraint from an earlier message is waited for before the later one,
// as a wait after the earlier one when they are adjacent, else by joining the wait forked after it.
func (s *scenario) waits(step *scenarioStep, prev string) string {
	for _, dc := range s.constraintsOn(step.msg) {
		if s.waited[dc] {
			continue
		}
		if p, ok := s.pending[dc]; ok {
			s.waited[dc] = true
			join := writeName(freshIn(s.used, p.base+"End"))
			s.m.w.line("join " + join + ";")
			s.m.w.line(firstKw + prev + thenKw + join + ";")
			s.m.w.line(firstKw + p.wait + thenKw + join + ";")
			prev = join
			s.m.add(dc, Approximated, p.wait, joinNotes("the time from "+p.from+", written as the wait "+p.wait+" forked after it and joined before "+step.name, p.note))
			s.observationsDone(dc, p.wait, step.name)
			continue
		}
		from, ok := s.waitFrom(dc, step)
		if !ok {
			continue
		}
		s.waited[dc] = true
		if from == "" {
			note := "the time it measures from " + s.names[s.otherEnd(dc, step.msg)] + " to " + step.name + " is not written: steps of other fragments lie between them, so no wait forked after the one can be joined before the other"
			s.m.w.lines(commentLines("duration constraint on " + step.name + notMigrated + note))
			s.m.add(dc, Unmapped, "", note)
			continue
		}
		expr, note, ok := s.waitExpr(dc)
		if !ok {
			s.m.w.lines(commentLines("duration constraint on " + step.name + notMigrated + note))
			s.m.add(dc, Unmapped, "", note)
			continue
		}
		name := writeName(freshIn(s.used, "wait"))
		s.m.w.line(actionKw + name + " accept after " + inSeconds(expr) + ";")
		s.m.w.line(firstKw + prev + thenKw + name + ";")
		prev = name
		s.m.add(dc, Approximated, name, joinNotes(from+", written as the wait "+name+" before "+step.name, note))
		s.observationsDone(dc, name, step.name)
	}
	s.names[step.msg] = step.name
	s.last = step.name
	return prev
}

// startWaits forks, after a message step, the wait for each constraint from it to a later message
// of its chain with steps between them, and returns the node the next step follows.
func (s *scenario) startWaits(step *scenarioStep) string {
	fork := ""
	for _, dc := range s.constraintsOn(step.msg) {
		if s.waited[dc] {
			continue
		}
		if _, pending := s.pending[dc]; pending {
			continue
		}
		other := s.otherEnd(dc, step.msg)
		if other == nil || !s.spansChain(step, other) {
			continue
		}
		expr, note, ok := s.waitExpr(dc)
		if !ok {
			s.waited[dc] = true
			s.m.w.lines(commentLines("duration constraint from " + step.name + notMigrated + note))
			s.m.add(dc, Unmapped, "", note)
			continue
		}
		if fork == "" {
			fork = writeName(freshIn(s.used, "timing"))
			s.m.w.line("fork " + fork + ";")
			s.m.w.line(firstKw + step.name + thenKw + fork + ";")
		}
		base := freshIn(s.used, "wait")
		wait := writeName(base)
		s.m.w.line(actionKw + wait + " accept after " + inSeconds(expr) + ";")
		s.m.w.line(firstKw + fork + thenKw + wait + ";")
		s.pending[dc] = pendingWait{wait: wait, base: base, from: step.name, note: note}
	}
	if fork == "" {
		return step.name
	}
	return fork
}

// spansChain reports whether other is a later message of step's chain with steps between them,
// so that a wait forked after step is joined before other.
func (s *scenario) spansChain(step *scenarioStep, other *sysmlv1.Element) bool {
	a, b := s.chain[step.msg], s.chain[other]
	return b.step != nil && b.chain == a.chain && b.pos > a.pos+1 && stepWaits(b.step)
}

// constraintsOn lists the duration constraints on a message or on either of
// its occurrences, each once.
func (s *scenario) constraintsOn(msg *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	for _, e := range s.messageEnds(msg) {
		for _, dc := range s.m.bounded[e] {
			if !seen[dc] {
				seen[dc] = true
				out = append(out, dc)
			}
		}
	}
	return out
}

// messageEnds lists a message and the occurrences that send and receive it.
func (s *scenario) messageEnds(msg *sysmlv1.Element) []*sysmlv1.Element {
	ends := []*sysmlv1.Element{msg}
	for _, role := range []string{"sendEvent", "receiveEvent"} {
		if ev := s.m.model.Ref(msg, role); ev != nil {
			ends = append(ends, ev)
		}
	}
	return ends
}

// messageOf is the message an element constrains: itself, or the one an
// occurrence sends or receives.
func (s *scenario) messageOf(e *sysmlv1.Element) *sysmlv1.Element {
	if e.Type == "Message" {
		return e
	}
	return s.m.model.Ref(e, "message")
}

// otherEnd is the message a duration constraint on msg measures from or to, nil
// when it constrains msg alone.
func (s *scenario) otherEnd(dc, msg *sysmlv1.Element) *sysmlv1.Element {
	for _, c := range s.m.model.Refs(dc, "constrainedElement") {
		if m := s.messageOf(c); m != nil && m != msg {
			return m
		}
	}
	return nil
}

// waitFrom says what a duration constraint on a step's message measures, and whether the
// step is the one to wait before: the message's own duration, or the time since the other
// message it constrains once that has been stepped; "" when steps lie between the two.
func (s *scenario) waitFrom(dc *sysmlv1.Element, step *scenarioStep) (string, bool) {
	other := s.otherEnd(dc, step.msg)
	if other == nil {
		return "the message's duration; a v2 send arrives at once, so the step waits for it first", true
	}
	name, done := s.names[other]
	if !done {
		return "", false
	}
	if s.last != name {
		return "", true
	}
	return "the time from " + name, true
}

// waitExpr writes the wait a duration constraint's interval stands for: a fixed
// delay for a point interval, a uniform draw over it otherwise.
func (s *scenario) waitExpr(dc *sysmlv1.Element) (expr, note string, ok bool) {
	spec := firstOwned(dc, "specification")
	if spec == nil || spec.Type != "DurationInterval" && spec.Type != "Interval" {
		return "", "the duration constraint has no interval", false
	}
	m := s.m
	lo, lok, lnote := m.durationExpr(m.model.Ref(spec, "min"), s.e)
	hi, hok, hnote := m.durationExpr(m.model.Ref(spec, "max"), s.e)
	if bound, bnote, ok := m.singleValue(spec, lo, lok, hok); ok {
		return bound, joinNotes(bnote, "so the wait is a fixed "+bound+" s"), true
	}
	if !lok || !hok {
		return "", m.openInterval(spec, lo, lok, lnote, hi, hok, hnote), false
	}
	note = joinNotes(lnote, hnote)
	lf, lerr := strconv.ParseFloat(lo, 64)
	hf, herr := strconv.ParseFloat(hi, 64)
	switch {
	case lerr == nil && herr == nil && lf > hf:
		return "", "the interval's min " + lo + " exceeds its max " + hi, false
	case lo == hi:
		return lo, joinNotes(note, "a fixed wait of "+lo+" s"), true
	}
	return "RandomFunctions::uniform(" + lo + ", " + hi + ")",
		joinNotes(note, "a wait drawn uniformly over ["+lo+", "+hi+"] s; a tool's fixed min or max mode is a run setting, not the model's"), true
}

// observationsDone reports the duration observations a written constraint's
// bounds refer to as realized by the wait.
func (s *scenario) observationsDone(dc *sysmlv1.Element, wait, step string) {
	spec := firstOwned(dc, "specification")
	for _, role := range []string{"min", "max"} {
		v := s.m.model.Ref(spec, role)
		for v != nil && (v.Type == "Duration" || v.Type == "TimeExpression") {
			for _, o := range s.m.model.Refs(v, "observation") {
				s.waited[o] = true
				s.m.add(o, Approximated, wait, "the duration it observes is written as the wait "+wait+" before "+step)
			}
			v = firstOwned(v, "expr")
		}
	}
}

// branches writes the operands of an alt or opt from i on as an if with an else.
func (s *scenario) branches(step *scenarioStep, i int) {
	o := step.operands[i]
	if o.guard == "" {
		s.operand(step, o, i)
		return
	}
	s.m.w.block("if "+o.guard, func() { s.hiding(func() { s.operand(step, o, i) }) })
	if i+1 < len(step.operands) {
		s.m.w.block("else", func() { s.hiding(func() { s.branches(step, i+1) }) })
	}
}

// nested writes body as the members of the action named name, nested in the
// actions being written.
func (s *scenario) nested(name string, body func()) {
	s.nest = append(s.nest, name)
	body()
	s.nest = s.nest[:len(s.nest)-1]
}

// hiding writes body inside an if or loop body, where no qualified name reaches.
func (s *scenario) hiding(body func()) {
	s.hidden++
	body()
	s.hidden--
}

// operand writes the i-th operand of a fragment as a nested action and returns its name.
func (s *scenario) operand(step *scenarioStep, o *scenarioOperand, i int) string {
	base := freshIn(s.used, step.base+"Op"+strconv.Itoa(i+1))
	name := writeName(base)
	if len(o.steps) == 0 {
		s.m.w.line(actionKw + name + ";")
	} else {
		s.m.w.block(actionKw+name, func() { s.nested(base, func() { s.writeSteps(o.steps) }) })
	}
	note := "written as the action " + name
	if o.guard != "" {
		note = joinNotes(note, "its guard is the condition ["+o.guard+"]")
	}
	s.m.add(o.e, verdictFor(o.gnote), name, joinNotes(note, o.gnote))
	if g := firstOwned(o.e, "guard"); g != nil {
		s.m.add(g, verdictFor(o.gnote), "", joinNotes("the guard is the operand's condition", o.gnote))
	}
	return name
}

// fragmentDone reports a combined fragment written as the v2 construct named form.
func (s *scenario) fragmentDone(step *scenarioStep, form string) {
	article := "a"
	if form == "if" {
		article = "an"
	}
	s.m.add(step.frag, Mapped, step.name, "written as the action "+step.name+", "+article+" "+form+" over the operands")
}

// messageDone reports a message step and the occurrences that order it.
func (s *scenario) messageDone(step *scenarioStep, note string) {
	switch {
	case step.base == "":
	case s.hidden > 0:
		s.m.wroteEdge(step.msg, s.e, "action", "")
	default:
		s.m.wroteNestedEdge(step.msg, s.e, "action", append([]string(nil), s.nest...), step.base)
	}
	s.m.add(step.msg, verdictFor(step.note), step.name, joinNotes(note, step.note))
	s.occurrencesDone(step, step.name)
}

func (s *scenario) occurrencesDone(step *scenarioStep, name string) {
	for _, role := range []string{"sendEvent", "receiveEvent"} {
		ev := s.m.model.Ref(step.msg, role)
		if ev == nil {
			continue
		}
		if name == "" {
			s.m.add(ev, Unmapped, "", "the occurrence belongs to the message "+describe(step.msg)+", which is not written")
			continue
		}
		s.m.add(ev, Mapped, name, "the occurrence orders the step "+name)
	}
}

// other reports a fragment that orders nothing the steps do not already order.
func (s *scenario) other(f *sysmlv1.Element) {
	switch f.Type {
	case "BehaviorExecutionSpecification", "ActionExecutionSpecification", "ExecutionOccurrenceSpecification":
		s.m.add(f, Skipped, "", "the execution spans the steps between its occurrences, which run in order without it")
	case "StateInvariant":
		s.m.add(f, Unmapped, "", "the state invariant asserts what holds at its point in the scenario, which no step checks")
	case "DestructionOccurrenceSpecification":
		s.m.add(f, Unmapped, "", "the destruction ends an object that exists for as long as its owner does")
	default:
		s.m.add(f, Skipped, "", "the occurrence orders nothing beyond the steps' order")
	}
}
