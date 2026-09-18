package runtime

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// The canonical form of a checked run's state is the text of what a future move
// can observe: the clock, where the modeled stream stands (what the run draws
// next), the executor holding the turn, then every executor on it in invocation
// order — an action's tokens by node and performance and its
// performances root-first with what they hold, a state machine's configuration,
// history, values, queue, timers and do progress — then the messages in flight
// and the objects reached by their materialization path. Identities a run hands
// out — token ids, event ids, object ids, step counters, activation numbers — are
// spelled by position instead, so two runs reaching one state spell it alike.

// stateKey is the SHA-256 of a state's canonical form, hex-encoded.
type stateKey string

// tokenKey names a token of one executor: ids are handed out per executor.
type tokenKey struct {
	owner checkedExecutor
	id    int64
}

// canonicalForm is a state's canonical text, the canonical name of each executor
// on the clock and of each token: what it spells as, numbered among those spelling alike.
type canonicalForm struct {
	text   string
	names  map[checkedExecutor]string
	tokens map[tokenKey]string
}

// key hashes the canonical text.
func (f canonicalForm) key() stateKey {
	sum := sha256.Sum256([]byte(f.text))
	return stateKey(hex.EncodeToString(sum[:]))
}

// canonicalState renders the state of the invocation's run in canonical form, the
// turn settled first.
func (r *invocationRun) canonicalState() canonicalForm {
	r.enabledMoves()
	return r.inv.canonicalState(r.turn)
}

// canonicalState renders the invocation's state with turn holding the turn, nil for none.
func (inv *Invocation) canonicalState(turn checkedExecutor) canonicalForm {
	execs := inv.executors()
	ctx := inv.Context()
	// Reading a feature may derive its default; a probe gives that back.
	defer ctx.beginProbe()()
	s := &stateSpeller{
		ctx:    ctx,
		paths:  make(map[int64]string),
		names:  make(map[checkedExecutor]string, len(execs)),
		tokens: make(map[tokenKey]string),
	}
	text := s.spell(execs, turn)
	return canonicalForm{text: text, names: s.names, tokens: s.tokens}
}

// stateSpeller writes the canonical form, naming objects by materialization
// path and performances by their path in the action.
type stateSpeller struct {
	ctx *Context
	// exec is the action being spelled.
	exec *ActionExecutor
	// paths are the objects mentioned so far by path; mentioned lists them in order.
	paths     map[int64]string
	mentioned []int64
	// names name the executors canonically; frames are the reachable performances
	// of the action being spelled, root first, labels their labels; tokens the tokens.
	names  map[checkedExecutor]string
	frames []*actionFrame
	labels map[*actionFrame]string
	tokens map[tokenKey]string
	out    strings.Builder
}

func (s *stateSpeller) spell(execs []checkedExecutor, turn checkedExecutor) string {
	fmt.Fprintf(&s.out, "clock t=%s\n", semantics.FormatReal(s.ctx.clock.now))
	if sched := s.ctx.run.scheduler; sched != nil {
		if at := sched.modeled.position(); at != "" {
			fmt.Fprintf(&s.out, "draws: %s\n", at)
		}
	}
	s.nameExecutors(execs)
	if turn != nil {
		fmt.Fprintf(&s.out, "turn: %s\n", s.names[turn])
	}
	for _, exec := range execs {
		switch e := exec.(type) {
		case *ActionExecutor:
			s.action(e)
		case *StateExecutor:
			s.machine(e)
		}
	}
	for i, msg := range s.ctx.messages {
		fmt.Fprintf(&s.out, "message %d: %s\n", i+1, s.message(msg))
	}
	// Every root object a run made is observable by name, whether or not a frame holds it.
	for _, id := range s.ctx.created {
		if inst, live := s.ctx.instances[id]; live {
			if owner, _ := inst.Owner(); owner == nil {
				s.objectPath(id)
			}
		}
	}
	s.objects()
	return s.out.String()
}

// nameExecutors names every executor by its kind, its behavior and the object it
// performs on, numbered among those spelling alike in invocation order.
func (s *stateSpeller) nameExecutors(execs []checkedExecutor) {
	alike := make(map[string]int, len(execs))
	for _, exec := range execs {
		var name string
		switch e := exec.(type) {
		case *ActionExecutor:
			name = "action " + symbolText(e.action) + s.performer(e.self)
		case *StateExecutor:
			name = "state machine " + symbolText(e.stateMachine) + s.performer(e.self)
		default:
			name = exec.dueLabel()
		}
		alike[name]++
		if n := alike[name]; n > 1 {
			name += " #" + strconv.Itoa(n)
		}
		s.names[exec] = name
	}
}

// performer spells the object a behavior performs on, by path; nothing for none.
func (s *stateSpeller) performer(self *Instance) string {
	if self == nil {
		return ""
	}
	return " of " + s.object(self.ID)
}

// action spells one action executor: its state, its performances root-first and its tokens.
func (s *stateSpeller) action(e *ActionExecutor) {
	defer s.enter(e)()
	fmt.Fprintf(&s.out, "%s: state %s\n", s.names[e], e.state)
	for _, perf := range s.frames {
		s.out.WriteString(s.frame(perf))
		s.out.WriteByte('\n')
	}
	for _, line := range s.tokenLines() {
		s.out.WriteString(line)
		s.out.WriteByte('\n')
	}
}

// enter makes e the action being spelled, its performances labelled, and returns
// the restorer of the one spelled around it.
func (s *stateSpeller) enter(e *ActionExecutor) func() {
	exec, frames, labels := s.exec, s.frames, s.labels
	s.exec = e
	s.frames = s.labelFrames()
	return func() { s.exec, s.frames, s.labels = exec, frames, labels }
}

// tokenLines spells the tokens of the action being spelled, sorted, naming each
// canonically among those spelling alike.
func (s *stateSpeller) tokenLines() []string {
	e := s.exec
	tokens := make([]string, 0, len(e.tokens))
	alike := make(map[string]int)
	for _, token := range slices.SortedFunc(slices.Values(e.tokens), func(a, b Token) int { return cmp.Compare(a.ID, b.ID) }) {
		text := s.token(token)
		tokens = append(tokens, text)
		alike[text]++
		s.tokens[tokenKey{e, token.ID}] = fmt.Sprintf("%s #%d", text, alike[text])
	}
	sort.Strings(tokens)
	return tokens
}

// machine spells one state machine executor: its state, active configuration,
// history, values, visits, queue in dispatch order, deferred events, timers,
// change latches and do progress.
func (s *stateSpeller) machine(e *StateExecutor) {
	fmt.Fprintf(&s.out, "%s: state %s", s.names[e], e.state)
	if e.machineExited {
		s.out.WriteString(" exited")
	}
	if e.activeConfig != nil {
		if e.activeConfig.simpleState != nil {
			fmt.Fprintf(&s.out, " active{%s}", e.statePath(e.activeConfig.simpleState))
		}
		for _, region := range e.orderedActiveRegions() {
			fmt.Fprintf(&s.out, " active{%s = %s}", regionKey(region), e.statePath(e.activeConfig.regionStates[region]))
		}
	}
	for _, state := range e.stateStack {
		fmt.Fprintf(&s.out, " stack{%s}", e.statePath(state))
	}
	for _, state := range sortedStates(e.history) {
		record := e.history[state]
		fmt.Fprintf(&s.out, " history{%s = %s", e.statePath(state), s.stateName(e, record.child))
		for _, region := range sortedRegions(record.regions) {
			fmt.Fprintf(&s.out, ", %s = %s", regionKey(region), s.stateName(e, record.regions[region]))
		}
		s.out.WriteString("}")
	}
	fmt.Fprintf(&s.out, " data{%s}", s.values(e.stateData))
	for _, state := range sortedStates(e.stateAttrs) {
		fmt.Fprintf(&s.out, " attrs{%s: %s}", e.statePath(state), s.values(e.stateAttrs[state]))
	}
	fmt.Fprintf(&s.out, " visits{%s}", strings.Join(e.stateVisits, ", "))
	if e.eventQueue != nil {
		events := slices.Clone(e.eventQueue.events)
		sort.SliceStable(events, func(i, j int) bool { return events.Less(i, j) })
		for _, event := range events {
			fmt.Fprintf(&s.out, " event{%s}", s.event(e, event))
		}
	}
	for _, event := range e.deferred {
		fmt.Fprintf(&s.out, " deferred{%s}", s.event(e, event))
	}
	for _, trans := range sortedTransitions(e.timerScheduled) {
		fmt.Fprintf(&s.out, " timer{%s}", s.transition(e, trans))
	}
	for _, trans := range sortedTransitions(e.changeFired) {
		fmt.Fprintf(&s.out, " latched{%s}", s.transition(e, trans))
	}
	for _, act := range e.doActions {
		fmt.Fprintf(&s.out, " do{%s: %d pending", e.statePath(act.state), len(act.pending))
		if slices.Contains(e.round, act) {
			s.out.WriteString(", in round")
		}
		if act.run != nil {
			fmt.Fprintf(&s.out, ", paused{%s}", s.body(act.run.body))
			if act.run.host.flow.leftStanding {
				s.out.WriteString(", left standing")
			}
		}
		s.out.WriteString("}")
	}
	if e.roundDone {
		s.out.WriteString(" round done")
	}
	s.out.WriteByte('\n')
}

// stateName spells a state by its path in the machine, nothing for none.
func (s *stateSpeller) stateName(e *StateExecutor, state *ast.StateNode) string {
	if state == nil {
		return ""
	}
	return e.statePath(state)
}

// regionKey identifies a region by its name and where it was written.
func regionKey(region *ast.StateRegion) string {
	span := region.Span()
	return fmt.Sprintf("%s@%d-%d", region.Name, span.Offset, span.End())
}

// sortedStates orders a map's state keys by identity, so the form is independent of map order.
func sortedStates[V any](m map[*ast.StateNode]V) []*ast.StateNode {
	states := make([]*ast.StateNode, 0, len(m))
	for state := range m {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool { return nodeKey(states[i]) < nodeKey(states[j]) })
	return states
}

// sortedRegions orders a map's region keys by identity.
func sortedRegions[V any](m map[*ast.StateRegion]V) []*ast.StateRegion {
	regions := make([]*ast.StateRegion, 0, len(m))
	for region := range m {
		regions = append(regions, region)
	}
	sort.Slice(regions, func(i, j int) bool { return regionKey(regions[i]) < regionKey(regions[j]) })
	return regions
}

// sortedTransitions orders a set's transitions by source and position.
func sortedTransitions(m map[*lower.Transition]bool) []*lower.Transition {
	transitions := make([]*lower.Transition, 0, len(m))
	for trans, set := range m {
		if set {
			transitions = append(transitions, trans)
		}
	}
	sort.Slice(transitions, func(i, j int) bool { return transitionKey(transitions[i]) < transitionKey(transitions[j]) })
	return transitions
}

// transitionKey identifies a transition by its source, target and where it was written.
func transitionKey(trans *lower.Transition) string {
	key := nodeKey(trans.Source) + "->" + nodeKey(trans.Target)
	if trans.Decl != nil {
		key += "@" + nodeKey(trans.Decl)
	}
	return key
}

// transition spells a transition by its source state and its position among the
// source's transitions, as the trace names it.
func (s *stateSpeller) transition(e *StateExecutor, trans *lower.Transition) string {
	transitions := e.graph.Transitions[trans.Source]
	if pos := slices.Index(transitions, trans); pos >= 0 {
		return StateVertexName(trans.Source) + " " + transitionName(transitions, pos)
	}
	return transitionDescription(trans)
}

// event spells a queued event by its instant, what it carries and where it goes;
// its id is arrival order, which the queue's order spells.
func (s *stateSpeller) event(e *StateExecutor, event Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "t=%s ", semantics.FormatReal(event.Timestamp))
	switch payload := event.Payload.(type) {
	case Message:
		b.WriteString(s.message(payload))
	case Call:
		fmt.Fprintf(&b, "call %s args{%s}", orAny(payload.Operation), s.values(payload.Args))
	case *lower.Transition:
		if payload.Trigger == nil {
			b.WriteString("completion ")
		}
		b.WriteString(s.transition(e, payload))
	default:
		b.WriteString(eventName(&event))
	}
	return b.String()
}

// objects spells every object the form mentioned, and those their features
// mention in turn, in path order.
func (s *stateSpeller) objects() {
	lines := make(map[string]string)
	for i := 0; i < len(s.mentioned); i++ {
		id := s.mentioned[i]
		inst, ok := s.ctx.Instance(id)
		if !ok {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "object %s: %s{%s}", s.paths[id], s.ctx.typeName(inst), s.features(inst))
		for i, end := range inst.Ends {
			name := end.Name
			if name == "" {
				name = strconv.Itoa(i)
			}
			fmt.Fprintf(&b, " end %s = %s", name, s.value(end.Value))
		}
		lines[s.paths[id]] = b.String()
	}
	paths := make([]string, 0, len(lines))
	for path := range lines {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		s.out.WriteString(lines[path])
		s.out.WriteByte('\n')
	}
}

// labelFrames names every reachable performance by its path, numbering the
// performances of one node by the order they began, and lists them root-first.
func (s *stateSpeller) labelFrames() []*actionFrame {
	frames := s.exec.reachableFrames()
	sort.Slice(frames, func(i, j int) bool {
		if pi, pj := frames[i].path(), frames[j].path(); pi != pj {
			return pi < pj
		}
		return frames[i].began < frames[j].began
	})
	s.labels = make(map[*actionFrame]string, len(frames))
	byPath := make(map[string]int)
	for _, perf := range frames {
		path := perf.path()
		byPath[path]++
		label := path
		if label == "" {
			label = "action"
		}
		if byPath[path] > 1 {
			label += "#" + strconv.Itoa(byPath[path])
		}
		s.labels[perf] = label
	}
	return frames
}

func (s *stateSpeller) frameLabel(perf *actionFrame) string {
	if perf == nil {
		return "action"
	}
	if label, ok := s.labels[perf]; ok {
		return label
	}
	return "unreached " + perf.path()
}

// frame spells one performance: its flags, held values, block locals, the
// deliveries its nodes' pins queue and which performance of each node is the latest.
func (s *stateSpeller) frame(perf *actionFrame) string {
	var b strings.Builder
	fmt.Fprintf(&b, "frame %s:", s.frameLabel(perf))
	if perf.ended {
		b.WriteString(" ended")
	}
	if perf.inBody {
		b.WriteString(" body")
	}
	fmt.Fprintf(&b, " live=%d", perf.live)
	fmt.Fprintf(&b, " data{%s}", s.values(perf.data))
	for _, local := range perf.locals {
		fmt.Fprintf(&b, " local{%s}", s.values(local))
	}
	for _, node := range sortedNodes(perf.pending) {
		pins := perf.pending[node]
		names := make([]string, 0, len(pins))
		for pin := range pins {
			names = append(names, pin)
		}
		sort.Strings(names)
		for _, pin := range names {
			fmt.Fprintf(&b, " pending{%s.%s = (%s)}", s.node(perf.graph, node), pin, s.elements(pins[pin]))
		}
	}
	for _, node := range sortedNodes(perf.nested) {
		for _, delivery := range perf.nested[node] {
			path := make([]string, 0, len(delivery.path)+1)
			path = append(path, s.node(perf.graph, node))
			for _, step := range delivery.path {
				path = append(path, nodeIdentifier(step))
			}
			fmt.Fprintf(&b, " nested{%s.%s = %s}", strings.Join(path, "."), delivery.pin, s.value(delivery.value))
		}
	}
	for _, node := range sortedNodes(perf.subactions) {
		fmt.Fprintf(&b, " latest{%s = %s}", s.node(perf.graph, node), s.frameLabel(perf.subactions[node]))
	}
	return b.String()
}

// sortedNodes orders a map's node keys by identity, so the form is independent of map order.
func sortedNodes[V any](m map[ast.Node]V) []ast.Node {
	nodes := make([]ast.Node, 0, len(m))
	for node := range m {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodeKey(nodes[i]) < nodeKey(nodes[j]) })
	return nodes
}

// nodeKey identifies a node by its name and where it was written.
func nodeKey(node ast.Node) string {
	span := node.Span()
	return fmt.Sprintf("%s@%d-%d", nodeIdentifier(node), span.Offset, span.End())
}

// node spells a node of a flow by its position in it, its name beside; one the
// flow does not hold is spelled by where it was written.
func (s *stateSpeller) node(graph *lower.ActionGraph, node ast.Node) string {
	if graph != nil {
		if i := slices.Index(graph.Nodes, node); i >= 0 {
			return fmt.Sprintf("%d:%s", i, nodeIdentifier(node))
		}
	}
	return nodeKey(node)
}

// token spells a token by its node, performance, the succession it arrived over
// and the wait it is parked in; its id is scheduling detail and is dropped.
func (s *stateSpeller) token(t Token) string {
	graph := s.exec.graph
	if t.frame != nil && t.frame.graph != nil {
		graph = t.frame.graph
	}
	var b strings.Builder
	fmt.Fprintf(&b, "token %s in %s", s.node(graph, t.Location), s.frameLabel(t.frame))
	if t.Via != (lower.ActionEdge{}) {
		fmt.Fprintf(&b, " via %s", s.edge(graph, t.Via))
	}
	if t.Wait != nil {
		fmt.Fprintf(&b, " wait{%s}", s.wait(*t.Wait))
	}
	if t.body != nil {
		fmt.Fprintf(&b, " paused{%s}", s.body(t.body))
	}
	return b.String()
}

// edge spells a succession by its source and its position among the source's successions.
func (s *stateSpeller) edge(graph *lower.ActionGraph, edge lower.ActionEdge) string {
	source := s.node(graph, edge.Source)
	if graph != nil {
		if i := slices.Index(graph.Edges[edge.Source], edge); i >= 0 {
			return fmt.Sprintf("%s/%d", source, i)
		}
	}
	return source + "->" + s.node(graph, edge.Target)
}

// wait spells what a token waits for; the step it parked at is a counter and is dropped.
func (s *stateSpeller) wait(w AcceptWait) string {
	switch {
	case w.Timed:
		return fmt.Sprintf("%s until t=%s", w.Trigger, semantics.FormatReal(w.Due))
	case w.Trigger != "":
		return w.Trigger
	}
	return fmt.Sprintf("accept %s of type %s%s", w.ParamName, orAny(w.SignalType), viaSuffix(w.ViaPort))
}

// message spells a message in flight: its type, destination and what it carries.
func (s *stateSpeller) message(m Message) string {
	var b strings.Builder
	b.WriteString(orAny(m.SignalType))
	if m.Signal != nil {
		b.WriteString(" (" + s.ctx.symbolName(m.Signal) + ")")
	}
	if m.EventName != "" || m.Event != nil {
		name := m.EventName
		if m.Event != nil {
			name = s.ctx.symbolName(m.Event)
		}
		fmt.Fprintf(&b, " from %s", name)
		if m.EventObject != 0 {
			fmt.Fprintf(&b, " of %s", s.object(m.EventObject))
		}
	}
	fmt.Fprintf(&b, " delivery=%d", m.Delivery)
	if m.Target != "" {
		fmt.Fprintf(&b, " to %s", m.Target)
	}
	if m.Port != "" {
		fmt.Fprintf(&b, " via %s", m.Port)
	}
	if m.Object != 0 {
		fmt.Fprintf(&b, " object %s", s.object(m.Object))
	}
	if m.PortID != 0 {
		fmt.Fprintf(&b, " port %s", s.object(m.PortID))
	}
	fmt.Fprintf(&b, " payload{%s}", s.values(m.Payload))
	if m.Value != nil {
		fmt.Fprintf(&b, " value %s", s.value(*m.Value))
	}
	return b.String()
}

// values spells a map of named values in name order.
func (s *stateSpeller) values(m map[string]Value) string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + " = " + s.value(m[name])
	}
	return strings.Join(parts, ", ")
}

func (s *stateSpeller) elements(elements []Value) string {
	parts := make([]string, len(elements))
	for i, element := range elements {
		parts[i] = s.value(element)
	}
	return strings.Join(parts, ", ")
}

// value spells a value through the trace's formatter, objects by their contents.
func (s *stateSpeller) value(v Value) string {
	switch v.Kind {
	case ValInstance:
		return s.object(v.Instance)
	case ValVariant:
		variant := v.Variant()
		if variant == nil {
			return FormatTraceValue(v)
		}
		if v.Instance != 0 {
			return variant.Name + " " + s.object(v.Instance)
		}
		return variant.Name
	case ValSequence:
		if v.Sequence() == nil {
			return "()"
		}
		return "(" + s.elements(v.Sequence().Elements()) + ")"
	case ValSet:
		if v.Set() == nil {
			return "{}"
		}
		return "{" + s.elements(v.Set().Elements()) + "}"
	case ValArray:
		if v.Array() == nil {
			return FormatTraceValue(v)
		}
		return v.Array().Format(s.value)
	}
	return FormatTraceValue(v)
}

// object spells an object as `@path`, its contents following in the objects section.
func (s *stateSpeller) object(id int64) string {
	if s.ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: id}) {
		return UnsetText
	}
	return "@" + s.objectPath(id)
}

// objectPath is the object's materialization path (see Context.objectPath), the
// holders on the way mentioned too so their contents follow in the objects section.
func (s *stateSpeller) objectPath(id int64) string {
	if path, ok := s.paths[id]; ok {
		return path
	}
	inst, ok := s.ctx.Instance(id)
	if !ok {
		return "<unknown object>"
	}
	var path string
	owner, feature := inst.Owner()
	if owner == nil {
		path = s.ctx.rankedRootPath(inst)
	} else {
		path = s.objectPath(owner.ID) + "." + feature
		if fv := owner.FeatureValues[feature]; fv != nil && !fv.Feature.Scalar() {
			if i := s.ctx.memberIndex(fv.Values, id); i >= 0 {
				path += "[" + strconv.Itoa(i) + "]"
			}
		}
	}
	s.paths[id] = path
	s.mentioned = append(s.mentioned, id)
	return path
}

// features spells what every feature of inst holds, in name order.
func (s *stateSpeller) features(inst *Instance) string {
	names := make([]string, 0, len(inst.FeatureValues))
	for name := range inst.FeatureValues {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+" = "+s.feature(inst, name))
	}
	return strings.Join(parts, ", ")
}

func (s *stateSpeller) feature(inst *Instance, name string) string {
	fv, err := inst.GetFeatureValue(s.ctx, name)
	if err != nil {
		return "<error: " + err.Error() + ">"
	}
	if !fv.Feature.Scalar() {
		if fv.Values.Kind == ValInvalid {
			return "()"
		}
		return s.value(fv.Values)
	}
	if !fv.Materialized || fv.Value.Kind == ValInvalid {
		return UnsetText
	}
	return s.value(fv.Value)
}
