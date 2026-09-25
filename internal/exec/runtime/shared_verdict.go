package runtime

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A check on an object whose conditions read only declared values decides the
// same for every object of its shape those values are still as declared on; one
// reading values not as declared decides the same for every object reading the same.
// While verdicts are shared, the first check on each distinct input is on record and
// the others take it.

// ShareVerdicts opens a span over which checks on objects of one shape share their
// verdicts; the function returned closes it. A span opened within another
// continues it.
func (ctx *Context) ShareVerdicts() (done func()) {
	if ctx.verdicts != nil {
		return func() {}
	}
	ctx.verdicts = &verdictMemo{verdicts: make(map[verdictKey][]*sharedVerdict)}
	return func() { ctx.verdicts = nil }
}

// SharedVerdictsTaken counts the verdicts taken from the open span rather than
// decided by evaluating; zero outside a span.
func (ctx *Context) SharedVerdictsTaken() int {
	if ctx.verdicts == nil {
		return 0
	}
	return ctx.verdicts.taken
}

// verdictKey names one element checked one way on one shape: a requirement checked
// directly and a satisfaction of it, which binds its subject, are two checks.
type verdictKey struct {
	element *symbols.Symbol
	kind    string
	shape   *shapeNode
}

// sharedVerdict is what a check decided about the first object of a shape reading
// its inputs, with the paths of the declared values it read from that object, the
// inputs it read not as declared, and the classifiers it gave it.
type sharedVerdict struct {
	inst       *Instance
	result     CheckResult
	err        error
	paths      [][]string
	inputs     []sharedInput
	classified []*symbols.Symbol
}

// verdictMemo holds the verdicts shared over one span, one per distinct input of
// each element checked on each shape.
type verdictMemo struct {
	verdicts map[verdictKey][]*sharedVerdict
	taken    int
}

// checkOn resolves the object a check of carrying is about, then answers check on
// it: shared across self's shape when self itself is that object, since finding it
// walks structure no verdict depends on, else evaluated on the object resolved to.
func (ctx *Context) checkOn(element *symbols.Symbol, kind, name string, carrying *symbols.Symbol, self *Instance, check func(carrier) (CheckResult, error)) (CheckResult, error) {
	resolved, err := ctx.checkSubject(kind, name, carrying, self)
	if err != nil {
		return CheckResult{}, err
	}
	if resolved.instance != self {
		return check(resolved)
	}
	return ctx.checkShared(element, kind, name, self, func() (CheckResult, error) { return check(resolved) })
}

// checkShared answers check on self: from the open span when a verdict of element on
// self's shape is on record whose declared reads are as declared on self and whose
// inputs self reads the same, else by evaluating it, which records the verdict when
// it may stand for the shape. kind is how element is checked; name is how it is named
// in self's messages.
func (ctx *Context) checkShared(element *symbols.Symbol, kind, name string, self *Instance, check func() (CheckResult, error)) (CheckResult, error) {
	memo := ctx.verdicts
	if memo == nil || !ctx.sharing() || element == nil || self == nil {
		return check()
	}
	shape := ctx.shapeOf(self)
	if shape == nil {
		return check()
	}
	key := verdictKey{element: element, kind: kind, shape: shape}
	for _, shared := range memo.verdicts[key] {
		if ctx.declaredAlongAll(self, shared.paths) && ctx.readsInputs(self, shared.inputs) && ctx.classifyAs(self, shared.classified) {
			memo.taken++
			return shared.on(self, name)
		}
	}
	top := ctx.beginTraceOn(self, nil, ctx.behaviorRunDepth == 0)
	result, err := check()
	clean, reads, classified := ctx.endTraceClassified(top)
	if !clean || !fansOut(result, err, self) {
		return result, err
	}
	if paths, inputs := ctx.sharedPaths(self, reads); paths != nil {
		memo.verdicts[key] = append(memo.verdicts[key], &sharedVerdict{
			inst: self, result: result, err: err, paths: paths, inputs: inputs, classified: classified,
		})
	}
	return result, err
}

// readsInputs reports whether inst reads, at the path of each input, the value the
// shared check read: the same kind of value, equal, expressed the same way.
func (ctx *Context) readsInputs(inst *Instance, inputs []sharedInput) bool {
	for _, input := range inputs {
		if ctx.bindingDeclaredFor(inst, strings.Join(input.path, ".")) {
			return false
		}
		val, ok := ctx.readAlong(inst, input.path)
		if !ok || !sameInput(val, input.value) {
			return false
		}
	}
	return true
}

// readAlong reads the value at path from inst, through single objects held on the way;
// false when the path leads through a collection, an object with classifiers, or fails.
func (ctx *Context) readAlong(inst *Instance, path []string) (Value, bool) {
	for len(path) > 1 {
		fv, err := inst.GetFeatureValue(ctx, path[0])
		if err != nil || !fv.Feature.Scalar() {
			return Value{}, false
		}
		held := fv.HeldValue()
		child, ok := ctx.instances[held.Instance]
		if held.Kind != ValInstance || !ok || len(child.classifiers) != 0 {
			return Value{}, false
		}
		inst, path = child, path[1:]
	}
	fv, err := inst.GetFeatureValue(ctx, path[0])
	if err != nil {
		return Value{}, false
	}
	return fv.Value, true
}

// sameInput reports whether two values a check read are indistinguishable to it:
// one kind, equal, and for a number or quantity carried and expressed the same way.
func sameInput(a, b Value) bool {
	if a.Kind != b.Kind || !valueEqual(a, b) {
		return false
	}
	switch a.Kind {
	case ValConst:
		return a.Const.Kind == b.Const.Kind
	case ValQuantity:
		return a.Quantity().Unit.Text == b.Quantity().Unit.Text
	}
	return true
}

// classifyAs gives inst the classifiers a shared check gave the object it was decided
// on, as one transaction; false, with inst as it was, when one is refused.
func (ctx *Context) classifyAs(inst *Instance, classified []*symbols.Symbol) bool {
	if len(classified) == 0 {
		return true
	}
	commit, rollback := ctx.beginJournal()
	for _, typ := range classified {
		if err := ctx.classify(inst, typ); err != nil {
			rollback()
			return false
		}
	}
	commit()
	return true
}

// on restates the verdict about inst, named name in its message.
func (v *sharedVerdict) on(inst *Instance, name string) (CheckResult, error) {
	result := v.result
	if result.Subject == v.inst {
		result.Subject = inst
	}
	if result.SubjectRoot == v.inst {
		result.SubjectRoot = inst
	}
	err := v.err
	if violation, ok := err.(*ViolationError); ok {
		restated := *violation
		restated.Element = name
		err = &restated
	}
	return result, err
}

// fansOut reports whether a verdict about inst is one every object of its shape may
// take: it resolved to inst itself or to no object, and its error, if any, states
// only the condition violated.
func fansOut(result CheckResult, err error, inst *Instance) bool {
	if result.Subject != nil && result.Subject != inst {
		return false
	}
	if err == nil {
		return true
	}
	_, violation := err.(*ViolationError)
	return violation
}

// declaredAlongAll reports whether every path from inst leads through values as
// declared, or not yet materialized, with no binding declared over any of them.
func (ctx *Context) declaredAlongAll(inst *Instance, paths [][]string) bool {
	for _, path := range paths {
		if ctx.bindingDeclaredFor(inst, strings.Join(path, ".")) {
			return false
		}
		if _, eligible := ctx.declaredAlong(inst, path, nil); !eligible {
			return false
		}
	}
	return true
}

// sharedElement is the element a satisfaction assertion's verdict is shared under:
// the requirement it references when the assertion states nothing of its own, so
// two assertions of it about objects of one shape share; else the assertion.
func sharedElement(a *SatisfyAssertion) *symbols.Symbol {
	if a.Requirement != nil && !a.Negated && len(unwrappedDeclMembers(a.Symbol.Decl)) == 0 {
		return a.Requirement
	}
	return a.Symbol
}
