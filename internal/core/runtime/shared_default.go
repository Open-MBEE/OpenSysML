package runtime

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A `=` value derived on an occurrence from nothing but what its declarations
// materialize is a property of the occurrence's shape, not of the occurrence: the
// context keeps it in a side table keyed by shape and feature, and every other
// occurrence of the shape whose reads are still as declared takes it from there
// instead of materializing what it would read. The feature value slot stays, so a
// value taken this way reads exactly as one derived in place, and is invalidated
// as one: what the derivation read is listed as read on the taking occurrence.

// SharedDefaultsEnvVar switches the sharing of derived defaults between
// occurrences of one shape off when set to 0 (or false/off/no).
const SharedDefaultsEnvVar = "OPENSYSML_SHARED_DEFAULTS"

// SharedDefaultsFromEnv reports whether the environment leaves derived-default
// sharing on, which it does unless SharedDefaultsEnvVar switches it off.
func SharedDefaultsFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(envvar.Lookup(SharedDefaultsEnvVar))) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// SetSharedDefaults turns derived-default sharing between occurrences on or off.
func (ctx *Context) SetSharedDefaults(on bool) { ctx.shareDefaults = on }

// SharedDefaults reports whether this context shares derived defaults between occurrences.
func (ctx *Context) SharedDefaults() bool { return ctx.shareDefaults }

// SharedDefaultsTaken counts the derived values occurrences took from the shared
// table rather than deriving; a measure of what the sharing saved.
func (ctx *Context) SharedDefaultsTaken() int64 { return ctx.sharedTaken }

// shapeNode is one interned occurrence shape: the type of an object and, for one
// held by another, the declaration of the feature holding it; a classifier the
// object has since been given is a node of its own over the shape it classifies.
// What the holder is otherwise does not reach the object's declared values: a read
// outside the object leaves a derivation unshared, and a binding declared over it
// is looked for on every occurrence taking a value.
type shapeNode struct {
	outer      *shapeNode
	feature    *symbols.Symbol
	typ        *symbols.Symbol
	classifier bool
}

// sharedKey names a derived default of a shape.
type sharedKey struct {
	shape   *shapeNode
	feature *EffectiveFeature
}

// sharedDefault is a value derived from declared reads alone, with the paths of
// those reads from the occurrence it was derived on, in the order they were read.
type sharedDefault struct {
	value Value
	paths [][]string
}

// sharedRead is one feature value a derivation read, as a path from the object it was
// read on: the feature's name, or what a shared value of that object read in turn. A
// check may read a value not as declared; then the read carries the value as an input.
type sharedRead struct {
	inst   *Instance
	path   []string
	value  Value
	valued bool
}

// sharedInput is a value a check read that is not as declared, as a path from the
// object checked: another object of the shape takes the verdict when it reads the same.
type sharedInput struct {
	path  []string
	value Value
}

// derivationTrace observes one derivation of a `=` value on inst, or one check on
// inst: clean while every read so far was of a declared value within inst and nothing
// else happened, save a check classifying inst by a type it already conforms to.
type derivationTrace struct {
	inst       *Instance
	fv         *FeatureValue
	attached   int64
	clean      bool
	reads      []sharedRead
	classified []*symbols.Symbol
}

// owedDefault is a value an occurrence took from the shared table before
// materializing all the derivation read: what it still owes materializing.
type owedDefault struct {
	fv     *FeatureValue
	shared *sharedDefault
}

// shapeOf interns the shape of inst, nil for an object materialized from no type or
// held by a feature declared by none: its declarations follow from its type, its
// classifiers and the declaration of the feature holding it.
func (ctx *Context) shapeOf(inst *Instance) *shapeNode {
	if inst.Type == nil {
		return nil
	}
	var feature *symbols.Symbol
	if owner, name := inst.Owner(); owner != nil {
		held, ok := owner.FeatureValues[name]
		if !ok || held.Feature.Symbol == nil {
			return nil
		}
		feature = held.Feature.Symbol
	}
	shape := ctx.internShape(shapeNode{feature: feature, typ: inst.Type})
	for _, classifier := range inst.classifiers {
		shape = ctx.internShape(shapeNode{outer: shape, typ: classifier, classifier: true})
	}
	return shape
}

// internShape returns the one node standing for node.
func (ctx *Context) internShape(node shapeNode) *shapeNode {
	if interned, ok := ctx.shapes[node]; ok {
		return interned
	}
	interned := &node
	ctx.shapes[node] = interned
	return interned
}

// declared reports whether the feature value holds what its declarations alone
// materialize: neither written, bound, assumed nor otherwise made up.
func (s *FeatureValue) declared() bool {
	return s.Materialized && s.intrinsic && !s.Written && !s.BindingDerived && !s.Assumed
}

// subsettersDeclared reports whether every feature of inst subsetting the named one
// holds a declared value, so what they contribute to it follows from declarations alone.
func (ctx *Context) subsettersDeclared(inst *Instance, name string) bool {
	for _, feat := range ctx.subsettingFeaturesOf(inst, name) {
		if fv, ok := inst.FeatureValues[feat.Name]; !ok || !fv.declared() {
			return false
		}
	}
	return true
}

// shareable reports whether a derived value may be held by every occurrence of the
// shape: a scalar whose payload no occurrence can change, and that names no object.
func shareable(val Value) bool {
	switch val.Kind {
	case ValConst, ValNull, ValString, ValQuantity, ValEnumLiteral, ValComplex:
		return true
	}
	return false
}

// sharesDefault reports whether fv's derived default on inst is one its shape may
// hold: sharing is on, the feature holds one value its subsetters do not populate,
// no binding or write determines it, and no behavior run makes the reads its own.
func (ctx *Context) sharesDefault(inst *Instance, fv *FeatureValue) bool {
	return ctx.shareDefaults && fv.Feature.Scalar() && !fv.Written && !fv.BindingDerived &&
		ctx.behaviorRunDepth == 0 && !ctx.defaultYieldsToSubsetters(inst, fv.Feature) &&
		ctx.shapeOf(inst) != nil
}

// beginTrace opens the observation of fv's derivation on inst.
func (ctx *Context) beginTrace(inst *Instance, fv *FeatureValue) int {
	return ctx.beginTraceOn(inst, fv, ctx.sharesDefault(inst, fv))
}

// beginTraceOn opens the observation of an evaluation on inst — fv's derivation, or
// a check with no feature value of its own — clean only when it may be shared at all.
func (ctx *Context) beginTraceOn(inst *Instance, fv *FeatureValue, clean bool) int {
	ctx.tracing = append(ctx.tracing, derivationTrace{
		inst: inst, fv: fv, clean: clean, attached: ctx.behaviorsAttached,
	})
	return len(ctx.tracing) - 1
}

// endTrace closes the observation opened at top, reporting the derivation clean when
// every read was declared and no behavior was attached under it.
func (ctx *Context) endTrace(top int) (clean bool, reads []sharedRead) {
	clean, reads, _ = ctx.endTraceClassified(top)
	return clean, reads
}

// endTraceClassified is endTrace also reporting the classifiers the evaluation gave
// the object it was on.
func (ctx *Context) endTraceClassified(top int) (clean bool, reads []sharedRead, classified []*symbols.Symbol) {
	t := ctx.tracing[top]
	ctx.tracing = ctx.tracing[:top]
	if !t.clean || ctx.behaviorsAttached != t.attached {
		return false, nil, nil
	}
	return true, t.reads, t.classified
}

// observeClassify notes, for every evaluation being observed, that inst is being
// classified by typ: a check classifying its own object is on record to classify
// every object it stands for alike; any other classification makes the evaluation
// the occurrence's own.
func (ctx *Context) observeClassify(inst *Instance, typ *symbols.Symbol) {
	for i := range ctx.tracing {
		t := &ctx.tracing[i]
		if !t.clean {
			continue
		}
		if t.fv != nil || t.inst != inst {
			t.clean = false
			continue
		}
		t.classified = append(t.classified, typ)
	}
}

// unshareTraces makes every derivation being observed its occurrence's own: what
// changed under it is not what every occurrence of the shape has.
func (ctx *Context) unshareTraces() {
	for i := range ctx.tracing {
		ctx.tracing[i].clean = false
	}
}

// observeRead notes, for every derivation being observed, the read of fv held by
// inst: a read outside the derivation's occurrence, or of a value not as declared,
// makes the derivation the occurrence's own — save that a check reading a scalar
// not as declared takes it as an input. A derived value read stands for what it read
// in turn, which its shape's record lists; one with no record is not shared.
func (ctx *Context) observeRead(inst *Instance, fv *FeatureValue) {
	for i := range ctx.tracing {
		t := &ctx.tracing[i]
		if !t.clean || t.fv == fv {
			continue
		}
		if !ctx.declaredWithin(inst, t.inst) {
			t.clean = false
			continue
		}
		if !fv.declared() {
			if t.fv != nil || !fv.Materialized || !shareable(fv.Value) || !ctx.singlyWithin(inst, t.inst) {
				t.clean = false
				continue
			}
			t.reads = append(t.reads, sharedRead{inst: inst, path: []string{fv.Feature.Name}, value: fv.Value, valued: true})
			continue
		}
		t.reads = append(t.reads, sharedRead{inst: inst, path: []string{fv.Feature.Name}})
		if len(fv.reads) == 0 {
			continue
		}
		shared, ok := ctx.sharedRecordOf(inst, fv)
		if !ok {
			t.clean = false
			continue
		}
		for _, path := range shared.paths {
			t.reads = append(t.reads, sharedRead{inst: inst, path: path})
		}
	}
}

// sharedRecordOf finds what the derivation of fv, held declared by inst, read: the
// record of inst's shape, or of the shape inst had before a classifier that left the
// value standing, which is what it would still derive.
func (ctx *Context) sharedRecordOf(inst *Instance, fv *FeatureValue) (*sharedDefault, bool) {
	for shape := ctx.shapeOf(inst); shape != nil; shape = shape.outer {
		if shared, ok := ctx.sharedDefaults[sharedKey{shape: shape, feature: fv.Feature}]; ok {
			return shared, true
		}
		if !shape.classifier {
			break
		}
	}
	return nil, false
}

// declaredWithin reports whether inst is root itself or an object root's declarations
// materialize under it: held, at every step, by a declared composite value of an
// unclassified object.
func (ctx *Context) declaredWithin(inst, root *Instance) bool {
	for inst != root {
		if len(inst.classifiers) != 0 {
			return false
		}
		owner, feature := inst.Owner()
		if owner == nil {
			return false
		}
		if held, ok := owner.FeatureValues[feature]; !ok || !held.declared() {
			return false
		}
		inst = owner
	}
	return true
}

// singlyWithin reports whether inst is root itself or held under it by single-valued
// features at every step, so a path of feature names from root names inst alone.
func (ctx *Context) singlyWithin(inst, root *Instance) bool {
	for inst != root {
		owner, feature := inst.Owner()
		if owner == nil {
			return false
		}
		if held, ok := owner.FeatureValues[feature]; !ok || !held.Feature.Scalar() {
			return false
		}
		inst = owner
	}
	return true
}

// sharedPaths turns the reads of a clean derivation on root into paths from root, in
// the order they were read — those of declared values, and those taken as inputs
// with the value read; nil when a binding anywhere on the chain to a read could
// determine it differently on another occurrence.
func (ctx *Context) sharedPaths(root *Instance, reads []sharedRead) (paths [][]string, inputs []sharedInput) {
	seen := make(map[string]bool, len(reads))
	paths = make([][]string, 0, len(reads))
	for _, read := range reads {
		var above []string
		for inst := read.inst; inst != root; {
			owner, feature := inst.Owner()
			above = append(above, feature)
			inst = owner
		}
		path := make([]string, 0, len(above)+len(read.path))
		for i := len(above) - 1; i >= 0; i-- {
			path = append(path, above[i])
		}
		path = append(path, read.path...)
		key := strings.Join(path, ".")
		if seen[key] {
			continue
		}
		seen[key] = true
		if ctx.bindingDeclaredFor(read.inst, strings.Join(read.path, ".")) {
			return nil, nil
		}
		if read.valued {
			inputs = append(inputs, sharedInput{path: path, value: read.value})
		} else {
			paths = append(paths, path)
		}
	}
	return paths, inputs
}

// bindingDeclaredFor reports whether any type on the chain holding inst declares a
// binding for the named feature, as resolveBindings would find one.
func (ctx *Context) bindingDeclaredFor(inst *Instance, name string) bool {
	path := name
	for current := inst; current != nil; {
		if len(ctx.bindingsOf(current, path)) != 0 {
			return true
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" {
			return false
		}
		path = ownerFeature + "." + path
		current = owner
	}
	return false
}

// shareDerived records val as the derived default of fv's feature for inst's shape,
// when the derivation was clean and the value can be held by every occurrence.
func (ctx *Context) shareDerived(inst *Instance, fv *FeatureValue, val Value, clean bool, reads []sharedRead) {
	if !clean || !shareable(val) || !ctx.sharesDefault(inst, fv) {
		return
	}
	paths, inputs := ctx.sharedPaths(inst, reads)
	if paths == nil || len(inputs) != 0 {
		return
	}
	key := sharedKey{shape: ctx.shapeOf(inst), feature: fv.Feature}
	if prior, ok := ctx.sharedDefaults[key]; ok {
		ctx.noteProbeUndo(func() { ctx.sharedDefaults[key] = prior })
	} else {
		ctx.noteProbeUndo(func() { delete(ctx.sharedDefaults, key) })
	}
	ctx.sharedDefaults[key] = &sharedDefault{value: val, paths: paths}
}

// takeShared holds on fv, unmaterialized on inst, the value its shape derived for the
// feature, if one is on record and every value that derivation read is, on inst, still
// as declared or not yet materialized. What it would have read is listed as read, and
// what it did not materialize on the way is owed (see settleOwed).
func (ctx *Context) takeShared(inst *Instance, fv *FeatureValue) bool {
	if !ctx.sharesDefault(inst, fv) {
		return false
	}
	shared, ok := ctx.sharedDefaults[sharedKey{shape: ctx.shapeOf(inst), feature: fv.Feature}]
	if !ok {
		return false
	}
	var sources []*FeatureValue
	owes := false
	for _, path := range shared.paths {
		if ctx.bindingDeclaredFor(inst, strings.Join(path, ".")) {
			return false
		}
		var eligible bool
		if sources, eligible = ctx.declaredAlong(inst, path, sources); !eligible {
			return false
		}
		owes = owes || !sources[len(sources)-1].Materialized
	}
	ctx.noteProbeWrite(fv)
	if ctx.derivable(fv) {
		ctx.forgetReads(fv)
		for _, src := range sources {
			if src != fv {
				ctx.listRead(src, fv)
			}
		}
	}
	fv.Value = shared.value
	fv.Materialized, fv.intrinsic = true, true
	if owes {
		inst.owe(ctx, fv, shared)
	}
	ctx.sharedTaken++
	return true
}

// declaredAlong follows path from inst, appending to sources every feature value on
// the way; eligible while each is as declared, stopping at one not yet materialized.
func (ctx *Context) declaredAlong(inst *Instance, path []string, sources []*FeatureValue) ([]*FeatureValue, bool) {
	fv, ok := inst.FeatureValues[path[0]]
	if !ok {
		return sources, false
	}
	sources = append(sources, fv)
	if !fv.Materialized {
		return sources, true
	}
	if !fv.declared() {
		return sources, false
	}
	if len(path) == 1 {
		return sources, true
	}
	for _, held := range elementsOf(fv.HeldValue()) {
		child, ok := ctx.instances[held.Instance]
		if held.Kind != ValInstance || !ok || len(child.classifiers) != 0 {
			return sources, false
		}
		if sources, ok = ctx.declaredAlong(child, path[1:], sources); !ok {
			return sources, false
		}
	}
	return sources, true
}

// owe records that fv took shared before materializing all its derivation read;
// the journal under way forgets it with the take.
func (inst *Instance) owe(ctx *Context, fv *FeatureValue, shared *sharedDefault) {
	for i := range inst.owed {
		if inst.owed[i].fv == fv {
			prior := inst.owed[i].shared
			ctx.noteProbeUndo(func() { inst.owed[i].shared = prior })
			inst.owed[i].shared = shared
			return
		}
	}
	n := len(inst.owed)
	ctx.noteProbeUndo(func() { inst.owed = inst.owed[:n] })
	inst.owed = append(inst.owed, owedDefault{fv: fv, shared: shared})
}

// settleOwed materializes, on inst and each object holding it, what the values they
// took shared would have materialized to be derived in place: an object about to
// be classified reads as one whose values were all derived on it.
func (ctx *Context) settleOwed(inst *Instance) error {
	for ; inst != nil; inst, _ = inst.Owner() {
		owed, at := inst.owed, inst
		if len(owed) == 0 {
			continue
		}
		ctx.noteProbeUndo(func() { at.owed = owed })
		at.owed = nil
		for _, o := range owed {
			if !o.fv.declared() {
				continue
			}
			for _, path := range o.shared.paths {
				if err := ctx.materializeAlong(at, path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// materializeAlong reads every feature value on path from inst.
func (ctx *Context) materializeAlong(inst *Instance, path []string) error {
	fv, err := inst.GetFeatureValue(ctx, path[0])
	if err != nil {
		return err
	}
	if len(path) == 1 {
		return nil
	}
	for _, held := range elementsOf(fv.HeldValue()) {
		if child, ok := ctx.instances[held.Instance]; held.Kind == ValInstance && ok {
			if err := ctx.materializeAlong(child, path[1:]); err != nil {
				return err
			}
		}
	}
	return nil
}
