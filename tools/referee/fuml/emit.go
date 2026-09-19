package fuml

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Package is the package every translated model declares its action definitions in.
const Package = "fuml"

// TranslateError reports a construct of an expressible activity the emitter
// does not spell: which activity, where in it, and why.
type TranslateError struct {
	Activity string
	Where    string
	Reason   string
}

func (e *TranslateError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Activity, e.Where, e.Reason)
}

// Emitted is one activity translated by rule into SysML v2 textual notation,
// with every activity it calls.
type Emitted struct {
	// Name is the model's file name; Text its notation.
	Name string
	Text string
	// Qualified is the qualified name of the action definition the activity became.
	Qualified string
	// Activity is the translated activity.
	Activity *Activity
	// Produces maps each action node of the activity's own flow to the keys
	// (`node.pin`) its output pins have among a run's outputs.
	Produces map[string][]string
}

// Emit translates an activity and the activities it transitively calls into
// one model. A construct the pilot emitter does not spell is a TranslateError;
// the classifier decides expressibility, the emitter what it can translate.
func Emit(a *Activity) (*Emitted, error) {
	defs, err := calledActivities(a)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, d := range defs {
		if names[d.Name] {
			return nil, &TranslateError{a.Name, "activity " + d.Name, "two activities in the call closure share the name"}
		}
		names[d.Name] = true
	}
	spelled, err := spellParameters(a, defs)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s {\n\tprivate import ScalarValues::*;\n", Package)
	em := &Emitted{Name: a.Name + ".sysml", Qualified: Package + "::" + a.Name, Activity: a}
	for _, d := range defs {
		text, s, err := emitActivity(d, names, spelled)
		if err != nil {
			return nil, err
		}
		b.WriteString(text)
		if d == a {
			em.Produces = s.produces()
		}
	}
	b.WriteString("}\n")
	em.Text = b.String()
	return em, nil
}

// calledActivities is the activity and every model activity it transitively
// calls, in name order.
func calledActivities(a *Activity) ([]*Activity, error) {
	seen := map[*Activity]bool{}
	var out []*Activity
	var visit func(*Activity) error
	visit = func(d *Activity) error {
		if seen[d] {
			return nil
		}
		seen[d] = true
		out = append(out, d)
		for _, n := range d.AllNodes() {
			if n.Kind != CallBehaviorAction || n.Behavior == nil || n.Behavior.Activity == nil {
				continue
			}
			if err := visit(n.Behavior.Activity); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(a); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// spellParameters names every parameter: a called activity's is `Activity_name` when
// another definition declares the name, since the runtime returns a nested action's
// outputs to same-named enclosing features; the translated activity's keep theirs.
func spellParameters(root *Activity, defs []*Activity) (map[*Parameter]string, error) {
	declared := map[string]int{}
	for _, d := range defs {
		for _, p := range d.Parameters {
			declared[p.Name]++
		}
	}
	spelled := map[*Parameter]string{}
	for _, d := range defs {
		for _, p := range d.Parameters {
			name := p.Name
			if d != root && declared[name] > 1 {
				name = d.Name + "_" + p.Name
				if declared[name] > 0 {
					return nil, &TranslateError{root.Name, parameterLabel(d.Name + "." + p.Name), "cannot be spelled " + name + ", which another parameter is named"}
				}
			}
			spelled[p] = name
		}
	}
	return spelled, nil
}

// Outputs lists the activity's output, inout and return parameters: the values
// the referee compares.
func (a *Activity) Outputs() []*Parameter {
	var out []*Parameter
	for _, p := range a.Parameters {
		if p.Direction != In {
			out = append(out, p)
		}
	}
	return out
}

// Inputs lists the activity's input and inout parameters.
func (a *Activity) Inputs() []*Parameter {
	var in []*Parameter
	for _, p := range a.Parameters {
		if p.Direction == In || p.Direction == InOut {
			in = append(in, p)
		}
	}
	return in
}

// scalarTypes maps the fUML primitive types to ScalarValues.
var scalarTypes = map[string]string{
	"Integer": "Integer",
	"Boolean": "Boolean",
	"String":  "String",
	"Real":    "Real",
}

// identRe is a name that needs no quoting.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// quote spells a name in the notation, quoting it unless it is a plain identifier.
func quote(name string) string {
	if identRe.MatchString(name) && !source.IsKeyword(name) {
		return name
	}
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(name) + "'"
}

// emitter translates one activity into one action definition.
type emitter struct {
	a    *Activity
	defs map[string]bool
	// spelled names every parameter of the model's definitions (spellParameters).
	spelled map[*Parameter]string
}

// scope is one flow: the activity's own or a structured node's. Its nodes are
// declared inside the action, its successions and flows join them.
type scope struct {
	e     *emitter
	owner *Node
	names *namer
	nodes []*snode
	of    map[*Node]*snode
	succs []succession
	flows []flow
	// pins names the feature each fUML pin is spelled as, on its action.
	pins map[*Node]string
	// boundary maps a crossing pin to the feature the structured node declares for it.
	boundary map[*Node]string
	// params are the parameters this scope's action declares, boundary pins included.
	params []string
}

// snode is one node of the emitted flow.
type snode struct {
	name string
	kind snodeKind
	decl string
	node *Node
	// collector is set for the node that assigns a parameter or boundary pin:
	// it never starts on its own.
	collector bool
	// pending is set for a node that may never fire: an unfed pin with a lower bound.
	pending bool
}

type snodeKind int

const (
	kindAction snodeKind = iota
	kindFork
	kindJoin
	kindMerge
	kindDecide
	kindDone
)

var controlDecl = map[snodeKind]string{kindFork: "fork", kindJoin: "join", kindMerge: "merge", kindDecide: "decide"}

type succession struct {
	src, tgt *snode
	guard    string
}

type flow struct {
	src, tgt       *snode
	srcPin, tgtPin string
}

// namer hands out names unique within a scope.
type namer struct{ used map[string]bool }

func newNamer(reserved ...string) *namer {
	n := &namer{used: map[string]bool{}}
	for _, r := range reserved {
		n.used[r] = true
	}
	return n
}

func (n *namer) name(want string) string {
	name := want
	for i := 2; n.used[name]; i++ {
		name = fmt.Sprintf("%s #%d", want, i)
	}
	n.used[name] = true
	return name
}

// emitActivity spells one activity as an action definition with its parameters.
func emitActivity(a *Activity, defs map[string]bool, spelled map[*Parameter]string) (string, *scope, error) {
	if a.Owner != nil {
		return "", nil, &TranslateError{a.Name, "activity", "an owned behavior of a class is not translated by the pilot emitter"}
	}
	e := &emitter{a: a, defs: defs, spelled: spelled}
	s, err := e.build(nil, a.Nodes)
	if err != nil {
		return "", nil, err
	}
	var params []string
	for _, p := range a.Parameters {
		dir := string(p.Direction)
		if p.Direction == Return {
			dir = "out"
		}
		t, err := e.typeOf(p.Type, parameterLabel(p.Name))
		if err != nil {
			return "", nil, err
		}
		params = append(params, parameter(dir, e.spelled[p], t, p.Multiplicity))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\taction def %s {\n", quote(a.Name))
	for _, p := range params {
		b.WriteString("\t\t" + p + "\n")
	}
	s.write(&b, "\t\t")
	b.WriteString("\t}\n")
	return b.String(), s, nil
}

// parameter spells a parameter. An optional output holds nothing until a token
// arrives, so it is declared empty rather than left without a value.
func parameter(dir, name, t string, m Multiplicity) string {
	decl := fmt.Sprintf("%s %s : %s%s", dir, quote(name), t, multiplicity(m))
	if dir == "out" && m.Lower == 0 {
		decl += " = ()"
	}
	return decl + ";"
}

// produces maps each named action node of the flow to the output keys its pins have.
func (s *scope) produces() map[string][]string {
	out := map[string][]string{}
	for _, n := range s.nodes {
		if n.node == nil || n.kind != kindAction || n.node.Name == "" || !n.node.Kind.Action() {
			continue
		}
		for _, pin := range n.node.Outputs() {
			if f := s.pins[pin]; f != "" {
				out[n.node.Name] = append(out[n.node.Name], n.name+"."+f)
			}
		}
	}
	return out
}

// multiplicity spells a multiplicity. A multi-valued one is `[0..*] nonunique`:
// it fills one token at a time and the implementation keeps every token whatever
// isUnique says; ordered is kept, as the referee compares an ordered parameter in order.
func multiplicity(m Multiplicity) string {
	switch {
	case m.Lower == 1 && m.Upper == 1:
		return ""
	case m.Upper == 1:
		return fmt.Sprintf("[%d..1]", m.Lower)
	case m.Ordered:
		return "[0..*] ordered nonunique"
	}
	return "[0..*] nonunique"
}

// typeOf spells a primitive type; the pilot emitter has no spelling for a class.
func (e *emitter) typeOf(t TypeRef, where string) (string, error) {
	if t.Zero() {
		return "", &TranslateError{e.a.Name, where, "has no type"}
	}
	if name, ok := scalarTypes[t.Name]; ok {
		return name, nil
	}
	return "", &TranslateError{e.a.Name, where, untranslated("type " + t.Name)}
}

func (e *emitter) fail(where, reason string) error {
	return &TranslateError{e.a.Name, where, reason}
}

// untranslated is the reason for a construct the pilot emitter has no spelling for.
func untranslated(what string) string {
	return what + " is not translated by the pilot emitter"
}

// parameterLabel names a parameter as the where of a translation error.
func parameterLabel(name string) string {
	return "parameter " + name
}

// undeclaredNode is the reason for an edge whose end the emitter never declared.
const undeclaredNode = "joins a node the emitter did not declare"

// pinType spells a pin's type. A library list function leaves its element type
// open, so an untyped input pin takes the type of the value flowing into it and an
// untyped result pin the element type of the call's inputs.
func (e *emitter) pinType(p *Node) (string, error) {
	if !p.Type.Zero() {
		return e.typeOf(p.Type, pinLabel(p))
	}
	if p.Kind == InputPin {
		if t := flowedType(p, map[*Node]bool{}); !t.Zero() {
			return e.typeOf(t, pinLabel(p))
		}
	} else if p.Owner != nil {
		for _, in := range p.Owner.Inputs() {
			if t, err := e.pinType(in); err == nil {
				return t, nil
			}
		}
	}
	return "", e.fail(pinLabel(p), "has no type")
}

// flowedType is the type of the value reaching an untyped node along its
// incoming object flows, looked for back through control nodes.
func flowedType(n *Node, seen map[*Node]bool) TypeRef {
	if seen[n] {
		return TypeRef{}
	}
	seen[n] = true
	for _, in := range n.Incoming {
		if in.Kind != ObjectFlow {
			continue
		}
		if t := effectiveType(in.Source); !t.Zero() {
			return t
		}
		if t := flowedType(in.Source, seen); !t.Zero() {
			return t
		}
	}
	return TypeRef{}
}

// build translates the nodes of one flow and the edges among them.
func (e *emitter) build(owner *Node, nodes []*Node) (*scope, error) {
	s := &scope{
		e: e, owner: owner,
		names:    newNamer("start", "done", "Start"),
		of:       map[*Node]*snode{},
		pins:     map[*Node]string{},
		boundary: map[*Node]string{},
	}
	for name := range e.defs {
		s.names.used[name] = true
	}
	for _, p := range e.a.Parameters {
		s.names.used[e.spelled[p]] = true
	}
	for _, n := range nodes {
		if err := s.declare(n); err != nil {
			return nil, err
		}
	}
	for _, edge := range e.a.Edges {
		if err := s.edge(edge); err != nil {
			return nil, err
		}
	}
	if err := s.trace(); err != nil {
		return nil, err
	}
	s.start()
	s.fanOut()
	return s, nil
}

// scopeOf is the structured node whose flow the node is in, nil for the
// activity's own flow; a pin is in its action's.
func scopeOf(n *Node) *Node {
	o := n.Owner
	if n.Kind.Pin() && o != nil {
		o = o.Owner
	}
	return o
}

// declare spells one node.
func (s *scope) declare(n *Node) error {
	e := s.e
	switch n.Kind {
	case FlowFinalNode:
		return nil
	case ActivityFinalNode:
		s.add(&snode{name: "done", kind: kindDone, node: n})
		return nil
	case InitialNode, ForkNode:
		s.control(n, kindFork)
	case JoinNode:
		s.control(n, kindJoin)
	case MergeNode:
		s.control(n, kindMerge)
	case DecisionNode:
		s.control(n, kindDecide)
	case ActivityParameterNode:
		return s.parameterNode(n)
	case ValueSpecificationAction:
		return s.valueNode(n)
	case CallBehaviorAction:
		return s.callNode(n)
	case StructuredActivityNode:
		return s.structuredNode(n)
	default:
		return e.fail(n.Label(), untranslated(string(n.Kind)))
	}
	return nil
}

func (s *scope) add(n *snode) *snode {
	s.nodes = append(s.nodes, n)
	if n.node != nil {
		s.of[n.node] = n
	}
	return n
}

func (s *scope) control(n *Node, kind snodeKind) {
	name := s.names.name(nodeName(n))
	s.add(&snode{name: name, kind: kind, node: n, decl: controlDecl[kind] + " " + quote(name) + ";"})
}

// nodeName is the name a node is declared under before uniquing.
func nodeName(n *Node) string {
	if n.Name != "" {
		return n.Name
	}
	return fmt.Sprintf("%s(%d)", n.Kind, n.Line)
}

// parameterNode spells an activity parameter node: an input's reads the
// parameter into a pin, an output's collects what arrives into the parameter.
func (s *scope) parameterNode(n *Node) error {
	p := n.Parameter
	if p == nil {
		return s.e.fail(n.Label(), "names no parameter")
	}
	if s.owner != nil {
		return s.e.fail(n.Label(), "an activity parameter node inside a structured node is not translated by the pilot emitter")
	}
	t, err := s.e.typeOf(p.Type, parameterLabel(p.Name))
	if err != nil {
		return err
	}
	name := s.names.name(nodeName(n))
	s.pins[n] = "v"
	feature := quote(s.e.spelled[p])
	if p.Direction == In || p.Direction == InOut {
		s.add(&snode{name: name, kind: kindAction, node: n,
			decl: fmt.Sprintf("action %s { out v : %s%s = %s; }", quote(name), t, multiplicity(p.Multiplicity), feature)})
		return nil
	}
	s.add(&snode{name: name, kind: kindAction, node: n, collector: true,
		decl: fmt.Sprintf("action %s { %s }", quote(name), collect(feature, t, p.Multiplicity))})
	return nil
}

// collect spells the body of an action that takes v and puts it into target: a
// scalar is assigned, a multi-valued target is appended to.
func collect(target, t string, m Multiplicity) string {
	if m.Upper == 1 {
		return fmt.Sprintf("in v : %s[0..1]; assign %s := v;", t, target)
	}
	return fmt.Sprintf("in v : %s[0..*] nonunique; assign %s := (%s, v);", t, target, target)
}

// valueNode spells a value specification action: an action with one output pin
// holding the literal.
func (s *scope) valueNode(n *Node) error {
	e := s.e
	outs := n.Outputs()
	if len(outs) != 1 || len(n.Inputs()) != 0 {
		return e.fail(n.Label(), "a value specification action has one result pin")
	}
	if n.Value == nil {
		return e.fail(n.Label(), "has no value")
	}
	// The implementation puts the evaluated literal on the pin whatever the pin's
	// type says, so the literal's kind types the pin.
	pin := outs[0]
	t, ok := literalTypes[n.Value.Kind]
	if !ok {
		return e.fail(n.Label(), untranslated(n.Value.Kind))
	}
	lit, err := e.literal(n.Value, t, n.Label())
	if err != nil {
		return err
	}
	name := s.names.name(nodeName(n))
	pinName := "result"
	s.pins[pin] = pinName
	s.add(&snode{name: name, kind: kindAction, node: n,
		decl: fmt.Sprintf("action %s { out %s : %s%s = %s; }", quote(name), pinName, t, multiplicity(pin.Multiplicity), lit)})
	return nil
}

// literalTypes is the primitive type each literal kind evaluates to.
var literalTypes = map[string]string{
	"LiteralInteger": "Integer",
	"LiteralBoolean": "Boolean",
	"LiteralString":  "String",
	"LiteralReal":    "Real",
}

// literal spells a literal value of the given primitive type.
func (e *emitter) literal(v *Value, t, where string) (string, error) {
	text := v.Text
	switch v.Kind {
	case "LiteralInteger":
		if !v.Given {
			text = "0"
		}
		if _, err := strconv.ParseInt(text, 10, 64); err != nil || t != "Integer" {
			return "", e.fail(where, fmt.Sprintf("LiteralInteger %q does not spell an %s", text, t))
		}
		return text, nil
	case "LiteralBoolean":
		if !v.Given {
			text = "false"
		}
		if (text != "true" && text != "false") || t != "Boolean" {
			return "", e.fail(where, fmt.Sprintf("LiteralBoolean %q does not spell a %s", text, t))
		}
		return text, nil
	case "LiteralString":
		if t != "String" {
			return "", e.fail(where, "LiteralString does not spell a "+t)
		}
		return strconv.Quote(text), nil
	case "LiteralReal":
		if !v.Given {
			text = "0"
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || t != "Real" {
			return "", e.fail(where, fmt.Sprintf("LiteralReal %q does not spell a %s", text, t))
		}
		return strconv.FormatFloat(f, 'f', -1, 64) + realSuffix(f), nil
	}
	return "", e.fail(where, untranslated(v.Kind))
}

// realSuffix makes a real literal spell as one: an integral value gets `.0`.
func realSuffix(f float64) string {
	if f == float64(int64(f)) {
		return ".0"
	}
	return ""
}

// callNode spells a call behavior action: of an activity, an action typed by its
// definition, whose pins are the definition's parameters; of a library function,
// an action computing its result from its argument pins.
func (s *scope) callNode(n *Node) error {
	e := s.e
	b := n.Behavior
	if b == nil {
		return e.fail(n.Label(), "names no behavior")
	}
	name := s.names.name(nodeName(n))
	if b.Activity != nil {
		if err := s.bindCallPins(n, b.Activity); err != nil {
			return err
		}
		s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
			decl: fmt.Sprintf("action %s : %s::%s;", quote(name), Package, quote(b.Activity.Name))})
		return nil
	}
	template, ok := libraryCalls[b.Name]
	if !ok {
		return e.fail(n.Label(), "calls "+b.Name+", which the pilot emitter does not spell")
	}
	pins := newNamer()
	var features, args []string
	for i, p := range n.Inputs() {
		t, err := e.pinType(p)
		if err != nil {
			return err
		}
		pn := pins.name(pinFeature(p, fmt.Sprintf("argument%d", i)))
		s.pins[p] = pn
		args = append(args, quote(pn))
		features = append(features, fmt.Sprintf("in %s : %s%s;", quote(pn), t, multiplicity(p.Multiplicity)))
	}
	outs := n.Outputs()
	if len(outs) != 1 {
		return e.fail(n.Label(), "a library function call has one result pin")
	}
	if len(args) != template.arity {
		return e.fail(n.Label(), fmt.Sprintf("%s takes %d arguments, the call has %d pins", b.Name, template.arity, len(args)))
	}
	r := outs[0]
	t, err := e.pinType(r)
	if err != nil {
		return err
	}
	rn := pins.name(pinFeature(r, "result"))
	s.pins[r] = rn
	features = append(features, fmt.Sprintf("out %s : %s%s = %s;", quote(rn), t, multiplicity(r.Multiplicity), template.spell(args)))
	s.add(&snode{name: name, kind: kindAction, node: n, pending: unfed(n),
		decl: fmt.Sprintf("action %s { %s }", quote(name), strings.Join(features, " "))})
	return nil
}

// pinFeature is the feature a pin of a generated action is spelled as.
func pinFeature(p *Node, fallback string) string {
	if p.Name != "" {
		return p.Name
	}
	return fallback
}

// unfed reports an action with an input pin that must hold a value and no edge
// to bring one: the implementation never fires it.
func unfed(n *Node) bool {
	for _, p := range n.Inputs() {
		if p.Multiplicity.Lower > 0 && len(p.Incoming) == 0 {
			return true
		}
	}
	return false
}

// bindCallPins names each pin of a call of an activity after the parameter it
// stands for: the argument pins are the in and inout parameters in order, the
// result pins the out, inout and return parameters.
func (s *scope) bindCallPins(n *Node, callee *Activity) error {
	e := s.e
	var ins, outs []*Parameter
	for _, p := range callee.Parameters {
		if p.Direction == In || p.Direction == InOut {
			ins = append(ins, p)
		}
		if p.Direction != In {
			outs = append(outs, p)
		}
	}
	for i, p := range n.Inputs() {
		if i >= len(ins) {
			return e.fail(pinLabel(p), fmt.Sprintf("%s has %d input parameters, the call %d argument pins", callee.Name, len(ins), len(n.Inputs())))
		}
		s.pins[p] = e.spelled[ins[i]]
	}
	for i, p := range n.Outputs() {
		if i >= len(outs) {
			return e.fail(pinLabel(p), fmt.Sprintf("%s has %d output parameters, the call %d result pins", callee.Name, len(outs), len(n.Outputs())))
		}
		s.pins[p] = e.spelled[outs[i]]
	}
	return nil
}

// structuredNode spells a structured activity node as a nested action owning
// the flow of its contents. A pin inside it that an edge crosses the boundary
// to reach becomes a parameter of the nested action.
func (s *scope) structuredNode(n *Node) error {
	e := s.e
	if len(n.Pins) > 0 {
		return e.fail(n.Label(), "a structured node with pins is not translated by the pilot emitter")
	}
	inner, err := e.build(n, n.Nodes)
	if err != nil {
		return err
	}
	name := s.names.name(nodeName(n))
	var b strings.Builder
	fmt.Fprintf(&b, "action %s {\n", quote(name))
	for _, p := range inner.params {
		b.WriteString("\t" + p + "\n")
	}
	inner.write(&b, "\t")
	b.WriteString("}")
	sn := s.add(&snode{name: name, kind: kindAction, node: n, decl: b.String()})
	// The boundary features the inner scope declared are this node's pins here.
	for pin, feature := range inner.boundary {
		s.pins[pin] = feature
		s.of[pin] = sn
	}
	return nil
}

// endpoint is the node an edge end is spelled at: a pin's action, a node's own.
func (s *scope) endpoint(n *Node) *snode {
	if n.Kind.Pin() {
		if sn, ok := s.of[n]; ok {
			return sn
		}
		return s.of[n.Owner]
	}
	return s.of[n]
}

// edge translates one edge whose ends are in this scope, or that crosses its
// boundary from the enclosing scope.
func (s *scope) edge(edge *Edge) error {
	e := s.e
	src, tgt := edge.Source, edge.Target
	if src == nil || tgt == nil {
		return e.fail("edge at line "+strconv.Itoa(edge.Line), "has an unresolved end")
	}
	from, to := scopeOf(src), scopeOf(tgt)
	if from != s.owner && to != s.owner {
		return nil
	}
	if from != to {
		return s.crossing(edge, from, to)
	}
	if !unitWeight(edge) {
		return e.fail(edgeLabel(edge), "an edge weight other than 1 is not translated by the pilot emitter")
	}
	if src.Kind == FlowFinalNode || src.Kind == ActivityFinalNode {
		return e.fail(edgeLabel(edge), "a final node has no outgoing edge")
	}
	if tgt.Kind == FlowFinalNode {
		return nil
	}
	a, b := s.endpoint(src), s.endpoint(tgt)
	if a == nil || b == nil {
		return e.fail(edgeLabel(edge), undeclaredNode)
	}
	guard := ""
	if edge.Guard != nil {
		if a.kind != kindDecide {
			return e.fail(edgeLabel(edge), "a guard is spelled on a decision's outgoing edge only")
		}
		expr, err := s.guard(src, edge)
		if err != nil {
			return err
		}
		guard = expr
	} else if a.kind == kindDecide {
		return e.fail(edgeLabel(edge), "a decision's outgoing edge needs a guard")
	}
	if edge.Kind == ObjectFlow && a.kind == kindAction && b.kind == kindAction {
		// The object flow itself is spelled by trace; here it enables its target.
		s.enable(a, b)
		return nil
	}
	s.succs = append(s.succs, succession{a, b, guard})
	return nil
}

// unitWeight reports an edge carrying one token per firing, UML's default.
func unitWeight(e *Edge) bool {
	w := e.Weight
	return w == nil || (w.Kind == "LiteralInteger" || w.Kind == "LiteralUnlimitedNatural") && w.Given && w.Text == "1"
}

// enable adds the succession an object flow implies, once per pair of actions;
// a collector performs once per flow, since each performance takes one delivery.
func (s *scope) enable(a, b *snode) {
	for _, x := range s.succs {
		if x.src == a && x.tgt == b && !b.collector {
			return
		}
	}
	s.succs = append(s.succs, succession{src: a, tgt: b})
}

// crossing translates an object flow between this scope and a structured node
// directly inside it: the node gets a parameter the flow ends at, and its inner
// scope a node reading or collecting it.
func (s *scope) crossing(edge *Edge, from, to *Node) error {
	e := s.e
	if edge.Kind != ObjectFlow {
		return e.fail(edgeLabel(edge), "a control flow across a structured node's boundary is not translated by the pilot emitter")
	}
	if edge.Guard != nil || !unitWeight(edge) {
		return e.fail(edgeLabel(edge), "a guarded or weighted flow across a structured node's boundary is not translated by the pilot emitter")
	}
	src, tgt := edge.Source, edge.Target
	switch {
	case s.owner == from && to != nil && to.Owner == s.owner && tgt.Kind.Pin():
		// Entering the structured node: the outer half, ending at its new parameter.
		sn, a := s.of[to], s.endpoint(src)
		if sn == nil || a == nil {
			return e.fail(edgeLabel(edge), undeclaredNode)
		}
		if _, ok := s.pins[tgt]; !ok {
			return e.fail(edgeLabel(edge), "enters a structured node at a pin its inner flow did not declare")
		}
		if s.pins[src] == "" {
			return e.fail(edgeLabel(edge), "enters a structured node from a node without a pin, which the pilot emitter does not translate")
		}
		s.flows = append(s.flows, flow{a, sn, s.pins[src], s.pins[tgt]})
		s.enable(a, sn)
	case s.owner == to && from != nil && from.Owner == s.owner && src.Kind.Pin():
		// Leaving the structured node: the outer half, starting at its new parameter.
		sn, b := s.of[from], s.endpoint(tgt)
		if sn == nil || b == nil {
			return e.fail(edgeLabel(edge), undeclaredNode)
		}
		if _, ok := s.pins[src]; !ok {
			return e.fail(edgeLabel(edge), "leaves a structured node at a pin its inner flow did not declare")
		}
		if b.kind != kindAction || s.pins[tgt] == "" {
			return e.fail(edgeLabel(edge), "a flow leaving a structured node into a control node is not translated by the pilot emitter")
		}
		s.flows = append(s.flows, flow{sn, b, s.pins[src], s.pins[tgt]})
		s.enable(sn, b)
	case s.owner == to && s.owner != nil && from == s.owner.Owner && tgt.Kind.Pin():
		// Entering: the inner half, a node reading the parameter into the pin.
		return s.boundaryIn(edge, tgt)
	case s.owner == from && s.owner != nil && to == s.owner.Owner && src.Kind.Pin():
		// Leaving: the inner half, a node collecting the pin into the parameter.
		return s.boundaryOut(edge, src)
	default:
		return e.fail(edgeLabel(edge), "crosses more than one structured node boundary, which the pilot emitter does not translate")
	}
	return nil
}

// boundaryIn declares the parameter an entering flow delivers to and the node
// that reads it to the pin it was headed for.
func (s *scope) boundaryIn(edge *Edge, pin *Node) error {
	t, err := s.e.typeOf(pin.Type, pinLabel(pin))
	if err != nil {
		return err
	}
	feature, ok := s.boundary[pin]
	if !ok {
		feature = s.names.name("in(" + pinLabel(pin) + ")")
		s.boundary[pin] = feature
		s.params = append(s.params, parameter("in", feature, t, pin.Multiplicity))
	}
	name := s.names.name("read(" + pinLabel(pin) + ")")
	reader := s.add(&snode{name: name, kind: kindAction,
		decl: fmt.Sprintf("action %s { out v : %s%s = %s; }", quote(name), t, multiplicity(pin.Multiplicity), quote(feature))})
	tgt := s.endpoint(pin)
	if tgt == nil {
		return s.e.fail(edgeLabel(edge), "enters a node the emitter did not declare")
	}
	s.flows = append(s.flows, flow{reader, tgt, "v", s.pins[pin]})
	s.enable(reader, tgt)
	return nil
}

// boundaryOut declares the parameter a leaving flow carries out and the node
// that collects the pin into it.
func (s *scope) boundaryOut(edge *Edge, pin *Node) error {
	t, err := s.e.typeOf(pin.Type, pinLabel(pin))
	if err != nil {
		return err
	}
	feature, ok := s.boundary[pin]
	if !ok {
		feature = s.names.name("out(" + pinLabel(pin) + ")")
		s.boundary[pin] = feature
		s.params = append(s.params, parameter("out", feature, t, pin.Multiplicity))
	}
	name := s.names.name("write(" + pinLabel(pin) + ")")
	writer := s.add(&snode{name: name, kind: kindAction, collector: true,
		decl: fmt.Sprintf("action %s { %s }", quote(name), collect(quote(feature), t, pin.Multiplicity))})
	src := s.endpoint(pin)
	if src == nil {
		return s.e.fail(edgeLabel(edge), "leaves a node the emitter did not declare")
	}
	s.flows = append(s.flows, flow{src, writer, s.pins[pin], "v"})
	s.enable(src, writer)
	return nil
}

func edgeLabel(e *Edge) string {
	if e.Name != "" {
		return string(e.Kind) + " " + e.Name
	}
	return fmt.Sprintf("%s %s -> %s", e.Kind, e.Source.Label(), e.Target.Label())
}

// guard spells the guard of a decision's outgoing edge as a test of the value
// the decision decides on.
func (s *scope) guard(decision *Node, edge *Edge) (string, error) {
	e := s.e
	input, err := s.decisionInput(decision)
	if err != nil {
		return "", err
	}
	t, err := e.typeOf(effectiveType(input), pinLabel(input))
	if err != nil {
		return "", err
	}
	lit, err := e.literal(edge.Guard, t, edgeLabel(edge))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s == %s", quote(s.endpoint(input).name), quote(s.pins[input]), lit), nil
}

// decisionInput is the pin whose value a decision decides on: its decision
// input flow's source, else the source of its one incoming object flow, looked
// for back through control nodes with a single input.
func (s *scope) decisionInput(d *Node) (*Node, error) {
	e := s.e
	var edge *Edge
	if d.DecisionInputFlow != nil {
		edge = d.DecisionInputFlow
	} else {
		for _, in := range d.Incoming {
			if in.Kind != ObjectFlow {
				continue
			}
			if edge != nil {
				return nil, e.fail(d.Label(), "decides on more than one incoming object flow")
			}
			edge = in
		}
	}
	if edge == nil {
		return nil, e.fail(d.Label(), "decides on a control token, which has no value to guard on")
	}
	src := edge.Source
	for src != nil && src.Kind.Control() {
		var in *Edge
		for _, x := range src.Incoming {
			if x.Kind != ObjectFlow {
				continue
			}
			if in != nil {
				return nil, e.fail(d.Label(), "decides on a value merged from more than one source")
			}
			in = x
		}
		if in == nil {
			return nil, e.fail(d.Label(), "decides on a control token, which has no value to guard on")
		}
		src = in.Source
	}
	if src == nil || s.endpoint(src) == nil || s.pins[src] == "" {
		return nil, e.fail(d.Label(), "decides on a value the emitter did not spell")
	}
	return src, nil
}

// trace spells every object flow end to end: from each pin or parameter node
// that produces a value, through the control nodes routing its token, to every
// pin or parameter node the token can reach. Each route is one flow.
func (s *scope) trace() error {
	for _, n := range s.nodes {
		if n.node == nil || n.kind != kindAction {
			continue
		}
		var sources []*Node
		if n.node.Kind == ActivityParameterNode {
			sources = []*Node{n.node}
		} else {
			sources = n.node.Outputs()
		}
		for _, src := range sources {
			if s.pins[src] == "" {
				continue
			}
			for _, out := range src.Outgoing {
				if out.Kind != ObjectFlow || scopeOf(out.Target) != s.owner {
					continue
				}
				if err := s.route(n, src, out, map[*Edge]bool{}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// route follows one object token from its source pin along an edge.
func (s *scope) route(src *snode, srcPin *Node, edge *Edge, seen map[*Edge]bool) error {
	e := s.e
	if seen[edge] {
		return e.fail(edgeLabel(edge), "an object flow cycle through control nodes is not translated by the pilot emitter")
	}
	seen[edge] = true
	defer delete(seen, edge)
	tgt := edge.Target
	switch {
	case tgt.Kind == FlowFinalNode || tgt.Kind == ActivityFinalNode:
		return nil
	case tgt.Kind.Control():
		for _, out := range tgt.Outgoing {
			if out.Kind != ObjectFlow {
				return e.fail(edgeLabel(out), "a control flow carries an object token on")
			}
			if err := s.route(src, srcPin, out, seen); err != nil {
				return err
			}
		}
		return nil
	case tgt.Kind.Pin() || tgt.Kind == ActivityParameterNode:
		to := s.endpoint(tgt)
		if to == nil || s.pins[tgt] == "" {
			return e.fail(edgeLabel(edge), "delivers to a pin the emitter did not spell")
		}
		s.flows = append(s.flows, flow{src, to, s.pins[srcPin], s.pins[tgt]})
		return nil
	}
	return e.fail(edgeLabel(edge), "delivers an object token to a "+string(tgt.Kind))
}

// start enables the nodes the implementation enables when the flow begins: those
// with nothing coming in. One is enabled by start itself, several through a fork.
func (s *scope) start() {
	incoming := map[*snode]int{}
	for _, x := range s.succs {
		incoming[x.tgt]++
	}
	var starts []*snode
	for _, n := range s.nodes {
		if n.kind == kindDone || n.collector || n.pending || incoming[n] > 0 {
			continue
		}
		starts = append(starts, n)
	}
	if len(starts) == 0 {
		return
	}
	from := &snode{name: "start"}
	if len(starts) > 1 {
		from = &snode{name: "Start", kind: kindFork, decl: "fork Start;"}
		s.nodes = append([]*snode{from}, s.nodes...)
		s.succs = append(s.succs, succession{src: &snode{name: "start"}, tgt: from})
	}
	for _, n := range starts {
		s.succs = append(s.succs, succession{src: from, tgt: n})
	}
}

// fanOut spells the implicit fork of an action with several successors, and the
// implicit merge of several edges into the final node or into a collector: a
// parameter node takes tokens as they come, so its collector performs per arrival.
func (s *scope) fanOut() {
	outgoing := map[*snode][]int{}
	incomingMerged := map[*snode][]int{}
	for i, x := range s.succs {
		outgoing[x.src] = append(outgoing[x.src], i)
		if x.tgt.kind == kindDone || x.tgt.collector {
			incomingMerged[x.tgt] = append(incomingMerged[x.tgt], i)
		}
	}
	for _, n := range append([]*snode(nil), s.nodes...) {
		if idx := outgoing[n]; len(idx) > 1 && n.kind != kindFork && n.kind != kindDecide {
			name := s.names.name(n.name + " fork")
			f := s.add(&snode{name: name, kind: kindFork, decl: "fork " + quote(name) + ";"})
			for _, i := range idx {
				s.succs[i].src = f
			}
			s.succs = append(s.succs, succession{src: n, tgt: f})
		}
		if idx := incomingMerged[n]; len(idx) > 1 {
			name := s.names.name(n.name + " merge")
			m := s.add(&snode{name: name, kind: kindMerge, decl: "merge " + quote(name) + ";"})
			for _, i := range idx {
				s.succs[i].tgt = m
			}
			s.succs = append(s.succs, succession{src: m, tgt: n})
		}
	}
}

// write spells the scope's declarations, then its flows and successions.
func (s *scope) write(b *strings.Builder, indent string) {
	for _, n := range s.nodes {
		if n.decl == "" {
			continue
		}
		for _, line := range strings.Split(n.decl, "\n") {
			b.WriteString(indent + line + "\n")
		}
	}
	for _, f := range s.flows {
		fmt.Fprintf(b, "%sflow %s.%s to %s.%s;\n", indent, quote(f.src.name), quote(f.srcPin), quote(f.tgt.name), quote(f.tgtPin))
	}
	for _, x := range s.succs {
		if x.guard != "" {
			fmt.Fprintf(b, "%ssuccession first %s if %s then %s;\n", indent, quote(x.src.name), x.guard, quote(x.tgt.name))
			continue
		}
		fmt.Fprintf(b, "%ssuccession first %s then %s;\n", indent, quote(x.src.name), quote(x.tgt.name))
	}
}

// libraryCall spells a call of a fUML library function over its arguments.
type libraryCall struct {
	arity int
	spell func(args []string) string
}

// call spells a KerML function applied to every argument in order.
func call(fn string, arity int) libraryCall {
	return libraryCall{arity, func(args []string) string { return fn + "(" + strings.Join(args, ", ") + ")" }}
}

// libraryCalls spells each library behavior LibraryCounterpart names.
var libraryCalls = map[string]libraryCall{
	"PrimitiveBehaviors::IntegerFunctions::+":   call("IntegerFunctions::'+'", 2),
	"PrimitiveBehaviors::IntegerFunctions::-":   call("IntegerFunctions::'-'", 2),
	"PrimitiveBehaviors::IntegerFunctions::*":   call("IntegerFunctions::'*'", 2),
	"PrimitiveBehaviors::IntegerFunctions::Neg": call("IntegerFunctions::'-'", 1),
	"PrimitiveBehaviors::IntegerFunctions::Div": {2, func(a []string) string {
		return fmt.Sprintf("RationalFunctions::ToInteger(IntegerFunctions::'/'(%s, %s))", a[0], a[1])
	}},
	"PrimitiveBehaviors::IntegerFunctions::Mod":      call("IntegerFunctions::'%'", 2),
	"PrimitiveBehaviors::IntegerFunctions::Abs":      call("IntegerFunctions::abs", 1),
	"PrimitiveBehaviors::IntegerFunctions::Max":      call("IntegerFunctions::max", 2),
	"PrimitiveBehaviors::IntegerFunctions::Min":      call("IntegerFunctions::min", 2),
	"PrimitiveBehaviors::IntegerFunctions::<":        call("IntegerFunctions::'<'", 2),
	"PrimitiveBehaviors::IntegerFunctions::<=":       call("IntegerFunctions::'<='", 2),
	"PrimitiveBehaviors::IntegerFunctions::>=":       call("IntegerFunctions::'>='", 2),
	"PrimitiveBehaviors::RealFunctions::+":           call("RealFunctions::'+'", 2),
	"PrimitiveBehaviors::RealFunctions::-":           call("RealFunctions::'-'", 2),
	"PrimitiveBehaviors::RealFunctions::*":           call("RealFunctions::'*'", 2),
	"PrimitiveBehaviors::RealFunctions::/":           call("RealFunctions::'/'", 2),
	"PrimitiveBehaviors::RealFunctions::Neg":         call("RealFunctions::'-'", 1),
	"PrimitiveBehaviors::RealFunctions::Inv":         {1, func(a []string) string { return "RealFunctions::'/'(1.0, " + a[0] + ")" }},
	"PrimitiveBehaviors::RealFunctions::Abs":         call("RealFunctions::abs", 1),
	"PrimitiveBehaviors::RealFunctions::Floor":       call("RealFunctions::floor", 1),
	"PrimitiveBehaviors::RealFunctions::Round":       call("RealFunctions::round", 1),
	"PrimitiveBehaviors::RealFunctions::Max":         call("RealFunctions::max", 2),
	"PrimitiveBehaviors::RealFunctions::Min":         call("RealFunctions::min", 2),
	"PrimitiveBehaviors::RealFunctions::<":           call("RealFunctions::'<'", 2),
	"PrimitiveBehaviors::RealFunctions::<=":          call("RealFunctions::'<='", 2),
	"PrimitiveBehaviors::RealFunctions::>":           call("RealFunctions::'>'", 2),
	"PrimitiveBehaviors::RealFunctions::>=":          call("RealFunctions::'>='", 2),
	"PrimitiveBehaviors::RealFunctions::ToInteger":   call("RealFunctions::ToInteger", 1),
	"PrimitiveBehaviors::BooleanFunctions::And":      call("BooleanFunctions::'&'", 2),
	"PrimitiveBehaviors::BooleanFunctions::Or":       call("BooleanFunctions::'|'", 2),
	"PrimitiveBehaviors::BooleanFunctions::Not":      call("BooleanFunctions::'not'", 1),
	"PrimitiveBehaviors::BooleanFunctions::Xor":      call("BooleanFunctions::'xor'", 2),
	"PrimitiveBehaviors::BooleanFunctions::Implies":  call("ControlFunctions::'implies'", 2),
	"PrimitiveBehaviors::StringFunctions::Concat":    call("StringFunctions::'+'", 2),
	"PrimitiveBehaviors::StringFunctions::Size":      call("StringFunctions::Length", 1),
	"PrimitiveBehaviors::StringFunctions::Substring": call("StringFunctions::Substring", 3),
	"PrimitiveBehaviors::ListFunctions::ListSize":    call("SequenceFunctions::size", 1),
	"PrimitiveBehaviors::ListFunctions::ListGet":     call("SequenceFunctions::'#'", 2),
	"PrimitiveBehaviors::ListFunctions::ListConcat":  call("SequenceFunctions::union", 2),
}

// IsTranslateError reports whether err is the emitter declining a construct.
func IsTranslateError(err error) bool {
	var t *TranslateError
	return errors.As(err, &t)
}
