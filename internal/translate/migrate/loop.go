package migrate

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// The parts of a LoopNode — setup, test and body — each written as its own
// action block by a graph writer restricted to the part's nodes and edges.
const (
	loopSetup = iota
	loopTest
	loopBody
)

// loopFeed is a pin of a part the loop feeds a value to as its default: the
// loop variable src is, or a pin of the earlier part srcPart.
type loopFeed struct {
	src     *sysmlv1.Element
	srcPart int // -1 when src is a loop variable
}

// loopShape is how a LoopNode's parts, pins and decider write as a v2 loop.
type loopShape struct {
	testedFirst bool
	parts       [3][]*sysmlv1.Element // setup, test and body nodes
	partOf      map[*sysmlv1.Element]int
	edges       [3][]*sysmlv1.Element // intra-part edges
	routers     []*sysmlv1.Element    // loop-level control nodes routing loop data
	loopVars    []*sysmlv1.Element
	lvIns       []*sysmlv1.Element
	results     []*sysmlv1.Element
	bodyOuts    []*sysmlv1.Element
	decider     *sysmlv1.Element
	deciderNode *sysmlv1.Element
	feed        [3]map[*sysmlv1.Element]loopFeed
	lvSrc       map[*sysmlv1.Element]*sysmlv1.Element // loop variable -> setup pin initializing it
	// routing state: the loop variable each routing node passes on, the nodes
	// one reaches, and the edges a feed was already reported for.
	routedSrc map[*sysmlv1.Element]*sysmlv1.Element
	routed    map[*sysmlv1.Element]bool
	feedDone  map[*sysmlv1.Element]bool
}

// partOrder is the position a part's block performs in: setup first, then test
// before body when the loop tests first, after it when it does not.
func (s *loopShape) partOrder(part int) int {
	if s.testedFirst {
		return part
	}
	switch part {
	case loopBody:
		return 1
	case loopTest:
		return 2
	}
	return 0
}

// earlier reports whether part a performs before part b in one pass.
func (s *loopShape) earlier(a, b int) bool {
	return s.partOrder(a) < s.partOrder(b)
}

// loopShape checks that a LoopNode can write as a v2 loop: a decider in the
// test part, every executable node in exactly one part (or a control node
// routing loop data), matching pin counts and body outputs of the body part;
// why names what fails when not.
func (a *activity) loopShape(n *sysmlv1.Element) (s *loopShape, why string) {
	s = &loopShape{
		testedFirst: n.Attrs["isTestedFirst"] != "false",
		partOf:      map[*sysmlv1.Element]int{},
		lvSrc:       map[*sysmlv1.Element]*sysmlv1.Element{},
	}
	s.parts[loopSetup] = a.m.model.Refs(n, "setupPart")
	s.parts[loopTest] = a.m.model.Refs(n, "test")
	s.parts[loopBody] = a.m.model.Refs(n, "bodyPart")
	s.loopVars = n.Owned("loopVariable")
	s.lvIns = n.Owned("loopVariableInput")
	s.results = n.Owned("result")
	s.bodyOuts = append(a.m.model.Refs(n, "bodyOutput"), n.Owned("bodyOutput")...)
	s.decider = a.m.model.Ref(n, "decider")
	if s.decider == nil {
		return nil, "the loop names no decider, so its test cannot be written"
	}
	if s.decider.Parent == nil || nodeKind(s.decider) != nodePin ||
		!slices.Contains(s.parts[loopTest], s.decider.Parent) || !slices.Contains(outputPins(s.decider.Parent), s.decider) {
		return nil, "the decider is not an output pin of a node in the test part"
	}
	s.deciderNode = s.decider.Parent
	for i, nodes := range s.parts {
		for _, nd := range nodes {
			if k := nodeKind(nd); k == nodeParam || k == nodePin {
				return nil, describe(nd) + " of the loop's parts is no executable node"
			}
			s.partOf[nd] = i
		}
	}
	for _, nd := range n.Owned("node") {
		if _, inPart := s.partOf[nd]; inPart {
			continue
		}
		if nodeKind(nd) == nodeControl {
			s.routers = append(s.routers, nd)
			continue
		}
		return nil, describe(nd) + " is in no part of the loop and routes no data"
	}
	if len(s.loopVars) != len(s.lvIns) || len(s.loopVars) != len(s.results) || len(s.bodyOuts) != len(s.loopVars) {
		return nil, "the loop variables, inputs, body outputs and results differ in count"
	}
	for _, bo := range s.bodyOuts {
		if slices.Contains(s.loopVars, bo) {
			continue
		}
		if bo.Parent == nil || nodeKind(bo) != nodePin || !slices.Contains(s.parts[loopBody], bo.Parent) ||
			!slices.Contains(outputPins(bo.Parent), bo) {
			return nil, "a body output is not an output pin written for a node in the body part"
		}
	}
	// A routing node may only pass a loop variable's value on: walk the
	// routing edges from the loop variables before the edges are sorted and
	// reported, so a rejected shape reports nothing.
	lvPin := map[*sysmlv1.Element]bool{}
	for _, lv := range s.loopVars {
		lvPin[lv] = true
	}
	if len(s.routers) > 0 {
		reached := map[*sysmlv1.Element]bool{}
		for changed := true; changed; {
			changed = false
			for _, e := range n.Owned("edge") {
				src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
				if lvPin[src] || reached[src] {
					if r := s.isRouter(tgt); r && !reached[tgt] {
						reached[tgt] = true
						changed = true
					}
				}
			}
		}
		for _, nd := range s.routers {
			if !reached[nd] {
				return nil, describe(nd) + " routes data no loop variable provides"
			}
		}
	}
	a.sortLoopEdges(n, s)
	for _, nd := range s.routers {
		a.m.add(nd, Approximated, "", "the node routes data only: the flows through it are written from their sources to the pins it leads to")
	}
	return s, ""
}

// isRouter reports whether e is one of the loop's routing control nodes.
func (s *loopShape) isRouter(e *sysmlv1.Element) bool {
	return slices.Contains(s.routers, e)
}

// sortLoopEdges sorts the loop's edges into its parts' writers and the feeds
// the loop gives pins by default, reporting each it cannot carry: a flow to an
// earlier part or a control flow across parts, a flow into a pin that takes no
// default, and a flow out of the loop's parts as today.
func (a *activity) sortLoopEdges(n *sysmlv1.Element, s *loopShape) {
	lvPin := map[*sysmlv1.Element]bool{}
	for _, lv := range s.loopVars {
		lvPin[lv] = true
	}
	router := map[*sysmlv1.Element]bool{}
	for _, r := range s.routers {
		router[r] = true
	}
	s.routed = map[*sysmlv1.Element]bool{}
	s.routedSrc = map[*sysmlv1.Element]*sysmlv1.Element{}
	var routing []*sysmlv1.Element
	for _, e := range n.Owned("edge") {
		src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
		if src == nil || tgt == nil {
			a.m.unmapped(e, "the edge's ends resolve to nothing")
			continue
		}
		from, to := ownerNode(src), ownerNode(tgt)
		pf, pfOk := s.partOf[from]
		pt, ptOk := s.partOf[to]
		switch {
		case router[src] || lvPin[src]:
			routing = append(routing, e)
		case pfOk && ptOk && pf == pt:
			s.edges[pf] = append(s.edges[pf], e)
		case pfOk && ptOk && e.Type == "ControlFlow":
			a.m.unmapped(e, "a control flow between parts of the loop is not written")
		case pfOk && ptOk && nodeKind(tgt) == nodePin && s.earlier(pf, pt):
			a.feedPin(e, s, pt, tgt, src, pf)
		case pfOk && ptOk:
			a.m.unmapped(e, "the flow runs from a later part of the loop to an earlier one; nothing carries the value back")
		case pfOk && lvPin[tgt] && pf == loopSetup && e.Type == "ObjectFlow":
			if _, taken := s.lvSrc[tgt]; taken {
				a.m.unmapped(e, "a second setup value for the loop variable "+describe(tgt))
			} else {
				s.lvSrc[tgt] = src
				a.m.add(e, Mapped, "", "the loop variable "+describe(tgt)+" takes the setup's "+describe(src))
			}
		default:
			a.m.unmapped(e, "the edge between the loop's parts is not written")
		}
	}
	// The routing edges reach pins of parts through loop-level control nodes;
	// each such node only passes a loop variable's value on.
	for changed := true; changed; {
		changed = false
		for _, e := range routing {
			src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
			lv := src
			if router[src] {
				lv = s.routedSrc[src]
				if lv == nil {
					continue
				}
			}
			if router[tgt] {
				if s.routedSrc[tgt] == nil {
					s.routedSrc[tgt], s.routed[tgt] = lv, true
					changed = true
				}
				continue
			}
			if s.feedDone == nil {
				s.feedDone = map[*sysmlv1.Element]bool{}
			}
			if !s.feedDone[e] {
				s.feedDone[e] = true
				pt, ptOk := s.partOf[ownerNode(tgt)]
				a.feedLoopPin(e, s, tgt, lv, pt, ptOk)
				changed = true
			}
		}
	}
}

// feedLoopPin gives the pin of a part a flow from a loop variable or a routing
// node carries as its default; a target in no part — a loop-level pin or a
// node nested deeper than a part — keeps the edge unmapped.
func (a *activity) feedLoopPin(e *sysmlv1.Element, s *loopShape, tgt, lv *sysmlv1.Element, pt int, ptOk bool) {
	if lv == nil || !ptOk || nodeKind(tgt) != nodePin {
		a.m.unmapped(e, "the edge from the loop's routing is not written")
		return
	}
	a.feedPin(e, s, pt, tgt, lv, -1)
}

// feedPin records that pin tgt of part pt defaults to the value source src
// carries — the loop variable, or the pin srcPart performs. A pin standing for
// a callee's parameter, or one already fed inside its part, takes no default.
func (a *activity) feedPin(e *sysmlv1.Element, s *loopShape, pt int, tgt, src *sysmlv1.Element, srcPart int) {
	owner := tgt.Parent
	if owner == nil || a.calleeOf(owner) != nil || isStructured(owner) {
		a.m.unmapped(e, "the pin "+describe(tgt)+" takes no default value, so the flow feeding it is not written")
		return
	}
	for _, ie := range s.edges[pt] {
		if a.m.model.Ref(ie, "target") == tgt {
			a.m.unmapped(e, "the pin "+describe(tgt)+" is already fed inside its part")
			return
		}
	}
	if s.feed[pt] == nil {
		s.feed[pt] = map[*sysmlv1.Element]loopFeed{}
	}
	s.feed[pt][tgt] = loopFeed{src: src, srcPart: srcPart}
	if srcPart < 0 {
		a.m.add(e, Mapped, "", "the pin takes the loop variable "+describe(src))
	} else {
		a.m.add(e, Mapped, "", "the pin takes "+describe(src)+", computed by an earlier part of the loop")
	}
}

// calleeOf returns the behavior or operation whose parameters a call node's
// pins stand for; their declarations take no default.
func (a *activity) calleeOf(n *sysmlv1.Element) *sysmlv1.Element {
	switch n.Type {
	case "CallBehaviorAction":
		if b := a.m.model.Ref(n, "behavior"); b != nil {
			if op := a.m.methodOf[b]; op != nil {
				return op
			}
			return b
		}
	case "CallOperationAction":
		if op := a.m.model.Ref(n, "operation"); op != nil {
			return op
		}
	}
	return nil
}

// loopNode writes a LoopNode as a v2 loop: its input and result pins, its loop
// variables and an `ended` flag as private features, the setup part once, the
// loop performing test before body — or body before test when the loop tests
// last — and the results read from the variables at the end.
func (a *activity) loopNode(n *sysmlv1.Element, name string, s *loopShape) {
	setup := a.fresh("setup")
	init := a.fresh("init")
	iterate := a.fresh("iterate")
	test := a.fresh("test")
	body := a.fresh("body")
	next := a.fresh("next")
	results := a.fresh("results")
	ended := a.fresh("ended")
	for _, madeUp := range []string{setup, init, iterate, test, body, next, results, ended} {
		a.m.take(n, madeUp)
	}
	partName := [3]string{setup, test, body}
	// Name the loop's pins and variables before the part writers spell feeds
	// with them; both calls settle the same names their declarations reuse.
	a.settlePins(n)
	for _, lv := range s.loopVars {
		a.name(lv, "variable")
	}
	writers := a.loopParts(n, s, partName, ended)
	a.m.w.block(actionKw+name, func() {
		for _, madeUp := range []string{iterate, results, ended} {
			a.madeUp(writeName(madeUp))
		}
		if len(s.parts[loopSetup]) > 0 {
			a.madeUp(writeName(setup))
		}
		a.pins(n, nil)
		for i, lv := range s.loopVars {
			vname, decl, note := a.variableFeature(lv)
			a.vars[lv] = a.names[lv]
			if s.lvSrc[lv] == nil {
				decl += " := " + writeName(a.m.pins[s.lvIns[i]].name)
			}
			a.m.w.line(decl + ";")
			a.m.add(lv, verdictFor(note), vname, note)
		}
		a.m.w.line("private attribute " + writeName(ended) + " : ScalarValues::Boolean := false;")
		if len(s.parts[loopSetup]) > 0 {
			a.m.w.line(firstKw + "start" + thenKw + writeName(setup) + ";")
			a.m.w.block(actionKw+writeName(setup), func() { writers[loopSetup].write() })
			// A variable the setup feeds is read once the setup has performed,
			// in a step of its own — a feature default would read it too soon.
			if len(s.lvSrc) > 0 {
				a.m.w.block("then "+actionKw+writeName(init), func() {
					nth := 0
					for _, lv := range s.loopVars {
						src := s.lvSrc[lv]
						if src == nil {
							continue
						}
						then := ""
						if nth > 0 {
							then = "then "
						}
						a.m.w.line(then + "assign " + writeName(a.names[lv]) + " := " + a.partRef(writers[loopSetup], setup, src) + ";")
						nth++
					}
				})
			}
		} else {
			a.m.w.line(firstKw + "start" + thenKw + writeName(iterate) + ";")
		}
		a.m.w.block("then "+actionKw+writeName(iterate), func() {
			a.m.w.line("loop {")
			a.m.w.indented(func() { a.loopCycle(writers, s, test, body, next, ended) })
			a.m.w.line("} until " + writeName(ended) + ";")
		})
		a.m.w.block("then "+actionKw+writeName(results), func() {
			for i, r := range s.results {
				then := ""
				if i > 0 {
					then = "then "
				}
				a.m.w.line(then + "assign " + writeName(a.m.pins[r].name) + " := " + writeName(a.names[s.loopVars[i]]) + ";")
			}
		})
	})
	a.m.add(n, Mapped, name, "")
}

// loopCycle writes one iteration: test before body when the loop tests first,
// else body before test — each followed by the assignments feeding the values
// forward, and the decider closing the loop when false.
func (a *activity) loopCycle(writers [3]*activity, s *loopShape, test, body, next, ended string) {
	partWriter := func(part int, name string, then bool) {
		prefix := ""
		if then {
			prefix = "then "
		}
		a.m.w.block(prefix+actionKw+writeName(name), func() { writers[part].write() })
	}
	nextAssigns := func() {
		a.m.w.block("then "+actionKw+writeName(next), func() {
			for i, bo := range s.bodyOuts {
				then := ""
				if i > 0 {
					then = "then "
				}
				var from string
				if slices.Contains(s.loopVars, bo) {
					from = writeName(a.names[bo])
				} else {
					from = a.partRef(writers[loopBody], body, bo)
				}
				a.m.w.line(then + "assign " + writeName(a.names[s.loopVars[i]]) + " := " + from + ";")
			}
		})
	}
	decision := func(withElse bool) {
		a.m.w.line("then if not " + a.partRef(writers[loopTest], test, s.decider) + " {")
		a.m.w.indented(func() { a.m.w.line("assign " + writeName(ended) + " := true;") })
		if withElse {
			a.m.w.line("} else {")
			a.m.w.indented(func() {
				partWriter(loopBody, body, false)
				nextAssigns()
			})
		}
		a.m.w.line("}")
	}
	if s.testedFirst {
		partWriter(loopTest, test, false)
		decision(true)
		return
	}
	partWriter(loopBody, body, false)
	nextAssigns()
	partWriter(loopTest, test, true)
	decision(false)
}

// loopParts builds the writer of each part: one graph over the part's nodes
// and intra-part edges, with the loop's names reserved so nothing in a part
// shadows them, and the pins an outside value feeds given their defaults once
// every writer exists to spell them.
func (a *activity) loopParts(n *sysmlv1.Element, s *loopShape, partName [3]string, ended string) (writers [3]*activity) {
	for part := loopSetup; part <= loopBody; part++ {
		inner := a.m.newActivity(n, n)
		inner.nodes = s.parts[part]
		inner.edges = s.edges[part]
		for v, declared := range a.vars {
			inner.vars[v] = declared
			inner.used[declared] = true
		}
		for _, reserved := range partName {
			inner.used[reserved] = true
		}
		inner.used[ended] = true
		for _, lv := range s.loopVars {
			if name := a.names[lv]; name != "" {
				inner.used[name] = true
			}
		}
		writers[part] = inner
	}
	for part := loopSetup; part <= loopBody; part++ {
		for pin, feed := range s.feed[part] {
			writers[part].computed[pin] = a.feedRef(writers, s, partName, feed)
			writers[part].fed[pin] = true
		}
	}
	return writers
}

// feedRef spells the value a feed gives a pin: the loop variable's name when
// the source is one, else the earlier part's pin reference.
func (a *activity) feedRef(writers [3]*activity, s *loopShape, partName [3]string, feed loopFeed) string {
	if feed.srcPart < 0 {
		return writeName(a.names[feed.src])
	}
	return a.partRef(writers[feed.srcPart], partName[feed.srcPart], feed.src)
}

// partRef spells a reference to a pin of a node of part w's graph: the part's
// block, the node, the pin — settled ahead of the part's write so a feed can
// name it.
func (a *activity) partRef(w *activity, part string, pin *sysmlv1.Element) string {
	owner := pin.Parent
	w.settlePins(owner)
	return part + "." + writeName(w.name(owner, baseName(owner))) + "." + writeName(a.m.pins[pin].name)
}
