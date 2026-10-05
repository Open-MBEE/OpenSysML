package migrate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// behaviorContext is the object an activity acts on, taken as a reference
// parameter: the classifier whose ports its actions go through when its owner
// is not it, or the owner's own classifier for a def, whose `this` is the def's
// own occurrence rather than the object it runs on.
type behaviorContext struct {
	name       string
	classifier *sysmlv1.Element
	declared   *sysmlv1.Element
	chain      string
	// holder is the def declaring the parameter, for the `Def::context` form a
	// nested call binds it by, where a bare name would be shadowed.
	holder *sysmlv1.Element
	// owner is whether the object is the def's own classifier, spelled for `this`.
	owner bool
	// implied is whether only the body's own reads or bindings call for it,
	// none of its actions going through ports: a caller is then not held to
	// name an object of the classifier for the def. Settled once, with the def.
	implied bool
	// used is whether the def's body spelled it; bound whether a usage bound it.
	// The parameter is written when either holds once the body is evaluated.
	used, bound, evaluated bool
}

// objectType is the classifier the parameter's object is: the definition it is typed by.
func (c *behaviorContext) objectType() *sysmlv1.Element {
	if c.declared != nil {
		return c.declared
	}
	return c.classifier
}

// contextVisit is an activity the context search has reached: its visit index, the
// earliest index its calls lead back to, and the port owners named so far.
type contextVisit struct {
	index, low int
	owners     []*sysmlv1.Element
	// implied are the classifiers the contexts its calls lead to take for their
	// own reads alone, which the visited behavior binds but need not act on.
	implied []*sysmlv1.Element
}

// contextOf settles, once, the context an activity needs: the one classifier whose
// ports it or the behaviors it calls name. A classifier's own behavior acts on its
// object unless nothing it needs is one: v1 runs a called behavior on the caller's
// object, whoever owns it, so such a behavior takes the object it acts on instead.
// A cycle of calls settles at once, with everything its members name.

// The note fragments the writer repeats.
const (
	actsOn       = "the behavior acts on a "
	throughParam = " through its parameter "
)

func (m *migration) contextOf(b *sysmlv1.Element) *behaviorContext {
	if b == nil || m.asUsage[b] {
		return nil
	}
	if c, settled := m.contexts[b]; settled {
		return c
	}
	if m.inUsageBody(b) {
		// Under a usage's body the object's features resolve on this.
		m.contexts[b] = nil
		return nil
	}
	if b.Type != "Activity" {
		c := m.ownerContext(b)
		m.contexts[b] = c
		if c != nil && m.usesFeaturesOf(b, c.classifier) {
			c.used = true
		}
		if c != nil && b.Type == "Operation" {
			if method := m.model.Ref(b, "method"); method != nil && m.usesFeaturesOf(method, c.classifier) {
				c.used = true
			}
		}
		return c
	}
	if m.visiting[b] != nil {
		// Reached again before settling: the cycle it closes settles it.
		return nil
	}
	m.visitContext(b)
	if m.contexts[b] == nil {
		m.contexts[b] = m.ownerContext(b)
	}
	return m.contexts[b]
}

// contextThroughPorts reports whether c is a context reached through ports: an
// object other than the def's owner, which a usage's this cannot stand for.
func contextThroughPorts(c *behaviorContext) bool {
	return c != nil && !c.owner
}

// ownerContext is the context a def takes for the object its body acts on: its
// own classifier, whose features the body reads through the parameter since
// the def's `this` is its own occurrence; nil when e is no def or in no
// classifier.
func (m *migration) ownerContext(e *sysmlv1.Element) *behaviorContext {
	if e == nil || !defScope(e) {
		return nil
	}
	if m.inUsageBody(e) {
		// Inside a usage's body the object's features resolve on this.
		return nil
	}
	owner := classifierOf(e)
	if owner == nil {
		return nil
	}
	declared, chain, _ := m.contextDeclaration(owner)
	if declared == nil {
		return nil
	}
	if c, ok := m.ownerCtx[e]; ok {
		return c
	}
	c := &behaviorContext{name: m.freshName(e, "context"), classifier: owner, holder: e, owner: true}
	if declared != owner {
		c.declared = declared
		c.chain = chain
	}
	m.ownerCtx[e] = c
	return c
}

// contextDeclaration is how a context parameter holds an object of c: the classifier it
// is written with and the feature path from that object to c; nil with why when none can.
func (m *migration) contextDeclaration(c *sysmlv1.Element) (declared *sysmlv1.Element, chain, why string) {
	if m.asUsage[c] {
		return nil, "", ""
	}
	if cat, _ := m.classify(c); cat == catView || cat == catViewpoint {
		d := m.featuringDef(c)
		if d == nil {
			return c, "", ""
		}
		if !m.definitionEnd(d) {
			defCat, _ := m.classify(d)
			return nil, "", "the " + cat.keyword() + " " + qualifiedName(c) + " is a feature of the " + defCat.keyword() + " " + qualifiedName(d) + ", which cannot type a context parameter"
		}
		return d, m.usageChain(c, d), ""
	}
	if m.definitionEnd(c) {
		return c, "", ""
	}
	return nil, "", ""
}

// inUsageBody reports whether e's body is written in a usage's body, where the
// object's features resolve bare: e is the usage or inline in it. A def nested
// in a usage is not: its `this` is its own occurrence, so it takes a context.
func (m *migration) inUsageBody(e *sysmlv1.Element) bool {
	for cur := e; cur != nil; cur = cur.Parent {
		if m.asUsage[cur] || m.asUsage[m.methodOf[cur]] {
			return true
		}
		if defScope(cur) {
			return false
		}
	}
	return false
}

// defScope reports whether e is written as a def whose `this` is its own
// occurrence: a behavior or operation declared as a member, not an inline body
// of a state, transition or operation.
func defScope(e *sysmlv1.Element) bool {
	switch e.Role {
	case "entry", "exit", "doActivity", "effect", "method", "guard":
		return false
	}
	return isBehavior(e) || e.Type == "Operation" || e.Type == "Reception"
}

// enclosingDef is the def e's body is written inside: the innermost enclosing
// element declared as a member; nil in a usage of the object or a bare member
// of the classifier, where the object's features resolve bare.
func (m *migration) enclosingDef(e *sysmlv1.Element) *sysmlv1.Element {
	for cur := e; cur != nil; cur = cur.Parent {
		if op := m.methodOf[cur]; op != nil {
			return op
		}
		if defScope(cur) {
			return cur
		}
		switch cur.Type {
		case "Package", "Model", "Profile":
			return nil
		default:
			if m.contextClassifier(cur) == cur {
				return nil
			}
		}
	}
	return nil
}

// defContext is the context parameter the def enclosing e declares for the
// object its `this` does not reach — the spelling of `this` — nil when no def
// encloses e, or the def's context is another object than its owner.
func (m *migration) defContext(e *sysmlv1.Element) *behaviorContext {
	if c := m.contextOf(m.enclosingDef(e)); c != nil && c.owner {
		return c
	}
	return nil
}

// selfContext is the context parameter the def enclosing e declares, whatever
// object it is: what a call inside binds on to the next behavior. Nil outside
// a def taking one.
func (m *migration) selfContext(e *sysmlv1.Element) *behaviorContext {
	return m.contextOf(m.enclosingDef(e))
}

// thisName spells `this` where e's body is written: the enclosing def's
// context parameter, or this itself inside the object's usages.
func (m *migration) thisName(e *sysmlv1.Element) string {
	if c := m.selfContext(e); c != nil {
		return m.contextSpelling(c, e)
	}
	return "this"
}

// selfPrefix spells the start of a path on the object the def enclosing e acts
// on — its context parameter followed by a dot — or nothing outside such a def,
// where `this` and the object's features resolve in scope.
func (m *migration) selfPrefix(e *sysmlv1.Element) string {
	if c := m.selfContext(e); c != nil {
		return m.contextSpelling(c, e) + "."
	}
	return ""
}

// ownerPrefix spells the start of a feature path on `this` where e's body is
// written: the def's context parameter followed by a dot inside a def, or
// nothing inside the object's usages, whose scope resolves its features bare.
func (m *migration) ownerPrefix(e *sysmlv1.Element) string {
	if c := m.defContext(e); c != nil {
		return m.contextSpelling(c, e) + "."
	}
	return ""
}

// contextSpelling spells def c's context parameter where e's body is written.
// The parameter resolves by its bare name anywhere inside the def's own scope:
// in the def's body, in usages nested in it, and in the transitions, regions
// and pseudostates a state machine's bodies hang from; a state usage's own
// context redefinition names the same object. Where e is not inside the def,
// the parameter takes the def's qualified name.
func (m *migration) contextSpelling(c *behaviorContext, e *sysmlv1.Element) string {
	withChain := func(name string) string {
		if c.chain != "" {
			return name + "." + c.chain
		}
		return name
	}
	if e == nil {
		return withChain(m.qualifiedContext(c, e))
	}
	cur := e
	for cur != nil && cur != c.holder {
		if op := m.methodOf[cur]; op != nil {
			cur = op
			continue
		}
		cur = cur.Parent
	}
	if cur != c.holder {
		return withChain(m.qualifiedContext(c, e))
	}
	m.markUsed(&c.used)
	return withChain(writeName(c.name))
}

// markUsed marks a context parameter as used by a name spelled through it,
// remembering the mark so that a body the translator refuses after spelling
// some of its names can unmark what it marked: the def declares the parameter
// only for names it writes.
func (m *migration) markUsed(used *bool) {
	if !*used {
		*used = true
		m.marked = append(m.marked, used)
	}
}

// unmark undoes the marks made since there were n of them.
func (m *migration) unmark(n int) {
	for _, used := range m.marked[n:] {
		*used = false
	}
	m.marked = m.marked[:n]
}

// respellThis rewrites an expression built on `this` — `this` itself or a
// `this.f` path — for the body e is written in.
func (m *migration) respellThis(expr string, e *sysmlv1.Element) string {
	if expr == "this" {
		return m.thisName(e)
	}
	if strings.HasPrefix(expr, "this.") {
		rest := strings.TrimPrefix(expr, "this.")
		if c := m.selfContext(e); c != nil {
			return m.contextSpelling(c, e) + "." + rest
		}
		if owner := m.contextClassifier(e); owner != nil {
			// Inside a usage the feature resolves bare — unless a nearer
			// declaration hides it, when the path goes through the owning def.
			first, _, _ := strings.Cut(rest, ".")
			visible, _ := m.visibleFrom(e)
			if members, _ := m.membersOf(owner, memberAny); members[first] != nil && members[first] != visible[first] {
				return m.ref(owner, e) + "::" + rest
			}
		}
		return rest
	}
	return expr
}

// anchorExpr spells a path rooted at a member of the context object — a
// swimlane's represented property written without `this` — where e's body is
// written: through the context parameter inside a def, bare inside a usage.
func (m *migration) anchorExpr(expr string, e *sysmlv1.Element) string {
	if expr == "this" || strings.HasPrefix(expr, "this.") {
		root := m.respellThis(expr, e)
		if root == "this" && m.selfContext(e) == nil {
			// Inside a usage `this` is the usage's own occurrence; as the root
			// of a member path the object has no name — spell the member bare.
			return ""
		}
		return root
	}
	return m.selfPrefix(e) + expr
}

// joinDot joins a member path's root and step, keeping a lone root or step.
func joinDot(root, step string) string {
	if root == "" {
		return step
	}
	return root + "." + step
}

// callBodyExpr respells names from e's body that a call usage's body could
// shadow; the caller's context parameter keeps its lexical spelling.
func (m *migration) callBodyExpr(expr string, e *sysmlv1.Element) string {
	if c := m.selfContext(e); c != nil {
		name := writeName(c.name)
		if expr == name {
			return m.qualifiedContext(c, e)
		}
		if strings.HasPrefix(expr, name+".") {
			return m.qualifiedContext(c, e) + expr[len(name):]
		}
	}
	first, _, _ := strings.Cut(expr, ".")
	if d := m.enclosingDef(e); d != nil && !m.asUsage[d] {
		if visible, _ := m.membersOf(d, memberAny); visible[first] != nil {
			return m.ref(d, e) + "::" + expr
		}
	}
	if owner := m.contextClassifier(e); owner != nil {
		if visible, _ := m.membersOf(owner, memberAny); visible[first] != nil {
			// Inside a usage's call body a bare name is the call's own; the
			// object's feature is spelled qualified through its def.
			return m.ref(owner, e) + "::" + expr
		}
	}
	return expr
}

// qualifiedContext writes a def's context parameter as it is named from inside
// its own body, where a call's own redefinition would shadow the bare name.
func (m *migration) qualifiedContext(c *behaviorContext, scope *sysmlv1.Element) string {
	m.markUsed(&c.used)
	return m.contextName(c, scope)
}

// contextName is qualifiedContext's spelling alone, for a binding that may
// come to nothing: the parameter is used once the binding is written.
func (m *migration) contextName(c *behaviorContext, scope *sysmlv1.Element) string {
	return m.ref(c.holder, scope) + "::" + writeName(c.name)
}

// contextBody orders a usage's parameter redeclarations and context binding
// as the callee's definition does.
func (m *migration) contextBody(callee *sysmlv1.Element, ins string) []string {
	var members []string
	cat, _ := m.classify(callee)
	leads := m.contextLeads(callee, cat)
	if leads {
		members = append(members, ins)
	}
	for _, p := range m.actionParameters(callee) {
		dir, _ := parameterDirection(p)
		members = append(members, dir+" "+writeName(m.nameFor(p))+m.parameterShape(p))
	}
	if !leads {
		members = append(members, ins)
	}
	return members
}

// contextIns writes the redefinition by which a usage of a behavior taking a
// context binds it to the object scope runs on: `in ref :>> context = expr`,
// the object qualified from the enclosing def or `this`; "" when the behavior
// takes none that any use declared, or the object is not one to bind, with the
// note contextBinding gives then.
func (m *migration) contextIns(c *behaviorContext, scope *sysmlv1.Element) (ins, note string) {
	if c == nil {
		return "", ""
	}
	// A def already written, whose body never read its owner: no parameter was declared, so nothing binds it.
	if c.owner && c.evaluated && !c.used && !c.bound {
		return "", ""
	}
	self, selfType := "this", m.contextClassifier(scope)
	dc := m.selfContext(scope)
	if dc != nil {
		self, selfType = m.contextName(dc, scope), dc.objectType()
	}
	expr, cnote := m.contextBinding(c, selfType, self, scope)
	if expr == "" {
		return "", cnote
	}
	if dc != nil {
		m.markUsed(&dc.used)
	}
	c.bound = true
	return "in ref :>> " + writeName(c.name) + " = " + expr, cnote
}

// visitContext reaches b and, through it, the behaviors it calls; once every call
// from b leads back no earlier than b, b and the cycle it heads are settled.
func (m *migration) visitContext(b *sysmlv1.Element) {
	v := &contextVisit{index: len(m.visits), low: len(m.visits), owners: m.namedPortOwners(b)}
	m.visiting[b] = v
	m.visits = append(m.visits, b)
	for _, c := range m.calledActivities(b) {
		if ctx, settled := m.contexts[c]; settled {
			v.reach(ctx)
			continue
		}
		if w := m.visiting[c]; w != nil {
			v.low = min(v.low, w.index)
			continue
		}
		m.visitContext(c)
		if ctx, settled := m.contexts[c]; settled {
			v.reach(ctx)
		} else {
			v.low = min(v.low, m.visiting[c].low)
		}
	}
	if v.low != v.index {
		return
	}
	members := slices.Clone(m.visits[v.index:])
	m.visits = m.visits[:v.index]
	var owners, implied []*sysmlv1.Element
	for _, e := range members {
		for _, o := range m.visiting[e].owners {
			owners = addOwner(owners, o)
		}
		for _, o := range m.visiting[e].implied {
			implied = addOwner(implied, o)
		}
		delete(m.visiting, e)
		// Settled as none while the members decide, so a cycle contributes nothing to itself.
		m.contexts[e] = nil
	}
	decided := make([]*behaviorContext, len(members))
	for i, e := range members {
		decided[i] = m.decideContext(e, owners)
	}
	// A member whose body reads its owner, or binds a callee's context to it or
	// to one of its parts, takes the owner as its own context; the fellow members
	// calling it then bind that, until no member's need is new to the cycle.
	for settled := false; !settled; {
		settled = true
		for i, e := range members {
			owner := classifierOf(e)
			if decided[i] != nil || owner == nil || !(m.usesFeaturesOf(e, owner) || m.providesAny(owner, implied)) {
				continue
			}
			if decided[i] = m.ownerContext(e); decided[i] != nil {
				decided[i].implied = true
				implied = addOwner(implied, owner)
				settled = false
			}
		}
	}
	for i, e := range members {
		if decided[i] == nil && len(owners) == 0 {
			decided[i] = m.impliedContext(e, implied)
		}
		m.contexts[e] = decided[i]
	}
}

// impliedContext is the context a behavior of no classifier takes for the
// behaviors it calls that act on their owners for their own reads: the one of
// those classifiers an object of which is, or holds a part that is, an object of
// each of the others, since nothing else the behavior has could be bound to them.
// A behavior of a classifier binds them to its own object instead, or not at all.
func (m *migration) impliedContext(b *sysmlv1.Element, implied []*sysmlv1.Element) *behaviorContext {
	if classifierOf(b) != nil {
		return nil
	}
	var eligible []*sysmlv1.Element
	for _, c := range implied {
		if m.definitionEnd(c) {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	if c := m.encompassing(eligible); c != nil {
		return &behaviorContext{name: m.freshName(b, "context"), classifier: c, holder: b, implied: true}
	}
	names := make([]string, len(eligible))
	for i, c := range eligible {
		names[i] = qualifiedName(c)
	}
	m.contextNotes[b] = "the behaviors it calls act on " + strings.Join(names, " and ") +
		", none of which is or holds a part that is each of the others, so no one object is written for them to act on"
	return nil
}

// encompassing is the classifier among cs an object of which is, or holds one
// part that is, an object of every other; nil when none is.
func (m *migration) encompassing(cs []*sysmlv1.Element) *sysmlv1.Element {
	for _, c := range cs {
		holds := true
		for _, o := range cs {
			if expr, _, _ := m.objectOf(o, c, "this"); o != c && expr == "" {
				holds = false
				break
			}
		}
		if holds {
			return c
		}
	}
	return nil
}

// reach records the context of a behavior the visit's calls lead to: the
// classifier whose ports it acts through as a port owner, one it takes only for
// its own reads as implied, which holds the caller to no object of it.
func (v *contextVisit) reach(ctx *behaviorContext) {
	switch {
	case ctx == nil:
	case ctx.implied:
		v.implied = addOwner(v.implied, ctx.classifier)
	default:
		v.owners = addOwner(v.owners, ctx.classifier)
	}
}

func addOwner(owners []*sysmlv1.Element, c *sysmlv1.Element) []*sysmlv1.Element {
	if c != nil && !slices.Contains(owners, c) {
		owners = append(owners, c)
	}
	return owners
}

// decideContext is the context b takes given the classifiers whose ports it or
// the behaviors it calls name; the note why none is written is kept for the report.
func (m *migration) decideContext(b *sysmlv1.Element, owners []*sysmlv1.Element) *behaviorContext {
	how := "its actions go through ports of "
	owners = m.eligibleContexts(b, owners, how)
	switch owner := classifierOf(b); {
	case owner != nil && b.Parent != owner && !defScope(b):
		// A state's or transition's behavior runs on the machine's object.
		return nil
	case owner != nil:
		if defScope(b) && slices.Contains(owners, owner) {
			return m.ownerContext(b)
		}
		if len(owners) == 0 || m.providesAny(owner, owners) || m.usesFeaturesOf(b, owner) {
			if b.Parent != owner {
				// A def nested in a behavior is settled with it, so it takes
				// its owner here rather than once asked for on its own.
				return m.ownerContext(b)
			}
			return nil
		}
	case len(owners) == 0:
		owners = m.invokerOwners(b)
		how = "its accepts take signals arriving at the ports of "
	}
	c := m.mostSpecific(owners)
	if c == nil {
		return m.contextAmong(b, owners, how)
	}
	declared, chain, _ := m.contextDeclaration(c)
	ctx := &behaviorContext{name: m.freshName(b, "context"), classifier: c, holder: b}
	if declared != c {
		ctx.declared = declared
		ctx.chain = chain
	}
	return ctx
}

// eligibleContexts keeps the owners written as a declaration that can type
// or be specialized by a context, noting for the report when none is.
func (m *migration) eligibleContexts(b *sysmlv1.Element, owners []*sysmlv1.Element, how string) []*sysmlv1.Element {
	eligible := make([]*sysmlv1.Element, 0, len(owners))
	notes := make([]string, 0, len(owners))
	for _, owner := range owners {
		declared, _, why := m.contextDeclaration(owner)
		if declared != nil {
			eligible = append(eligible, owner)
		} else if why != "" {
			notes = append(notes, why)
		}
	}
	if len(owners) > 0 && len(eligible) == 0 {
		m.contextNotes[b] = how + strings.Join(ownerNames(owners), " and ") +
			", none of which is written as a definition that can type a context parameter or a usage it can specialize"
		if len(notes) > 0 {
			m.contextNotes[b] += ": " + strings.Join(notes, "; ")
		}
	}
	return eligible
}

// contextAmong is the context taken from owners none of which is a special of
// the others: the most specific of their declarations, else none, noted.
func (m *migration) contextAmong(b *sysmlv1.Element, owners []*sysmlv1.Element, how string) *behaviorContext {
	if len(owners) <= 1 {
		return nil
	}
	types := make([]*sysmlv1.Element, 0, len(owners))
	for _, owner := range owners {
		declared, _, _ := m.contextDeclaration(owner)
		types = addOwner(types, declared)
	}
	if d := m.mostSpecific(types); d != nil {
		return &behaviorContext{name: m.freshName(b, "context"), classifier: d, holder: b}
	}
	m.contextNotes[b] = how + strings.Join(ownerNames(owners), " and ") +
		", none of which is a special of the others, so no one object is written for them to act on"
	return nil
}

func ownerNames(owners []*sysmlv1.Element) []string {
	names := make([]string, len(owners))
	for i, owner := range owners {
		names[i] = qualifiedName(owner)
	}
	return names
}

// providesAny reports whether an object of owner is, or holds one part that is,
// one of the classifiers cs: then a behavior it owns acts on it.
func (m *migration) providesAny(owner *sysmlv1.Element, cs []*sysmlv1.Element) bool {
	for _, c := range cs {
		if expr, _ := m.contextBinding(&behaviorContext{classifier: c}, owner, "this", nil); expr != "" {
			return true
		}
	}
	return false
}

// usesFeaturesOf reports whether b reads itself or a structural feature of c, so
// that it plainly acts on an object of c.
func (m *migration) usesFeaturesOf(b, c *sysmlv1.Element) bool {
	uses := false
	m.walkActions(b, func(e *sysmlv1.Element) {
		if m.actionReads(e, c) {
			uses = true
		}
	})
	return uses
}

// actionReads reports whether one action or expression of a behavior reads
// itself or a structural feature of c.
func (m *migration) actionReads(e, c *sysmlv1.Element) bool {
	switch e.Type {
	case "ReadSelfAction":
		return true
	case "ReadStructuralFeatureAction", "AddStructuralFeatureValueAction", "RemoveStructuralFeatureValueAction", "ClearStructuralFeatureAction":
		f := m.model.Ref(e, "structuralFeature")
		return f != nil && m.hasFeature(c, f)
	case "OpaqueAction":
		// A body written in its own language reads a feature by its name.
		body, lang := opaqueBody(e)
		return m.bodyReads(body, lang, e, c, true)
	case "OpaqueExpression":
		// So does a guard, a value, or any other expression the body carries.
		body, lang := opaqueBody(e)
		return m.bodyReads(body, lang, opaqueScope(e), c, false)
	case "DurationConstraint":
		return m.durationConstraintReads(e, c)
	case "AcceptEventAction":
		for _, trigger := range e.Owned("trigger") {
			if ev := m.model.Ref(trigger, "event"); ev != nil && m.eventReads(ev, e, c) {
				return true
			}
		}
	}
	return false
}

func (m *migration) durationConstraintReads(e, c *sysmlv1.Element) bool {
	spec := firstOwned(e, "specification")
	if spec == nil {
		return false
	}
	for _, scope := range m.model.Refs(e, "constrainedElement") {
		for _, endpoint := range []string{"min", "max"} {
			if m.durationReads(m.model.Ref(spec, endpoint), scope, c) {
				return true
			}
		}
	}
	return false
}

// eventReads reports whether the event an accept waits for — a relative time
// event's duration or a change event's expression — reads a feature of c.
func (m *migration) eventReads(ev, e, c *sysmlv1.Element) bool {
	switch ev.Type {
	case "TimeEvent":
		return ev.Attrs["isRelative"] == "true" && m.durationReads(firstOwned(ev, "when"), e, c)
	case "ChangeEvent":
		v := firstOwned(ev, "changeExpression")
		if v == nil {
			return false
		}
		var body, lang string
		switch v.Type {
		case "LiteralString":
			body = v.Attrs["value"]
		case "OpaqueExpression":
			body, lang = opaqueBody(v)
		default:
			return false
		}
		return m.bodyReads(body, lang, e, c, false)
	}
	return false
}

func (m *migration) durationReads(v, scope, c *sysmlv1.Element) bool {
	for v != nil && (v.Type == "Duration" || v.Type == "TimeExpression") {
		v = firstOwned(v, "expr")
	}
	if v == nil {
		return false
	}
	var text, lang string
	switch v.Type {
	case "LiteralString":
		text = v.Attrs["value"]
	case "OpaqueExpression":
		text, lang = opaqueBody(v)
	default:
		return false
	}
	body, _, _ := durationBody(text)
	if _, _, ok := parseDuration(text); ok {
		return false
	}
	return m.bodyReads(body, lang, scope, c, false)
}

// bodyReads reports whether an opaque body, read where scope's names resolve,
// names a feature of the classifier c as the writer spells it: a name the
// translator resolves reading the body — as statements or as one expression —
// none being a local the body declares; or, for a body in a language it does
// not read, each name the body copied as v2 syntax reads. A body the writer
// keeps as a comment reads nothing. A name after a dot is a member of what
// precedes it, not a read of its own; a name reads c directly or through a
// swimlane over one of its parts.
func (m *migration) bodyReads(body, lang string, scope, c *sysmlv1.Element, statements bool) bool {
	body = strings.TrimSpace(body)
	if body == "" || scope == nil {
		return false
	}
	s := m.bodyScope(scope)
	s.probe = true
	names, ok, final := s.namesRead(body, lang, statements)
	switch {
	case final:
		return false
	case !ok:
		names = m.v2Names(body, lang, scope, statements)
	}
	for _, name := range names {
		if s.readsFeatureOf(name, c) {
			return true
		}
	}
	return false
}

// v2Names lists the first step of every name a body the writer copies as v2
// syntax reads at scope — the features its statements assign and the roots of
// its expressions — none when the writer refuses the body and keeps it as a comment.
func (m *migration) v2Names(body, lang string, scope *sysmlv1.Element, statements bool) []string {
	var names []string
	roots := func(refs []reference) {
		for _, r := range refs {
			if !r.global && r.local == "" && len(r.steps) > 0 {
				names = append(names, r.steps[0].name)
			}
		}
	}
	if !statements {
		if refs, ok, _ := m.v2Refs(body, lang, scope); ok {
			roots(refs)
		}
		return names
	}
	assigns, ok, _ := m.v2Assignments(body, lang, scope)
	if !ok {
		return nil
	}
	for _, a := range assigns {
		names = append(names, a.name)
		roots(a.refs)
	}
	return names
}

// opaqueScope is the element whose names an opaque expression's body reads:
// the node or edge holding it, a pin's value reading at the pin's node.
func opaqueScope(v *sysmlv1.Element) *sysmlv1.Element {
	scope := v.Parent
	if scope != nil && nodeKind(scope) == nodePin {
		scope = scope.Parent
	}
	return scope
}

// portOwners lists the classifiers whose ports the actions of b, or the
// behaviors it calls, name.
func (m *migration) portOwners(b *sysmlv1.Element) []*sysmlv1.Element {
	owners := m.namedPortOwners(b)
	for _, c := range m.calledActivities(b) {
		if ctx := m.contextOf(c); ctx != nil {
			owners = addOwner(owners, ctx.classifier)
		}
	}
	return owners
}

// namedPortOwners lists the classifiers whose ports the actions of b itself name.
func (m *migration) namedPortOwners(b *sysmlv1.Element) []*sysmlv1.Element {
	var owners []*sysmlv1.Element
	port := func(p *sysmlv1.Element) {
		if p != nil && p.Parent != nil && m.written(p) && m.written(p.Parent) {
			owners = addOwner(owners, p.Parent)
		}
	}
	m.walkActions(b, func(e *sysmlv1.Element) {
		switch e.Type {
		case "SendSignalAction", "SendObjectAction", "CallOperationAction":
			port(m.model.Ref(e, "onPort"))
		case "Trigger":
			for _, p := range m.model.Refs(e, "port") {
				port(p)
			}
		}
	})
	return owners
}

// calledActivities lists the activities the call actions of b name, once each.
func (m *migration) calledActivities(b *sysmlv1.Element) []*sysmlv1.Element {
	var called []*sysmlv1.Element
	m.walkActions(b, func(e *sysmlv1.Element) {
		if e.Type != "CallBehaviorAction" {
			return
		}
		if _, _, why := m.lanePerformer(e); why != "" {
			return
		}
		if c := m.model.Ref(e, "behavior"); c != nil && c.Type == "Activity" && !slices.Contains(called, c) {
			called = append(called, c)
		}
	})
	return called
}

// invokerOwners lists the classifiers whose behaviors run b, when a signal an
// accept of b names no port for arrives at a port of theirs: v1 hands the
// object's accepts what its ports receive, which v2 takes only via the port.
func (m *migration) invokerOwners(b *sysmlv1.Element) []*sysmlv1.Element {
	sigs := m.unportedSignals(b)
	if len(sigs) == 0 {
		return nil
	}
	var owners []*sysmlv1.Element
	for _, c := range m.runningClassifiers(b, map[*sysmlv1.Element]bool{b: true}) {
		if slices.Contains(owners, c) || !m.written(c) {
			continue
		}
		for _, sig := range sigs {
			if len(m.arrivalPorts(c, sig)) > 0 {
				owners = append(owners, c)
				break
			}
		}
	}
	return owners
}

// unportedSignals lists the signals the accept actions of b name no port for.
func (m *migration) unportedSignals(b *sysmlv1.Element) []*sysmlv1.Element {
	var sigs []*sysmlv1.Element
	m.walkActions(b, func(e *sysmlv1.Element) {
		if e.Type != "Trigger" || e.Parent == nil || e.Parent.Type != "AcceptEventAction" || len(m.model.Refs(e, "port")) > 0 {
			return
		}
		ev := m.model.Ref(e, "event")
		if ev == nil || ev.Type != "SignalEvent" {
			return
		}
		if sig := m.model.Ref(ev, "signal"); sig != nil && !slices.Contains(sigs, sig) {
			sigs = append(sigs, sig)
		}
	})
	return sigs
}

// runningClassifiers lists the classifiers whose behaviors run b: the owners of
// what invokes it, followed up through activities no classifier owns.
func (m *migration) runningClassifiers(b *sysmlv1.Element, seen map[*sysmlv1.Element]bool) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	add := func(c *sysmlv1.Element) {
		if c != nil && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	for _, e := range m.invokers[b] {
		host := enclosingActivity(e)
		switch {
		case e.Type != "CallBehaviorAction":
			if behaviorScope(e) {
				add(classifierOf(e))
			} else {
				add(e)
			}
		case host == nil:
		case classifierOf(host) != nil && m.contextOf(host) == nil:
			add(classifierOf(host))
		case classifierOf(host) != nil:
			add(m.contextOf(host).classifier)
		case seen[host]:
		default:
			seen[host] = true
			if c := m.mostSpecific(m.portOwners(host)); c != nil {
				add(c)
				continue
			}
			for _, c := range m.runningClassifiers(host, seen) {
				add(c)
			}
		}
	}
	return out
}

// invoke indexes e as an invoker of each behavior a role names without owning it.
func (m *migration) invoke(e *sysmlv1.Element, roles ...string) {
	for _, role := range roles {
		for _, b := range m.model.Refs(e, role) {
			if b.Parent != e && (isBehavior(b) || b.Type == "Operation") {
				m.invokers[b] = append(m.invokers[b], e)
			}
		}
	}
}

// walkActions visits every element of an activity's node graph, structured
// nodes included; a behavior nested in it is left to its own walk.
func (m *migration) walkActions(b *sysmlv1.Element, visit func(*sysmlv1.Element)) {
	var walk func(*sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		for _, c := range e.Children {
			if isBehavior(c) {
				continue
			}
			visit(c)
			walk(c)
		}
	}
	walk(b)
}

// mostSpecific is the classifier of the list every other one generalizes, nil
// when the list is empty or none is.
func (m *migration) mostSpecific(cs []*sysmlv1.Element) *sysmlv1.Element {
	for _, c := range cs {
		special := true
		for _, o := range cs {
			if o != c && !m.inherits(c, o) {
				special = false
				break
			}
		}
		if special {
			return c
		}
	}
	return nil
}

// contextParameter declares the reference parameter an activity takes for the
// object its actions act on, or reports why none is written. A def's own
// context parameter is written only once the body or a call settled on it.
func (m *migration) contextParameter(b *sysmlv1.Element) {
	c := m.selfContext(b)
	if c == nil {
		if note := m.contextNotes[b]; note != "" {
			m.add(b, Approximated, "", note)
		}
		return
	}
	if c.owner && !c.used && !c.bound {
		return
	}
	objectType := c.objectType()
	typing := m.typing(objectType)
	m.w.line("in ref " + writeName(c.name) + typing + m.ref(objectType, b) + "[1];")
	usageNote := ""
	if c.chain != "" {
		cat, _ := m.classify(c.classifier)
		defCat, _ := m.classify(c.declared)
		usageNote = " its context classifier is a feature of the " + defCat.keyword() + " " + qualifiedName(c.declared) + ", so the parameter is typed by that definition and reaches the " + cat.keyword() + " as " + writeName(c.name) + "." + c.chain
	} else if typing == " :> " {
		usageNote = " its context classifier is written as a usage, so the parameter specializes it rather than being typed by it"
	}
	if c.owner {
		m.add(b, Mapped, "", "acts on its owner "+qualifiedName(c.classifier)+", which it takes as its parameter "+c.name+", since its this is the def's own occurrence"+usageNote)
		return
	}
	note := "acts on a " + qualifiedName(c.classifier) + " through its ports, which it takes as its parameter " + c.name
	if owner := classifierOf(b); owner != nil {
		m.add(b, Approximated, "", note+usageNote+" rather than its owner "+qualifiedName(owner)+", which is no such object and holds no one part that is: v1 ran it on whichever object called it")
		return
	}
	m.add(b, Mapped, "", note+usageNote)
}

// bodyWithContext writes e's body after its parameters were written, declaring
// the context parameter the body's reads and the calls passing the object on
// settled: it is evaluated by the time the parameter is decided.
func (m *migration) bodyWithContext(e *sysmlv1.Element, body func()) {
	inner := m.w.captureAt(body)
	if c := m.contextOf(e); c != nil {
		c.evaluated = true
	}
	m.contextParameter(e)
	_, _ = m.w.buf().WriteString(inner)
}

// enclosingActivity is the activity whose object the nodes written for e act
// on: e itself or the one its structured node belongs to; nil for an operation's body.
func enclosingActivity(e *sysmlv1.Element) *sysmlv1.Element {
	for cur := e; cur != nil && cur.Type != "Operation"; cur = cur.Parent {
		if cur.Type == "Activity" {
			return cur
		}
	}
	return nil
}

// self writes the object the activity's actions act on: this, or the context
// parameter as it is spelled where the activity's body is written.
func (a *activity) self() string {
	if a.ctx != nil {
		return a.m.contextSpelling(a.ctx, a.act)
	}
	return "this"
}

// markSelf notes that the activity's context parameter is spelled where the
// caller is writing output, so the def declares it.
func (a *activity) markSelf() {
	if a.ctx != nil {
		a.m.markUsed(&a.ctx.used)
	}
}

// selfType is the classifier of the object the activity acts on, nil when none is known.
func (a *activity) selfType() *sysmlv1.Element {
	if a.ctx != nil {
		return a.ctx.classifier
	}
	if !isStructured(a.act) {
		return classifierOf(a.act)
	}
	act := enclosingActivity(a.act)
	if act == nil {
		return nil
	}
	if c := classifierOf(act); c != nil {
		return c
	}
	// An activity with no context acts on its own execution.
	return act
}

// hasPort reports whether port is a port of the object the activity acts on.
func (a *activity) hasPort(port *sysmlv1.Element) bool {
	t := a.selfType()
	return t != nil && a.m.written(port) && a.m.hasFeature(t, port)
}

// portPath spells port from an object of c: its own port, or a port of a view or
// viewpoint c features, through that usage's path; false when neither.
func (m *migration) portPath(c, port *sysmlv1.Element) (string, bool) {
	if c == nil || !m.written(port) {
		return "", false
	}
	if m.hasFeature(c, port) {
		return writeName(m.nameFor(port)), true
	}
	u := port.Parent
	if !m.written(u) {
		return "", false
	}
	cat, _ := m.classify(u)
	if cat != catView && cat != catViewpoint {
		return "", false
	}
	d := m.featuringDef(u)
	if d == nil || c != d && !m.inherits(c, d) {
		return "", false
	}
	return m.usageChain(u, d) + "." + writeName(m.nameFor(port)), true
}

func (a *activity) portPath(port *sysmlv1.Element) (string, bool) {
	return a.m.portPath(a.selfType(), port)
}

// on writes a member of the object obj holds as a perform or an accept names it:
// bare for a member of this, else under the object's path.
func (a *activity) on(obj, member string) string {
	if obj == "this" {
		return member
	}
	if a.ctx != nil && obj == a.self() {
		a.markSelf()
	}
	return strings.TrimPrefix(obj, "this.") + "." + member
}

// performUsage writes a call of a behavior written as an action usage of its
// owner: the call performs that usage of the one object of the owner in
// reach; when there is none, an empty step stands where the call was.
func (a *activity) performUsage(name string, b *sysmlv1.Element) string {
	owner := b.Parent
	obj, _, why := a.m.objectOf(owner, a.selfType(), a.self())
	if obj == "" {
		a.m.w.line(actionKw + name + ";")
		return a.m.nameOf(b) + " is an action of " + qualifiedName(owner) + ", performed on an object of it; " + why + ", so an empty step stands for the call"
	}
	usage := a.on(obj, writeName(a.m.nameOf(b)))
	a.m.w.line("perform action " + name + " ::> " + a.unshadowed(usage) + ";")
	if obj == a.self() {
		return ""
	}
	return "performed on " + obj + why + ", as its usage " + usage
}

// unshadowed writes the path to a feature of the object the activity acts on so
// that a member of the body named like its first step does not capture it: the
// step is qualified by the object's type when a member of the body bears its name.
func (a *activity) unshadowed(path string) string {
	head, rest, name := path, "", path
	if strings.HasPrefix(path, "'") {
		if end := strings.Index(path[1:], "'"); end >= 0 {
			head, name = path[:end+2], path[1:end+1]
			rest = strings.TrimPrefix(path[end+2:], ".")
		}
	} else {
		head, rest, _ = strings.Cut(path, ".")
		name = head
	}
	shadowed := a.used[name] || a.m.taken[a.def][name]
	for _, n := range a.nodes {
		shadowed = shadowed || a.m.nameOf(n) == name
	}
	if !shadowed || a.ctx != nil && name == a.ctx.name {
		return path
	}
	if t := a.selfType(); t != nil {
		head = a.m.ref(t, a.act) + "::" + head
	}
	if rest == "" {
		return head
	}
	return head + "." + rest
}

// viaPrefix is what an accept's via path starts with to name a port of the object
// the activity acts on: nothing for this, whose ports the action's scope sees.
func (a *activity) viaPrefix() string {
	if a.ctx != nil {
		return a.m.contextSpelling(a.ctx, a.act) + "."
	}
	return ""
}

// contextArgument writes the object bound to a called behavior's context
// parameter: the caller's object — its context parameter qualified from the
// def — when it is one, else its one part that is.
func (a *activity) contextArgument(c *behaviorContext) (expr, note string) {
	self := "this"
	selfType := a.selfType()
	if a.ctx != nil {
		self = a.m.contextName(a.ctx, a.def)
		selfType = a.ctx.objectType()
	}
	expr, note = a.m.contextBinding(c, selfType, self, a.def)
	if expr != "" && a.ctx != nil {
		a.m.markUsed(&a.ctx.used)
	}
	return expr, note
}

// callContext is contextArgument for the call behavior action n: the object its
// swimlane names as the performer when there is one, else the caller's.
func (a *activity) callContext(n *sysmlv1.Element, c *behaviorContext) (expr, note string) {
	if l, _, _ := a.m.lanePerformer(n); l != nil {
		return a.m.contextBinding(c, l.typ, a.m.callBodyExpr(a.m.respellThis(l.expr, a.def), a.def), a.def)
	}
	return a.contextArgument(c)
}

// contextBinding writes the object bound to a run behavior's context parameter,
// where the runner's object is a self of type selfType: that object when it is
// one, else its one part that is, spelled for the body of scope when there is one.
func (m *migration) contextBinding(c *behaviorContext, selfType *sysmlv1.Element, self string, scope *sysmlv1.Element) (expr, note string) {
	kind := qualifiedName(c.classifier)
	expr, _, why := m.objectOf(c.objectType(), selfType, self)
	if expr == "" {
		return "", actsOn + kind + throughParam + c.name + ", which is left unbound: " + why
	}
	if scope != nil && strings.HasPrefix(expr, "this.") {
		expr = m.callBodyExpr(m.respellThis(expr, scope), scope)
	}
	return expr, actsOn + kind + throughParam + c.name + ", which is bound to " + expr + why
}

// objectOf finds, from an activity acting on self of selfType, the one object
// of classifier c in reach: self itself, or self's one part that is a c and
// holds one object, which part is then returned too. A part holding a
// collection, or a number of objects the bounds do not tell, is no one object;
// when it is the caller's only part that is a c it is still returned, with no
// expr, so the caller can say what it holds. The last result completes a
// sentence: how the object was found, or why none was.
func (m *migration) objectOf(c, selfType *sysmlv1.Element, self string) (expr string, part *sysmlv1.Element, why string) {
	kind := qualifiedName(c)
	switch {
	case selfType == nil:
		return "", nil, "the caller acts on no object"
	case selfType == c || m.inherits(selfType, c):
		return self, nil, ""
	}
	if c != nil {
		cat, _ := m.classify(c)
		if cat == catView || cat == catViewpoint {
			if d := m.featuringDef(c); d != nil && (selfType == d || m.inherits(selfType, d)) {
				return self + "." + m.usageChain(c, d), nil, ""
			}
		}
	}
	var parts, collections []*sysmlv1.Element
	for _, f := range m.attributesOf(selfType) {
		if f.Type != "Property" || !m.written(f) {
			continue
		}
		if t := m.model.Ref(f, "type"); t != nil && (t == c || m.inherits(t, c)) {
			if manyValued(f) || unreadableBounds(f) {
				collections = append(collections, f)
			} else {
				parts = append(parts, f)
			}
		}
	}
	switch {
	case len(parts) == 1:
		return self + "." + writeName(m.nameFor(parts[0])), parts[0], ", the caller's one part that is one"
	case len(parts) > 1:
		why = "has " + strconv.Itoa(len(parts)) + " parts that are one, so no one of them is chosen"
	case len(collections) > 0:
		names := make([]string, len(collections))
		for i, f := range collections {
			names[i] = writeName(m.nameFor(f))
		}
		why = "holds them only as the collection " + strings.Join(names, " and ") + ", no one object of which is chosen"
		if len(collections) == 1 {
			part = collections[0]
		}
	default:
		why = "has no part that is one"
	}
	return "", part, "the caller is a " + qualifiedName(selfType) + ", which is no " + kind + " and " + why
}
