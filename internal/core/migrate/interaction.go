package migrate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// scenario is an interaction resolved to the steps its v2 form performs in
// occurrence order: signal sends, operation calls, replies and combined fragments.
type scenario struct {
	m       *migration
	e       *xmi.Element
	context *xmi.Element
	self    string
	// order gives each fragment its position among the interaction's fragments.
	order map[*xmi.Element]int
	lines map[*xmi.Element]lifelineRef
	used  map[string]bool
	// placed holds each message some occurrence has already stepped.
	placed map[*xmi.Element]bool
	steps  []*scenarioStep
	// calls lists the call steps resolved so far, which a reply answers.
	calls []*scenarioStep
	// others are the fragments that order nothing: executions, invariants, orderings.
	others []*xmi.Element
	// waited holds the duration constraints written as waits before a step.
	waited map[*xmi.Element]bool
	// names gives each stepped message its step's name; last is the latest one.
	names map[*xmi.Element]string
	last  string
}

// lifelineRef is the object a lifeline stands for: the feature path that reads
// it from the scenario's self, and the classifier of the object at its end.
type lifelineRef struct {
	line *xmi.Element
	// path names the object from the scenario's self; chain is the same path as a
	// feature chain a perform subsets, which names no `this`.
	path  string
	chain string
	typ   *xmi.Element
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
	// name is the step's written name, base the unquoted name a fragment's operands extend.
	name     string
	base     string
	msg      *xmi.Element
	frag     *xmi.Element
	signal   *xmi.Element
	op       *xmi.Element
	sender   *lifelineRef
	receiver *lifelineRef
	// args are the bindings a send or call writes, note what the bindings leave out.
	args string
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
	e     *xmi.Element
	guard string
	gnote string
	steps []*scenarioStep
}

// messagelessNote says why an interaction without messages has no steps; one of
// state invariants under time constraints is a recorded timing trace, not a behavior.
func messagelessNote(e *xmi.Element) string {
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
func (m *migration) interactionNote(e *xmi.Element) string {
	_, note := m.scenario(e, "this")
	return note
}

// scenario resolves interaction e to the steps its v2 form performs, reading the
// context's features through self; a note says why it has none.
func (m *migration) scenario(e *xmi.Element, self string) (*scenario, string) {
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
		order:  map[*xmi.Element]int{},
		lines:  map[*xmi.Element]lifelineRef{},
		used:   map[string]bool{"start": true, "done": true},
		placed: map[*xmi.Element]bool{},
		waited: map[*xmi.Element]bool{},
		names:  map[*xmi.Element]string{},
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
func (s *scenario) resolve(fragments, messages []*xmi.Element, body *[]*scenarioStep) ([]*scenarioStep, string) {
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
				return nil, "the message " + describe(msg) + " " + note
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
			return nil, "the message " + describe(msg) + " " + note
		}
		steps = append(steps, step)
	}
	return steps, ""
}

// lifeline resolves the object a lifeline stands for: a part, port or reference
// reached through the context's part tree, or an in parameter of the interaction.
func (s *scenario) lifeline(line *xmi.Element) (lifelineRef, string) {
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
			return lifelineRef{}, "stands for " + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which has no v2 declaration"
		}
		paths := s.m.partPaths(s.context, rep)
		switch len(paths) {
		case 0:
			return lifelineRef{}, "stands for " + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which no part of " + qualifiedName(s.context) + " reaches"
		case 1:
		default:
			return lifelineRef{}, "stands for " + describe(rep) + ", which " + qualifiedName(s.context) + " reaches as both " + paths[0] + " and " + paths[1]
		}
		ref = lifelineRef{line: line, path: s.self + "." + paths[0], chain: paths[0], typ: s.m.model.Ref(rep, "type")}
		if s.self != "this" {
			ref.chain = ref.path
		}
	default:
		return lifelineRef{}, "stands for " + describe(rep) + ", a " + rep.Type + " rather than a part or parameter"
	}
	s.lines[line] = ref
	return ref, ""
}

// partPaths lists the feature paths from classifier c to property p through the
// parts, ports and references c and their types own or inherit, nearest first; a
// type is not re-entered along its own path, while sibling parts of one type each count.
func (m *migration) partPaths(c, p *xmi.Element) []string {
	var paths []string
	type node struct {
		c     *xmi.Element
		path  []string
		along []*xmi.Element
	}
	queue := []node{{c: c, along: []*xmi.Element{c}}}
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
			queue = append(queue, node{c: t, path: path, along: append(append([]*xmi.Element{}, n.along...), t)})
		}
	}
	return paths
}

// attributesOf lists the attributes of a classifier and its generals, nearest first.
func (m *migration) attributesOf(c *xmi.Element) []*xmi.Element {
	return m.signalAttributes(c)
}

func isBlockLike(t *xmi.Element) bool {
	switch t.Type {
	case "Class", "Component", "Node", "Device", "ExecutionEnvironment", "Actor", "Interface":
		return true
	}
	return false
}

// message resolves one message to a step, or says why it has no v2 form.
func (s *scenario) message(msg *xmi.Element, body *[]*scenarioStep) (*scenarioStep, string) {
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
		return nil, "is received on no lifeline"
	}
	return step, ""
}

// end resolves the lifeline the occurrence in role of msg covers; nil when the
// message has no such occurrence (a found or lost message).
func (s *scenario) end(msg *xmi.Element, role string) (*lifelineRef, string) {
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
		return nil, "is received on no lifeline"
	}
	step.kind = stepSend
	step.signal = sig
	step.args, step.note = s.bindArguments(step.msg, s.m.signalAttributes(sig), "attribute of "+sig.Name, false)
	step.name = s.stepName(step.msg, "send"+s.m.nameFor(sig))
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
		return nil, "is received on no lifeline"
	}
	switch {
	case step.receiver.typ == nil:
		return nil, "calls " + op.Name + " on " + step.receiver.path + ", which has no type to hold the operation"
	case !s.m.hasFeature(step.receiver.typ, op):
		return nil, "calls " + op.Name + " on " + step.receiver.path + ", a " + qualifiedName(step.receiver.typ) + ", which has no such operation"
	}
	var ins []*xmi.Element
	for _, p := range op.Owned("ownedParameter") {
		if dir, _ := parameterDirection(p); dir != "out" {
			ins = append(ins, p)
		}
	}
	var note string
	step.args, note = s.bindArguments(step.msg, ins, "parameter of "+op.Name, true)
	if strings.HasPrefix(note, "unbound: ") {
		return nil, strings.TrimPrefix(note, "unbound: ")
	}
	step.kind = stepCall
	step.op = op
	step.note = note
	if sort == "asynchCall" {
		step.note = joinNotes(step.note, "the asynchronous call is performed to completion before the next step: an action performs what it calls")
	}
	step.name = s.stepName(step.msg, "call"+s.m.nameFor(op))
	s.calls = append(s.calls, step)
	return step, ""
}

// reply resolves a reply: it answers the last call of its operation between the
// same lifelines, and binds the call's results to the caller's attributes it names.
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
			break
		}
	}
	if call == nil {
		return nil, "answers no call of " + op.Name + " between its lifelines before it"
	}
	step.kind = stepReply
	step.call = call
	step.op = op
	var outs []*xmi.Element
	for _, p := range op.Owned("ownedParameter") {
		if dir, _ := parameterDirection(p); dir != "in" {
			outs = append(outs, p)
		}
	}
	for i, arg := range step.msg.Owned("argument") {
		p := s.parameterFor(arg, outs, i)
		switch {
		case p == nil:
			step.note = joinNotes(step.note, "the value "+describeValue(arg)+" has no out parameter of "+op.Name+" to stand for")
			continue
		case call.body != step.body:
			step.note = joinNotes(step.note, "the result "+s.m.nameOf(p)+" is not bound: the reply is not in the fragment of the call it answers")
			continue
		}
		target, value := assignmentOf(arg)
		if target == "" {
			step.note = joinNotes(step.note, "the value "+describeValue(arg)+" of "+s.m.nameOf(p)+" is the operation's own result, which the call computes")
			continue
		}
		attr := s.attributeNamed(step.receiver, target)
		if attr == nil {
			step.note = joinNotes(step.note, "the result "+s.m.nameOf(p)+" is not bound: "+step.receiver.path+" has no attribute "+target)
			continue
		}
		if value != "" {
			step.note = joinNotes(step.note, "the value "+value+" the reply states for "+s.m.nameOf(p)+" is the operation's own result, which the call computes")
		}
		step.assigns = append(step.assigns, "assign "+step.receiver.path+"."+writeName(s.m.nameOf(attr))+" := "+call.name+"."+writeName(s.m.nameOf(p))+";")
	}
	if len(step.assigns) > 0 {
		step.name = s.stepName(step.msg, "reply"+s.m.nameFor(op))
	}
	return step, ""
}

func sameLine(a, b *lifelineRef) bool {
	return a != nil && b != nil && a.line == b.line
}

// assignmentOf reads a reply argument written as `attribute = value` (an
// Expression with symbol "=" or an opaque body of that form): the attribute the
// caller stores the result in, and the value the reply states for it, if any.
func assignmentOf(arg *xmi.Element) (target, value string) {
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
func (s *scenario) attributeNamed(ref *lifelineRef, name string) *xmi.Element {
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

// parameterFor pairs an argument with a target by name when the argument is
// named, by position otherwise; nil when neither matches.
func (s *scenario) parameterFor(arg *xmi.Element, targets []*xmi.Element, i int) *xmi.Element {
	if arg.Name != "" {
		for _, t := range targets {
			if s.m.nameOf(t) == arg.Name {
				return t
			}
		}
		return nil
	}
	if i < len(targets) {
		return targets[i]
	}
	return nil
}

// bindArguments writes a message's arguments as bindings of the targets, by name or position;
// when required, a target no argument binds is reported as "unbound: ..." for the caller to refuse on.
func (s *scenario) bindArguments(msg *xmi.Element, targets []*xmi.Element, what string, required bool) (string, string) {
	var out []string
	var note string
	bound := map[*xmi.Element]bool{}
	for i, arg := range msg.Owned("argument") {
		t := s.parameterFor(arg, targets, i)
		if t == nil {
			note = joinNotes(note, "the argument "+describeValue(arg)+" has no "+what+" to bind to and is dropped")
			continue
		}
		if bound[t] {
			note = joinNotes(note, "the argument "+describeValue(arg)+" binds "+s.m.nameOf(t)+" a second time and is dropped")
			continue
		}
		expr, ok, vnote := s.m.typedBehaviorValue(arg, t, s.e)
		if !ok {
			if required && requiresValue(t) {
				return "", "unbound: leaves the parameter " + s.m.nameOf(t) + " unbound: the argument " + describeValue(arg) + " is not written: " + vnote
			}
			note = joinNotes(note, "the argument "+describeValue(arg)+" for "+s.m.nameOf(t)+" is dropped: "+vnote)
			continue
		}
		bound[t] = true
		out = append(out, writeName(s.m.nameOf(t))+" = "+expr)
		note = joinNotes(note, vnote)
	}
	if required {
		for _, t := range targets {
			if !bound[t] && requiresValue(t) {
				return "", "unbound: binds no argument to the parameter " + s.m.nameOf(t) + ", which must hold a value"
			}
		}
	}
	return strings.Join(out, ", "), note
}

// stepName names a step after its message, or after what it does when the message is anonymous.
func (s *scenario) stepName(msg *xmi.Element, fallback string) string {
	name := s.m.nameOf(msg)
	if name == "" {
		name = lowerFirst(fallback)
	}
	return writeName(freshIn(s.used, name))
}

// fragment resolves a combined fragment: alt and opt to if, loop to while or for,
// par to fork and join, seq and strict to their operands in order.
func (s *scenario) fragment(f *xmi.Element, body *[]*scenarioStep) (*scenarioStep, string) {
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
	if step.kind != stepAlt && step.kind != stepSeq && len(operands) > 1 {
		if step.kind != stepPar {
			return nil, "is a " + kind + " fragment with " + strconv.Itoa(len(operands)) + " operands; it takes one"
		}
	}
	for i, o := range operands {
		operand := &scenarioOperand{e: o}
		var steps []*scenarioStep
		operand.steps = steps
		guard := firstOwned(o, "guard")
		var note string
		switch step.kind {
		case stepAlt:
			operand.guard, operand.gnote, note = s.guard(guard)
			if note != "" {
				return nil, "has an operand whose guard is not written: " + note
			}
			if operand.guard == "" && i != len(operands)-1 {
				return nil, "has an operand without a guard before its last, so the operands after it would never run"
			}
		case stepOpt:
			operand.guard, operand.gnote, note = s.guard(guard)
			if note != "" {
				return nil, "has a guard that is not written: " + note
			}
			if operand.guard == "" {
				return nil, "has no guard, so whether its operand runs is unspecified"
			}
		case stepLoop:
			operand.guard, step.count, operand.gnote, note = s.loopBounds(guard)
			if note != "" {
				return nil, note
			}
		case stepPar, stepSeq:
			if guard != nil && !trueGuard(guard) {
				return nil, "has a guard on an operand of a " + kind + " fragment, which runs its operands regardless"
			}
		}
		steps, note = s.resolve(o.Owned("fragment"), nil, &operand.steps)
		if note != "" {
			return nil, note
		}
		operand.steps = steps
		step.operands = append(step.operands, operand)
	}
	step.base = freshIn(s.used, kind)
	step.name = writeName(step.base)
	return step, ""
}

// guard translates an interaction constraint: "" for none, true or else, the v2
// expression otherwise; a note says why it cannot be written.
func (s *scenario) guard(g *xmi.Element) (expr, note, refusal string) {
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
func trueGuard(g *xmi.Element) bool {
	spec := firstOwned(g, "specification")
	return spec == nil || spec.Type == "LiteralBoolean" && (spec.Attrs["value"] == "true" || spec.Attrs["value"] == "")
}

// loopBounds reads a loop's guard and bounds: a guard alone repeats while it
// holds; equal literal bounds without a guard repeat that many times.
func (s *scenario) loopBounds(g *xmi.Element) (guard, count, note, refusal string) {
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
func (m *migration) interactionBody(e *xmi.Element) {
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
	prev := "start"
	for _, step := range steps {
		if next := s.step(step, prev); next != "" {
			prev = next
		}
	}
	s.m.w.line("first " + prev + " then done;")
}

// step writes one step after prev and returns the node the next step follows;
// "" when the step writes no node.
func (s *scenario) step(step *scenarioStep, prev string) string {
	m := s.m
	if step.kind == stepSend || step.kind == stepCall || step.kind == stepReply && len(step.assigns) > 0 {
		prev = s.waits(step, prev)
	}
	switch step.kind {
	case stepSend:
		m.w.line("action " + step.name + " send new " + m.ref(step.signal, s.e) + "(" + step.args + ") to " + step.receiver.path + ";")
		s.messageDone(step, "written as a send to "+step.receiver.path)
	case stepCall:
		decl := "perform action " + step.name + " : " + m.ref(step.op, s.e) + " ::> " + step.receiver.chain + "." + writeName(m.operationUsage(step.op))
		if step.args == "" {
			m.w.line(decl + ";")
		} else {
			m.w.line(decl + " { in " + strings.ReplaceAll(step.args, ", ", "; in ") + "; }")
		}
		s.messageDone(step, "written as a call of "+m.nameOf(step.op)+" on "+step.receiver.path)
	case stepReply:
		if len(step.assigns) == 0 {
			s.messageDone(step, "the reply is the completion of the call "+step.call.name+", which the next step follows")
			return ""
		}
		m.w.block("action "+step.name, func() {
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
		m.add(step.msg, Unmapped, "", "the message "+what+" "+step.receiver.path+", a part that exists for as long as its owner does; the steps after it address it as it is")
		s.occurrencesDone(step, "")
		return ""
	case stepAlt, stepOpt:
		m.w.block("action "+step.name, func() { s.branches(step, 0) })
		s.fragmentDone(step, "if")
	case stepLoop:
		o := step.operands[0]
		head := "while " + o.guard
		if step.count != "" {
			head = "for " + freshIn(s.used, "i") + " in 1.." + step.count
		}
		m.w.block("action "+step.name, func() {
			m.w.block(head, func() { s.operand(step, o, 0) })
		})
		s.fragmentDone(step, strings.Fields(head)[0])
	case stepPar:
		join := writeName(step.base + "End")
		m.w.line("fork " + step.name + ";")
		m.w.line("first " + prev + " then " + step.name + ";")
		for i, o := range step.operands {
			name := s.operand(step, o, i)
			m.w.line("first " + step.name + " then " + name + ";")
			m.w.line("first " + name + " then " + join + ";")
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
	m.w.line("first " + prev + " then " + step.name + ";")
	return step.name
}

// waits chains the duration constraints on a message step as waits after prev and returns the
// node the step follows: a constraint between two messages is waited for before the later one.
func (s *scenario) waits(step *scenarioStep, prev string) string {
	for _, dc := range s.constraintsOn(step.msg) {
		if s.waited[dc] {
			continue
		}
		from, ok := s.waitFrom(dc, step)
		if !ok {
			continue
		}
		s.waited[dc] = true
		expr, note, ok := s.waitExpr(dc)
		if !ok {
			s.m.w.lines(commentLines("duration constraint on " + step.name + " not migrated — " + note))
			s.m.add(dc, Unmapped, "", note)
			continue
		}
		name := writeName(freshIn(s.used, "wait"))
		s.m.w.line("action " + name + " accept after " + expr + " [SI::s];")
		s.m.w.line("first " + prev + " then " + name + ";")
		prev = name
		s.m.add(dc, Approximated, name, joinNotes(from+", written as the wait "+name+" before "+step.name, note))
		s.observationsDone(dc, name, step.name)
	}
	s.names[step.msg] = step.name
	s.last = step.name
	return prev
}

// constraintsOn lists the duration constraints on a message or on either of
// its occurrences, each once.
func (s *scenario) constraintsOn(msg *xmi.Element) []*xmi.Element {
	var out []*xmi.Element
	seen := map[*xmi.Element]bool{}
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
func (s *scenario) messageEnds(msg *xmi.Element) []*xmi.Element {
	ends := []*xmi.Element{msg}
	for _, role := range []string{"sendEvent", "receiveEvent"} {
		if ev := s.m.model.Ref(msg, role); ev != nil {
			ends = append(ends, ev)
		}
	}
	return ends
}

// messageOf is the message an element constrains: itself, or the one an
// occurrence sends or receives.
func (s *scenario) messageOf(e *xmi.Element) *xmi.Element {
	if e.Type == "Message" {
		return e
	}
	return s.m.model.Ref(e, "message")
}

// waitFrom says what a duration constraint on a step's message measures, and
// whether the step is the one to wait before: the message's own duration, or
// the time since the other message it constrains, once that has been stepped.
func (s *scenario) waitFrom(dc *xmi.Element, step *scenarioStep) (string, bool) {
	var other *xmi.Element
	for _, c := range s.m.model.Refs(dc, "constrainedElement") {
		if m := s.messageOf(c); m != nil && m != step.msg {
			other = m
		}
	}
	if other == nil {
		return "the message's duration; a v2 send arrives at once, so the step waits for it first", true
	}
	name, done := s.names[other]
	if !done {
		return "", false
	}
	if s.last != name {
		return "the time from " + name + ", measured from the step before " + step.name + " since steps lie between them", true
	}
	return "the time from " + name, true
}

// waitExpr writes the wait a duration constraint's interval stands for: a fixed
// delay for a point or half-open interval, a uniform draw over it otherwise.
func (s *scenario) waitExpr(dc *xmi.Element) (expr, note string, ok bool) {
	spec := firstOwned(dc, "specification")
	if spec == nil || spec.Type != "DurationInterval" && spec.Type != "Interval" {
		return "", "the duration constraint has no interval", false
	}
	m := s.m
	lo, lok, lnote := m.durationExpr(m.model.Ref(spec, "min"), s.e)
	hi, hok, hnote := m.durationExpr(m.model.Ref(spec, "max"), s.e)
	if bound, bnote, ok := m.openBound(spec, lo, lok, hi, hok); ok {
		return bound, joinNotes(bnote, "so the wait is a fixed "+bound+" s"), true
	}
	if !lok || !hok {
		note := lnote
		if !lok && m.model.Ref(spec, "min") == nil {
			note = "the interval has no min"
		}
		if !hok {
			note = joinNotes(note, hnote)
			if m.model.Ref(spec, "max") == nil {
				note = joinNotes(note, "the interval has no max")
			}
		}
		return "", note, false
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
func (s *scenario) observationsDone(dc *xmi.Element, wait, step string) {
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
	s.m.w.block("if "+o.guard, func() { s.operand(step, o, i) })
	if i+1 < len(step.operands) {
		s.m.w.block("else", func() { s.branches(step, i+1) })
	}
}

// operand writes the i-th operand of a fragment as a nested action and returns its name.
func (s *scenario) operand(step *scenarioStep, o *scenarioOperand, i int) string {
	name := writeName(step.base + "Op" + strconv.Itoa(i+1))
	if len(o.steps) == 0 {
		s.m.w.line("action " + name + ";")
	} else {
		s.m.w.block("action "+name, func() { s.writeSteps(o.steps) })
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
func (s *scenario) other(f *xmi.Element) {
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
