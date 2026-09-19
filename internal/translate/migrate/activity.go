package migrate

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// activityBody writes the nodes and edges of act as the body of def, the v2
// action def being written: act itself, or the operation whose method it is.
func (m *migration) activityBody(act, def *sysmlv1.Element) {
	for _, c := range act.Children {
		switch c.Role {
		case "ownedBehavior", "nestedClassifier", "ownedAttribute":
			m.member(c)
		}
	}
	a := m.newActivity(act, def)
	a.write()
	a.partitions()
	a.rules()
	a.observations()
}

// activity writes one node graph: an activity's, or a structured node's.
type activity struct {
	m     *migration
	act   *sysmlv1.Element // the owner of the nodes and edges
	def   *sysmlv1.Element // the element whose v2 body is written; names are distinct in it
	names map[*sysmlv1.Element]string
	used  map[string]bool
	// next lists, for each node, the nodes its edges lead to, once each; succ the
	// edges out of it that stand for a succession, in the order they are owned.
	next map[*sysmlv1.Element][]*sysmlv1.Element
	prev map[*sysmlv1.Element][]*sysmlv1.Element
	succ map[*sysmlv1.Element][]*sysmlv1.Element
	// entry names what a succession into a node leads to: its join (its merge,
	// for a final) when several edges lead to it, else its wait when a duration constrains it.
	entry  map[*sysmlv1.Element]string
	joins  map[*sysmlv1.Element]string
	merges map[*sysmlv1.Element]string
	waits  map[*sysmlv1.Element]waitNode
	// selfFed marks pins the object read by a ReadSelfAction flows into.
	selfFed map[*sysmlv1.Element]bool
	// pinType is the classifier an untyped pin is declared with, when what
	// the pin feeds settles it.
	pinType map[*sysmlv1.Element]*sysmlv1.Element
	// payload is the signal an accept's result pin holds, which types the pin
	// whatever v1 said.
	payload map[*sysmlv1.Element]*sysmlv1.Element
	// sources maps each pin to what flows into it, followed back through control
	// and buffer nodes to the pins and parameter nodes that produce it, once each.
	sources map[*sysmlv1.Element][]*sysmlv1.Element
	// edgeSources and edgeSelf record the same per object flow: the producers
	// its own source leads back to, and whether one is a ReadSelfAction.
	edgeSources map[*sysmlv1.Element][]*sysmlv1.Element
	edgeSelf    map[*sysmlv1.Element]bool
	// inert marks the nodes written as placeholders, whose output pins no value reaches.
	inert map[*sysmlv1.Element]bool
	// dataOnly marks the object flows that carry a value into an action without
	// starting it: a control flow leads to the action, and that is what starts it.
	dataOnly map[*sysmlv1.Element]bool
	nodes    []*sysmlv1.Element
	edges    []*sysmlv1.Element
	// data lists the object flows to write once the nodes are declared.
	data []string
	// written marks the (producer, pin) pairs a flow or bind is already written for.
	written map[[2]*sysmlv1.Element]bool
}

func (m *migration) newActivity(act, def *sysmlv1.Element) *activity {
	a := &activity{
		m: m, act: act, def: def,
		names:       map[*sysmlv1.Element]string{},
		used:        inheritedActionNames(),
		next:        map[*sysmlv1.Element][]*sysmlv1.Element{},
		prev:        map[*sysmlv1.Element][]*sysmlv1.Element{},
		succ:        map[*sysmlv1.Element][]*sysmlv1.Element{},
		entry:       map[*sysmlv1.Element]string{},
		joins:       map[*sysmlv1.Element]string{},
		merges:      map[*sysmlv1.Element]string{},
		waits:       map[*sysmlv1.Element]waitNode{},
		selfFed:     map[*sysmlv1.Element]bool{},
		pinType:     map[*sysmlv1.Element]*sysmlv1.Element{},
		payload:     map[*sysmlv1.Element]*sysmlv1.Element{},
		sources:     map[*sysmlv1.Element][]*sysmlv1.Element{},
		dataOnly:    map[*sysmlv1.Element]bool{},
		edgeSources: map[*sysmlv1.Element][]*sysmlv1.Element{},
		edgeSelf:    map[*sysmlv1.Element]bool{},
		written:     map[[2]*sysmlv1.Element]bool{},
		inert:       map[*sysmlv1.Element]bool{},
		nodes:       act.Owned("node"),
		edges:       act.Owned("edge"),
	}
	for _, owner := range []*sysmlv1.Element{def, act} {
		for _, c := range owner.Children {
			if c.Role != "node" && c.Role != "edge" && m.nameOf(c) != "" {
				a.used[m.nameOf(c)] = true
			}
		}
	}
	// Every action inherits start and done, which shadow outer names.
	m.take(def, "start")
	m.take(def, "done")
	return a
}

// inheritedActionNames are the members every action usage inherits, which a
// synthesized member must not be called.
func inheritedActionNames() map[string]bool {
	return map[string]bool{"start": true, "done": true, "self": true}
}

// waitNode is the wait written before a node a duration constrains.
type waitNode struct{ name, delay string }

// The notation keywords and note fragments the writer repeats.
const (
	actionKw  = "action "
	firstKw   = "first "
	thenKw    = " then "
	noV2UML   = "no v2 form for a UML "
	noFeature = ", which has no feature "
)

// nodeKinds classifies the UML activity node metaclasses the writer knows.
const (
	nodeAction    = iota // an action written as an action usage
	nodeInitial          // start
	nodeFinal            // a terminate node: the activity ends
	nodeFlowFinal        // done: the token ends
	nodeParam            // an activity parameter node, named by its parameter
	nodeControl          // fork, join, decide, merge
	nodeBuffer           // an object node tokens pass through
	nodePin
)

func nodeKind(n *sysmlv1.Element) int {
	switch n.Type {
	case "InitialNode":
		return nodeInitial
	case "ActivityFinalNode":
		return nodeFinal
	case "FlowFinalNode":
		return nodeFlowFinal
	case "ActivityParameterNode":
		return nodeParam
	case "ForkNode", "JoinNode", "DecisionNode", "MergeNode":
		return nodeControl
	case "CentralBufferNode", "DataStoreNode", "ExpansionNode":
		return nodeBuffer
	case "InputPin", "OutputPin", "ValuePin", "ActionInputPin":
		return nodePin
	}
	return nodeAction
}

// ownerNode returns the node an edge end stands for: the action a pin belongs
// to, or the node itself.
func ownerNode(e *sysmlv1.Element) *sysmlv1.Element {
	if nodeKind(e) == nodePin && e.Parent != nil {
		return e.Parent
	}
	return e
}

// name returns the v2 name of a node, distinct in the body written, taking
// the node's own name when it has one and base with a number otherwise.
func (a *activity) name(n *sysmlv1.Element, base string) string {
	if s, ok := a.names[n]; ok {
		return s
	}
	name := a.m.nameOf(n)
	if name == "" || a.used[name] || a.m.taken[a.def][name] {
		if name == "" {
			name = base
		}
		name = a.fresh(name)
	}
	a.used[name] = true
	a.m.take(a.def, name)
	a.names[n] = name
	return name
}

// fresh returns base, or base with a number, not yet used in the body.
func (a *activity) fresh(base string) string {
	name := base
	for i := 2; a.used[name] || a.m.taken[a.def][name]; i++ {
		name = base + strconv.Itoa(i)
	}
	a.used[name] = true
	a.m.take(a.def, name)
	return name
}

// baseName is the name synthesized for an anonymous node of a type.
func baseName(n *sysmlv1.Element) string {
	switch n.Type {
	case "ForkNode":
		return "fork"
	case "JoinNode":
		return "join"
	case "DecisionNode":
		return "decide"
	case "MergeNode":
		return "merge"
	case "SendSignalAction":
		return "send"
	case "AcceptEventAction":
		return "accept"
	case "ActivityFinalNode":
		return "final"
	case "CallBehaviorAction", "CallOperationAction":
		return "call"
	case "ValueSpecificationAction":
		return "value"
	case "ReadStructuralFeatureAction", "ReadSelfAction":
		return "read"
	case "AddStructuralFeatureValueAction":
		return "write"
	}
	return "action"
}

// write writes the graph: the successions from start, then each node with
// its wait, declaration and outgoing successions, then the object flows.
func (a *activity) write() {
	a.link()
	a.resolveData()
	for _, n := range a.nodes {
		if k := nodeKind(n); k == nodeAction || k == nodeControl || k == nodeBuffer || k == nodeFinal {
			a.entries(n)
		}
	}
	a.startSuccessions()
	for _, n := range a.nodes {
		switch nodeKind(n) {
		case nodeAction, nodeControl, nodeBuffer:
			a.declare(n)
			a.successions(n)
		case nodeInitial:
			a.m.add(n, Mapped, "start", "")
		case nodeFinal:
			a.declare(n)
		case nodeFlowFinal:
			a.m.add(n, Mapped, "done", "a flow final ends the token, as done does")
		case nodeParam:
			a.parameterNode(n)
		default:
			a.m.unmapped(n, noV2UML+n.Type)
		}
	}
	a.m.w.lines(a.data)
	for _, e := range a.edges {
		if e.Type == "ObjectFlow" {
			a.objectFlow(e)
		}
	}
}

// link records the succession each edge stands for: every control flow, each with its
// own guard, and the first object flow between two nodes no control flow joins.
// An object flow into an action control flows also reach carries a value only.
func (a *activity) link() {
	controlled := map[*sysmlv1.Element]bool{}
	control := map[[2]*sysmlv1.Element]bool{}
	for _, e := range a.edges {
		src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
		if e.Type == "ControlFlow" && src != nil && tgt != nil {
			controlled[tgt] = true
			control[[2]*sysmlv1.Element{ownerNode(src), ownerNode(tgt)}] = true
		}
	}
	linked := map[[2]*sysmlv1.Element]bool{}
	for _, e := range a.edges {
		src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
		if src == nil || tgt == nil {
			continue
		}
		from, to := ownerNode(src), ownerNode(tgt)
		if nodeKind(from) == nodeParam || nodeKind(to) == nodeParam || from == to {
			// A parameter is there from the start, not a step of the flow.
			continue
		}
		if nodeKind(tgt) == nodePin && controlled[to] {
			a.dataOnly[e] = true
			continue
		}
		pair := [2]*sysmlv1.Element{from, to}
		if e.Type != "ControlFlow" && (control[pair] || linked[pair]) {
			continue
		}
		a.succ[from] = append(a.succ[from], e)
		if !linked[pair] {
			linked[pair] = true
			a.next[from] = append(a.next[from], to)
			a.prev[to] = append(a.prev[to], from)
		}
	}
}

// resolveData follows each object flow back through control and buffer nodes
// to the pin or parameter node its value comes from, `this` for a ReadSelfAction.
func (a *activity) resolveData() {
	into := map[*sysmlv1.Element][]*sysmlv1.Element{}
	for _, e := range a.edges {
		if e.Type != "ObjectFlow" {
			continue
		}
		src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
		if src != nil && tgt != nil {
			into[tgt] = append(into[tgt], src)
		}
	}
	var trace func(e *sysmlv1.Element, seen map[*sysmlv1.Element]bool) []*sysmlv1.Element
	trace = func(e *sysmlv1.Element, seen map[*sysmlv1.Element]bool) []*sysmlv1.Element {
		if seen[e] {
			return nil
		}
		seen[e] = true
		switch nodeKind(e) {
		case nodePin, nodeParam:
			return []*sysmlv1.Element{e}
		}
		var out []*sysmlv1.Element
		for _, s := range into[e] {
			out = append(out, trace(s, seen)...)
		}
		return out
	}
	for _, e := range a.edges {
		if e.Type != "ObjectFlow" {
			continue
		}
		src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
		if src == nil || tgt == nil || nodeKind(tgt) != nodePin && nodeKind(tgt) != nodeParam && tgt.Type != "DecisionNode" {
			continue
		}
		for _, s := range trace(src, map[*sysmlv1.Element]bool{}) {
			if s.Parent != nil && s.Parent.Type == "ReadSelfAction" {
				a.selfFed[tgt] = true
				a.edgeSelf[e] = true
				continue
			}
			a.edgeSources[e] = append(a.edgeSources[e], s)
			if !slices.Contains(a.sources[tgt], s) {
				a.sources[tgt] = append(a.sources[tgt], s)
			}
		}
	}
}

// entries names n and what leads into it: a join when several edges do (a merge
// into an activity final, which one token ends), a wait when a duration bounds it.
func (a *activity) entries(n *sysmlv1.Element) {
	name := writeName(a.name(n, baseName(n)))
	if delay, ok := a.waitFor(n); ok {
		w := writeName(a.fresh("wait"))
		a.waits[n] = waitNode{w, delay}
		name = w
	}
	switch {
	case len(a.prev[n]) <= 1 || n.Type == "JoinNode" || n.Type == "MergeNode":
	case nodeKind(n) == nodeFinal:
		m := writeName(a.fresh("merge"))
		a.merges[n] = m
		name = m
	default:
		j := writeName(a.fresh("join"))
		a.joins[n] = j
		name = j
	}
	a.entry[n] = name
}

// endpointIn names what a succession into n leads to: done for a flow final,
// which ends the token, and "" for a node nothing may lead to.
func (a *activity) endpointIn(n *sysmlv1.Element) string {
	switch nodeKind(n) {
	case nodeFlowFinal:
		return "done"
	case nodeInitial, nodeParam, nodePin:
		return ""
	}
	return a.entry[n]
}

// unwritableEdge reports an edge into a node no succession may lead to.
func (a *activity) unwritableEdge(e *sysmlv1.Element) {
	a.m.unmapped(e, "the edge leads to "+describe(ownerNode(a.m.model.Ref(e, "target")))+", "+a.unwritableTarget(e))
}

// unwritableTarget says why no succession may lead to an edge's target.
func (a *activity) unwritableTarget(e *sysmlv1.Element) string {
	n := ownerNode(a.m.model.Ref(e, "target"))
	switch {
	case n.IsProxy():
		return "which another document defines"
	case nodeKind(n) == nodeInitial:
		return "an initial node, which nothing may lead to"
	case a.entry[n] == "":
		return "which is not a node of the activity"
	}
	return "which no succession may lead to"
}

// startSuccessions writes the successions from start to the initial nodes' targets
// and to every node no edge leads to, forked when several, after the activity's wait.
func (a *activity) startSuccessions() {
	var targets []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	for _, n := range a.nodes {
		if nodeKind(n) != nodeInitial {
			continue
		}
		for _, t := range a.next[n] {
			if !seen[t] {
				seen[t] = true
				targets = append(targets, t)
			}
		}
	}
	for _, n := range a.nodes {
		k := nodeKind(n)
		if k != nodeAction && k != nodeControl && k != nodeBuffer || len(a.prev[n]) > 0 || seen[n] {
			continue
		}
		if k == nodeControl && n.Type != "ForkNode" {
			// A join or merge nothing leads to would never fire.
			continue
		}
		seen[n] = true
		targets = append(targets, n)
		a.m.add(n, Approximated, "", "no edge leads to the node, so it starts with the activity")
	}
	from := "start"
	if w, ok := a.waitFor(a.act); ok {
		name := a.fresh("wait")
		a.m.w.line("first start then " + writeName(name) + ";")
		a.m.w.line(actionKw + writeName(name) + " accept after " + w + ";")
		from = writeName(name)
	}
	if len(targets) > 1 {
		f := a.fresh("fork")
		a.m.w.line(firstKw + from + thenKw + writeName(f) + ";")
		a.m.w.line("fork " + writeName(f) + ";")
		from = writeName(f)
	}
	for _, t := range targets {
		if to := a.endpointIn(t); to != "" {
			a.m.w.line(firstKw + from + thenKw + to + ";")
		}
	}
}

// waitFor writes the delay a duration constraint on e stands for: a fixed
// delay for a point interval and a uniform draw over the interval otherwise.
func (a *activity) waitFor(e *sysmlv1.Element) (string, bool) {
	dcs := a.m.bounded[e]
	if len(dcs) == 0 {
		return "", false
	}
	dc := dcs[0]
	for _, other := range dcs[1:] {
		a.m.add(other, Unmapped, "", "a second duration constraint on "+describe(e)+"; only "+describe(dc)+" is written as its wait")
	}
	spec := firstOwned(dc, "specification")
	if spec == nil || spec.Type != "DurationInterval" && spec.Type != "Interval" {
		a.m.unmapped(dc, "the duration constraint has no interval")
		return "", false
	}
	lo, lok, lnote := a.m.durationExpr(a.m.model.Ref(spec, "min"), a.act)
	hi, hok, hnote := a.m.durationExpr(a.m.model.Ref(spec, "max"), a.act)
	if !lok || !hok {
		note := lnote
		if !lok && a.m.model.Ref(spec, "min") == nil {
			note = "the interval has no min"
		}
		if !hok {
			note = joinNotes(note, hnote)
			if a.m.model.Ref(spec, "max") == nil {
				note = joinNotes(note, "the interval has no max")
			}
		}
		a.unmappedWait(dc, e, note)
		return "", false
	}
	note := joinNotes(lnote, hnote)
	var expr string
	lf, lerr := strconv.ParseFloat(lo, 64)
	hf, herr := strconv.ParseFloat(hi, 64)
	switch {
	case lerr == nil && herr == nil && lf > hf:
		a.unmappedWait(dc, e, "the interval's min "+lo+" exceeds its max "+hi)
		return "", false
	case lo == hi:
		expr = lo
		note = joinNotes(note, "written as a fixed wait of "+lo+" s before "+describe(e))
	default:
		expr = "RandomFunctions::uniform(" + lo + ", " + hi + ")"
		note = joinNotes(note, "written as a wait drawn uniformly over ["+lo+", "+hi+"] s before "+describe(e)+"; a tool's fixed min or max mode is a run setting, not the model's")
	}
	a.m.add(dc, Approximated, a.m.v2Name(a.def), note)
	return expr + " [SI::s]", true
}

func (a *activity) unmappedWait(dc, e *sysmlv1.Element, note string) {
	a.m.w.lines(commentLines("duration constraint on " + describe(e) + " not migrated — " + note))
	a.m.add(dc, Unmapped, "", note)
}

// successions writes the successions out of n: through a fork when an
// ordinary node has several, guarded and weighted out of a decision.
func (a *activity) successions(n *sysmlv1.Element) {
	from := writeName(a.name(n, baseName(n)))
	outs := a.succ[n]
	if len(outs) > 1 && n.Type != "ForkNode" && n.Type != "DecisionNode" {
		f := a.fresh("fork")
		a.m.w.line(firstKw + from + thenKw + writeName(f) + ";")
		a.m.w.line("fork " + writeName(f) + ";")
		from = writeName(f)
		a.m.downgrade(n, "several edges leave the node, which a fork "+f+" carries")
	}
	if n.Type == "DecisionNode" {
		a.decisionSuccessions(n, from, outs)
		return
	}
	for _, e := range outs {
		to := a.endpointIn(ownerNode(a.m.model.Ref(e, "target")))
		if to == "" {
			a.unwritableEdge(e)
			continue
		}
		g := a.guard(e, "")
		a.m.w.lines(g.comment)
		a.m.w.line(firstKw + from + g.expr + thenKw + to + ";")
		if g.ok {
			a.m.add(e, Mapped, "", "")
		}
	}
}

// decisionSuccessions writes a decision's branches: guarded where the guard is a
// v2 expression, else for an else guard, weighted where one carries a «Probability».
func (a *activity) decisionSuccessions(n *sysmlv1.Element, from string, outs []*sysmlv1.Element) {
	if in := a.m.model.Ref(n, "decisionInput"); in != nil {
		a.m.downgrade(n, "the decision input behavior "+qualifiedName(in)+" is not run; the guards read the features they name")
	}
	input := a.decisionInput(n)
	tos := make([]string, len(outs))
	for i, e := range outs {
		tos[i] = a.endpointIn(ownerNode(a.m.model.Ref(e, "target")))
	}
	weights := a.probabilities(outs, tos)
	guards := make([]guardText, len(outs))
	unconditional, elseAt := 0, -1
	for i, e := range outs {
		if tos[i] == "" {
			continue
		}
		if weights == nil && a.isElse(e) {
			if elseAt < 0 {
				elseAt = i
			} else {
				a.m.add(e, Approximated, "", "a second else branch is written unguarded")
			}
			unconditional++
			continue
		}
		guards[i] = a.guard(e, input)
		if guards[i].expr == "" {
			unconditional++
		}
	}
	if weights == nil && unconditional > 1 {
		weights = a.arbitraryChoice(n, outs, tos, guards, elseAt)
		elseAt = -1
	}
	for i, e := range outs {
		to := tos[i]
		if to == "" {
			a.unwritableEdge(e)
			continue
		}
		if i == elseAt {
			continue
		}
		a.m.w.lines(guards[i].comment)
		line := firstKw + from + guards[i].expr + thenKw + to
		if weights != nil {
			line += " { @Stochastic::Probability { p = " + weights[i] + "; } }"
		} else {
			line += ";"
		}
		a.m.w.line(line)
		if guards[i].ok {
			a.m.add(e, Mapped, "", "")
		}
	}
	if elseAt >= 0 {
		a.m.w.line("else " + tos[elseAt] + ";")
		a.m.add(outs[elseAt], Mapped, "", "")
	}
}

// arbitraryChoice weights the unconditional branches of a decision equally, since
// UML leaves the branch taken among several true guards unspecified.
func (a *activity) arbitraryChoice(n *sysmlv1.Element, outs []*sysmlv1.Element, tos []string, guards []guardText, elseAt int) []string {
	written := 0
	for _, to := range tos {
		if to != "" {
			written++
		}
	}
	share := realLiteral(1 / float64(written))
	weights := make([]string, len(outs))
	var dropped []string
	for i, e := range outs {
		weights[i] = share
		if tos[i] != "" && guards[i].expr == "" && !guards[i].ok {
			dropped = append(dropped, describe(e))
		}
	}
	note := "several branches leave the decision unconditionally, so one is drawn at random with the model seed: each branch is weighted " + share
	if len(dropped) > 0 {
		note += "; the guards of " + strings.Join(dropped, ", ") + " were not migrated"
	}
	a.m.downgrade(n, note)
	if elseAt >= 0 {
		a.m.add(outs[elseAt], Approximated, "", "the else branch competes equally with the other unconditional branches, since the guards it stands beside were not all migrated")
	}
	return weights
}

// guardText is a guard as written on an edge: expr is ` if <expr>` or "", comment
// keeps a guard that was not migrated, and ok is false when the edge is approximated.
type guardText struct {
	expr    string
	comment []string
	ok      bool
}

// isElse reports whether an edge's guard is the else guard.
func (a *activity) isElse(e *sysmlv1.Element) bool {
	g := firstOwned(e, "guard")
	if g == nil {
		return false
	}
	switch g.Type {
	case "OpaqueExpression":
		body, _ := opaqueBody(g)
		return strings.TrimSpace(body) == "else"
	case "LiteralString":
		return strings.TrimSpace(g.Attrs["value"]) == "else"
	}
	return false
}

// decisionInput writes the value the object flow into a decision node
// carries, which its guards are compared with; "" when nothing names it.
func (a *activity) decisionInput(n *sysmlv1.Element) string {
	if a.selfFed[n] {
		return "this"
	}
	if srcs := a.sources[n]; len(srcs) == 1 {
		if ref, ok := a.pinRef(srcs[0]); ok {
			return ref
		}
	}
	return ""
}

// guard reads an edge's guard as ` if <expr>`, comparing a bare value with the
// decision's input; a guard that is no v2 expression is kept as a comment.
func (a *activity) guard(e *sysmlv1.Element, input string) guardText {
	g := firstOwned(e, "guard")
	if g == nil {
		return guardText{ok: true}
	}
	if g.Type == "LiteralBoolean" && (g.Attrs["value"] == "true" || g.Attrs["value"] == "") {
		return guardText{ok: true}
	}
	unguarded := func(note string) guardText {
		a.m.add(e, Approximated, "", "the guard ["+describeValue(g)+"] is kept as a comment and the edge written unguarded: "+note)
		return guardText{comment: commentLines("guard not migrated: [" + describeValue(g) + "] — " + note)}
	}
	expr, ok, note := a.m.behaviorValue(g, a.act)
	if !ok {
		return unguarded(note)
	}
	if note != "" {
		a.m.add(e, Approximated, "", note)
	}
	if guardIsValue(g) {
		if input == "" {
			return unguarded("the guard is a value, and no object flow into the decision names what to compare it with")
		}
		return guardText{expr: " if " + input + " == " + expr, ok: true}
	}
	return guardText{expr: " if " + expr, ok: true}
}

// guardIsValue reports whether a guard is a value the decision's input is
// compared with, rather than a condition of its own.
func guardIsValue(g *sysmlv1.Element) bool {
	switch g.Type {
	case "InstanceValue", "LiteralInteger", "LiteralReal", "LiteralString", "LiteralUnlimitedNatural":
		return true
	}
	return false
}

// probabilities weights each branch of a decision carrying a «Probability»: unmarked
// branches share the remainder to 1, marked sums other than 1 are scaled; nil when none.
func (a *activity) probabilities(outs []*sysmlv1.Element, tos []string) []string {
	values := make([]float64, len(outs))
	var unmarked []int
	sum, marked := 0.0, false
	var notes []string
	for i, e := range outs {
		if tos[i] == "" {
			notes = append(notes, "the edge "+describe(e)+" leads to "+describe(ownerNode(a.m.model.Ref(e, "target")))+", "+a.unwritableTarget(e))
		}
		s := e.Stereotype("Probability")
		if s == nil {
			unmarked = append(unmarked, i)
			continue
		}
		marked = true
		text := s.Tag("probability")
		if text == "" {
			notes = append(notes, "the «Probability» on "+describe(e)+" has no probability value")
			continue
		}
		v, ok := a.probability(text)
		if !ok {
			notes = append(notes, "the probability "+strconv.Quote(text)+" on "+describe(e)+" is neither a number nor a property with a numeric default")
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 || f > 1 {
			notes = append(notes, "the probability "+v+" on "+describe(e)+" is not between 0 and 1")
			continue
		}
		sum += f
		values[i] = f
	}
	if !marked {
		return nil
	}
	remainder := 1 - sum
	switch {
	case len(notes) > 0:
	case len(unmarked) > 0 && remainder < -probabilityTolerance:
		notes = append(notes, "the probabilities out of the decision sum to "+realLiteral(sum)+", leaving nothing for the "+strconv.Itoa(len(unmarked))+" branch(es) without one")
	case len(unmarked) == 0 && sum <= 0:
		notes = append(notes, "the probabilities out of the decision sum to 0")
	}
	if len(notes) > 0 {
		note := "no «Probability» is written on the decision's branches: " + strings.Join(notes, "; ")
		for _, e := range outs {
			if e.Stereotype("Probability") != nil {
				a.m.add(e, Approximated, "", note)
			}
		}
		return nil
	}
	switch {
	case len(unmarked) > 0:
		share := math.Max(remainder, 0) / float64(len(unmarked))
		for _, i := range unmarked {
			values[i] = share
			a.m.add(outs[i], Approximated, "", "the edge carries no «Probability»: it is weighted "+realLiteral(share)+", its share of what the marked branches leave of 1")
		}
	case math.Abs(remainder) > probabilityTolerance:
		for i, e := range outs {
			values[i] /= sum
			a.m.add(e, Approximated, "", "the probabilities out of the decision sum to "+realLiteral(sum)+", not 1: each is scaled by the sum")
		}
	}
	weights := make([]string, len(outs))
	for i, v := range values {
		weights[i] = realLiteral(v)
	}
	return weights
}

// probabilityTolerance is how far probabilities out of one decision may sum
// from 1 and still be written as they are.
const probabilityTolerance = 1e-6

// probability reads a probability tag: a number, or the name of a property
// visible from the activity whose default value is a number.
func (a *activity) probability(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if v, ok := finiteNumber(text); ok {
		return v, true
	}
	if t := a.m.model.Lookup(text); t != nil {
		text = t.Name
	}
	visible, _ := a.m.visibleFrom(a.act)
	p := visible[text]
	if p == nil || p.Type != "Property" {
		return "", false
	}
	dv := firstOwned(p, "defaultValue")
	if dv == nil {
		return "", false
	}
	switch dv.Type {
	case "LiteralReal", "LiteralInteger":
		return finiteNumber(dv.Attrs["value"])
	}
	return "", false
}

// finiteNumber reads text as a finite number written as a v2 real literal.
func finiteNumber(text string) (string, bool) {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return "", false
	}
	return realLiteral(f), true
}

// declare writes a node's declaration.
func (a *activity) declare(n *sysmlv1.Element) {
	name := writeName(a.name(n, baseName(n)))
	into := name
	if w, ok := a.waits[n]; ok {
		a.m.w.line(actionKw + w.name + " accept after " + w.delay + ";")
		a.m.w.line(firstKw + w.name + thenKw + name + ";")
		into = w.name
	}
	if j, ok := a.joins[n]; ok {
		a.m.w.line("join " + j + ";")
		a.m.w.line(firstKw + j + thenKw + into + ";")
		a.m.add(n, Approximated, "", "several edges lead to the node, which waits for all of them through the join "+j)
	}
	if m, ok := a.merges[n]; ok {
		a.m.w.line("merge " + m + ";")
		a.m.w.line(firstKw + m + thenKw + into + ";")
	}
	switch n.Type {
	case "ActivityFinalNode":
		a.m.w.line(actionKw + name + " terminate;")
		a.m.add(n, Mapped, name, "")
	case "ForkNode":
		a.m.w.line("fork " + name + ";")
		a.m.add(n, Mapped, name, "")
	case "JoinNode":
		a.m.w.line("join " + name + ";")
		a.m.add(n, Mapped, name, "")
	case "DecisionNode":
		a.m.w.line("decide " + name + ";")
		a.m.add(n, Mapped, name, "")
	case "MergeNode":
		a.m.w.line("merge " + name + ";")
		a.m.add(n, Mapped, name, "")
	case "CentralBufferNode", "DataStoreNode", "ExpansionNode":
		a.m.w.line(actionKw + name + ";")
		a.m.add(n, Approximated, name, "an object node holds no tokens in v2: the flows through it are written from its sources to its targets")
	case "CallBehaviorAction":
		a.callBehavior(n, name)
	case "CallOperationAction":
		a.callOperation(n, name)
	case "OpaqueAction":
		a.opaqueAction(n, name)
	case "ValueSpecificationAction":
		a.valueAction(n, name)
	case "ReadSelfAction":
		a.m.w.line(actionKw + name + ";")
		a.m.add(n, Approximated, name, "the object read is this, which the actions its result flows into name directly")
	case "ReadStructuralFeatureAction":
		a.readFeature(n, name)
	case "AddStructuralFeatureValueAction":
		a.writeFeature(n, name)
	case "SendSignalAction":
		a.sendSignal(n, name)
	case "AcceptEventAction":
		a.acceptEvent(n, name)
	case "StructuredActivityNode", "ExpansionRegion", "LoopNode", "ConditionalNode", "SequenceNode":
		a.structured(n, name)
	default:
		a.placeholder(n, name, noV2UML+n.Type, Unmapped)
	}
	a.m.writeComments(n, false)
}

// placeholder writes an action usage that keeps the node's place in the
// graph while its behavior is kept as a comment.
func (a *activity) placeholder(n *sysmlv1.Element, name, note string, v Verdict) {
	a.inert[n] = true
	a.m.w.block(actionKw+name, func() {
		a.pins(n, false, nil)
		a.m.w.lines(commentLines("not migrated: " + kindOf(n) + " " + describe(n) + " — " + note))
	})
	a.m.add(n, v, name, note)
}

// inputPins lists the input pins of an action, arguments first.
func inputPins(n *sysmlv1.Element) []*sysmlv1.Element {
	ins := append(n.Owned("argument"), n.Owned("inputValue")...)
	ins = append(ins, n.Owned("object")...)
	ins = append(ins, n.Owned("value")...)
	ins = append(ins, n.Owned("target")...)
	return append(ins, n.Owned("insertAt")...)
}

// pins declares an untyped action usage's pins as its parameters and records how a
// flow refers to each; a typed usage's pins stand for params, its definition's, in order.
func (a *activity) pins(n *sysmlv1.Element, typed bool, params []*sysmlv1.Element) {
	a.declarePins(n, inputPins(n), append(n.Owned("result"), n.Owned("outputValue")...), typed, params)
}

// declarePins declares the given input and output pins of n; see pins.
func (a *activity) declarePins(n *sysmlv1.Element, ins, outs []*sysmlv1.Element, typed bool, params []*sysmlv1.Element) {
	var inParams, outParams []*sysmlv1.Element
	for _, p := range params {
		switch p.Attrs["direction"] {
		case "out", "return":
			outParams = append(outParams, p)
		case "inout":
			inParams = append(inParams, p)
			outParams = append(outParams, p)
		default:
			inParams = append(inParams, p)
		}
	}
	used := map[string]bool{}
	name := a.m.nameOf(n)
	declare := func(pin *sysmlv1.Element, dir string, byPos []*sysmlv1.Element, i int) {
		if typed {
			if i < len(byPos) {
				pname := a.m.nameFor(byPos[i])
				a.names[pin] = pname
				a.m.add(pin, Mapped, a.m.v2Name(n)+"."+pname, "the pin stands for the parameter "+pname+" of the definition, which the flows name")
				return
			}
			a.m.add(pin, Unmapped, "", "the definition has no "+dir+" parameter for the pin; a flow into it has nowhere to go")
			return
		}
		pname := a.m.nameOf(pin)
		if pname == "" || used[pname] || pname == name {
			base := dir
			if pname != "" {
				base = pname
			}
			pname = base
			for j := 2; used[pname]; j++ {
				pname = base + strconv.Itoa(j)
			}
		}
		used[pname] = true
		a.names[pin] = pname
		typ, note := a.m.typeRef(a.pinClassifier(pin), a.def)
		decl := dir + " " + writeName(pname)
		if typ != "" {
			decl += " : " + typ
		}
		mult, mnote := a.m.multiplicity(pin)
		note = joinNotes(note, mnote)
		decl += mult
		if v := firstOwned(pin, "value"); v != nil && pin.Type == "ValuePin" {
			expr, ok, vnote := a.m.typedBehaviorValue(v, pin, n)
			if ok {
				decl += " = " + expr
				note = joinNotes(note, vnote)
			} else {
				note = joinNotes(note, "the value pin's value "+describeValue(v)+" is not written: "+vnote)
			}
		}
		a.m.w.line(decl + ";")
		a.m.add(pin, verdictFor(note), a.m.v2Name(n)+"."+pname, note)
	}
	for i, pin := range ins {
		declare(pin, "in", inParams, i)
	}
	for i, pin := range outs {
		declare(pin, "out", outParams, i)
	}
}

// pinClassifier is the classifier a pin is typed by: its own type, else the
// one settled for it.
func (a *activity) pinClassifier(pin *sysmlv1.Element) *sysmlv1.Element {
	if t := a.m.model.Ref(pin, "type"); t != nil {
		return t
	}
	return a.pinType[pin]
}

// endType is the classifier a flow end is typed by: the parameter's type for
// a parameter node, else the pin's.
func (a *activity) endType(p *sysmlv1.Element) *sysmlv1.Element {
	if nodeKind(p) == nodeParam {
		return a.m.model.Ref(a.m.model.Ref(p, "parameter"), "type")
	}
	if sig := a.payload[p]; sig != nil {
		return sig
	}
	return a.pinClassifier(p)
}

// pinRef writes how a flow names a pin or parameter node.
func (a *activity) pinRef(p *sysmlv1.Element) (string, bool) {
	if nodeKind(p) == nodeParam {
		param := a.m.model.Ref(p, "parameter")
		if param == nil {
			return "", false
		}
		return writeName(a.m.nameFor(param)), true
	}
	pname, ok := a.names[p]
	if !ok || p.Parent == nil {
		return "", false
	}
	owner, ok := a.names[p.Parent]
	if !ok {
		return "", false
	}
	return writeName(owner) + "." + writeName(pname), true
}

// parameterNode reports an activity parameter node, which the flows name by
// its parameter.
func (a *activity) parameterNode(n *sysmlv1.Element) {
	param := a.m.model.Ref(n, "parameter")
	if param == nil {
		a.m.unmapped(n, "the parameter node names no parameter of the activity")
		return
	}
	a.m.add(n, Mapped, a.m.v2Name(param), "the flows to and from the node name the parameter "+a.m.nameFor(param))
}

// objectFlow writes the data an object flow carries from the pins producing it to
// the pin or parameter it reaches: a flow between pins, a binding at a parameter.
func (a *activity) objectFlow(e *sysmlv1.Element) {
	src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
	if src == nil || tgt == nil {
		a.m.unmapped(e, joinNotes(a.m.dangling(e, "source", "target"), "the flow lacks an end"))
		return
	}
	if k := nodeKind(tgt); k != nodePin && k != nodeParam {
		if tgt.Type == "DecisionNode" {
			a.m.add(e, Mapped, "", "the value the flow carries is what the decision's guards are compared with")
			return
		}
		if nodeKind(tgt) == nodeControl || nodeKind(tgt) == nodeBuffer {
			a.m.add(e, Approximated, "", "the flow into "+describe(tgt)+" is written from its sources to the pins the node leads to")
			return
		}
		a.m.add(e, Approximated, "", "an object flow into "+describe(tgt)+" is written as a succession")
		return
	}
	if len(a.edgeSources[e]) == 0 && !a.edgeSelf[e] {
		a.m.add(e, Unmapped, "", "nothing the flow carries comes from a pin or parameter")
		return
	}
	if a.edgeSelf[e] {
		a.m.add(e, Approximated, "", "the flow carries this, which the action names directly")
	}
	if a.dataOnly[e] {
		a.m.add(e, Approximated, "", "the flow carries its value only: the control flow into "+describe(tgt.Parent)+" starts the action, so the action does not wait for the value on each pass")
	}
	to, ok := a.pinRef(tgt)
	if !ok {
		a.m.add(e, Unmapped, "", "the flow's target "+describe(tgt)+" has no v2 name")
		return
	}
	for _, s := range a.edgeSources[e] {
		from, ok := a.pinRef(s)
		if !ok {
			a.m.add(e, Unmapped, "", "the flow's source "+describe(s)+" has no v2 name")
			continue
		}
		if a.written[[2]*sysmlv1.Element{s, tgt}] {
			a.m.add(e, Mapped, "", "the flow from "+from+" to "+to+" is written once, though several edges carry it")
			continue
		}
		a.written[[2]*sysmlv1.Element{s, tgt}] = true
		if a.inert[s.Parent] {
			a.m.w.line("/* flow " + from + " to " + to + " not written: " + describe(s.Parent) + " is not migrated and produces no value */")
			a.m.add(e, Approximated, "", "the flow is kept as a comment: its source "+describe(s.Parent)+" is not migrated, so no value reaches "+describe(s))
			if nodeKind(tgt) == nodePin {
				a.m.add(tgt.Parent, Approximated, "", "its input "+to+" receives no value, since "+describe(s.Parent)+" is not migrated; the action cannot be performed until one is bound")
			}
			continue
		}
		st, tt := a.endType(s), a.endType(tgt)
		if !a.m.conform(st, tt) {
			a.m.w.line("/* flow " + from + " to " + to + " not written: " + qualifiedName(st) + " and " + qualifiedName(tt) + " do not conform */")
			a.m.add(e, Approximated, "", "the flow is kept as a comment: its ends are typed by "+qualifiedName(st)+" and "+qualifiedName(tt)+", which do not conform")
			continue
		}
		if nodeKind(s) == nodeParam || nodeKind(tgt) == nodeParam {
			a.m.w.line("bind " + to + " = " + from + ";")
		} else {
			a.m.w.line("flow " + from + " to " + to + ";")
		}
		a.m.add(e, Mapped, "", "")
	}
	if g := firstOwned(e, "guard"); g != nil {
		a.m.add(e, Approximated, "", "the guard ["+describeValue(g)+"] on an object flow is not written")
	}
}

// callBehavior writes a call behavior action as an action usage typed by the called
// behavior's definition or, when that is a calc def, an action evaluating it over its pins.
func (a *activity) callBehavior(n *sysmlv1.Element, name string) {
	b := a.m.model.Ref(n, "behavior")
	if b == nil {
		a.placeholder(n, name, joinNotes(a.m.dangling(n, "behavior"), "the action calls no behavior"), Unmapped)
		return
	}
	if op := a.m.methodOf[b]; op != nil {
		b = op
	}
	if !a.m.written(b) {
		a.placeholder(n, name, "the behavior "+qualifiedName(b)+" it calls has no v2 declaration", Unmapped)
		return
	}
	cat, _ := a.m.classify(b)
	switch cat {
	case catActionDef:
		a.m.w.line(actionKw + name + " : " + a.m.ref(b, a.def) + ";")
		a.pins(n, true, b.Owned("ownedParameter"))
		note := ""
		if owner, here := classifierOf(b), classifierOf(a.act); owner != nil && owner != here && (here == nil || !a.m.inherits(here, owner)) {
			note = "the behavior belongs to " + qualifiedName(owner) + " and runs here in the caller's context"
		}
		a.m.add(n, verdictFor(note), name, note)
	case catCalcDef:
		a.m.w.block(actionKw+name, func() {
			a.pins(n, false, nil)
			var args []string
			for _, pin := range n.Owned("argument") {
				args = append(args, writeName(a.names[pin]))
			}
			results := n.Owned("result")
			call := a.m.ref(b, a.def) + "(" + strings.Join(args, ", ") + ")"
			if len(results) == 0 {
				a.m.w.lines(commentLines("evaluates " + call + ", whose result no pin takes"))
				return
			}
			for _, r := range results[1:] {
				a.m.add(r, Unmapped, "", "a calc has one result; the pin takes nothing")
			}
			a.m.w.line("out " + writeName(a.names[results[0]]) + " = " + call + ";")
		})
		a.m.add(n, Approximated, name, "the calc "+qualifiedName(b)+" is evaluated when the action runs; a calc is no action node")
	default:
		a.placeholder(n, name, "the behavior "+qualifiedName(b)+" is written as a "+cat.keyword()+", which an action cannot call", Unmapped)
	}
}

// classifierOf returns the classifier a behavior belongs to: the nearest
// enclosing element that is neither a behavior nor an operation, nil for a package.
func classifierOf(b *sysmlv1.Element) *sysmlv1.Element {
	for cur := b.Parent; cur != nil; cur = cur.Parent {
		if isBehavior(cur) || cur.Type == "Operation" {
			continue
		}
		if cur.Type == "Package" || cur.Type == "Model" || cur.Type == "Profile" {
			return nil
		}
		return cur
	}
	return nil
}

// callOperation writes a call on the object its target pin holds as `perform
// action x ::> obj.op`; any other target leaves the call an action typed by the operation.
func (a *activity) callOperation(n *sysmlv1.Element, name string) {
	op := a.m.model.Ref(n, "operation")
	if op == nil {
		a.placeholder(n, name, joinNotes(a.m.dangling(n, "operation"), "the action calls no operation"), Unmapped)
		return
	}
	if !a.m.written(op) {
		a.placeholder(n, name, "the operation "+qualifiedName(op)+" it calls has no v2 declaration", Unmapped)
		return
	}
	t := firstOwned(n, "target")
	ins := slices.DeleteFunc(inputPins(n), func(p *sysmlv1.Element) bool { return p == t })
	outs := append(n.Owned("result"), n.Owned("outputValue")...)
	receiver, note, ok := a.receiverOf(t, op)
	switch {
	case ok:
		a.m.w.line("perform action " + name + " ::> " + receiver + ";")
		a.m.add(t, Mapped, a.m.v2Name(n), note)
		note = ""
	case t != nil:
		a.m.w.line(actionKw + name + " : " + a.m.ref(op, a.def) + ";")
		a.m.add(t, Approximated, "", "the target pin is not written; the call runs in the caller's context")
	default:
		a.m.w.line(actionKw + name + " : " + a.m.ref(op, a.def) + ";")
	}
	a.declarePins(n, ins, outs, true, op.Owned("ownedParameter"))
	a.m.add(n, verdictFor(note), name, note)
}

// receiverOf writes the operation usage a call performs on its target pin's object:
// `op` for this, `f.g.op` for a feature of this; the note says why it is not written so.
func (a *activity) receiverOf(t, op *sysmlv1.Element) (receiver, note string, ok bool) {
	if t == nil {
		return "", "", false
	}
	obj, typ, ok := a.objectOf(t)
	switch {
	case !ok && len(a.sources[t]) == 0:
		return "", "the call runs in the caller's context: no flow feeds its target pin", false
	case !ok:
		return "", "the call runs in the caller's context: the target the flow into its pin names is not an object read from this", false
	case typ == nil:
		return "", "the call runs in the caller's context: the target " + obj + " has no known type to hold the operation", false
	case !a.m.hasFeature(typ, op):
		return "", "the call runs in the caller's context: the target " + obj + " is a " + qualifiedName(typ) + ", which has no operation " + a.m.nameOf(op), false
	}
	usage := writeName(a.m.operationUsage(op))
	if obj == "this" {
		return usage, "the target is this, whose usage " + usage + " the call performs", true
	}
	path := strings.TrimPrefix(obj, "this.")
	return path + "." + usage, "the call performs the usage " + usage + " of the target " + obj, true
}

// opaqueAction writes an opaque action as an action of assignments when its body is
// a sequence of them, else with the body kept as a comment; its pins stay unwritten.
func (a *activity) opaqueAction(n *sysmlv1.Element, name string) {
	body, lang := opaqueBody(n)
	a.inert[n] = true
	a.m.w.block(actionKw+name, func() {
		a.pins(n, false, nil)
		lines, ok, note := a.m.statements(body, lang, n)
		if !ok {
			a.m.opaqueComment(body, lang, note)
			a.m.add(n, Approximated, name, "the body is kept as a comment: "+note)
			return
		}
		a.m.w.lines(lines)
		a.m.add(n, Approximated, name, "the "+langName(lang)+" body is written as v2 assignments")
	})
}

// valueAction writes a value specification action as an action whose result
// is the value.
func (a *activity) valueAction(n *sysmlv1.Element, name string) {
	v := firstOwned(n, "value")
	results := n.Owned("result")
	if v == nil {
		a.placeholder(n, name, "the action has no value", Unmapped)
		return
	}
	var expr, note string
	var ok bool
	if len(results) == 0 {
		expr, ok, note = a.m.behaviorValue(v, n)
	} else {
		expr, ok, note = a.m.typedBehaviorValue(v, results[0], n)
	}
	if !ok {
		a.placeholder(n, name, "the value "+describeValue(v)+" is not written: "+note, Approximated)
		return
	}
	a.m.w.block(actionKw+name, func() {
		for _, r := range results[1:] {
			a.m.add(r, Unmapped, "", "the action has one value; the pin takes nothing")
		}
		if len(results) == 0 {
			a.m.w.lines(commentLines("the value " + expr + " flows nowhere"))
			return
		}
		r := results[0]
		pname := a.m.nameOf(r)
		if pname == "" {
			pname = "result"
		}
		a.names[r] = pname
		typ, tnote := a.m.typeRef(a.m.model.Ref(r, "type"), a.def)
		decl := "out " + writeName(pname)
		if typ != "" {
			decl += " : " + typ
		}
		a.m.w.line(decl + " = " + expr + ";")
		a.m.add(r, verdictFor(tnote), a.m.v2Name(n)+"."+pname, tnote)
	})
	a.m.add(n, verdictFor(note), name, note)
}

// objectOf writes the object a pin holds when it is read from this (`this`, `this.f`
// and so on down); typ is that object's classifier, nil when it is not known.
func (a *activity) objectOf(pin *sysmlv1.Element) (expr string, typ *sysmlv1.Element, ok bool) {
	if a.selfFed[pin] {
		return "this", classifierOf(a.act), true
	}
	srcs := a.sources[pin]
	if len(srcs) != 1 || srcs[0].Parent == nil || srcs[0].Parent.Type != "ReadStructuralFeatureAction" {
		return "", nil, false
	}
	read := srcs[0].Parent
	f := a.m.model.Ref(read, "structuralFeature")
	if f == nil || !a.m.written(f) {
		return "", nil, false
	}
	base, t, ok := a.readObject(firstOwned(read, "object"))
	if !ok || !a.m.mayHaveFeature(t, f) {
		return "", nil, false
	}
	return base + "." + writeName(a.m.nameOf(f)), a.m.model.Ref(f, "type"), true
}

// readObject is the object a structural feature action reads or writes: what
// its object pin holds, or this when the pin is absent or nothing feeds it.
func (a *activity) readObject(obj *sysmlv1.Element) (expr string, typ *sysmlv1.Element, ok bool) {
	if obj == nil || len(a.sources[obj]) == 0 && !a.selfFed[obj] {
		return "this", classifierOf(a.act), true
	}
	return a.objectOf(obj)
}

// mayHaveFeature reports whether classifier t has feature f; an unknown
// classifier is not contradicted.
func (m *migration) mayHaveFeature(t, f *sysmlv1.Element) bool {
	return t == nil || f.Parent == nil || m.hasFeature(t, f)
}

// featureOn writes the feature f read on the object a pin holds: `this.f` for this,
// `<pin>.f` when another flow feeds the pin; the reason when the object lacks f.
func (a *activity) featureOn(objPin, f *sysmlv1.Element) (string, string) {
	if obj, t, ok := a.readObject(objPin); ok {
		if !a.m.mayHaveFeature(t, f) {
			return "", "the object read, " + obj + ", is a " + qualifiedName(t) + noFeature + a.m.nameOf(f)
		}
		return obj + "." + writeName(a.m.nameOf(f)), ""
	}
	if objPin == nil || len(a.sources[objPin]) == 0 {
		return "", ""
	}
	pname, ok := a.names[objPin]
	if !ok {
		return "", ""
	}
	if t := a.pinClassifier(objPin); !a.m.mayHaveFeature(t, f) {
		return "", "the object pin is a " + qualifiedName(t) + noFeature + a.m.nameOf(f)
	}
	return writeName(pname) + "." + writeName(a.m.nameOf(f)), ""
}

// readFeature writes a read of a structural feature as an action whose
// result is the feature of the object read.
func (a *activity) readFeature(n *sysmlv1.Element, name string) {
	f := a.m.model.Ref(n, "structuralFeature")
	if f == nil || !a.m.written(f) || a.m.nameOf(f) == "" {
		a.placeholder(n, name, "the feature read has no v2 declaration", Unmapped)
		return
	}
	obj := firstOwned(n, "object")
	results := n.Owned("result")
	a.m.w.block(actionKw+name, func() {
		if obj != nil && !a.selfFed[obj] {
			if _, _, ok := a.objectOf(obj); !ok {
				// UML types the object pin by the classifier that owns the feature.
				a.pinType[obj] = f.Parent
				a.declarePins(n, inputPins(n), nil, false, nil)
			}
		}
		expr, why := a.featureOn(obj, f)
		if expr == "" {
			if why == "" {
				why = "the object whose " + a.m.nameOf(f) + " is read has no v2 name"
			}
			a.inert[n] = true
			a.m.w.lines(commentLines("not migrated: " + why))
			a.m.add(n, Unmapped, name, why)
			return
		}
		if len(results) == 0 {
			a.m.w.lines(commentLines("reads " + expr + ", which flows nowhere"))
			return
		}
		r := results[0]
		pname := a.m.nameOf(r)
		if pname == "" {
			pname = "result"
		}
		a.names[r] = pname
		a.m.w.line("out " + writeName(pname) + " = " + expr + ";")
		a.m.add(r, Mapped, a.m.v2Name(n)+"."+pname, "")
		a.m.add(n, Mapped, name, "")
	})
}

// writeFeature writes an add structural feature value action as an
// assignment to the feature of this.
func (a *activity) writeFeature(n *sysmlv1.Element, name string) {
	f := a.m.model.Ref(n, "structuralFeature")
	if f == nil || !a.m.written(f) || a.m.nameOf(f) == "" {
		a.placeholder(n, name, "the feature written has no v2 declaration", Unmapped)
		return
	}
	obj, val := firstOwned(n, "object"), firstOwned(n, "value")
	target, t, ok := a.readObject(obj)
	if !ok {
		a.placeholder(n, name, "the object whose "+a.m.nameOf(f)+" is written comes from a flow, not from this", Approximated)
		return
	}
	if !a.m.mayHaveFeature(t, f) {
		a.placeholder(n, name, "the object written, "+target+", is a "+qualifiedName(t)+noFeature+a.m.nameOf(f), Unmapped)
		return
	}
	if val == nil {
		a.placeholder(n, name, "the action has no value pin", Unmapped)
		return
	}
	a.m.w.block(actionKw+name, func() {
		a.pins(n, false, nil)
		a.m.w.line("assign " + target + "." + writeName(a.m.nameOf(f)) + " := " + writeName(a.names[val]) + ";")
	})
	note := ""
	if mult, _ := a.m.multiplicity(f); mult != "" && n.Attrs["isReplaceAll"] != "true" {
		note = "the value replaces the feature's; adding to a collection is not written"
	}
	a.m.add(n, verdictFor(note), name, note)
}

// sendSignal writes a send signal action as an action sending a new instance of the
// signal, over the port named or to the object the target pin holds.
func (a *activity) sendSignal(n *sysmlv1.Element, name string) {
	sig := a.m.model.Ref(n, "signal")
	if sig == nil || !a.m.written(sig) {
		a.placeholder(n, name, "the signal sent has no v2 declaration", Unmapped)
		return
	}
	var note string
	a.m.w.block(actionKw+name, func() {
		a.pins(n, false, nil)
		args, anote := a.signalArguments(n, sig)
		note = anote
		line := "send new " + a.m.ref(sig, a.def) + "(" + strings.Join(args, ", ") + ")"
		if port := a.m.model.Ref(n, "onPort"); port != nil {
			if a.m.written(port) && a.m.ownedByClassifier(port, a.act) {
				line += " via this." + writeName(a.m.nameOf(port))
			} else {
				note = "the port " + qualifiedName(port) + " is no port of the sender's type; the signal is sent to the sender"
			}
		} else if t := firstOwned(n, "target"); t != nil {
			obj, _, ok := a.objectOf(t)
			switch {
			case ok && obj == "this":
			case ok:
				line += " to " + obj
			case len(a.sources[t]) > 0:
				line += " to " + writeName(a.names[t])
			default:
				note = joinNotes(note, "the target pin holds nothing a flow names; the signal is sent to the sender")
			}
		}
		a.m.w.line(line + ";")
	})
	a.m.add(n, verdictFor(note), name, note)
}

// signalArguments writes a send's arguments, one per argument pin standing for an
// attribute of the signal it can bind; the rest are left out and the note says why.
func (a *activity) signalArguments(n, sig *sysmlv1.Element) ([]string, string) {
	attrs := a.m.signalAttributes(sig)
	var args []string
	var notes []string
	for i, pin := range n.Owned("argument") {
		if i >= len(attrs) {
			notes = append(notes, "the signal has no attribute for the argument pin "+a.names[pin]+", which is not sent")
			continue
		}
		pt, at := a.m.model.Ref(pin, "type"), a.m.model.Ref(attrs[i], "type")
		if pt != nil && at != nil && pt != at && !a.m.inherits(pt, at) && a.m.written(at) {
			notes = append(notes, "the argument pin "+a.names[pin]+" is a "+qualifiedName(pt)+", which the signal's "+a.m.nameOf(attrs[i])+" : "+qualifiedName(at)+" cannot take; it is not sent")
			continue
		}
		args = append(args, writeName(a.names[pin]))
	}
	return args, strings.Join(notes, "; ")
}

// acceptEvent writes an accept event action: accepting the signal, the delay
// or the condition its trigger's event names.
func (a *activity) acceptEvent(n *sysmlv1.Element, name string) {
	triggers := n.Owned("trigger")
	if len(triggers) == 0 {
		a.placeholder(n, name, "the action has no trigger", Unmapped)
		return
	}
	for _, t := range triggers[1:] {
		a.m.add(t, Unmapped, "", "an accept takes one trigger; only the first is written")
	}
	accept, note, ok := a.trigger(triggers[0], n)
	if !ok {
		a.placeholder(n, name, note, Approximated)
		return
	}
	if len(triggers) > 1 {
		note = joinNotes(note, "the action has "+strconv.Itoa(len(triggers))+" triggers; only the first is written")
	}
	a.m.w.line(actionKw + name + " " + accept + ";")
	a.m.add(n, verdictFor(note), name, note)
}

// trigger writes the accept clause a trigger stands for; result, when set,
// is the action holding the payload.
func (a *activity) trigger(t, n *sysmlv1.Element) (clause, note string, ok bool) {
	ev := a.m.model.Ref(t, "event")
	if ev == nil {
		return "", joinNotes(a.m.dangling(t, "event"), "the trigger names no event"), false
	}
	clause, note, ok = a.m.triggerClause(ev, a.act, a.resultName(n))
	if ok {
		a.m.add(t, Mapped, "", "")
		if sig := a.m.model.Ref(ev, "signal"); ev.Type == "SignalEvent" && sig != nil && len(n.Owned("result")) > 0 {
			a.payload[n.Owned("result")[0]] = sig
		}
	}
	return clause, note, ok
}

// resultName names the payload an accept binds: the action's result pin, "" for none.
func (a *activity) resultName(n *sysmlv1.Element) string {
	results := n.Owned("result")
	if len(results) == 0 {
		return ""
	}
	r := results[0]
	pname := a.m.nameOf(r)
	if pname == "" {
		pname = "result"
	}
	a.names[r] = pname
	a.m.add(r, Mapped, a.m.v2Name(n)+"."+pname, "")
	return pname
}

// triggerClause writes the accept clause a trigger's event stands for and
// reports the event, which is written wherever a trigger refers to it.
func (m *migration) triggerClause(ev, scope *sysmlv1.Element, payload string) (clause, note string, ok bool) {
	clause, note, ok = m.acceptClause(ev, scope, payload)
	if !ok {
		m.add(ev, Unmapped, "", note)
		return clause, note, ok
	}
	m.add(ev, verdictFor(note), "", joinNotes("written where a trigger refers to it, as "+clause, note))
	return clause, note, ok
}

// acceptClause writes the accept clause an event stands for, read in scope: `accept
// p : Sig` (signal), `accept after d` (relative time), `accept when c` (change).
func (m *migration) acceptClause(ev, scope *sysmlv1.Element, payload string) (clause, note string, ok bool) {
	switch ev.Type {
	case "SignalEvent":
		sig := m.model.Ref(ev, "signal")
		if sig == nil || !m.written(sig) {
			return "", "the signal event names no migrated signal", false
		}
		if payload != "" {
			return "accept " + writeName(payload) + " : " + m.ref(sig, scope), "", true
		}
		return "accept " + m.ref(sig, scope), "", true
	case "TimeEvent":
		d, ok, note := m.durationExpr(firstOwned(ev, "when"), scope)
		if !ok {
			return "", "the time event's time is not written: " + note, false
		}
		if ev.Attrs["isRelative"] != "true" {
			return "", "an absolute time event needs a TimeInstantValue, which no literal writes", false
		}
		return "accept after " + d + " [SI::s]", note, true
	case "ChangeEvent":
		expr, ok, note := m.behaviorValue(firstOwned(ev, "changeExpression"), scope)
		if !ok {
			return "", "the change event's condition is not written: " + note, false
		}
		return "accept when " + expr, note, true
	case "AnyReceiveEvent":
		return "", "an any-receive event names no signal to accept", false
	}
	return "", "a " + ev.Type + " has no v2 trigger form", false
}

// structured writes a structured node as a nested action holding its own
// graph.
func (a *activity) structured(n *sysmlv1.Element, name string) {
	note := ""
	switch n.Type {
	case "ExpansionRegion":
		note = "the region's body is written once; its expansion over the collection is not"
	case "LoopNode":
		note = "the loop's body is written once; its test and iteration are not"
	case "ConditionalNode":
		note = "the node's clauses are written as one graph; their tests are not"
	}
	a.m.w.block(actionKw+name, func() {
		a.pins(n, false, nil)
		for _, v := range n.Owned("variable") {
			a.m.add(v, Unmapped, "", "a structured node's variable is not written")
		}
		inner := a.m.newActivity(n, n)
		inner.write()
	})
	if note != "" {
		a.m.add(n, Approximated, name, note)
		return
	}
	a.m.add(n, Mapped, name, "")
}

// partitions writes the activity's partitions as comments naming the nodes
// each holds, since v2 has no partition.
func (a *activity) partitions() {
	for _, g := range a.act.Owned("group") {
		if g.Type != "ActivityPartition" {
			if g.Type == "StructuredActivityNode" || g.Type == "ExpansionRegion" {
				// Also listed as a node, where it is written.
				continue
			}
			a.m.unmapped(g, noV2UML+g.Type)
			continue
		}
		var held []string
		for _, n := range a.m.model.Refs(g, "node") {
			if s, ok := a.names[n]; ok {
				held = append(held, s)
			}
		}
		text := "partition " + describe(g)
		if r := a.m.model.Ref(g, "represents"); r != nil {
			text += " represents " + qualifiedName(r)
		}
		if len(held) > 0 {
			text += ": " + strings.Join(held, ", ")
		}
		a.m.w.lines(commentLines(text))
		a.m.add(g, Approximated, "", "a partition names who performs its nodes, which v2 has no form for; it is kept as a comment")
	}
}

// rules writes the activity's constraints the nodes did not consume.
func (a *activity) rules() {
	for _, r := range a.act.Owned("ownedRule") {
		if a.m.reported(r) {
			continue
		}
		switch r.Type {
		case "DurationConstraint":
			var on []string
			for _, c := range a.m.model.Refs(r, "constrainedElement") {
				on = append(on, describe(c))
			}
			a.m.unmapped(r, "the duration constraint constrains "+strings.Join(on, ", ")+", which is no node of the activity a wait can precede")
		case "TimeConstraint":
			a.m.unmapped(r, "a time constraint bounds an instant, which no v2 wait writes")
		default:
			a.m.rule(r)
		}
	}
}

// observations reports the activity's observations: what a run measures.
func (a *activity) observations() {
	for _, o := range a.act.Owned("observation") {
		a.m.add(o, Unmapped, "", "an observation records what a run measures; the runtime reports a run's clock instead")
	}
}
