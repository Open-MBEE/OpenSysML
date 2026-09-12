package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// classifierBehaviorDecl is a behavior a type binds to its objects, paired with
// the member symbol that binds it.
type classifierBehaviorDecl struct {
	behavior lower.ClassifierBehavior
	member   *symbols.Symbol
}

// ObjectBehavior is a behavior one object runs because its type exhibits or
// performs it: an execution of its own, bound to that object's identity.
type ObjectBehavior struct {
	// Name is the name the behavior answers to on the object.
	Name string
	Kind lower.ClassifierBehaviorKind
	// Symbol is the state machine or action holding the body being run, which is
	// the binding declaration itself when it states one.
	Symbol *symbols.Symbol
	// Object is the object performing the behavior.
	Object *Instance
	// member is the declaration binding the behavior to the object's type, which
	// tells two behaviors apart even when neither is named.
	member *symbols.Symbol
	// bindings is the chain from member to Symbol: each element names the next,
	// so a machine addressed by any of them is this one.
	bindings []*symbols.Symbol
	// kinds are the types the bindings conform to, so a machine stating its own
	// body under `exhibit state m : M { ... }` is still a machine of kind M.
	kinds []*symbols.Symbol
	// binding is member's position among the type's behavior bindings, which
	// outlives the symbols and so tells the behavior a restart puts in its place.
	binding int
	// State is the machine the object exhibits, nil for a performed action.
	State *StateExecutor
	// Action is the action the object performs, nil for an exhibited machine.
	Action *ActionExecutor
}

// Describe names the behavior and the object running it, for diagnostics.
func (b *ObjectBehavior) Describe() string {
	name := b.Name
	if name == "" {
		name = symbolText(b.Symbol)
	}
	return fmt.Sprintf("%s %s of object #%d", b.Kind, name, b.Object.ID)
}

// forgetBehaviorWrites drops what a run wrote, so a restarted behavior reads the
// object's declared initial values instead of what the discarded run left.
func (inst *Instance) forgetBehaviorWrites(ctx *Context) {
	for _, fv := range inst.FeatureValues {
		if !fv.Written {
			continue
		}
		fv.Value, fv.Values = Value{}, Value{}
		fv.Materialized, fv.Written = false, false
		ctx.invalidateDependents(fv)
	}
}

// Behaviors are the behaviors the object runs, in declaration order.
func (inst *Instance) Behaviors() []*ObjectBehavior {
	return inst.behaviors
}

// Behavior returns the behavior of the given name the object runs. An unnamed
// behavior answers to no name and so is never returned.
func (inst *Instance) Behavior(name string) (*ObjectBehavior, bool) {
	if name == "" {
		return nil, false
	}
	for _, b := range inst.behaviors {
		if b.Name == name {
			return b, true
		}
	}
	return nil, false
}

// BehaviorNamed returns the behavior the object runs under the given name or
// under another name of the same feature: a redefinition renames the behavior
// it redefines (KerML 1.0 §7.3.4.5), so both names denote one execution. The
// renaming may come from any type of the object, a classifier included.
func (ctx *Context) BehaviorNamed(inst *Instance, name string) (*ObjectBehavior, bool) {
	if b, ok := inst.Behavior(name); ok {
		return b, true
	}
	if name == "" {
		return nil, false
	}
	for _, typ := range inst.types() {
		for _, b := range inst.behaviors {
			if b.member != nil && slices.Contains(ctx.redefinedNames(b.member, typ), name) {
				return b, true
			}
		}
		for _, feat := range ctx.FeaturesOf(typ) {
			if feat.Name != name || feat.Symbol == nil {
				continue
			}
			for _, redefined := range ctx.redefinedNames(feat.Symbol, typ) {
				if b, ok := inst.Behavior(redefined); ok {
					return b, true
				}
			}
		}
	}
	return nil, false
}

// runsBound reports whether the object already runs the behavior a member of typ binds,
// under that member or one redefinition makes the same feature: a start reached twice, or
// a classifier renaming a running behavior, attaches nothing.
func (ctx *Context) runsBound(inst *Instance, member, typ *symbols.Symbol) bool {
	for _, b := range inst.behaviors {
		if b.member == member {
			return true
		}
		if b.member == nil || member == nil {
			continue
		}
		if slices.Contains(ctx.redefinedFeatures(member, typ), b.member) ||
			slices.Contains(ctx.redefinedFeatures(b.member, typ), member) {
			return true
		}
	}
	return false
}

// ExhibitedState returns the machine the object exhibits, and false when it
// exhibits none. With several, it returns the first declared.
func (inst *Instance) ExhibitedState() (*ObjectBehavior, bool) {
	for _, b := range inst.behaviors {
		if b.Kind == lower.ExhibitedState {
			return b, true
		}
	}
	return nil, false
}

// ExhibitedStatesOf returns the machines the object exhibits under sym's declaration:
// the one sym itself binds, or else every one reaching sym through its bindings
// (the usage it names, or the definition holding its body) or typed by it, since
// one definition can be the body or the kind of several exhibited usages.
// Declarations are compared.
func (inst *Instance) ExhibitedStatesOf(sym *symbols.Symbol) []*ObjectBehavior {
	if sym == nil || sym.Decl == nil {
		return nil
	}
	var bodies []*ObjectBehavior
	for _, b := range inst.behaviors {
		if b.Kind != lower.ExhibitedState {
			continue
		}
		if b.member != nil && b.member.Decl == sym.Decl {
			return []*ObjectBehavior{b}
		}
		if (len(b.bindings) > 1 && declaresAny(b.bindings[1:], sym)) || declaresAny(b.kinds, sym) {
			bodies = append(bodies, b)
		}
	}
	return bodies
}

// ExhibitsState reports whether member is an exhibit declaration whose objects
// run sym's machine: member itself, anything its bindings reach, or a type they
// conform to. Declarations are compared.
func (ctx *Context) ExhibitsState(member, sym *symbols.Symbol) bool {
	if member == nil || member.Decl == nil || sym == nil || sym.Decl == nil {
		return false
	}
	behavior, ok := lower.ClassifierBehaviorOf(member.Decl)
	if !ok || behavior.Kind != lower.ExhibitedState {
		return false
	}
	chain, err := ctx.classifierBehaviorChain(classifierBehaviorDecl{behavior: behavior, member: member})
	return err == nil && (declaresAny(chain, sym) || declaresAny(ctx.behaviorKinds(chain), sym))
}

// behaviorKinds collects the types the bindings of a behavior conform to, in
// chain order, so a machine is addressable by the definition it is typed by
// even when a usage on the way states the body itself.
func (ctx *Context) behaviorKinds(chain []*symbols.Symbol) []*symbols.Symbol {
	var kinds []*symbols.Symbol
	for _, sym := range chain {
		for _, sup := range ctx.model.semantics.AllSupertypes(sym) {
			if !slices.Contains(chain, sup) && !slices.Contains(kinds, sup) {
				kinds = append(kinds, sup)
			}
		}
	}
	return kinds
}

// declaresAny reports whether one of syms declares what sym declares.
func declaresAny(syms []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, s := range syms {
		if s != nil && s.Decl == sym.Decl {
			return true
		}
	}
	return false
}

// Member is the declaration binding the behavior to the object's type: the
// exhibiting or performing usage, which is what addresses this behavior
// when several run the same body.
func (b *ObjectBehavior) Member() *symbols.Symbol {
	return b.member
}

// classifierBehaviorsOf reports the behaviors every object of a type runs:
// those its own declaration binds and those it inherits.
func (ctx *Context) classifierBehaviorsOf(typeSym *symbols.Symbol) []classifierBehaviorDecl {
	if typeSym == nil {
		return nil
	}
	if cached, ok := ctx.model.classifierBehaviors[typeSym]; ok {
		return cached
	}
	var out []classifierBehaviorDecl
	for _, member := range ctx.model.semantics.MembersOf(typeSym) {
		if member.Decl == nil {
			continue
		}
		if behavior, ok := lower.ClassifierBehaviorOf(member.Decl); ok {
			out = append(out, classifierBehaviorDecl{behavior: behavior, member: member})
		}
	}
	ctx.model.classifierBehaviors[typeSym] = out
	return out
}

// startClassifierBehaviors gives the object an execution of every behavior its
// type exhibits or performs, and runs those executions to quiescence: no due
// event, no runnable do action, no deliverable message. A start reached from
// inside a running behavior only attaches, leaving the run to the outermost
// start, so materializing objects that exhibit each other terminates.
func (ctx *Context) startClassifierBehaviors(inst *Instance, mark int) error {
	return ctx.startClassifierBehaviorsOf([]*Instance{inst}, mark)
}

// startClassifierBehaviorsOf starts the behaviors of every one of the objects as
// one collective run, so objects materialized together exchange messages.
func (ctx *Context) startClassifierBehaviorsOf(objects []*Instance, mark int) error {
	if ctx.declarative {
		return nil
	}
	attached := len(ctx.objectBehaviors)
	if err := ctx.startBehaviorsOfAll(objects); err != nil {
		ctx.abandonCreationSince(mark, attached)
		return err
	}
	return nil
}

// abandonCreationSince undoes a creation that failed: neither the objects it
// registered after mark nor the behaviors attached after attached survive it.
func (ctx *Context) abandonCreationSince(mark, attached int) {
	ctx.abandonCreationBetween(mark, len(ctx.created), attached, len(ctx.objectBehaviors))
}

// abandonCreationBetween undoes one stretch of creation: the objects registered
// from mark up to end and the behaviors attached from attached up to started.
func (ctx *Context) abandonCreationBetween(mark, end, attached, started int) {
	ctx.forgetBehaviorsBetween(attached, started)
	ctx.abandonInstancesBetween(mark, end)
}

// abandonInstancesSince removes objects registered after mark, which a failed
// creation would otherwise leave behind, along with occurrences naming them.
func (ctx *Context) abandonInstancesSince(mark int) {
	ctx.abandonInstancesBetween(mark, len(ctx.created))
}

// abandonInstancesBetween removes the objects registered from mark up to end,
// keeping those registered since, along with occurrences naming the removed.
func (ctx *Context) abandonInstancesBetween(mark, end int) {
	abandoned := make(map[int64]bool)
	var gone []*Instance
	for _, id := range ctx.created[mark:end] {
		if inst, live := ctx.instances[id]; live {
			abandoned[id] = true
			gone = append(gone, inst)
			delete(ctx.instances, id)
		}
	}
	ctx.created = append(ctx.created[:mark], ctx.created[end:]...)
	if len(abandoned) == 0 {
		return
	}
	for sym, id := range ctx.occurrences {
		if _, live := ctx.instances[id]; !live {
			delete(ctx.occurrences, sym)
		}
	}
	for annotation, id := range ctx.metadataObjects {
		if _, live := ctx.instances[id]; !live {
			delete(ctx.metadataObjects, annotation)
		}
	}
	ctx.forgetLives(abandoned)
	ctx.forgetVariantsNaming(abandoned)
	ctx.forgetEdgesOf(gone)
	ctx.forgetValuesNaming(abandoned)
	ctx.forgetMessagesTo(abandoned)
}

// forgetVariantsNaming unselects every variant whose object is abandoned, so the
// selection is made again, and its object built again, when next read.
func (ctx *Context) forgetVariantsNaming(abandoned map[int64]bool) {
	for key, id := range ctx.variantObjects {
		if !abandoned[id] {
			continue
		}
		delete(ctx.variantObjects, key)
		if key.variation == nil {
			continue
		}
		selection := variantSelection{owner: key.owner, variation: key.variation.Name}
		if ctx.selectedVariants[selection] == key.variant.Name {
			delete(ctx.selectedVariants, selection)
		}
	}
}

// forgetValuesNaming unmaterializes every feature value of a surviving object
// that names an abandoned one, so a later read materializes an object the
// session holds rather than reading one it does not.
func (ctx *Context) forgetValuesNaming(abandoned map[int64]bool) {
	for _, inst := range ctx.instances {
		for _, fv := range inst.FeatureValues {
			if !fv.Materialized || !namesAbandoned(fv, abandoned) {
				continue
			}
			fv.Value, fv.Values = Value{}, Value{}
			fv.Materialized, fv.Written = false, false
			ctx.invalidateDependents(fv)
		}
	}
}

// namesAbandoned reports whether a feature value holds an object that is gone,
// directly or as the object a selected variant stands for.
func namesAbandoned(fv *FeatureValue, abandoned map[int64]bool) bool {
	if namesAbandonedObject(fv.Value, abandoned) {
		return true
	}
	for _, val := range elementsOf(fv.Values) {
		if namesAbandonedObject(val, abandoned) {
			return true
		}
	}
	return false
}

// namesAbandonedObject reports whether a value is, or a variant standing for, an
// object that is gone, or an array, vector or tensor keeping one or an array holding one.
func namesAbandonedObject(val Value, abandoned map[int64]bool) bool {
	if abandoned[keptObject(val)] {
		return true
	}
	switch val.Kind {
	case ValInstance, ValVariant:
		return abandoned[val.Instance]
	case ValArray:
		for _, elem := range val.Array().Elements {
			if namesAbandonedObject(elem, abandoned) {
				return true
			}
		}
	}
	return false
}

// forgetMessagesTo drops the messages addressed to an abandoned object, which
// nothing can consume once the object holding its consumers is gone.
func (ctx *Context) forgetMessagesTo(abandoned map[int64]bool) {
	kept := make([]Message, 0, len(ctx.messages))
	for _, msg := range ctx.messages {
		if !abandoned[msg.Object] {
			kept = append(kept, msg)
		}
	}
	ctx.messages = kept
}

// restartClassifierBehaviors gives every object a fresh execution of the
// behaviors its type binds, run as one start so machines restarted alongside
// each other still exchange messages. A failure attaches nothing, leaving the
// objects running no behavior at all.
func (ctx *Context) restartClassifierBehaviors(objects []*Instance) error {
	attached := len(ctx.objectBehaviors)
	err := ctx.startBehaviorsOfAll(objects)
	if err != nil {
		ctx.forgetBehaviorsFrom(attached)
	}
	return err
}

// startBehaviorsOfAll attaches the behaviors of every object before running any
// of them, so their starts are one collective run.
func (ctx *Context) startBehaviorsOfAll(objects []*Instance) error {
	defer ctx.holdDrivenWork()()
	ctx.behaviorRunDepth++
	for _, inst := range objects {
		if err := ctx.startBehaviorsOf(inst); err != nil {
			ctx.behaviorRunDepth--
			return err
		}
		if err := ctx.materializeBehavingParts(inst); err != nil {
			ctx.behaviorRunDepth--
			return err
		}
	}
	ctx.behaviorRunDepth--
	return ctx.runAttachedBehaviors()
}

// materializeBehavingParts materializes the required composite parts of an
// object whose type runs behaviors, so the object runs to quiescence as a whole
// when it is created rather than part by part in the order its parts are first
// read. An optional part (lower bound 0) is required to hold nothing, so it is
// left unread. A part that fails to materialize or start fails its holder.
func (ctx *Context) materializeBehavingParts(inst *Instance) error {
	for _, typ := range inst.types() {
		features := ctx.FeaturesOf(typ)
		for _, i := range ctx.behavingParts(typ) {
			fv, ok := inst.FeatureValues[features[i].Name]
			if !ok || fv.Materialized {
				continue
			}
			// An adopted object's feature may differ from its type's; decide on it.
			if fv.Feature != &features[i] && !ctx.holdsBehavingPart(fv.Feature) {
				continue
			}
			if _, err := inst.GetFeatureValue(ctx, features[i].Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// behavingParts returns the positions in FeaturesOf(typeSym) of the required
// composite parts whose objects run behaviors, memoized per type.
func (ctx *Context) behavingParts(typeSym *symbols.Symbol) []int {
	if parts, ok := ctx.model.behavingFeatures[typeSym]; ok {
		return parts
	}
	features := ctx.FeaturesOf(typeSym)
	parts := []int{}
	for i := range features {
		if !ctx.model.semantics.IsConnectorUsage(features[i].Symbol) && ctx.holdsBehavingPart(&features[i]) {
			parts = append(parts, i)
		}
	}
	ctx.model.behavingFeatures[typeSym] = parts
	return parts
}

// holdsBehavingPart reports whether a feature is required to hold objects that run behaviors.
func (ctx *Context) holdsBehavingPart(feat *EffectiveFeature) bool {
	composite := ctx.requiredPartType(feat)
	return composite != nil && ctx.runsBehaviors(composite, make(map[*symbols.Symbol]bool))
}

// requiredPartType is the type of the objects a composite feature is required to
// hold, or nil when it may hold none: an optional part (finite lower bound 0) or
// one whose lower bound is unknown.
func (ctx *Context) requiredPartType(feat *EffectiveFeature) *symbols.Symbol {
	composite := ctx.CompositeTypeOf(feat)
	mult := feat.Multiplicity
	if composite == nil || !mult.Lower.Known {
		return nil
	}
	if !mult.Lower.Infinite && mult.Lower.Value == 0 {
		return nil
	}
	return composite
}

// runsBehaviors reports whether objects of a type run behaviors, of their own or
// of a part they are required to hold; an optional part is left absent, so what
// it would run does not count. A type on the path being decided answers false: a
// composition cycle has no finite object, so nothing is lost by cutting it.
func (ctx *Context) runsBehaviors(typeSym *symbols.Symbol, visiting map[*symbols.Symbol]bool) bool {
	if known, ok := ctx.model.behaving[typeSym]; ok {
		return known
	}
	if visiting[typeSym] {
		return false
	}
	visiting[typeSym] = true
	defer delete(visiting, typeSym)
	runs := len(ctx.classifierBehaviorsOf(typeSym)) > 0
	features := ctx.FeaturesOf(typeSym)
	for i := range features {
		if runs {
			break
		}
		if ctx.model.semantics.IsConnectorUsage(features[i].Symbol) {
			continue
		}
		if composite := ctx.requiredPartType(&features[i]); composite != nil && ctx.runsBehaviors(composite, visiting) {
			runs = true
		}
	}
	ctx.model.behaving[typeSym] = runs
	return runs
}

// startBehaviorsOf attaches the object's behaviors and, at the outermost start,
// runs everything attached.
func (ctx *Context) startBehaviorsOf(inst *Instance) error {
	defer ctx.holdDrivenWork()()
	for _, typ := range inst.types() {
		for i, decl := range ctx.classifierBehaviorsOf(typ) {
			if ctx.runsBound(inst, decl.member, typ) {
				continue
			}
			if ctx.trace != nil {
				ctx.trace.RecordBehaviorStart(decl.behavior.Kind.String(), decl.behavior.Name, inst.ID)
			}
			behavior, err := ctx.attachClassifierBehavior(inst, decl)
			if err != nil {
				return err
			}
			behavior.binding = i
			inst.behaviors = append(inst.behaviors, behavior)
			ctx.pendingBehaviors = append(ctx.pendingBehaviors, behavior)
			ctx.objectBehaviors = append(ctx.objectBehaviors, behavior)
		}
	}

	return ctx.runAttachedBehaviors()
}

// holdDrivenWork marks, at an outermost start, the behaviors already holding
// work: a driver put it in flight, so the start leaves it to that driver. Once
// the start returns, the behaviors it attached are as their start left them.
func (ctx *Context) holdDrivenWork() func() {
	if ctx.behaviorRunDepth > 0 || ctx.heldBehaviors != nil {
		return func() { /* an outer start already holds them */ }
	}
	held := make(map[*ObjectBehavior]bool)
	ctx.behaviorRunDepth++
	for _, behavior := range ctx.objectBehaviors {
		if behavior.hasPendingWork() {
			held[behavior] = true
		}
	}
	ctx.behaviorRunDepth--
	ctx.heldBehaviors = held
	attached := len(ctx.objectBehaviors)
	return func() {
		ctx.heldBehaviors = nil
		for _, behavior := range ctx.objectBehaviors[min(attached, len(ctx.objectBehaviors)):] {
			behavior.settle()
		}
	}
}

// settle records the execution as its start left it: what it does from here on
// is a move, and an object whose executions are all unmoved is pristine.
func (b *ObjectBehavior) settle() {
	switch {
	case b.State != nil:
		b.State.moved = false
	case b.Action != nil:
		b.Action.moved = false
	}
}

// Moved reports whether the execution has left the state its start put it in.
func (b *ObjectBehavior) Moved() bool {
	switch {
	case b.State != nil:
		return b.State.moved
	case b.Action != nil:
		return b.Action.moved
	default:
		return false
	}
}

// armedWaits lists the waits on the clock that hold the execution, due or not, those
// of the actions its paused work performs included.
func (b *ObjectBehavior) armedWaits() []ClockWait {
	switch {
	case b.State != nil:
		return b.State.visibleArmedWaits()
	case b.Action != nil:
		return b.Action.visibleArmedWaits()
	default:
		return nil
	}
}

// runAttachedBehaviors runs everything attached, at the outermost start: a start
// reached from inside a running behavior leaves the run to that one.
func (ctx *Context) runAttachedBehaviors() error {
	if ctx.behaviorRunDepth > 0 {
		return nil
	}
	ctx.behaviorRunDepth++
	defer func() { ctx.behaviorRunDepth-- }()
	return ctx.drainObjectBehaviors()
}

// forgetBehaviorsFrom drops the behaviors attached since a start began, and the
// work queued for them: a start that failed queues nothing for a later one.
func (ctx *Context) forgetBehaviorsFrom(attached int) {
	ctx.forgetBehaviorsBetween(attached, len(ctx.objectBehaviors))
}

// forgetBehaviorsBetween drops the behaviors attached from attached up to end,
// keeping those attached since, and the work queued for the dropped.
func (ctx *Context) forgetBehaviorsBetween(attached, end int) {
	if attached >= end || end > len(ctx.objectBehaviors) {
		return
	}
	ctx.forgetBehaviors(ctx.objectBehaviors[attached:end])
}

// forgetBehaviors detaches the given behaviors from their objects and from the
// context, wherever they stand among the behaviors attached.
func (ctx *Context) forgetBehaviors(behaviors []*ObjectBehavior) {
	if len(behaviors) == 0 {
		return
	}
	dropped := make(map[*ObjectBehavior]bool, len(behaviors))
	for _, behavior := range behaviors {
		dropped[behavior] = true
	}
	for behavior := range dropped {
		behavior.Object.behaviors = behaviorsExcept(behavior.Object.behaviors, dropped)
		behavior.leaveClock()
	}
	ctx.objectBehaviors = behaviorsExcept(ctx.objectBehaviors, dropped)
	ctx.pendingBehaviors = behaviorsExcept(ctx.pendingBehaviors, dropped)
}

// leaveClock releases the behavior's execution, ending the work it left paused
// and withdrawing it from the clock, so a behavior dropped from its object is never driven again.
func (b *ObjectBehavior) leaveClock() {
	switch {
	case b.State != nil:
		b.State.Release()
	case b.Action != nil:
		b.Action.Release()
	}
}

// behaviorsExcept returns the behaviors none of which is one being dropped.
func behaviorsExcept(behaviors []*ObjectBehavior, dropped map[*ObjectBehavior]bool) []*ObjectBehavior {
	kept := make([]*ObjectBehavior, 0, len(behaviors))
	for _, behavior := range behaviors {
		if !dropped[behavior] {
			kept = append(kept, behavior)
		}
	}
	return kept
}

// drainObjectBehaviors runs the attached behaviors until the objects are
// collectively quiescent: nothing left to start, and no behavior holding an
// event a sibling's send put in flight. Bounded by the event budget, so
// endlessly signalling objects report a typed error instead of spinning.
func (ctx *Context) drainObjectBehaviors() error {
	for rounds := int64(0); ; rounds++ {
		if rounds >= ctx.maxStateEvents {
			return budgetExceeded(ErrStateEventLimitExceeded,
				fmt.Sprintf("%s: exceeded max events (%d rounds; raise %s to allow more), possible non-terminating exchange between objects",
					ErrBehaviorBudget, ctx.maxStateEvents, MaxStateEventsEnvVar), ErrBehaviorBudget)
		}
		behavior, ok := ctx.nextRunnableBehavior()
		if !ok {
			return nil
		}
		if ctx.trace != nil {
			ctx.trace.RecordBehaviorRun(behavior.Kind.String(), behavior.Name, behavior.Object.ID)
		}
		if err := behavior.run(); err != nil {
			return fmt.Errorf("%s: %w", behavior.Describe(), err)
		}
	}
}

// nextRunnableBehavior returns the next behavior with work to do: one not yet
// started, else one holding an event delivered while suspended — unless it held
// work before the start began (holdDrivenWork), or was attached before the
// innermost run boundary, as what it does cannot be undone with the change under way.
func (ctx *Context) nextRunnableBehavior() (*ObjectBehavior, bool) {
	first, attached := 0, 0
	if n := len(ctx.runBoundaries); n > 0 {
		boundary := ctx.runBoundaries[n-1]
		first = min(boundary.pending, len(ctx.pendingBehaviors))
		attached = min(boundary.behaviors, len(ctx.objectBehaviors))
	}
	if first < len(ctx.pendingBehaviors) {
		behavior := ctx.pendingBehaviors[first]
		ctx.pendingBehaviors = slices.Delete(ctx.pendingBehaviors, first, first+1)
		return behavior, true
	}
	for _, behavior := range ctx.objectBehaviors[attached:] {
		if !ctx.heldBehaviors[behavior] && behavior.hasPendingWork() {
			return behavior, true
		}
	}
	return nil, false
}

// hasPendingWork reports whether running the behavior again would advance it: a
// machine woken by an event due now or a signal in flight, or an action whose
// awaited message a sibling has since sent. An event scheduled for a later time
// is not work materialization waits for, and an execution that reached its end
// takes no step whatever is left addressed to it.
func (b *ObjectBehavior) hasPendingWork() bool {
	switch {
	case b.State != nil:
		return b.State.State() != StateCompleted && (b.State.HasDueEvent() || b.State.HasPendingSignal())
	case b.Action != nil:
		return b.Action.State() != StateCompleted && b.Action.HasPendingSignal()
	default:
		return false
	}
}

// attachClassifierBehavior builds the object's own execution of one behavior its
// type binds, seeded with the values the binding declaration supplies, and
// initializes it so its start is reported where every other behavior's is.
func (ctx *Context) attachClassifierBehavior(inst *Instance, decl classifierBehaviorDecl) (*ObjectBehavior, error) {
	behavior, occurrence, err := ctx.bindClassifierBehavior(inst, decl)
	if err != nil {
		return nil, err
	}
	sym := behavior.Symbol

	arguments, err := ctx.classifierBehaviorArguments(inst, decl)
	if err != nil {
		return nil, err
	}

	switch decl.behavior.Kind {
	case lower.ExhibitedState:
		exec, err := newStateExecutorForOccurrence(ctx, sym, inst, occurrence)
		if err != nil {
			return nil, fmt.Errorf("exhibited state machine %s of %s: %w", decl.behavior.Name, symbolText(inst.Type), err)
		}
		for name, value := range arguments {
			exec.stateData[name] = value
		}
		if err := exec.initialize(); err != nil {
			exec.Release()
			return nil, fmt.Errorf("exhibited state machine %s of %s: %w", decl.behavior.Name, symbolText(inst.Type), err)
		}
		behavior.State = exec
	case lower.PerformedAction:
		exec, err := newActionExecutorOf(ctx, decl.member, sym, inst, occurrence)
		if err != nil {
			return nil, fmt.Errorf("performed action %s of %s: %w", decl.behavior.Name, symbolText(inst.Type), err)
		}
		if len(arguments) > 0 {
			exec.SetInputs(arguments)
		}
		// An action stating no flow performs no step; the object still performs it,
		// completed at once, rather than failing to be created.
		begin := (*ActionExecutor).completeWithoutFlow
		if exec.hasFlow() {
			begin = (*ActionExecutor).initialize
		}
		if err := ctx.startAction(exec, begin); err != nil {
			exec.Release()
			return nil, fmt.Errorf("performed action %s of %s: %w", decl.behavior.Name, symbolText(inst.Type), err)
		}
		behavior.Action = exec
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedClassifierBehavior, decl.behavior.Kind)
	}
	return behavior, nil
}

// bindClassifierBehavior is the object's binding of one behavior its type declares,
// its execution still to be made, and the performance occurrence the binding holds.
func (ctx *Context) bindClassifierBehavior(inst *Instance, decl classifierBehaviorDecl) (*ObjectBehavior, *Instance, error) {
	chain, err := ctx.classifierBehaviorChain(decl)
	if err != nil {
		return nil, nil, err
	}
	sym := chain[len(chain)-1]
	behavior := &ObjectBehavior{
		Name:     decl.behavior.Name,
		Kind:     decl.behavior.Kind,
		Symbol:   sym,
		Object:   inst,
		member:   decl.member,
		bindings: chain,
		kinds:    ctx.behaviorKinds(chain),
	}
	var occurrence *Instance
	switch decl.behavior.Kind {
	case lower.ExhibitedState:
		occurrence, err = ctx.performanceOccurrence(inst, decl, sym, ErrStatePerformanceOccurrence)
	case lower.PerformedAction:
		occurrence, err = ctx.performanceOccurrence(inst, decl, sym, ErrActionPerformanceOccurrence)
	default:
		return nil, nil, fmt.Errorf("%w: %s", ErrUnsupportedClassifierBehavior, decl.behavior.Kind)
	}
	if err != nil {
		return nil, nil, err
	}
	return behavior, occurrence, nil
}

// performanceOccurrence returns the performance occurrence the binding
// declaration holds: the object the exhibited or performed usage's feature
// names, materialized when the feature holds none yet. sentinel types the
// failures, telling an exhibited machine's from a performed action's.
func (ctx *Context) performanceOccurrence(
	inst *Instance,
	decl classifierBehaviorDecl,
	behavior *symbols.Symbol,
	sentinel error,
) (*Instance, error) {
	name := decl.behavior.Name
	fv, ok := inst.FeatureValues[name]
	if !ok || fv.Feature == nil || fv.Feature.Symbol != decl.member {
		ok = false
		for candidate, value := range inst.FeatureValues {
			if value.Feature != nil && value.Feature.Symbol == decl.member {
				name, ok = candidate, true
				break
			}
		}
	}
	if !ok {
		return nil, fmt.Errorf("%w: object #%d has no feature for %s %s",
			sentinel, inst.ID, decl.behavior.Kind, decl.behavior.Name)
	}
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("%w: materialize %s of object #%d: %w",
			sentinel, name, inst.ID, err)
	}
	if fv.HeldValue().Kind == ValInvalid {
		occurrence, err := ctx.materialize(behavior, 0, inst, name)
		if err != nil {
			return nil, fmt.Errorf("%w: materialize %s of object #%d: %w",
				sentinel, name, inst.ID, err)
		}
		ctx.noteProbeWrite(fv)
		before := ctx.beforeWrite(fv)
		fv.Value = Value{Kind: ValInstance, Instance: occurrence.ID}
		fv.Materialized = true
		ctx.afterWrite(fv, before)
		return occurrence, nil
	}
	id, ok := fv.HeldValue().Object()
	if !ok {
		return nil, fmt.Errorf("%w: %s of object #%d holds %s, not an occurrence",
			sentinel, name, inst.ID, fv.HeldValue().Kind)
	}
	occurrence, ok := ctx.Instance(id)
	if !ok {
		return nil, fmt.Errorf("%w: %s of object #%d names unknown object #%d",
			sentinel, name, inst.ID, id)
	}
	return occurrence, nil
}

// run advances the object's behavior until it is quiescent: an action until it
// completes or waits for a message, a machine until no event is due and no do
// action is runnable.
func (b *ObjectBehavior) run() error {
	switch {
	case b.State != nil:
		return b.State.RunToQuiescence()
	case b.Action != nil:
		if !b.Action.hasFlow() {
			return nil
		}
		return b.Action.RunToQuiescence()
	default:
		return fmt.Errorf("%w: %s has no execution", ErrUnsupportedClassifierBehavior, b.Name)
	}
}

// classifierBehaviorSymbol resolves the element holding the body a binding
// declaration runs: the declaration itself when it states one, otherwise what it
// names — the feature it refers to or the definition it is typed by.
func (ctx *Context) classifierBehaviorSymbol(decl classifierBehaviorDecl) (*symbols.Symbol, error) {
	chain, err := ctx.classifierBehaviorChain(decl)
	if err != nil {
		return nil, err
	}
	return chain[len(chain)-1], nil
}

// classifierBehaviorChain resolves the bindings from a binding declaration to the
// element holding the body it runs: the declaration first, then what each names in
// turn, ending at the one stating a body.
func (ctx *Context) classifierBehaviorChain(decl classifierBehaviorDecl) ([]*symbols.Symbol, error) {
	sym := decl.member
	chain := []*symbols.Symbol{sym}
	for depth := 0; depth < maxBehaviorBindingDepth; depth++ {
		stated := sym == decl.member && decl.behavior.StatesBody
		if !stated && sym != decl.member {
			stated = statesBehaviorBody(sym)
		}
		if stated {
			return chain, nil
		}
		next := ctx.namedBehavior(sym)
		if next == nil || next == sym {
			// A declaration naming nothing that holds a body is not executable:
			// the type binds a behavior no element states.
			if sym != decl.member {
				return chain, nil
			}
			return nil, fmt.Errorf("%w: %s %s of %s names no behavior body",
				ErrUnresolvedClassifierBehavior, decl.behavior.Kind, decl.behavior.Name, symbolText(decl.member))
		}
		sym = next
		chain = append(chain, sym)
	}
	return nil, fmt.Errorf("%w: %s %s of %s names itself through %d bindings",
		ErrUnresolvedClassifierBehavior, decl.behavior.Kind, decl.behavior.Name, symbolText(decl.member), maxBehaviorBindingDepth)
}

// namedBehavior reports the element a binding declaration names: what it
// reference-subsets, the type it states, or — for `exhibit m;`, whose name is
// the state usage declared elsewhere — that usage.
func (ctx *Context) namedBehavior(sym *symbols.Symbol) *symbols.Symbol {
	if ref := ctx.model.semantics.ReferencedFeature(sym); ref != nil {
		return ref
	}
	if typ := ctx.extractType(sym); typ != nil {
		return typ
	}
	if sym.Name != "" && sym.OwnerScope != nil {
		if named, ok := ctx.model.resolver.LookupNameExcluding(sym.OwnerScope, sym.Name, sym.Decl); ok {
			return named
		}
	}
	return nil
}

// classifierBehaviorArguments evaluates the values a binding declaration
// supplies to the behavior's parameters, against the object running it.
func (ctx *Context) classifierBehaviorArguments(inst *Instance, decl classifierBehaviorDecl) (map[string]Value, error) {
	if len(decl.behavior.Arguments) == 0 {
		return nil, nil
	}
	scope := declScope(decl.member)
	if scope == nil {
		scope = declScope(inst.Type)
	}
	args := make(map[string]Value, len(decl.behavior.Arguments))
	for _, arg := range decl.behavior.Arguments {
		ec := NewEvalContextIn(ctx, scope, inst)
		value, err := ec.Eval(arg.Value)
		if err != nil {
			return nil, fmt.Errorf("%s %s of %s: bind %s: %w",
				decl.behavior.Kind, decl.behavior.Name, symbolText(inst.Type), arg.Name, err)
		}
		args[ctx.argumentParameter(scope, arg)] = value
	}
	return args, nil
}

// argumentParameter names the behavior parameter an argument binds: the feature
// its declaration redefines (`in <a> :>> x = 4` binds x), else its own name.
func (ctx *Context) argumentParameter(scope *symbols.Scope, arg lower.Attribute) string {
	for _, redefined := range ctx.model.semantics.RedefinedFeatures(memberSymbol(scope, arg.Node)) {
		if redefined.Name != "" {
			return redefined.Name
		}
	}
	return arg.Name
}

// actionBodySymbol resolves the element holding the body an action symbol
// performs: itself when it states one, otherwise the action it names — the
// definition typing it, or the feature it refers to. The symbol itself is
// returned when nothing it names states a body, so the missing flow is reported
// against the declaration that was asked for.
func (ctx *Context) actionBodySymbol(action *symbols.Symbol) *symbols.Symbol {
	sym := action
	for depth := 0; depth < maxBehaviorBindingDepth; depth++ {
		if statesBehaviorBody(sym) {
			return sym
		}
		next := ctx.namedBehavior(sym)
		if next == nil || next == sym ||
			(next.Kind != symbols.SymbolActionUsage && next.Kind != symbols.SymbolActionDef) {
			return action
		}
		sym = next
	}
	return action
}

// statesBehaviorBody reports whether a symbol's declaration states a behavior
// body of its own rather than naming an element that holds one.
func statesBehaviorBody(sym *symbols.Symbol) bool {
	if sym == nil || sym.Decl == nil {
		return false
	}
	members, err := lower.BehaviorMembers(sym.Decl)
	if err != nil {
		return false
	}
	return lower.StatesBehaviorBody(members)
}

// assignPerformerFeature writes a value to the feature of that name of the
// object performing a behavior, and reports whether the object has one: a body
// that assigns a feature of its object writes that object, not shared data.
// The write is refused when the name does not resolve to that feature where the
// statement was written.
func assignPerformerFeature(ctx *Context, self *Instance, scope *symbols.Scope, name string, value Value) (bool, error) {
	if self == nil {
		return false, nil
	}
	if _, ok := self.FeatureValues[name]; !ok {
		return false, nil
	}
	if !namesPerformerFeature(ctx, self, scope, name) {
		return true, fmt.Errorf("write %s of object #%d: %w: %s is a feature of %s, which the body does not name: "+
			"pass it as a parameter, or write the body in the declaration that holds it",
			name, self.ID, ErrPerformerFeatureNotInScope, name, symbolText(self.Type))
	}
	if err := self.SetFeatureValue(ctx, name, value); err != nil {
		return true, fmt.Errorf("write %s of object #%d: %w", name, self.ID, err)
	}
	ctx.noteObjectWrite(self, name, value)
	return true, nil
}

// namesPerformerFeature reports whether name, resolved where the statement was
// written, denotes a feature of the object performing the behavior under any of
// its types: the performer is not a namespace the body's names are looked up in.
func namesPerformerFeature(ctx *Context, self *Instance, scope *symbols.Scope, name string) bool {
	if ctx == nil || ctx.model.resolver == nil || self == nil || scope == nil {
		return false
	}
	sym, ok := ctx.model.resolver.LookupName(scope, name)
	if !ok || sym == nil {
		return false
	}
	for _, typ := range self.types() {
		if ctx.typeHoldsFeature(typ, sym) {
			return true
		}
	}
	return false
}

// typeHoldsFeature reports whether a feature symbol is one the type holds:
// declared by the type itself or by one of its supertypes.
func (ctx *Context) typeHoldsFeature(typeSym, feature *symbols.Symbol) bool {
	if typeSym == nil || feature == nil {
		return false
	}
	owner := ctx.findOwnerType(feature)
	if owner == nil {
		return false
	}
	if owner == typeSym {
		return true
	}
	for _, super := range ctx.model.semantics.AllSupertypes(typeSym) {
		if super == owner {
			return true
		}
	}
	return false
}

// symbolText names a symbol in diagnostics, falling back to its kind when it is
// anonymous.
func symbolText(sym *symbols.Symbol) string {
	if sym == nil {
		return unknownText
	}
	if sym.Name != "" {
		return sym.Name
	}
	return sym.Kind.String()
}
