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
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The canonical form of a checked run's state is the text of what a future step
// can observe: the tokens by node and performance, the performances root-first
// with what they hold, the messages in flight, the clock, and the objects
// reached by their materialization path. Identities a run hands out — token ids,
// object ids, step counters, activation numbers — are spelled by position
// instead, so two runs reaching one state spell it alike.

// stateKey is the SHA-256 of a state's canonical form, hex-encoded.
type stateKey string

// canonicalForm is a state's canonical text and the canonical name of each of its
// tokens: what the token spells as, numbered among tokens spelling alike.
type canonicalForm struct {
	text   string
	tokens map[int64]string
}

// key hashes the canonical text.
func (f canonicalForm) key() stateKey {
	sum := sha256.Sum256([]byte(f.text))
	return stateKey(hex.EncodeToString(sum[:]))
}

// canonicalState renders the state of the executor's run in canonical form; a
// token whose paused work the snapshot cannot capture is refused as the snapshot refuses it.
func (e *ActionExecutor) canonicalState() (canonicalForm, error) {
	for _, token := range e.tokens {
		if token.body != nil {
			return canonicalForm{}, fmt.Errorf("%w: token %d of %s at %s", ErrSnapshotPausedBody,
				token.ID, symbolText(e.action), ActionNodeName(token.Location))
		}
	}
	// Reading a feature may derive its default; a probe gives that back.
	defer e.ctx.beginProbe()()
	s := &stateSpeller{exec: e, ctx: e.ctx, paths: make(map[int64]string), tokens: make(map[int64]string)}
	text := s.spell()
	return canonicalForm{text: text, tokens: s.tokens}, nil
}

// stateSpeller writes the canonical form, naming objects by materialization
// path and performances by their path in the action.
type stateSpeller struct {
	exec *ActionExecutor
	ctx  *Context
	// paths are the objects mentioned so far by path; mentioned lists them in order.
	paths     map[int64]string
	mentioned []int64
	// labels name the reachable performances canonically; tokens name the tokens.
	labels map[*actionFrame]string
	tokens map[int64]string
	out    strings.Builder
}

func (s *stateSpeller) spell() string {
	e := s.exec
	frames := s.labelFrames()
	fmt.Fprintf(&s.out, "state %s\n", e.state)
	fmt.Fprintf(&s.out, "clock t=%s\n", semantics.FormatReal(s.ctx.clock.now))
	if e.self != nil {
		fmt.Fprintf(&s.out, "self %s\n", s.object(e.self.ID))
	}
	for _, perf := range frames {
		s.frame(perf)
	}
	tokens := make([]string, 0, len(e.tokens))
	alike := make(map[string]int)
	for _, token := range slices.SortedFunc(slices.Values(e.tokens), func(a, b Token) int { return cmp.Compare(a.ID, b.ID) }) {
		text := s.token(token)
		tokens = append(tokens, text)
		alike[text]++
		s.tokens[token.ID] = fmt.Sprintf("%s #%d", text, alike[text])
	}
	sort.Strings(tokens)
	for _, line := range tokens {
		s.out.WriteString(line)
		s.out.WriteByte('\n')
	}
	for i, msg := range s.ctx.messages {
		fmt.Fprintf(&s.out, "message %d: %s\n", i+1, s.message(msg))
	}
	s.objects()
	return s.out.String()
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
		fmt.Fprintf(&b, "object %s: %s{%s}", s.paths[id], s.typeName(inst), s.features(inst))
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
func (s *stateSpeller) frame(perf *actionFrame) {
	fmt.Fprintf(&s.out, "frame %s:", s.frameLabel(perf))
	if perf.ended {
		s.out.WriteString(" ended")
	}
	if perf.inBody {
		s.out.WriteString(" body")
	}
	fmt.Fprintf(&s.out, " live=%d", perf.live)
	fmt.Fprintf(&s.out, " data{%s}", s.values(perf.data))
	for _, local := range perf.locals {
		fmt.Fprintf(&s.out, " local{%s}", s.values(local))
	}
	for _, node := range sortedNodes(perf.pending) {
		pins := perf.pending[node]
		names := make([]string, 0, len(pins))
		for pin := range pins {
			names = append(names, pin)
		}
		sort.Strings(names)
		for _, pin := range names {
			fmt.Fprintf(&s.out, " pending{%s.%s = (%s)}", s.node(perf.graph, node), pin, s.elements(pins[pin]))
		}
	}
	for _, node := range sortedNodes(perf.nested) {
		for _, delivery := range perf.nested[node] {
			path := make([]string, 0, len(delivery.path)+1)
			path = append(path, s.node(perf.graph, node))
			for _, step := range delivery.path {
				path = append(path, nodeIdentifier(step))
			}
			fmt.Fprintf(&s.out, " nested{%s.%s = %s}", strings.Join(path, "."), delivery.pin, s.value(delivery.value))
		}
	}
	for _, node := range sortedNodes(perf.subactions) {
		fmt.Fprintf(&s.out, " latest{%s = %s}", s.node(perf.graph, node), s.frameLabel(perf.subactions[node]))
	}
	s.out.WriteByte('\n')
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
		b.WriteString(" (" + s.symbolName(m.Signal) + ")")
	}
	if m.EventName != "" || m.Event != nil {
		name := m.EventName
		if m.Event != nil {
			name = s.symbolName(m.Event)
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

func (s *stateSpeller) symbolName(sym *symbols.Symbol) string {
	if fqn := s.ctx.fqnOf(sym); fqn != "" {
		return fqn
	}
	return sym.Name
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

// objectPath is the object's materialization path: the holder's path and the
// feature holding it, indexed within a feature holding several; a root object is
// its type and its rank among the roots of that type by the order they were made.
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
		path = s.rootPath(inst)
	} else {
		path = s.objectPath(owner.ID) + "." + feature
		if fv := owner.FeatureValues[feature]; fv != nil && !fv.Feature.Scalar() && fv.Values.Kind == ValSequence && fv.Values.Sequence() != nil {
			for i, element := range fv.Values.Sequence().Elements() {
				if element.Kind == ValInstance && element.Instance == id {
					path += "[" + strconv.Itoa(i) + "]"
					break
				}
			}
		}
	}
	s.paths[id] = path
	s.mentioned = append(s.mentioned, id)
	return path
}

// rootPath names an object no other holds by its type and its rank among the
// roots of that type, in the order the run made them.
func (s *stateSpeller) rootPath(inst *Instance) string {
	rank := 0
	for _, id := range s.ctx.created {
		other, ok := s.ctx.Instance(id)
		if !ok || other.Type != inst.Type {
			continue
		}
		if owner, _ := other.Owner(); owner != nil {
			continue
		}
		rank++
		if id == inst.ID {
			break
		}
	}
	return fmt.Sprintf("%s#%d", s.typeName(inst), rank)
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

func (s *stateSpeller) typeName(inst *Instance) string {
	if inst.Type == nil {
		return "object"
	}
	return s.symbolName(inst.Type)
}
