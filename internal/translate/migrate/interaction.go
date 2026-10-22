package migrate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

const (
	noV2Declaration         = ", which has no v2 declaration"
	stateInvariantCondition = "the state invariant's condition ["
	refersToWord            = "refers to "
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
	lines map[*sysmlv1.Element]*lifelineRef
	used  map[string]bool
	// params are the attribute (or in parameter) lines the interaction's own in parameters are written as.
	params     []string
	paramNotes map[*sysmlv1.Element]string
	// placed holds each message some occurrence has already stepped.
	placed map[*sysmlv1.Element]bool
	steps  []*scenarioStep
	// calls lists the call steps resolved so far, which a reply answers.
	calls []*scenarioStep
	// others are the fragments that order nothing: executions, orderings, and the
	// state invariants no assertion is written for, with the reason in unwritten.
	others    []*sysmlv1.Element
	unwritten map[*sysmlv1.Element]string
	// uses gives each interaction use its step.
	uses map[*sysmlv1.Element]*scenarioStep
	// def reports whether the sequence is written as an occurrence def of its own, which
	// declares no parameter and reaches no feature of the interaction's context.
	def bool
	// body collects the messages, occurrence usages and fragment comments in order;
	// messages counts the messages among them.
	body     []string
	messages int
}

// lifelineRef is a lifeline resolved to the usage written for it: the object it
// stands for, how the usage is declared, and the event occurrences it holds.
type lifelineRef struct {
	// after is the occurrence usage of an interaction use the lifeline's next event follows.
	after string
	line  *sysmlv1.Element
	// rep is the element the lifeline represents.
	rep *sysmlv1.Element
	// name is the usage's name; kw, prefix, mult and typing spell its declaration.
	name, kw, prefix, mult, typing string
	// bound is the feature path of the context the usage is bound to in the
	// context's occurrence usage of the def; "" when the usage stands alone.
	bound string
	// path names the object from the scenario's self; "" for an object of the interaction's own.
	path string
	typ  *sysmlv1.Element
	// from spells the object as an expression of the interaction reads it.
	from string
	// events are the usage's event occurrences in order; used holds their names.
	events []*sequenceEvent
	used   map[string]bool
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
	stepUse
	stepGate
	stepAssert
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
	// call is the call step a reply answers; perform is the path of the performance
	// a call's receiving event owns, whose result the reply carries.
	call    *scenarioStep
	perform string
	// sort is the v1 message sort.
	sort string
	body *[]*scenarioStep
	// operands are a combined fragment's, guard the comment label of each one's
	// guard; an else or unguarded operand has "".
	operands []*scenarioOperand
	// use is an interaction use's referenced interaction, whose def the occurrence
	// usage is typed by as target.
	use    *sysmlv1.Element
	target string
	// cond is the condition a state invariant's assertion checks.
	cond string
}

type scenarioOperand struct {
	e     *sysmlv1.Element
	guard string
	gnote string
	steps []*scenarioStep
}

// The note fragments the writer repeats.
const (
	theMessage  = "the message "
	standsFor   = "stands for "
	noLifeline  = "is received on no lifeline"
	theValue    = "the value "
	theArgument = "the argument "
	notMigrated = " not migrated — "
)

// interactionNote says why an interaction has no v2 form; "" when it becomes a scenario.
func (m *migration) interactionNote(e *sysmlv1.Element) string {
	_, note := m.scenario(e, "this")
	return note
}

// scenario resolves interaction e to the steps its v2 sequence orders, reading the
// context's features through self (`this` for an occurrence def of its own, which
// reads none); a note says why it has no v2 form.
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
	saved := m.self
	m.self = self
	defer func() { m.self = saved }()
	s := &scenario{
		m: m, e: e, context: context, self: self,
		order:      map[*sysmlv1.Element]int{},
		lines:      map[*sysmlv1.Element]*lifelineRef{},
		used:       map[string]bool{},
		placed:     map[*sysmlv1.Element]bool{},
		unwritten:  map[*sysmlv1.Element]string{},
		uses:       map[*sysmlv1.Element]*scenarioStep{},
		def:        self == "this",
		paramNotes: map[*sysmlv1.Element]string{},
	}
	for i, f := range e.Owned("fragment") {
		s.order[f] = i
	}
	for _, line := range e.Owned("lifeline") {
		if _, note := s.lifeline(line); note != "" {
			return nil, "the lifeline " + describe(line) + " " + note
		}
	}
	s.declareParameters()
	steps, note := s.resolve(e.Owned("fragment"), e.Owned("message"), &s.steps)
	if note == "" {
		note = gatesAdjoin(steps)
	}
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
			step, note := s.use(f, body)
			if note != "" {
				return nil, "the interaction use " + describe(f) + " " + note
			}
			steps = append(steps, step)
		case "StateInvariant":
			step, note := s.invariant(f, body)
			if note != "" {
				s.unwritten[f] = note
				s.others = append(s.others, f)
				continue
			}
			steps = append(steps, step)
		case "ExecutionOccurrenceSpecification", "BehaviorExecutionSpecification", "ActionExecutionSpecification",
			"DestructionOccurrenceSpecification", "OccurrenceSpecification", "Continuation":
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

// gatesAdjoin says why a message entering an interaction use's gate is not
// sent just before the use in the order steps are written, seq operands inlined.
func gatesAdjoin(steps []*scenarioStep) string {
	steps = inlineSeq(steps)
	for i, step := range steps {
		if step.kind != stepGate {
			for _, o := range step.operands {
				if note := gatesAdjoin(o.steps); note != "" {
					return note
				}
			}
			continue
		}
		j := i + 1
		for j < len(steps) && steps[j].kind == stepGate && steps[j].frag == step.frag {
			j++
		}
		if j < len(steps) && steps[j].kind == stepUse && steps[j].frag == step.frag {
			continue
		}
		return theMessage + describe(step.msg) + " enters the interaction use " + describe(step.frag) +
			" through its gate but is not sent just before it, and the performed scenario carries the message where the use is"
	}
	return ""
}

// inlineSeq lists steps with each seq or strict fragment replaced by its operands' steps.
func inlineSeq(steps []*scenarioStep) []*scenarioStep {
	var out []*scenarioStep
	for _, step := range steps {
		if step.kind != stepSeq {
			out = append(out, step)
			continue
		}
		for _, o := range step.operands {
			out = append(out, inlineSeq(o.steps)...)
		}
	}
	return out
}

// use resolves an interaction use: an occurrence usage typed by the referenced
// interaction's def, which the messages through its gates reach.
func (s *scenario) use(f *sysmlv1.Element, body *[]*scenarioStep) (*scenarioStep, string) {
	ref := s.m.model.Ref(f, "refersTo")
	switch {
	case ref == nil:
		return nil, "refers to no interaction" + suffixNote(s.m.dangling(f, "refersTo"))
	case ref.Type != "Interaction":
		return nil, refersToWord + describe(ref) + ", a " + ref.Type + " rather than an interaction"
	}
	if note := s.m.interactionNote(ref); note != "" {
		return nil, refersToWord + describe(ref) + ", which is not migrated: " + note
	}
	if !s.m.written(ref) {
		return nil, refersToWord + describe(ref) + noV2Declaration
	}
	step := &scenarioStep{kind: stepUse, frag: f, use: ref, body: body, target: " : " + s.m.ref(ref, s.e)}
	base := lowerFirst(s.m.nameFor(ref))
	if n := s.m.nameOf(f); n != "" {
		base = n
	}
	step.base = freshIn(s.used, base)
	step.name = writeName(step.base)
	s.returnValue(step)
	s.uses[f] = step
	return step, ""
}

// returnValue reports the result an interaction use names a recipient for: an
// occurrence usage binds no return parameter, so there is no value to assign.
func (s *scenario) returnValue(step *scenarioStep) {
	f := step.frag
	if r := s.m.model.Ref(f, "returnValueRecipient"); r != nil {
		step.note = joinNotes(step.note, "the return value is not assigned to "+describe(r)+": the occurrence usage of "+describe(step.use)+" binds no return parameter")
	} else if v := firstOwned(f, "returnValue"); v != nil {
		step.note = joinNotes(step.note, "the return value "+describeValue(v)+" is not checked: the scenario of "+describe(step.use)+" binds no return parameter")
	}
}

// gated resolves a message that enters an interaction use through one of its
// gates: the performed scenario's message leaving the matching formal gate carries it.
func (s *scenario) gated(step *scenarioStep, gate *sysmlv1.Element) (*scenarioStep, string) {
	use := gate.Parent
	if use.Type != "InteractionUse" || !within(use, s.e) {
		return nil, "is received on the gate " + describe(gate) + " of " + describe(use) + ", which is not an interaction use of the interaction"
	}
	ref := s.m.model.Ref(use, "refersTo")
	if ref == nil {
		return nil, "enters the interaction use " + describe(use) + ", which refers to no interaction"
	}
	inner := s.formalMessage(ref, s.m.nameOf(gate))
	if inner == nil {
		return nil, "enters the interaction use " + describe(use) + " through the gate " + describe(gate) + ", which no message of " + describe(ref) + " leaves"
	}
	step.kind = stepGate
	step.frag = use
	step.op = inner
	step.base, step.name = s.stepName(step.msg, "into"+upperFirst(s.m.nameFor(use)))
	if len(step.msg.Owned("argument")) > 0 {
		step.note = "the arguments it carries into the gate are those the message " + describe(inner) + " of " + describe(ref) + " states"
	}
	return step, ""
}

// formalMessage finds the message of interaction ref that leaves its formal gate named name.
func (s *scenario) formalMessage(ref *sysmlv1.Element, name string) *sysmlv1.Element {
	for _, g := range ref.Owned("formalGate") {
		if s.m.nameOf(g) != name {
			continue
		}
		for _, msg := range ref.Owned("message") {
			if s.m.model.Ref(msg, "sendEvent") == g {
				return msg
			}
		}
	}
	return nil
}

// invariant resolves a state invariant to the assertion of its condition at its
// point in the scenario: its names read the covered lifeline's object first, as
// UML evaluates it there, else the interaction's context.
func (s *scenario) invariant(f *sysmlv1.Element, body *[]*scenarioStep) (*scenarioStep, string) {
	c := firstOwned(f, "invariant")
	if c == nil {
		return nil, "the state invariant states no condition"
	}
	spec := firstOwned(c, "specification")
	if spec == nil {
		return nil, "the state invariant's constraint has no specification"
	}
	var lines []*lifelineRef
	for _, line := range s.m.model.Refs(f, "covered") {
		if r, ok := s.lines[line]; ok {
			lines = append(lines, r)
		}
	}
	if len(lines) != 1 {
		return nil, "the state invariant covers " + strconv.Itoa(len(lines)) + " lifelines of the interaction; an event occurrence of one lifeline asserts it"
	}
	cond, note, ok := "", "", false
	if lines[0].typ != nil && spec.Type == "OpaqueExpression" {
		text, lang := opaqueBody(spec)
		_, read, lost := s.m.onObject(strings.TrimSpace(text), lines[0].from, lines[0].typ)
		switch {
		case lost != "":
			return nil, stateInvariantCondition + describeValue(spec) + "] reads " + lost + " of the lifeline's object " + lines[0].path + noV2Declaration
		case read:
			// The assertion sits in the lifeline usage's body, where the object's features are in scope.
			cond, ok, note = s.m.behaviorExpr(strings.TrimSpace(text), lang, lines[0].typ)
			if !ok {
				return nil, stateInvariantCondition + describeValue(spec) + "] is not written: " + note
			}
		}
	}
	if !ok {
		var refusal string
		cond, ok, refusal = s.m.behaviorValue(spec, s.e)
		if !ok {
			return nil, stateInvariantCondition + describeValue(spec) + "] is not written: " + refusal
		}
		note = refusal
		if s.def && readsNames(cond) {
			return nil, stateInvariantCondition + describeValue(spec) + "] reads the interaction's context, which its occurrence def has no access to; only the covered lifeline's object " + lines[0].name + " is readable"
		}
	}
	if kind, _ := exprLiteral(cond); kind != "" && kind != "boolean" {
		return nil, "the state invariant's condition " + cond + " is not a Boolean"
	}
	base := s.m.nameOf(f)
	if base == "" {
		base = s.m.nameOf(c)
	}
	if base == "" {
		base = "invariant"
	}
	return &scenarioStep{kind: stepAssert, frag: f, body: body, cond: cond, note: note, sender: lines[0], base: base}, ""
}

// readsNames reports whether an expression reads any feature by name.
func readsNames(expr string) bool {
	refs, ok := exprRefs(expr)
	return !ok || len(refs) > 0
}

// onObject qualifies each name of text that is a written attribute of typ with
// path, the object it is read on; read reports whether any name was, and lost
// names an attribute of typ text reads that is not written.
func (m *migration) onObject(text, path string, typ *sysmlv1.Element) (on string, read bool, lost string) {
	refs, ok := exprRefs(text)
	if !ok {
		return "", false, ""
	}
	owned := map[string]*sysmlv1.Element{}
	for _, f := range m.attributesOf(typ) {
		owned[m.nameOf(f)] = f
	}
	var starts []int
	for _, r := range refs {
		if r.global || r.local != "" || len(r.steps) == 0 || r.steps[0].chain {
			continue
		}
		f := owned[r.steps[0].name]
		switch {
		case f == nil:
		case !m.written(f):
			return "", false, describe(f)
		default:
			starts = append(starts, r.start)
		}
	}
	if len(starts) == 0 {
		return "", false, ""
	}
	slices.Sort(starts)
	for i := len(starts) - 1; i >= 0; i-- {
		text = text[:starts[i]] + path + "." + text[starts[i]:]
	}
	return text, true, ""
}

// lifeline resolves a lifeline to the usage written for it: a part the context
// reaches, bound to it; a property or parameter of the interaction's own, declared
// here; named after the lifeline, else after what it represents.
func (s *scenario) lifeline(line *sysmlv1.Element) (*lifelineRef, string) {
	if ref, ok := s.lines[line]; ok {
		return ref, ""
	}
	rep := s.m.model.Ref(line, "represents")
	if rep == nil {
		return nil, "stands for nothing" + suffixNote(s.m.dangling(line, "represents"))
	}
	if sel := firstOwned(line, "selector"); sel != nil {
		return nil, "selects one of several " + s.m.nameOf(rep) + " by " + describeValue(sel) + ", which no usage stands for"
	}
	ref := &lifelineRef{line: line, rep: rep, typ: s.m.model.Ref(rep, "type"), used: map[string]bool{}}
	name := s.m.nameOf(line)
	if name == "" {
		name = s.m.nameOf(rep)
	}
	if name == "" {
		name = s.m.nameFor(line)
	}
	switch rep.Type {
	case "Parameter":
		if rep.Parent != s.e {
			return nil, "stands for the parameter " + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which the interaction has no access to"
		}
		if dir, _ := parameterDirection(rep); dir == "out" {
			return nil, "stands for the " + dir + " parameter " + describe(rep) + ", which holds no object when the interaction starts"
		}
		ref.prefix = "ref "
		ref.kw, ref.typing = s.lifelineTyping(ref.typ)
	case "Property", "Port":
		if rep.Parent == s.e {
			// The lifeline is the property's own usage, under the property's name.
			name = s.m.nameOf(rep)
			s.ownLifeline(ref)
			break
		}
		if !s.m.written(rep) {
			return nil, standsFor + describe(rep) + " of " + qualifiedName(rep.Parent) + noV2Declaration
		}
		paths := s.m.partPaths(s.context, rep)
		switch len(paths) {
		case 0:
			return nil, standsFor + describe(rep) + " of " + qualifiedName(rep.Parent) + ", which no part of " + qualifiedName(s.context) + " reaches"
		case 1:
		default:
			return nil, standsFor + describe(rep) + ", which " + qualifiedName(s.context) + " reaches as both " + paths[0] + " and " + paths[1]
		}
		ref.prefix = "ref "
		ref.kw, ref.typing = s.lifelineTyping(ref.typ)
		if s.def {
			// The def reaches no feature of the context; its occurrence usage in the context binds the lifeline.
			ref.bound = paths[0]
			ref.path = paths[0]
		} else {
			ref.path = joinDot(s.self, paths[0])
			ref.typing = " :> " + ref.path
		}
	default:
		return nil, standsFor + describe(rep) + ", a " + rep.Type + " rather than a part or parameter"
	}
	ref.name = freshIn(s.used, name)
	ref.from = writeName(ref.name)
	s.lines[line] = ref
	return ref, ""
}

// ownLifeline declares a lifeline standing for a property of the interaction's own
// as that property would be declared: by its type, aggregation and multiplicity.
func (s *scenario) ownLifeline(ref *lifelineRef) {
	ref.kw, ref.prefix, _ = s.m.featureKeyword(ref.rep, catOccurrenceDef)
	if ref.rep.Type == "Port" {
		ref.kw, ref.prefix = "port", ""
	}
	if typ, _ := s.m.typeRef(ref.typ, s.e); typ != "" {
		ref.typing = s.m.typing(ref.typ) + typ
	}
	switch ref.kw {
	case "part", "item", "occurrence", "port":
	default:
		// A lifeline is an occurrence: an untyped property is a part, one typed by a value an occurrence of no type.
		ref.kw = "occurrence"
		if ref.typ == nil {
			ref.kw = "part"
		}
		ref.typing = ""
	}
	ref.mult, _ = s.m.multiplicity(ref.rep)
}

// lifelineTyping spells the usage keyword and typing of a lifeline by the type of
// the object it stands for: a part, item or occurrence usage typed by the type's
// def; an occurrence usage, untyped, when the type has no such def.
func (s *scenario) lifelineTyping(t *sysmlv1.Element) (kw, typing string) {
	if t == nil {
		return "part", ""
	}
	kw, _ = s.m.typeKeyword(t)
	switch kw {
	case "part", "item", "occurrence", "port":
	default:
		return "occurrence", ""
	}
	typ, _ := s.m.typeRef(t, s.e)
	if typ == "" {
		return kw, ""
	}
	return kw, s.m.typing(t) + typ
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
	step := &scenarioStep{msg: msg, body: body, sort: sort}
	var note string
	if step.sender, note = s.end(msg, "sendEvent"); note != "" {
		return nil, "is sent from " + note
	}
	if gate := s.m.model.Ref(msg, "receiveEvent"); gate != nil && gate.Type == "Gate" && gate.Parent != s.e {
		return s.gated(step, gate)
	}
	if step.receiver, note = s.end(msg, "receiveEvent"); note != "" {
		return nil, "is received on " + note
	}
	switch sort {
	case "asynchSignal":
		return s.send(step)
	case "synchCall", "asynchCall":
		return s.call(step)
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
	step.base, step.name = s.stepName(msg, strings.TrimSuffix(sort, "Message")+upperFirst(step.receiver.name))
	return step, ""
}

// end resolves the lifeline the occurrence in role of msg covers; nil when the
// message has no such occurrence (a found or lost message).
func (s *scenario) end(msg *sysmlv1.Element, role string) (*lifelineRef, string) {
	ev := s.m.model.Ref(msg, role)
	if ev == nil || ev.Type == "Gate" && ev.Parent == s.e {
		// A formal gate is the interaction's edge: the message comes from or goes to its environment.
		return nil, ""
	}
	if ev.Type == "Gate" {
		return nil, "the gate " + describe(ev) + " of " + describe(ev.Parent) + ", which no lifeline of the interaction stands behind"
	}
	line := s.m.model.Ref(ev, "covered")
	if line == nil || line.Type != "Lifeline" {
		return nil, "no lifeline" + suffixNote(s.m.dangling(ev, "covered"))
	}
	ref, note := s.lifeline(line)
	if note != "" {
		return nil, "the lifeline " + describe(line) + ", which " + note
	}
	return ref, ""
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
	step.args, step.note = s.bindArguments(step.msg, s.m.signalAttributes(sig), "attribute", sig, nil)
	step.base, step.name = s.stepName(step.msg, "send"+upperFirst(s.m.nameFor(sig)))
	return step, ""
}

// call resolves an operation call: the receiver's type must have the operation,
// which its receiving event performs with the arguments bound to its in parameters.
func (s *scenario) call(step *scenarioStep) (*scenarioStep, string) {
	op := s.m.model.Ref(step.msg, "signature")
	if step.receiver == nil {
		return nil, noLifeline
	}
	if op == nil {
		// A call of no operation is still a message between its events; it performs nothing.
		step.kind = stepCall
		step.note = "the call names no operation" + suffixNote(s.m.dangling(step.msg, "signature")) + ", so its receiving event performs nothing"
		step.base, step.name = s.stepName(step.msg, "call")
		return step, ""
	}
	if op.Type != "Operation" || !s.m.written(op) {
		return nil, "names " + describe(op) + ", which is not a migrated operation"
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
	step.args, step.note = s.bindArguments(step.msg, ins, "parameter", op, step.receiver)
	step.kind = stepCall
	step.op = op
	step.base, step.name = s.stepName(step.msg, "call"+upperFirst(s.m.nameFor(op)))
	s.calls = append(s.calls, step)
	return step, ""
}

// reply resolves a reply: it answers the latest unanswered call of its operation
// between the same lifelines, and carries the call's result as its payload.
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
		if p == nil {
			step.note = joinNotes(step.note, theValue+describeValue(arg)+" has no out parameter of "+op.Name+" to stand for")
			continue
		}
		target, value := assignmentOf(arg)
		if target != "" {
			step.note = joinNotes(step.note, "the caller's "+target+" is not assigned the result "+s.m.nameOf(p)+": a sequence states transfers, not assignments")
		}
		switch {
		case value != "":
			step.note = joinNotes(step.note, theValue+value+" the reply states for "+s.m.nameOf(p)+" is the operation's own result, which the call computes")
		case target == "":
			step.note = joinNotes(step.note, theValue+describeValue(arg)+" of "+s.m.nameOf(p)+" is the operation's own result, which the call computes")
		}
	}
	step.base, step.name = s.stepName(step.msg, "reply"+upperFirst(s.m.nameFor(op)))
	return step, ""
}

func sameLine(a, b *lifelineRef) bool {
	return a != nil && b != nil && a.line == b.line
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
// parameters (each with its direction) or attributes, by name or position; an argument
// that is not written, or that reads what the occurrence def cannot, leaves its target
// unbound with a note, as does a target that must hold a value and that none binds.
// line is the lifeline whose body the bindings are written in; nil for the def's body.
func (s *scenario) bindArguments(msg *sysmlv1.Element, targets []*sysmlv1.Element, kind string, owner *sysmlv1.Element, line *lifelineRef) (args []string, note string) {
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
			note = joinNotes(note, theArgument+describeValue(arg)+" for "+s.m.nameOf(t)+" is not written, which stays unbound: "+vnote)
			continue
		}
		if s.def {
			if lost := s.inaccessible(expr, line); lost != "" {
				note = joinNotes(note, theArgument+describeValue(arg)+" for "+s.m.nameOf(t)+" reads "+lost+" of the interaction's context, which its occurrence def has no access to; the "+kind+" stays unbound")
				continue
			}
		}
		bound[t] = true
		if kind == "parameter" {
			if !s.def {
				expr = s.m.callBodyExpr(expr, s.e)
			}
			args = append(args, s.m.parameterBinding(t, s.m.nameOf(t), expr))
		} else {
			args = append(args, writeName(s.m.nameOf(t))+" = "+expr)
		}
		note = joinNotes(note, vnote)
	}
	for _, t := range targets {
		if !bound[t] && requiresValue(t) {
			note = joinNotes(note, "no argument binds the "+kind+" "+s.m.nameOf(t)+" of "+owner.Name+", which must hold a value; it stays unbound")
		}
	}
	return args, note
}

// inaccessible names the first feature expr reads that the occurrence def cannot
// reach: neither a lifeline usage nor, within line's body, a feature of its object.
func (s *scenario) inaccessible(expr string, line *lifelineRef) string {
	refs, ok := exprRefs(expr)
	if !ok {
		return ""
	}
	for _, r := range refs {
		if r.global || r.local != "" || len(r.steps) == 0 || r.steps[0].chain {
			continue
		}
		first := r.steps[0].name
		if s.lifelineNamed(first) != nil || line != nil && s.attributeNamed(line, first) != nil || s.parameterNamed(first) {
			continue
		}
		return first
	}
	return ""
}

// parameterNamed reports whether the interaction's own parameters are written as an attribute of the name.
func (s *scenario) parameterNamed(name string) bool {
	for _, p := range sequenceParameters(s.e) {
		if s.m.sequenceParams[p] == name {
			return true
		}
	}
	return false
}

// lifelineNamed finds the lifeline usage of the given name.
func (s *scenario) lifelineNamed(name string) *lifelineRef {
	for _, ref := range s.lines {
		if ref.name == name {
			return ref
		}
	}
	return nil
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

// fragment resolves a combined fragment to its operands' steps in order, each
// operand labelled by the operator and its guard.
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
		// critical, neg, assert, ignore, consider, break: one operand written in order under comments.
		step.kind = stepSeq
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
	step.base = kind
	return step, ""
}

// fragmentOperands resolves each operand's steps and collects the calls each
// leaves open; isolated operands all start from the calls open before the fragment.
func (s *scenario) fragmentOperands(step *scenarioStep, kind string, operands []*sysmlv1.Element, in []*scenarioStep, body *[]*scenarioStep) (outs [][]*scenarioStep, err string) {
	isolated := step.kind != stepSeq
	for _, o := range operands {
		operand := &scenarioOperand{e: o}
		var steps []*scenarioStep
		operand.steps = steps
		guard := firstOwned(o, "guard")
		var note string
		s.operandGuard(step, operand, guard)
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

// operandGuard labels an operand by its guard: the v2 expression in brackets, or
// the guard's own text when it has no v2 expression, with a note; a loop's bounds too.
func (s *scenario) operandGuard(step *scenarioStep, operand *scenarioOperand, guard *sysmlv1.Element) {
	if step.kind == stepLoop {
		operand.guard, operand.gnote = s.loopLabel(guard)
		return
	}
	operand.guard, operand.gnote = s.guardLabel(guard)
}

// guardLabel is the bracketed label of a guard; "" for none, true or else.
func (s *scenario) guardLabel(g *sysmlv1.Element) (label, note string) {
	if g == nil || trueGuard(g) {
		return "", ""
	}
	spec := firstOwned(g, "specification")
	if isElseGuard(spec) {
		return "", ""
	}
	expr, note, refusal := s.guard(g)
	if refusal != "" {
		return "[" + describeValue(spec) + "]", "the guard has no v2 expression and is kept as its own text: " + refusal
	}
	if s.def {
		if lost := s.inaccessible(expr, nil); lost != "" {
			return "[" + describeValue(spec) + "]", "the guard reads " + lost + " of the interaction's context, which its occurrence def has no access to, and is kept as its own text"
		}
	}
	return "[" + expr + "]", note
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

// loopLabel labels a loop's operand by its bounds and guard: `(min, max) [guard]`.
func (s *scenario) loopLabel(g *sysmlv1.Element) (label, note string) {
	var lo, hi string
	if g != nil {
		if v := firstOwned(g, "minint"); v != nil {
			lo = describeValue(v)
		}
		if v := firstOwned(g, "maxint"); v != nil {
			hi = describeValue(v)
		}
	}
	label, note = s.guardLabel(g)
	if lo != "" || hi != "" {
		label = strings.TrimSpace("(" + orAny(lo) + ", " + orAny(hi) + ") " + label)
	}
	return label, note
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

// interactionBody writes an interaction as the body of its occurrence def: a usage
// per lifeline holding its event occurrences in order, and a message per v1 message.
func (m *migration) interactionBody(e *sysmlv1.Element) {
	m.unwrittenMembers(e, "lifeline", "message", "fragment", "generalOrdering", "ownedRule", "observation", "formalGate", "ownedAttribute")
	s, note := m.scenario(e, "this")
	if note != "" {
		// classifyBehavior does not let this happen; keep the body honest anyway.
		m.w.lines(commentLines("interaction not migrated — " + note))
		return
	}
	s.writeSequence()
}

// sequenceEvent is one event occurrence of a lifeline usage, with the body it owns.
type sequenceEvent struct {
	name string
	body []string
}

// lifelineBinding binds a lifeline usage of an interaction's occurrence def to the
// feature of the owner it stands for, in the owner's occurrence usage of the def.
type lifelineBinding struct {
	kw, name, path string
}

// writeSequence writes the scenario as a sequence: the lifeline usages, each with
// its event occurrences ordered by then, and the messages between the events in
// occurrence order; every element they stand for is reported.
func (s *scenario) writeSequence() {
	saved := s.m.self
	s.m.self = s.self
	defer func() { s.m.self = saved }()
	s.place(s.steps)
	s.m.w.lines(s.params)
	var bindings []lifelineBinding
	for _, line := range s.e.Owned("lifeline") {
		ref := s.lines[line]
		s.writeLifeline(ref)
		s.m.names[line] = ref.name
		note := "written as the " + strings.TrimSpace(ref.prefix+ref.kw) + " usage " + ref.name + " holding its " + plural(len(ref.events), "event occurrence")
		switch {
		case ref.bound != "":
			bindings = append(bindings, lifelineBinding{kw: ref.kw, name: ref.name, path: ref.bound})
			note += ", bound to " + ref.bound + " of " + qualifiedName(s.context) + " where the def occurs"
		case ref.rep.Parent == s.e:
			note += ", the interaction's own " + describe(ref.rep)
		case ref.path != "":
			note += ", subsetting " + ref.path
		}
		s.m.add(line, Mapped, s.m.v2Name(s.e)+"::"+ref.name, note)
	}
	s.m.w.lines(s.body)
	if s.def {
		s.m.sequenceBindings[s.e] = bindings
		s.ownMembers()
	}
	for _, g := range s.e.Owned("formalGate") {
		s.m.add(g, Mapped, "", "the gate is the interaction's edge: a message leaving or entering it states no end there, as v2 sequence notation has no gate")
	}
	for _, f := range s.others {
		s.other(f)
	}
	for _, o := range s.e.Owned("generalOrdering") {
		s.m.add(o, Skipped, "", "the general ordering is not written: each lifeline's events are ordered by then successions in fragment order")
	}
	for _, r := range s.e.Owned("ownedRule") {
		s.m.unmapped(r, "a "+r.Type+" has no form in v2 sequence notation; the events it spans are ordered but not timed")
	}
	for _, o := range s.e.Owned("observation") {
		s.m.unmapped(o, "a "+o.Type+" has no v2 form")
	}
	lifelines := plural(len(s.e.Owned("lifeline")), "lifeline usage")
	note := "written as an occurrence def of " + lifelines + " and " + plural(s.messages, "message") +
		" between their event occurrences in occurrence order"
	if !s.def {
		note = "written as a verification def of " + lifelines + " and " + plural(s.messages, "message") +
			" between their event occurrences in occurrence order, the lifelines bound to the subject's parts"
	}
	if has(s.e, "TestCase") {
		note = joinNotes(note, s.verdictNote())
	}
	s.m.add(s.e, Mapped, s.m.v2Name(s.e), note)
}

// ownMembers reports the interaction's own parameters and properties: a property a
// lifeline stands for is that lifeline's usage; an occurrence def declares no parameter.
func (s *scenario) ownMembers() {
	represented := map[*sysmlv1.Element]*lifelineRef{}
	for _, ref := range s.lines {
		if ref.rep.Parent == s.e {
			represented[ref.rep] = ref
		}
	}
	for _, p := range s.e.Owned("ownedParameter") {
		if ref, ok := represented[p]; ok {
			s.m.add(p, Approximated, s.m.v2Name(s.e)+"::"+ref.name, "the parameter is the object the lifeline "+describe(ref.line)+" stands for, written as the usage "+ref.name+"; an occurrence def declares no parameter")
			continue
		}
		if name, ok := s.m.sequenceParams[p]; ok {
			s.m.add(p, Approximated, s.m.v2Name(s.e)+"::"+name, joinNotes("written as the attribute "+name+" of the occurrence def, which declares no parameter; the occurrence usages of the def bind it", s.paramNotes[p]))
			continue
		}
		s.m.unmapped(p, "an occurrence def declares no parameter, and no lifeline stands for its "+p.Attrs["direction"]+" parameter")
	}
	for _, p := range s.e.Owned("ownedAttribute") {
		if ref, ok := represented[p]; ok {
			s.m.add(p, Mapped, s.m.v2Name(s.e)+"::"+ref.name, "written as the usage "+ref.name+", the lifeline "+describe(ref.line)+" that stands for it")
			continue
		}
		s.m.unmapped(p, "owned by an Interaction, whose v2 body is its lifelines and messages, not a place for a "+kindOf(p)+" no lifeline stands for")
	}
}

// verdictNote says how the verification case written from a test case reaches a
// verdict: a return parameter is its verdict, and without one it stays inconclusive.
func (s *scenario) verdictNote() string {
	for _, p := range s.e.Owned("ownedParameter") {
		if p.Attrs["direction"] == "return" {
			return "its verdict is the return parameter " + s.m.nameFor(p) + ", which no message binds"
		}
	}
	return "the test case has no return parameter, so the verification case states no verdict and is inconclusive once its events complete"
}

// writeLifeline writes a lifeline's usage with its event occurrences, each after the one before.
func (s *scenario) writeLifeline(ref *lifelineRef) {
	head := ref.prefix + ref.kw + " " + writeName(ref.name) + ref.mult + ref.typing
	if len(ref.events) == 0 {
		s.m.w.line(head + ";")
		return
	}
	s.m.w.block(head, func() {
		for i, ev := range ref.events {
			line := "event occurrence " + writeName(ev.name)
			if i > 0 {
				line = "then " + line
			}
			if len(ev.body) == 0 {
				s.m.w.line(line + ";")
				continue
			}
			s.m.w.block(line, func() {
				s.m.w.lines(ev.body)
			})
		}
	})
}

// event adds an event occurrence to a lifeline usage for the occurrence ev (named
// after it, or fallback when it is anonymous) and names it as a message end.
func (s *scenario) event(ref *lifelineRef, ev *sysmlv1.Element, fallback string) *sequenceEvent {
	name := ""
	if ev != nil {
		name = s.m.nameOf(ev)
	}
	if name == "" {
		name = fallback
	}
	e := &sequenceEvent{name: freshIn(ref.used, name)}
	ref.events = append(ref.events, e)
	if ref.after != "" {
		s.body = append(s.body, "succession first "+ref.after+" then "+endRef(ref, e)+";")
		ref.after = ""
	}
	return e
}

// endRef spells an event of a lifeline as a message end.
func endRef(ref *lifelineRef, ev *sequenceEvent) string {
	return writeName(ref.name) + "." + writeName(ev.name)
}

// place lays the steps out in order: an event on each lifeline a message touches
// and the message between them, a combined fragment's operands under comments.
func (s *scenario) place(steps []*scenarioStep) {
	for _, step := range steps {
		switch step.kind {
		case stepSend, stepCall, stepReply, stepCreate, stepDelete, stepGate:
			s.placeMessage(step)
		case stepUse:
			s.placeUse(step)
		case stepAssert:
			s.placeAssert(step)
		default:
			s.placeFragment(step)
		}
	}
}

// placeFragment writes a combined fragment's operands in order, each under a comment
// naming the operator and its guard: v2 sequence notation has no fragment of its own.
func (s *scenario) placeFragment(step *scenarioStep) {
	kind := step.base
	for i, o := range step.operands {
		label := kind
		switch {
		case i > 0 && step.kind == stepAlt:
			label = "else"
		case i > 0 && step.kind == stepPar:
			label = "and"
		case i > 0:
			label = "then"
		}
		if o.guard != "" {
			label += " " + o.guard
		}
		s.body = append(s.body, commentLines(label)...)
		s.place(o.steps)
		s.m.add(o.e, Approximated, "", joinNotes("the operand's messages are written in occurrence order under the comment "+label, o.gnote))
	}
	s.body = append(s.body, commentLines("end "+kind)...)
	note := "the " + kind + " fragment has no form in v2 sequence notation: its operands' messages are written in occurrence order, the operator and guards as comments"
	if step.kind == stepPar {
		note += "; the concurrent operands are written one after the other"
	}
	s.m.add(step.frag, Approximated, "", note)
}

// placeUse writes an interaction use as an occurrence usage of the referenced
// interaction's def; the messages entering its gates reach the usage.
func (s *scenario) placeUse(step *scenarioStep) {
	bindings, note := s.useArguments(step)
	note = joinNotes(note, step.note)
	line := "occurrence " + step.name + step.target
	if len(bindings) > 0 {
		line += " { " + strings.Join(bindings, " ") + " }"
	} else {
		line += ";"
	}
	var ordered []string
	for _, covered := range s.m.model.Refs(step.frag, "covered") {
		ref := s.lines[covered]
		if ref == nil {
			continue
		}
		if n := len(ref.events); n > 0 {
			at := endRef(ref, ref.events[n-1])
			s.body = append(s.body, "succession first "+at+" then "+step.name+";")
			ordered = append(ordered, at)
		}
		ref.after = step.name
	}
	report := "written as the occurrence usage " + step.name + " of " + describe(step.use)
	if len(ordered) > 0 {
		report = joinNotes(report, "ordered after "+strings.Join(ordered, ", ")+" and before the covered lifelines' next events by successions")
	}
	s.body = append(s.body, line)
	s.m.add(step.frag, verdictFor(note), step.name, joinNotes(report, note))
}

// useArguments binds an interaction use's arguments to the attributes the referenced
// interaction's parameters are written as, by name or position, in the occurrence usage.
func (s *scenario) useArguments(step *scenarioStep) (bindings []string, note string) {
	args := step.frag.Owned("argument")
	if len(args) == 0 {
		return nil, ""
	}
	targets := sequenceParameters(step.use)
	bound := map[*sysmlv1.Element]bool{}
	for i, t := range s.pairArguments(args, targets) {
		arg := args[i]
		switch {
		case t == nil:
			note = joinNotes(note, theArgument+describeValue(arg)+" has no parameter of "+step.use.Name+" to bind to and is dropped")
			continue
		case bound[t]:
			note = joinNotes(note, theArgument+describeValue(arg)+" binds "+s.m.nameOf(t)+" a second time and is dropped")
			continue
		}
		name, ok := s.m.sequenceParams[t]
		if !ok {
			note = joinNotes(note, theArgument+describeValue(arg)+" for "+s.m.nameOf(t)+" is dropped: the parameter has no v2 declaration")
			continue
		}
		expr, ok, vnote := s.m.typedBehaviorValue(arg, t, s.e)
		if !ok {
			note = joinNotes(note, theArgument+describeValue(arg)+" for "+s.m.nameOf(t)+" is not written, which stays unbound: "+vnote)
			continue
		}
		if s.def {
			if lost := s.inaccessible(expr, nil); lost != "" {
				note = joinNotes(note, theArgument+describeValue(arg)+" for "+s.m.nameOf(t)+" reads "+lost+" of the interaction's context, which its occurrence def has no access to; the parameter stays unbound")
				continue
			}
		}
		bound[t] = true
		bindings = append(bindings, "attribute :>> "+writeName(name)+" = "+expr+";")
	}
	return bindings, note
}

// sequenceParameters are an interaction's parameters that take a value in: those
// its occurrence def declares as attributes.
func sequenceParameters(e *sysmlv1.Element) []*sysmlv1.Element {
	var in []*sysmlv1.Element
	for _, p := range e.Owned("ownedParameter") {
		switch p.Attrs["direction"] {
		case "", "in", "inout":
			in = append(in, p)
		}
	}
	return in
}

// declareParameters names the attributes the interaction's in parameters are written
// as, before the lifelines take their names; a parameter a lifeline stands for is that usage.
func (s *scenario) declareParameters() {
	for _, p := range sequenceParameters(s.e) {
		represented := false
		for _, ref := range s.lines {
			if ref.rep == p {
				represented = true
			}
		}
		if represented {
			continue
		}
		if !s.def {
			// verificationBody writes a test case's parameters; the bindings name them.
			s.m.sequenceParams[p] = s.m.nameFor(p)
			continue
		}
		name := freshIn(s.used, s.m.nameOf(p))
		s.m.sequenceParams[p] = name
		line := "attribute " + writeName(name)
		if mult, _ := s.m.multiplicity(p); mult != "" {
			line += mult
		}
		if typ, _ := s.m.typeRef(s.m.model.Ref(p, "type"), s.e); typ != "" {
			line += s.m.typing(s.m.model.Ref(p, "type")) + typ
		}
		if dv := firstOwned(p, "defaultValue"); dv != nil {
			expr, ok, vnote := s.m.typedBehaviorValue(dv, p, s.e)
			if ok {
				line += " default = " + expr
			} else {
				vnote = "default value not migrated: " + describeValue(dv) + " — " + vnote
				s.params = append(s.params, commentLines(vnote)...)
			}
			s.paramNotes[p] = vnote
		}
		s.params = append(s.params, line+";")
	}
}

// placeAssert writes a state invariant as an event occurrence of the covered
// lifeline that asserts its condition.
func (s *scenario) placeAssert(step *scenarioStep) {
	ev := s.event(step.sender, step.frag, step.base)
	ev.body = append(ev.body, "assert constraint { "+step.cond+" }")
	at := endRef(step.sender, ev)
	s.m.add(step.frag, verdictFor(step.note), at, joinNotes("written as the event occurrence "+at+", which asserts the condition where the invariant holds", step.note))
	if c := firstOwned(step.frag, "invariant"); c != nil {
		s.m.add(c, Mapped, at, "the constraint is the condition asserted at the event occurrence "+at)
	}
}

// placeMessage writes a message between the events of its lifelines: a signal is
// the payload, a call is performed at the receiving event, a reply carries the
// result, and a create or delete message is an ordinary message to the object.
func (s *scenario) placeMessage(step *scenarioStep) {
	var from, to string
	var sent, received *sequenceEvent
	if step.sender != nil {
		sent = s.event(step.sender, s.m.model.Ref(step.msg, "sendEvent"), step.base+"Sent")
		from = endRef(step.sender, sent)
	}
	if step.kind == stepGate {
		if u := s.uses[step.frag]; u != nil {
			to = u.name
		}
	} else if step.receiver != nil {
		received = s.event(step.receiver, s.m.model.Ref(step.msg, "receiveEvent"), step.base+"Received")
		to = endRef(step.receiver, received)
	}
	payload, note := s.payload(step, received)
	line := "message " + step.name
	if payload != "" {
		line += " of " + payload
	}
	switch {
	case from != "" && to != "":
		line += " from " + from + " to " + to
	case from != "":
		note = joinNotes(note, "the message goes to the interaction's gate "+s.gateName(step.msg, "receiveEvent")+", its environment: v2 sequence notation has no gate, so the message states no target")
	case to != "":
		note = joinNotes(note, "the message comes from the interaction's gate "+s.gateName(step.msg, "sendEvent")+", its environment: v2 sequence notation has no gate, so the message states no source")
	}
	s.body = append(s.body, line+";")
	s.messages++
	s.m.wroteEdge(step.msg, s.e, "message", step.base)
	s.m.add(step.msg, verdictFor(joinNotes(note, step.note)), step.name, joinNotes(note, step.note))
	s.occurrencesDone(step, from, to)
}

// gateName names the formal gate at a message's end.
func (s *scenario) gateName(msg *sysmlv1.Element, role string) string {
	if g := s.m.model.Ref(msg, role); g != nil {
		return describe(g)
	}
	return "(unnamed)"
}

// payload spells what a message carries, and notes what of the v1 message it
// does not: a signal and its arguments, a call's result on the reply.
func (s *scenario) payload(step *scenarioStep, received *sequenceEvent) (payload, note string) {
	switch step.kind {
	case stepSend:
		sig := s.m.ref(step.signal, s.e)
		if len(step.args) == 0 {
			return sig + "[1]", ""
		}
		name := lowerFirst(s.m.nameFor(step.signal))
		if name == step.base {
			name += "Payload"
		}
		return writeName(name) + " : " + sig + " = new " + sig + "(" + strings.Join(step.args, ", ") + ")", ""
	case stepCall:
		if received == nil || step.op == nil {
			return "", ""
		}
		usage := writeName(s.m.operationUsage(step.op))
		name := step.base
		if name == usage {
			name = freshIn(step.receiver.used, step.base+"Call")
		}
		decl := performActionKeyword + writeName(name)
		if s.m.asUsage[step.op] {
			// Qualified through the operation's def: a message of the operation's name is a sibling of the lifeline.
			decl += " ::> " + s.m.ref(step.op.Parent, s.e) + "::" + usage
		} else {
			decl += " : " + s.m.ref(step.op, s.e)
		}
		if len(step.args) > 0 {
			decl += " { " + strings.Join(step.args, "; ") + "; }"
		} else {
			decl += ";"
		}
		received.body = append(received.body, decl)
		step.perform = endRef(step.receiver, received) + "." + writeName(name)
		note = "the call is the message; its receiving event performs " + s.m.nameOf(step.op) + " on " + step.receiver.name
		if step.sort == "asynchCall" {
			note += ", to completion within the event"
		}
		return "", note
	case stepReply:
		return s.replyPayload(step)
	case stepCreate:
		return "", "the message creates " + step.receiver.name + ": v2 sequence notation has no creation message, so it is an ordinary message to the object's first event"
	case stepDelete:
		return "", "the message deletes " + step.receiver.name + ": v2 sequence notation has no deletion message, so it is an ordinary message to the object's event"
	}
	return "", ""
}

// replyPayload carries the answered call's one result as the reply's payload.
func (s *scenario) replyPayload(step *scenarioStep) (payload, note string) {
	call := step.call
	var outs []*sysmlv1.Element
	for _, p := range s.m.actionParameters(step.op) {
		if dir, _ := parameterDirection(p); dir != "in" {
			outs = append(outs, p)
		}
	}
	switch {
	case len(outs) == 0:
		return "", ""
	case call.perform == "":
		return "", "the call it answers performs nothing, so the reply carries no result"
	case len(outs) > 1:
		return "", "the reply carries no result: " + s.m.nameOf(step.op) + " has " + strconv.Itoa(len(outs)) + " results, and a message carries one payload"
	}
	p := outs[0]
	typ, tnote := s.m.typeRef(s.m.model.Ref(p, "type"), s.e)
	if typ == "" {
		return "", joinNotes("the reply carries no result: the result "+s.m.nameOf(p)+" has no written type to carry it as", tnote)
	}
	name := writeName(s.m.nameOf(p))
	return name + " : " + typ + " = " + call.perform + "." + name, joinNotes("the reply carries the result "+s.m.nameOf(p)+" of the call as its payload", tnote)
}

// occurrencesDone reports the occurrences at a message's ends as the events written for them.
func (s *scenario) occurrencesDone(step *scenarioStep, from, to string) {
	for _, end := range []struct{ role, at string }{{"sendEvent", from}, {"receiveEvent", to}} {
		role, at := end.role, end.at
		ev := s.m.model.Ref(step.msg, role)
		if ev == nil || ev.Type == "Gate" {
			continue
		}
		if at == "" {
			s.m.add(ev, Unmapped, "", "the occurrence belongs to the message "+describe(step.msg)+", whose end it is has no v2 form")
			continue
		}
		verb := "leaves"
		if role == "receiveEvent" {
			verb = "reaches"
		}
		s.m.add(ev, Mapped, at, "written as the event occurrence "+at+", which the message "+step.name+" "+verb)
	}
}

// other reports a fragment that orders nothing the events do not already order.
func (s *scenario) other(f *sysmlv1.Element) {
	switch f.Type {
	case "BehaviorExecutionSpecification", "ActionExecutionSpecification", "ExecutionOccurrenceSpecification":
		s.m.add(f, Skipped, "", "the execution spans the events between its start and finish, which the then successions order without it")
	case "StateInvariant":
		s.m.add(f, Unmapped, "", s.unwritten[f])
	case "DestructionOccurrenceSpecification":
		s.m.add(f, Unmapped, "", "the destruction has no form in v2 sequence notation; the object exists for as long as its owner does")
	default:
		s.m.add(f, Skipped, "", "the occurrence orders nothing beyond the events' order")
	}
}

// interactionOccurrences writes, for each interaction of c whose lifelines stand
// for features of c, the occurrence usage of its def that binds them to those features.
func (m *migration) interactionOccurrences(c *sysmlv1.Element) {
	for _, b := range c.Owned("ownedBehavior") {
		bindings := m.sequenceBindings[b]
		if b.Type != "Interaction" || len(bindings) == 0 {
			continue
		}
		name := m.freshName(c, lowerFirst(m.nameFor(b)))
		m.w.madeUp(writeName(name))
		m.w.block("occurrence "+writeName(name)+" : "+m.ref(b, c), func() {
			for _, bd := range bindings {
				m.w.line("ref " + bd.kw + " :>> " + writeName(bd.name) + " = " + bd.path + ";")
			}
		})
		m.add(b, Mapped, "", "its lifelines are bound to the features of "+qualifiedName(c)+" they stand for in the occurrence usage "+name)
	}
}
