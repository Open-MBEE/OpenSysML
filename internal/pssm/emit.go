package pssm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
)

// Model is one test's state machine spelled in SysML v2 textual notation,
// with the events the tester's stimulation queues to drive it. The machine
// keeps a String attribute `log` to which every trace(...) call appends its
// segment, "::"-joined, so the final log is comparable with Test.Expected.
type Model struct {
	Test *Test
	// Name is the model's file name, e.g. "Deferred001_SemanticTest.sysml".
	Name string
	// Package and Machine are the emitted package and the state usage inside
	// it; Qualified is the machine's qualified name, e.g. "Deferred001_SemanticTest::M".
	Package   string
	Machine   string
	Qualified string
	// Text is the model.
	Text string
	// Events are the queued events, in order.
	Events []Stimulus
}

// Stimulus is one event the tester sends the target: a signal, with its scalar
// payload when the signal carries one, or an operation call with its arguments.
type Stimulus struct {
	Signal string
	Call   string
	// Value is the scalar payload of a signal, nil for a plain signal.
	Value *Literal
	// Args are the call's arguments in parameter order.
	Args []Argument
}

// Argument is one argument of a queued operation call.
type Argument struct {
	Name  string
	Value *Literal
}

// String spells the stimulus for a report.
func (s Stimulus) String() string {
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

// TranslateError reports a construct of a test the emitter has no exact
// translation for. It never drops the construct instead.
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

// scalarTypes maps the UML primitive types the suite uses to ScalarValues.
var scalarTypes = map[string]string{
	"Boolean":          "Boolean",
	"Integer":          "Integer",
	"String":           "String",
	"Real":             "Real",
	"UnlimitedNatural": "Natural",
}

// Emit translates one expressible test into a model. The signals of the suite
// resolve the sends' payloads.
func Emit(s *Suite, t *Test) (*Model, error) {
	if t.Machine == nil {
		return nil, &TranslateError{Test: t.ID, Reason: "no state machine"}
	}
	e := &emitter{suite: s, test: t, names: map[*Vertex]string{}, signals: map[string]bool{}}
	e.nameVertices(t.Machine.Regions)
	var body strings.Builder
	if err := e.machine(&body); err != nil {
		return nil, err
	}
	events, err := e.stimulation()
	if err != nil {
		return nil, err
	}
	var text strings.Builder
	fmt.Fprintf(&text, "package %s {\n", t.ID)
	text.WriteString("    private import ScalarValues::*;\n")
	text.WriteString("    private import BaseFunctions::*;\n")
	for _, name := range sortedKeys(e.signals) {
		sig := s.Signals[name]
		switch {
		case sig == nil || len(sig.Attributes) == 0:
			fmt.Fprintf(&text, "    attribute def %s;\n", name)
		case len(sig.Attributes) == 1 && scalarTypes[sig.Attributes[0].Type] != "":
			fmt.Fprintf(&text, "    attribute def %s :> %s;\n", name, scalarTypes[sig.Attributes[0].Type])
		default:
			return nil, &TranslateError{Test: t.ID, Where: "signal " + name, Reason: "a signal with a structured payload has no scalar binding"}
		}
	}
	text.WriteString(body.String())
	text.WriteString("}\n")
	return &Model{
		Test:      t,
		Name:      t.ID + ".sysml",
		Package:   t.ID,
		Machine:   machineName,
		Qualified: t.ID + "::" + machineName,
		Text:      text.String(),
		Events:    events,
	}, nil
}

// machineName is the state usage every emitted model declares.
const machineName = "M"

type emitter struct {
	suite *Suite
	test  *Test
	// names are the emitted names of the machine's vertices, unique per machine.
	names map[*Vertex]string
	// signals are the signals the model references, to be declared.
	signals map[string]bool
	// placed are transitions emitted in a scope other than their own region's.
	placed map[*Region][]*Transition
	// carried maps a triggered transition to the attribute its scalar payload is
	// stored in, for a guard on a pseudostate downstream that reads it.
	carried map[*Transition]string
	// carriedAttrs declares those attributes, in first-use order.
	carriedAttrs []string
}

func (e *emitter) fail(where, reason string) error {
	return &TranslateError{Test: e.test.ID, Where: where, Reason: reason}
}

// nameVertices names every state and pseudostate by its path as an identifier,
// suffixing a name two vertices share so each is one endpoint. An initial
// pseudostate is named for the helper state startTarget may declare for it.
func (e *emitter) nameVertices(regions []*Region) {
	taken := map[string]bool{}
	var visit func([]*Region)
	visit = func(regions []*Region) {
		for _, r := range regions {
			for _, v := range r.Vertices {
				if v.Kind == VertexFinal {
					continue
				}
				base := identifier(v.Path())
				if v.Kind == VertexInitial {
					base += "_start"
				}
				name := base
				for n := 2; taken[name]; n++ {
					name = fmt.Sprintf("%s_%d", base, n)
				}
				taken[name] = true
				e.names[v] = name
				visit(v.Regions)
			}
		}
	}
	visit(regions)
}

// identifier turns a vertex path such as "S1.S1.1" into a bare identifier,
// since pseudostate declarations take no quoted name.
func identifier(path string) string {
	var b strings.Builder
	for i, r := range path {
		switch {
		case r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z', i > 0 && r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" || lexer.IsKeyword(name) {
		return "v_" + name
	}
	return name
}

// spell quotes a name the notation cannot take bare.
func spell(name string) string {
	if lexer.IsIdentifier(name) && !lexer.IsKeyword(name) {
		return name
	}
	return lexer.UnrestrictedNameText(name)
}

func (e *emitter) machine(b *strings.Builder) error {
	m := e.test.Machine
	e.placed = map[*Region][]*Transition{}
	if err := e.placeTransitions(m.Regions); err != nil {
		return err
	}
	e.carryPayloads(m.Regions)
	attrs := append([]string{`attribute log : String = "";`}, e.carriedAttrs...)
	if e.test.Target != nil {
		for _, a := range e.test.Target.Attributes {
			typ := scalarTypes[a.Type]
			if typ == "" {
				return e.fail("attribute "+a.Name, fmt.Sprintf("type %s has no ScalarValues counterpart", a.Type))
			}
			decl := fmt.Sprintf("attribute %s : %s", spell(a.Name), typ)
			if a.Default != nil {
				lit, err := e.literal(a.Default)
				if err != nil {
					return err
				}
				decl += " = " + lit
			}
			attrs = append(attrs, decl+";")
		}
	}
	return e.stateBody(b, 1, machineName, "", nil, m.Regions, attrs)
}

// payloadRead matches a guard's read of a scalar payload, `data.value`.
var payloadRead = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.value\b`)

// carryPayloads finds guards out of a choice or junction that read the payload
// of the event that reached it and arranges for the triggered transition to
// store that payload in an attribute the guard then reads (UML 14.2.3.8.5).
func (e *emitter) carryPayloads(regions []*Region) {
	e.carried = map[*Transition]string{}
	all := allTransitions(regions)
	declared := map[string]bool{}
	for _, t := range all {
		if t.Source == nil || t.Source.Kind != VertexChoice && t.Source.Kind != VertexJunction || len(t.Triggers) > 0 {
			continue
		}
		if t.Guard == nil || t.Guard.Kind != GuardOpaque {
			continue
		}
		for _, m := range payloadRead.FindAllStringSubmatch(t.Guard.Opaque.Body, -1) {
			e.carryPayload(m[1], t.Source, all, declared)
		}
	}
}

// allTransitions collects the transitions of regions and of every region nested
// in their vertices.
func allTransitions(regions []*Region) []*Transition {
	var all []*Transition
	for _, r := range regions {
		all = append(all, r.Transitions...)
		for _, v := range r.Vertices {
			all = append(all, allTransitions(v.Regions)...)
		}
	}
	return all
}

// carryPayload arranges for each transition into pseudo that is triggered by a
// scalar signal whose payload binds to param to store that payload in an attribute.
func (e *emitter) carryPayload(param string, pseudo *Vertex, all []*Transition, declared map[string]bool) {
	for _, in := range all {
		if in.Target != pseudo || len(in.Triggers) != 1 {
			continue
		}
		sig := in.Triggers[0].Event
		if sig == nil || sig.Kind != EventSignal || sig.Signal == nil || len(sig.Signal.Attributes) != 1 {
			continue
		}
		typ := scalarTypes[sig.Signal.Attributes[0].Type]
		if typ == "" || payloadParam(sig.Signal.Name) != param {
			continue
		}
		attr := param + "_value"
		e.carried[in] = attr
		if !declared[attr] {
			declared[attr] = true
			e.carriedAttrs = append(e.carriedAttrs, fmt.Sprintf("attribute %s : %s = %s;", attr, typ, zeroLiteral(typ)))
		}
	}
}

// payloadParam names the accept parameter a scalar signal's payload binds to.
func payloadParam(signal string) string {
	return strings.ToLower(signal[:1]) + signal[1:]
}

// zeroLiteral is a ScalarValues type's default, for a carried attribute.
func zeroLiteral(typ string) string {
	switch typ {
	case "Boolean":
		return "false"
	case "String":
		return `""`
	case "Real":
		return "0.0"
	}
	return "0"
}

// placeTransitions files each transition under the region whose scope must
// declare it: its own, or the region of the final state it targets, where
// `done` names that final state.
func (e *emitter) placeTransitions(regions []*Region) error {
	var visit func([]*Region) error
	visit = func(regions []*Region) error {
		for _, r := range regions {
			for _, t := range r.Transitions {
				scope := r
				if t.Target != nil && t.Target.Kind == VertexFinal && t.Target.Region != nil {
					scope = t.Target.Region
				}
				e.placed[scope] = append(e.placed[scope], t)
			}
			for _, v := range r.Vertices {
				if err := visit(v.Regions); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(regions)
}

// stateBody emits `state <name> [parallel] { ... }` for a state or the machine:
// its attributes, entry/do/exit behaviors, deferred triggers, vertices and
// transitions, with an orthogonal state's regions as parallel substates.
func (e *emitter) stateBody(b *strings.Builder, depth int, name, path string, state *Vertex, regions []*Region, attrs []string) error {
	ind := strings.Repeat("    ", depth)
	where := "state " + path
	parts, err := e.stateParts(state, where)
	if err != nil {
		return err
	}
	parallel := ""
	if len(regions) > 1 {
		parallel = " parallel"
	}
	fmt.Fprintf(b, "%sstate %s%s {\n", ind, spell(name), parallel)
	inner := ind + "    "
	for _, a := range attrs {
		b.WriteString(inner + a + "\n")
	}
	entryName := "initial"
	if path != "" {
		entryName = path + initialSuffix
	}
	var initialTarget string
	if len(regions) == 1 {
		if initialTarget, err = e.startEntry(b, inner, regions[0], where); err != nil {
			return err
		}
	}
	writeEntry(b, inner, entryName, parts.entry, initialTarget)
	if parts.do != nil {
		if err := e.doBody(b, inner, parts.do, where+" do"); err != nil {
			return err
		}
	}
	if len(parts.exit) > 0 {
		fmt.Fprintf(b, "%sexit action {\n", inner)
		writeStmts(b, inner+"    ", parts.exit)
		fmt.Fprintf(b, "%s}\n", inner)
	}
	if err := e.writeDeferred(b, inner, parts.deferred, where); err != nil {
		return err
	}
	if len(regions) == 1 {
		if err := e.region(b, depth+1, regions[0], path); err != nil {
			return err
		}
		fmt.Fprintf(b, "%s}\n", ind)
		return nil
	}
	for _, r := range regions {
		if err := e.parallelRegion(b, depth, path, r); err != nil {
			return err
		}
	}
	fmt.Fprintf(b, "%s}\n", ind)
	return nil
}

// startEntry spells a single region's initial transition as the destination of
// the state's entry; an empty region contributes no destination.
func (e *emitter) startEntry(b *strings.Builder, inner string, region *Region, where string) (string, error) {
	init, tr, err := e.initial(region, where)
	if err != nil || init == nil {
		return "", err
	}
	return e.startTarget(b, inner, init, tr, where)
}

// initialSuffix names the entry action a state's or region's path is suffixed with.
const initialSuffix = ".initial"

// effectOf locates a transition's effect for a diagnostic.
func effectOf(tr *Transition) string {
	return tr.Describe() + " effect"
}

// regionWhere locates a region for a diagnostic.
func regionWhere(name string) string {
	return "region " + name
}

// stateParts is what a state's own declaration contributes to its body.
type stateParts struct {
	entry, exit []string
	do          *Body
	deferred    []*Trigger
}

// stateParts translates a state's entry, exit and do behaviors and reads its
// deferred triggers; the machine itself (state nil) contributes nothing.
func (e *emitter) stateParts(state *Vertex, where string) (stateParts, error) {
	var parts stateParts
	if state == nil {
		return parts, nil
	}
	var err error
	if parts.entry, err = e.plainBody(state.Entry, where+" entry"); err != nil {
		return parts, err
	}
	if parts.exit, err = e.plainBody(state.Exit, where+" exit"); err != nil {
		return parts, err
	}
	if state.Do != nil {
		parts.do = state.Do.Body
		if parts.do == nil {
			return parts, e.fail(where+" do", "an opaque do behavior has no translation")
		}
	}
	parts.deferred = state.Deferred
	return parts, nil
}

// writeEntry emits a state's entry: bare `entry; then`, an entry action a
// transition follows, or an entry action alone.
func writeEntry(b *strings.Builder, inner, entryName string, entryStmts []string, initialTarget string) {
	switch {
	case len(entryStmts) == 0 && initialTarget != "":
		fmt.Fprintf(b, "%sentry; then %s;\n", inner, initialTarget)
	case len(entryStmts) > 0 && initialTarget != "":
		fmt.Fprintf(b, "%sentry action %s {\n", inner, spell(entryName))
		writeStmts(b, inner+"    ", entryStmts)
		fmt.Fprintf(b, "%s}\n%stransition %s then %s;\n", inner, inner, spell(entryName), initialTarget)
	case len(entryStmts) > 0:
		fmt.Fprintf(b, "%sentry action {\n", inner)
		writeStmts(b, inner+"    ", entryStmts)
		fmt.Fprintf(b, "%s}\n", inner)
	}
}

// writeDeferred emits a state's deferred signals, if any.
func (e *emitter) writeDeferred(b *strings.Builder, inner string, deferred []*Trigger, where string) error {
	if len(deferred) == 0 {
		return nil
	}
	names := make([]string, len(deferred))
	for i, trig := range deferred {
		if trig.Event == nil || trig.Event.Kind != EventSignal || trig.Event.Signal == nil {
			return e.fail(where, "a deferred trigger that is not a signal event has no spelling")
		}
		e.signals[trig.Event.Signal.Name] = true
		names[i] = trig.Event.Signal.Name
	}
	fmt.Fprintf(b, "%sdefer %s;\n", inner, strings.Join(names, ", "))
	return nil
}

// parallelRegion emits one region of an orthogonal state as a parallel substate
// that starts where the region's entry transition leads; a region with none is
// entered only where a fork's branches lead, which the lowerer checks.
func (e *emitter) parallelRegion(b *strings.Builder, depth int, path string, r *Region) error {
	inner := strings.Repeat("    ", depth+1)
	regionName := path + "/" + r.Name
	if path == "" {
		regionName = r.Name
	}
	fmt.Fprintf(b, "%sstate %s {\n", inner, spell(regionName))
	init, tr, err := e.initial(r, regionWhere(regionName))
	if err != nil {
		return err
	}
	if init != nil {
		target, err := e.startTarget(b, inner+"    ", init, tr, regionWhere(regionName))
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%s    entry; then %s;\n", inner, target)
	}
	if err := e.region(b, depth+2, r, path); err != nil {
		return err
	}
	fmt.Fprintf(b, "%s}\n", inner)
	return nil
}

// initial finds a region's initial pseudostate and its one outgoing transition.
func (e *emitter) initial(r *Region, where string) (*Vertex, *Transition, error) {
	var init *Vertex
	for _, v := range r.Vertices {
		if v.Kind == VertexInitial {
			if init != nil {
				return nil, nil, e.fail(where, "two initial pseudostates in one region")
			}
			init = v
		}
	}
	if init == nil {
		return nil, nil, nil
	}
	var out *Transition
	for _, t := range r.Transitions {
		if t.Source == init {
			if out != nil {
				return nil, nil, e.fail(where, "two transitions out of the initial pseudostate")
			}
			out = t
		}
	}
	if out == nil {
		return nil, nil, e.fail(where, "an initial pseudostate with no outgoing transition")
	}
	if len(out.Triggers) > 0 || out.Guard != nil {
		return nil, nil, e.fail(out.Describe(), "a triggered or guarded initial transition has no spelling")
	}
	return init, out, nil
}

// startTarget spells where a region starts: an initial transition with an effect
// or a pseudostate target starts in a helper state whose completion carries it.
func (e *emitter) startTarget(b *strings.Builder, ind string, init *Vertex, tr *Transition, where string) (string, error) {
	target, err := e.target(tr, where)
	if err != nil {
		return "", err
	}
	if tr.Effect == nil && (tr.Target == nil || !tr.Target.Kind.IsPseudostate()) {
		return target, nil
	}
	helper := e.names[init]
	fmt.Fprintf(b, "%sstate %s;\n%stransition first %s", ind, helper, ind, helper)
	if tr.Effect != nil {
		stmts, err := e.plainBody(tr.Effect, effectOf(tr))
		if err != nil {
			return "", err
		}
		b.WriteString(" do {\n")
		writeStmts(b, ind+"    ", stmts)
		b.WriteString(ind + "}")
	}
	fmt.Fprintf(b, " then %s;\n", target)
	return helper, nil
}

// region emits a region's vertices other than its initial and final states,
// then the transitions placed in its scope.
func (e *emitter) region(b *strings.Builder, depth int, r *Region, path string) error {
	ind := strings.Repeat("    ", depth)
	for _, v := range r.Vertices {
		switch v.Kind {
		case VertexInitial, VertexFinal:
			continue
		case VertexState:
			if v.Submachine != "" || len(v.ConnectionPoints) > 0 || v.ConnectionPointReferences > 0 || v.Redefines != "" {
				return e.fail("state "+v.Path(), "submachine states, connection points and redefinitions have no spelling")
			}
			if len(v.Regions) == 0 {
				if v.Entry == nil && v.Exit == nil && v.Do == nil && len(v.Deferred) == 0 {
					fmt.Fprintf(b, "%sstate %s;\n", ind, spell(e.names[v]))
					continue
				}
			}
			if err := e.stateBody(b, depth, e.names[v], v.Path(), v, v.Regions, nil); err != nil {
				return err
			}
		case VertexJunction:
			fmt.Fprintf(b, "%sjunction %s;\n", ind, spell(e.names[v]))
		case VertexChoice:
			fmt.Fprintf(b, "%schoice %s;\n", ind, spell(e.names[v]))
		case VertexFork:
			fmt.Fprintf(b, "%sfork %s;\n", ind, spell(e.names[v]))
		case VertexJoin:
			fmt.Fprintf(b, "%sjoin %s;\n", ind, spell(e.names[v]))
		case VertexShallowHistory:
			fmt.Fprintf(b, "%shistory %s;\n", ind, spell(e.names[v]))
		case VertexDeepHistory:
			fmt.Fprintf(b, "%sdeep history %s;\n", ind, spell(e.names[v]))
		default:
			return e.fail(v.Describe(), "a pseudostate kind with no spelling")
		}
	}
	for _, t := range e.placed[r] {
		if t.Source != nil && t.Source.Kind == VertexInitial {
			continue
		}
		if err := e.transition(b, ind, t); err != nil {
			return err
		}
	}
	return nil
}

// transition emits `transition first s accept e if g do { ... } then t;`, once
// per trigger when the transition has several.
func (e *emitter) transition(b *strings.Builder, ind string, t *Transition) error {
	where := t.Describe()
	if t.Kind != TransitionExternal {
		return e.fail(where, "local and internal transitions have no spelling")
	}
	if t.Redefines != "" {
		return e.fail(where, "a redefined transition has no spelling")
	}
	if t.Source == nil || t.Target == nil {
		return e.fail(where, "a transition without both ends")
	}
	source := e.names[t.Source]
	if source == "" {
		return e.fail(where, "a transition out of an unnamed vertex")
	}
	target, err := e.target(t, where)
	if err != nil {
		return err
	}
	var effect []string
	if t.Effect != nil {
		if effect, err = e.plainBody(t.Effect, effectOf(t)); err != nil {
			return err
		}
	}
	accepts := []string{""}
	params := []string{""}
	if len(t.Triggers) > 0 {
		accepts, params = nil, nil
		for _, trig := range t.Triggers {
			accept, param, err := e.trigger(trig, where)
			if err != nil {
				return err
			}
			accepts = append(accepts, " accept "+accept)
			params = append(params, param)
		}
	}
	if attr := e.carried[t]; attr != "" {
		effect = append([]string{fmt.Sprintf("assign %s := %s;", attr, params[0])}, effect...)
	}
	for i, accept := range accepts {
		param := params[i]
		if param == "" && t.Source.Kind.IsPseudostate() {
			param = carriedParam
		}
		guard, err := e.guard(t.Guard, param, where)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%stransition first %s%s%s", ind, spell(source), accept, guard)
		if len(effect) > 0 {
			b.WriteString(" do {\n")
			writeStmts(b, ind+"    ", effect)
			b.WriteString(ind + "}")
		}
		fmt.Fprintf(b, " then %s;\n", target)
	}
	return nil
}

// target spells a transition's target: `done` for a final state, else its name.
func (e *emitter) target(t *Transition, where string) (string, error) {
	if t.Target == nil {
		return "", e.fail(where, "a transition without a target")
	}
	if t.Target.Kind == VertexFinal {
		return "done", nil
	}
	name := e.names[t.Target]
	if name == "" || t.Target.Kind == VertexInitial {
		return "", e.fail(where, "a transition into an unnamed vertex")
	}
	return spell(name), nil
}

// trigger spells a trigger as an accept clause, returning the parameter name a
// scalar payload or the call's arguments are bound to.
func (e *emitter) trigger(trig *Trigger, where string) (accept, param string, err error) {
	if trig == nil || trig.Event == nil {
		return "", "", e.fail(where, "a trigger without an event")
	}
	ev := trig.Event
	switch ev.Kind {
	case EventSignal:
		if ev.Signal == nil {
			return "", "", e.fail(where, "a signal event without a signal")
		}
		e.signals[ev.Signal.Name] = true
		if len(ev.Signal.Attributes) == 0 {
			return ev.Signal.Name, "", nil
		}
		if len(ev.Signal.Attributes) != 1 || scalarTypes[ev.Signal.Attributes[0].Type] == "" {
			return "", "", e.fail(where, fmt.Sprintf("signal %s carries a structured payload with no scalar binding", ev.Signal.Name))
		}
		param = payloadParam(ev.Signal.Name)
		return fmt.Sprintf("%s : %s", spell(param), ev.Signal.Name), param, nil
	case EventCall:
		if ev.Operation == nil {
			return "", "", e.fail(where, "a call event without an operation")
		}
		var names []string
		for _, p := range ev.Operation.Params {
			if p.Direction == "return" || p.Direction == "out" || p.Direction == "inout" {
				return "", "", e.fail(where, fmt.Sprintf("operation %s returns a value the caller would observe", ev.Operation.Name))
			}
			names = append(names, spell(p.Name))
		}
		return fmt.Sprintf("%s(%s)", spell(ev.Operation.Name), strings.Join(names, ", ")), "", nil
	}
	return "", "", e.fail(where, fmt.Sprintf("a %s trigger has no spelling", ev.Type))
}

// alfGuard matches the Alf guard bodies the suite writes: comparisons of a
// scalar payload, an attribute or a literal.
var alfGuard = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*\s*(==|!=|<=|>=|<|>)\s*[A-Za-z0-9_]+$|^(true|false)$`)

// guard spells a guard as an `if` clause; a literal true or an `else` guard is
// the unguarded default.
func (e *emitter) guard(g *Guard, param, where string) (string, error) {
	switch {
	case g == nil, g.Kind == GuardElse:
		return "", nil
	case g.Kind == GuardLiteral:
		if g.Literal {
			return "", nil
		}
		return " if false", nil
	case g.Kind == GuardOpaque:
		body := strings.TrimSpace(g.Opaque.Body)
		if !alfGuard.MatchString(body) {
			return "", e.fail(where, fmt.Sprintf("guard %q is not a comparison the translation spells", body))
		}
		expr := strings.ReplaceAll(body, "this.", "")
		if strings.Contains(expr, ".value") {
			switch param {
			case "":
				return "", e.fail(where, fmt.Sprintf("guard %q reads a payload the transition's trigger does not bind", body))
			case carriedParam:
				for _, m := range payloadRead.FindAllStringSubmatch(expr, -1) {
					if !e.carries(m[1] + "_value") {
						return "", e.fail(where, fmt.Sprintf("guard %q reads a payload no transition into its source carries", body))
					}
				}
				expr = payloadRead.ReplaceAllString(expr, "${1}_value")
			default:
				expr = payloadRead.ReplaceAllString(expr, param)
			}
		}
		if strings.Contains(expr, ".") {
			return "", e.fail(where, fmt.Sprintf("guard %q reads through an object", body))
		}
		return " if " + expr, nil
	}
	return "", e.fail(where, fmt.Sprintf("a %s guard has no spelling", g.Type))
}

// carriedParam marks a guard on a pseudostate, which reads carried payloads.
const carriedParam = "\x00carried"

// carries reports whether a carried-payload attribute is declared.
func (e *emitter) carries(attr string) bool {
	for _, decl := range e.carriedAttrs {
		if strings.HasPrefix(decl, "attribute "+attr+" ") {
			return true
		}
	}
	return false
}

// plainBody translates a behavior with no accept into statements.
func (e *emitter) plainBody(bh *Behavior, where string) ([]string, error) {
	if bh == nil {
		return nil, nil
	}
	if bh.Body == nil {
		return nil, e.fail(where, "an opaque behavior has no translation")
	}
	steps, err := e.steps(bh.Body, where, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, st := range steps {
		if st.accept != "" {
			return nil, e.fail(where, "an accept outside a do activity has no spelling")
		}
		out = append(out, st.stmts...)
	}
	return out, nil
}

// step is a run of statements or one accept of an ordered body.
type step struct {
	stmts  []string
	accept string
}

// stepList accumulates steps: statements join the current run, an accept
// closes it.
type stepList []step

func (out *stepList) add(s string) {
	if n := len(*out); n == 0 || (*out)[n-1].accept != "" {
		*out = append(*out, step{})
	}
	last := &(*out)[len(*out)-1]
	last.stmts = append(last.stmts, s)
}

func (out *stepList) acceptOf(signal string) {
	*out = append(*out, step{accept: signal})
}

// steps translates a body into runs of statements split at each accept.
func (e *emitter) steps(body *Body, where string, depth int) ([]step, error) {
	if depth > 8 {
		return nil, e.fail(where, "operation methods call each other too deeply to inline")
	}
	if len(body.Unsupported) > 0 {
		return nil, e.fail(where, "activity nodes with no translation: "+strings.Join(body.Unsupported, "; "))
	}
	var out stepList
	for _, st := range body.Statements {
		var err error
		switch st.Kind {
		case StmtCall:
			err = e.stepCall(&out, st, where, depth)
		case StmtSend:
			err = e.stepSend(&out, st, where)
		case StmtAccept:
			err = e.stepAccept(&out, st, where)
		case StmtAssign:
			err = e.stepAssign(&out, st, where)
		case StmtReturn:
			err = e.fail(where, "a return has no translation")
		case StmtStart:
			err = e.fail(where, "starting an object's behavior has no translation")
		default:
			err = e.fail(where, fmt.Sprintf("statement %s has no translation", st))
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// stepCall translates a call: a trace appends to the log, any other call
// inlines the method it names.
func (e *emitter) stepCall(out *stepList, st Statement, where string, depth int) error {
	if st.Name == "trace" && isSelf(st.Receiver) && len(st.Args) == 1 && st.Args[0].Kind == ExprLiteral && st.Args[0].Literal.Kind == LiteralString {
		seg := lexer.StringText(st.Args[0].Literal.Text)
		out.add(fmt.Sprintf(`assign log := if log == "" ? %s else log + "::" + %s;`, seg, seg))
		return nil
	}
	if len(st.Args) > 0 {
		return e.fail(where, fmt.Sprintf("call %s passes arguments the translation cannot bind", st))
	}
	method := e.method(st)
	if method == nil {
		return e.fail(where, fmt.Sprintf("call %s names no method of the target class", st))
	}
	if method.Body == nil {
		return e.fail(where, fmt.Sprintf("call %s names an opaque behavior", st))
	}
	inner, err := e.steps(method.Body, where+" > "+method.Name, depth+1)
	if err != nil {
		return err
	}
	for _, s := range inner {
		if s.accept != "" {
			return e.fail(where, fmt.Sprintf("call %s reaches an accept", st))
		}
		for _, x := range s.stmts {
			out.add(x)
		}
	}
	return nil
}

// stepSend translates a send to the machine itself; one to the harness is dropped.
func (e *emitter) stepSend(out *stepList, st Statement, where string) error {
	if isHarness(st.Receiver) {
		return nil
	}
	if !isSelf(st.Receiver) {
		return e.fail(where, fmt.Sprintf("send %s addresses an object other than the machine", st))
	}
	if len(st.Args) > 0 {
		return e.fail(where, fmt.Sprintf("send %s carries a payload the translation cannot bind", st))
	}
	e.signals[st.Name] = true
	out.add(fmt.Sprintf("send new %s() to %s;", st.Name, machineName))
	return nil
}

// stepAccept translates an accept of one signal event.
func (e *emitter) stepAccept(out *stepList, st Statement, where string) error {
	if len(st.Events) != 1 || st.Events[0].Kind != EventSignal || st.Events[0].Signal == nil {
		return e.fail(where, fmt.Sprintf("%s waits for more than one signal event", st))
	}
	if st.Result != "" {
		return e.fail(where, fmt.Sprintf("%s binds the occurrence, which the translation cannot spell", st))
	}
	e.signals[st.Events[0].Signal.Name] = true
	out.acceptOf(st.Events[0].Signal.Name)
	return nil
}

// stepAssign translates a replacing assignment to the machine's own feature.
func (e *emitter) stepAssign(out *stepList, st Statement, where string) error {
	if !isSelf(st.Receiver) {
		return e.fail(where, fmt.Sprintf("%s writes a feature of another object", st))
	}
	if !st.Replace {
		return e.fail(where, fmt.Sprintf("%s adds to a feature instead of replacing it", st))
	}
	value, err := e.expr(st.Value, where)
	if err != nil {
		return err
	}
	out.add(fmt.Sprintf("assign %s := %s;", spell(st.Feature), value))
	return nil
}

// method finds the behavior a call on the target names: an operation's method
// or an owned behavior of the target class.
func (e *emitter) method(st Statement) *Behavior {
	if e.test.Target == nil {
		return nil
	}
	if st.Receiver == nil {
		for _, bh := range e.test.Target.Behaviors {
			if bh.Name == st.Name {
				return bh
			}
		}
		return nil
	}
	if !isSelf(st.Receiver) {
		return nil
	}
	for _, op := range e.test.Target.Operations {
		if op.Name == st.Name && len(op.Params) == 0 {
			return op.Method
		}
	}
	return nil
}

// doBody emits a do activity: a plain body, or an ordered chain of actions
// when the body waits for signals.
func (e *emitter) doBody(b *strings.Builder, ind string, body *Body, where string) error {
	steps, err := e.steps(body, where, 0)
	if err != nil {
		return err
	}
	chained := false
	for _, s := range steps {
		if s.accept != "" {
			chained = true
		}
	}
	if !chained {
		var stmts []string
		for _, s := range steps {
			stmts = append(stmts, s.stmts...)
		}
		if len(stmts) == 0 {
			return nil
		}
		fmt.Fprintf(b, "%sdo action {\n", ind)
		writeStmts(b, ind+"    ", stmts)
		fmt.Fprintf(b, "%s}\n", ind)
		return nil
	}
	fmt.Fprintf(b, "%sdo action {\n%s    first start;\n", ind, ind)
	for i, s := range steps {
		if s.accept != "" {
			fmt.Fprintf(b, "%s    then action step%d accept %s;\n", ind, i+1, s.accept)
			continue
		}
		fmt.Fprintf(b, "%s    then action step%d {\n", ind, i+1)
		writeStmts(b, ind+"        ", s.stmts)
		fmt.Fprintf(b, "%s    }\n", ind)
	}
	fmt.Fprintf(b, "%s    then done;\n%s}\n", ind, ind)
	return nil
}

// expr spells a value: a literal, the machine's own attribute, a bound
// parameter, or an arithmetic application of those.
func (e *emitter) expr(x *Expr, where string) (string, error) {
	if x == nil {
		return "", e.fail(where, "a statement without a value")
	}
	switch x.Kind {
	case ExprLiteral:
		return e.literal(x.Literal)
	case ExprRead:
		if !isSelf(x.Object) {
			return "", e.fail(where, fmt.Sprintf("%s reads a feature of another object", x))
		}
		return spell(x.Name), nil
	case ExprParam, ExprEvent:
		return spell(x.Name), nil
	case ExprApply:
		op := map[string]string{"plus": "+", "minus": "-", "times": "*", "Concat": "+", "==": "==", "<": "<", ">": ">"}[x.Name]
		if op == "" || len(x.Args) != 2 {
			return "", e.fail(where, fmt.Sprintf("%s applies a behavior the translation does not spell", x))
		}
		l, err := e.expr(&x.Args[0], where)
		if err != nil {
			return "", err
		}
		r, err := e.expr(&x.Args[1], where)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", l, op, r), nil
	case ExprCall:
		return "", e.fail(where, fmt.Sprintf("%s uses an operation's result", x))
	case ExprNew:
		return "", e.fail(where, fmt.Sprintf("%s creates an object", x))
	}
	return "", e.fail(where, fmt.Sprintf("%s has no translation", x))
}

// literal spells a UML literal as a SysML literal.
func (e *emitter) literal(l *Literal) (string, error) {
	if l == nil {
		return "", e.fail("", "a missing literal")
	}
	switch l.Kind {
	case LiteralString:
		return lexer.StringText(l.Text), nil
	case LiteralBoolean, LiteralInteger, LiteralReal, LiteralUnlimitedNatural:
		return l.String(), nil
	}
	return "", e.fail("", fmt.Sprintf("literal %s has no spelling", l))
}

// stimulation reads the tester's behavior as the events to queue: Start when
// the machine reacts to it, then each send or operation call to the target.
func (e *emitter) stimulation() ([]Stimulus, error) {
	var events []Stimulus
	if e.signals["Start"] {
		events = append(events, Stimulus{Signal: "Start"})
	}
	if e.test.Stimulation == nil {
		return events, nil
	}
	where := "tester"
	if len(e.test.Stimulation.Unsupported) > 0 {
		return nil, e.fail(where, "activity nodes with no translation: "+strings.Join(e.test.Stimulation.Unsupported, "; "))
	}
	for _, st := range e.test.Stimulation.Statements {
		var ev Stimulus
		var err error
		switch st.Kind {
		case StmtAccept:
			continue
		case StmtSend:
			ev, err = e.sentStimulus(st, where)
		case StmtCall:
			ev, err = e.calledStimulus(st, where)
		default:
			err = e.fail(where, fmt.Sprintf("%s has no translation as a queued event", st))
		}
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

// sentStimulus reads a send to the target as a queued signal, with its scalar
// payload when it carries one.
func (e *emitter) sentStimulus(st Statement, where string) (Stimulus, error) {
	if !isTarget(st.Receiver) {
		return Stimulus{}, e.fail(where, fmt.Sprintf("%s addresses an object other than the target", st))
	}
	sig := e.suite.Signals[st.Name]
	ev := Stimulus{Signal: st.Name}
	if len(st.Args) > 0 {
		if sig == nil || len(sig.Attributes) != 1 || len(st.Args) != 1 || st.Args[0].Kind != ExprLiteral {
			return Stimulus{}, e.fail(where, fmt.Sprintf("%s carries a payload the translation cannot bind", st))
		}
		ev.Value = st.Args[0].Literal
	}
	e.signals[st.Name] = true
	return ev, nil
}

// calledStimulus reads an operation call on the target as a queued call with
// its literal arguments bound to the operation's in parameters.
func (e *emitter) calledStimulus(st Statement, where string) (Stimulus, error) {
	if !isTarget(st.Receiver) {
		return Stimulus{}, e.fail(where, fmt.Sprintf("%s calls an object other than the target", st))
	}
	op := e.operation(st.Name)
	if op == nil {
		return Stimulus{}, e.fail(where, fmt.Sprintf("%s names no operation of the target", st))
	}
	ev := Stimulus{Call: st.Name}
	var ins []Param
	for _, p := range op.Params {
		if p.Direction == "return" || p.Direction == "out" || p.Direction == "inout" {
			return Stimulus{}, e.fail(where, fmt.Sprintf("%s returns a value the tester would observe", st))
		}
		ins = append(ins, p)
	}
	if len(ins) != len(st.Args) {
		return Stimulus{}, e.fail(where, fmt.Sprintf("%s passes %d arguments to %d parameters", st, len(st.Args), len(ins)))
	}
	for i, a := range st.Args {
		if a.Kind != ExprLiteral {
			return Stimulus{}, e.fail(where, fmt.Sprintf("%s passes an argument that is not a literal", st))
		}
		ev.Args = append(ev.Args, Argument{Name: ins[i].Name, Value: a.Literal})
	}
	return ev, nil
}

func (e *emitter) operation(name string) *Operation {
	if e.test.Target == nil {
		return nil
	}
	for _, op := range e.test.Target.Operations {
		if op.Name == name {
			return op
		}
	}
	return nil
}

func isSelf(x *Expr) bool { return x != nil && x.Kind == ExprSelf }

// isHarness reports a reference to the test's own bookkeeping objects: the
// tester and the semantic test that receives the End signal.
func isHarness(x *Expr) bool {
	return x != nil && x.Kind == ExprRead && isSelf(x.Object) && (x.Name == "test" || x.Name == "tester")
}

// isTarget reports the tester's reference to the class under test.
func isTarget(x *Expr) bool {
	return x != nil && x.Kind == ExprRead && isSelf(x.Object) && x.Name == "testable"
}

func writeStmts(b *strings.Builder, ind string, stmts []string) {
	for _, s := range stmts {
		b.WriteString(ind + s + "\n")
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
