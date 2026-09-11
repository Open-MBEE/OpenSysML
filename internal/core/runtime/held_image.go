package runtime

import (
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrImageIdentityTaken is the typed error for an image whose object's identity
// the destination context already holds an object under.
var ErrImageIdentityTaken = errors.New("identity already held by the destination context")

// ErrImageClock is the typed error for a destination clock already past the
// instant the image was taken at.
var ErrImageClock = errors.New("destination clock is past the image's instant")

// ErrImageBound is the typed error for execution state bound to the context that
// made it, which no image carries: an evaluation under way, or a frame of a calc.
var ErrImageBound = errors.New("execution state bound to its context")

// ErrImageRoot is the typed error for an object asked of an image that was not
// taken in the context of the object.
var ErrImageRoot = errors.New("object is not of the context imaged")

// HeldImageError reports what of an object kept an image from being taken or
// materialized: the object, and the typed reason.
type HeldImageError struct {
	ID   int64
	Type *symbols.Symbol
	What string
	Err  error
}

// Error names what was being done to the object and the reason it could not be.
func (e *HeldImageError) Error() string {
	if e.ID == 0 {
		return fmt.Sprintf("%s: %v", e.What, e.Err)
	}
	return fmt.Sprintf("%s of object #%d (%s): %v", e.What, e.ID, symbolText(e.Type), e.Err)
}

// Unwrap exposes the typed reason.
func (e *HeldImageError) Unwrap() error { return e.Err }

// HeldImage is the state of a closure of held objects by value, taken between steps
// of the context holding them: the objects, their lifetimes, the executions of their
// behaviors and the messages in flight to them. It materializes into other contexts
// over the same declarations, each getting objects of its own under the same
// identities, so a run there writes nothing of the context imaged.
//
// Adopt is no substitute: it registers the very *Instance structures in the taker,
// shares the identity sequence and restarts every behavior from its initial state.
// Snapshot.Restore is no substitute either: its captures point at the executors,
// frames and journal of the context they were taken in.
type HeldImage struct {
	roots   []int64
	objects []imagedObject // in the creation order of the context imaged
	// held holds the identity of every object imaged.
	held map[int64]bool

	activations, runs int64
	clock             float64

	occurrences      map[*symbols.Symbol]int64
	metadataObjects  map[metadataAnnotation]int64
	variantObjects   map[variantObject]int64
	selectedVariants map[variantSelection]string

	runStates []imagedRun
	clockRun  int              // index into runStates, -1 for none
	behaviors []imagedBehavior // in the attachment order of the context imaged
	messages  []Message
}

// imagedObject is one object by value: its identity, types, owner, lifetime,
// feature values and connector ends.
type imagedObject struct {
	id           int64
	typ          *symbols.Symbol
	classifiers  []*symbols.Symbol
	explicit     bool
	owner        int64
	ownerFeature string
	life         life
	features     []imagedFeature
	ends         []ConnectorEnd
	anonymous    []int64
	keptAnon     []keptAnonymous
	keptConn     []keptConnector
}

// imagedFeature is one feature value by value, with every name the object reads it under.
type imagedFeature struct {
	names          []string
	feature        EffectiveFeature
	value, values  Value
	materialized   bool
	written        bool
	bindingDerived bool
	dependents     []imagedFeatureRef
	reads          []imagedFeatureRef
}

// imagedFeatureRef names a feature value of the image: an object's, by position.
type imagedFeatureRef struct {
	object int64
	index  int
}

// keptConnector is the identity a named connector's object had before a carry-over.
type keptConnector struct {
	feature int
	id      int64
}

// imagedRun is one run's bookkeeping by value: what it spent and noted, and its
// seeded generator where the schedule has one.
type imagedRun struct {
	steps, elements int64
	notes           []RunNote
	seeded          bool
	generator       rand.PCG
}

// Holds reports whether the image holds an object under id.
func (img *HeldImage) Holds(id int64) bool { return img.held[id] }

// Roots are the identities the image was taken of, in the order asked.
func (img *HeldImage) Roots() []int64 { return slices.Clone(img.roots) }

// Image takes the state of the given objects and everything they hold, own or name,
// by value: the closure of the objects, the executions of their behaviors and the
// messages bound for them. It refuses, typed, a context inside a step
// (ErrSnapshotMidRun), a body paused mid-statement (ErrSnapshotPausedBody), a value
// no other context carries (NotPortableError) and execution state bound to this
// context (ErrImageBound), each wrapped in a HeldImageError naming the object.
func (ctx *Context) Image(objects ...*Instance) (*HeldImage, error) {
	if ctx.midRun() {
		return nil, ErrSnapshotMidRun
	}
	img := &HeldImage{
		held:             make(map[int64]bool),
		activations:      ctx.activations,
		runs:             ctx.runs,
		clock:            ctx.clock.now,
		occurrences:      make(map[*symbols.Symbol]int64),
		metadataObjects:  make(map[metadataAnnotation]int64),
		variantObjects:   make(map[variantObject]int64),
		selectedVariants: make(map[variantSelection]string),
		clockRun:         -1,
	}
	t := &imaging{ctx: ctx, img: img, runs: make(map[*runState]int), declared: make(map[*symbols.Symbol]bool)}
	for _, inst := range objects {
		if inst == nil {
			continue
		}
		if held, ok := ctx.instances[inst.ID]; !ok || held != inst {
			return nil, &HeldImageError{ID: inst.ID, Type: inst.Type, What: "image", Err: ErrImageRoot}
		}
		if !slices.Contains(img.roots, inst.ID) {
			img.roots = append(img.roots, inst.ID)
		}
		t.reach(inst.ID)
	}
	if err := t.close(); err != nil {
		return nil, err
	}
	return img, nil
}

// midRun reports a context inside a step: a run, action or calc on the stack, a
// body paused there, or a probe under way.
func (ctx *Context) midRun() bool {
	return ctx.runDepth > 0 || ctx.actionDepth > 0 || ctx.calcDepth > 0 || ctx.pausable != nil || ctx.probes > 0
}

// imaging takes an image: the closure under way, and the runs taken so far.
type imaging struct {
	ctx   *Context
	img   *HeldImage
	queue []int64
	runs  map[*runState]int
	// declared are the types and behaviors of the objects reached: a usage declared
	// under one denotes an occurrence of the held graph's own.
	declared map[*symbols.Symbol]bool
	// open is set once the messages open to any consumer were reached.
	open bool
}

// reach adds an object to the closure, once.
func (t *imaging) reach(id int64) {
	if id == 0 || t.img.held[id] {
		return
	}
	t.img.held[id] = true
	t.queue = append(t.queue, id)
}

// bring is the Bring that records every object a value names as reached, so the
// closure grows to what the imaged values name.
func (t *imaging) bring(id int64) (*Instance, error) {
	inst, ok := t.ctx.instances[id]
	if !ok {
		return nil, fmt.Errorf("%w: object #%d", ErrImageRoot, id)
	}
	t.reach(id)
	return inst, nil
}

// value checks that v carries, recording the objects it names.
func (t *imaging) value(v Value) error {
	_, err := t.ctx.Carry(v, t.bring)
	return err
}

// values checks that every value carries.
func (t *imaging) values(vals map[string]Value) error {
	for _, v := range vals {
		if err := t.value(v); err != nil {
			return err
		}
	}
	return nil
}

// close takes the closure to a fixpoint: every object reached, its owner, what its
// values name, its behaviors' executions, the occurrences of usages declared under
// its types and behaviors, and the messages bound for it.
func (t *imaging) close() error {
	ctx := t.ctx
	for len(t.queue) > 0 || !t.open {
		if len(t.queue) == 0 {
			for sym, id := range ctx.occurrences {
				if _, live := ctx.instances[id]; live && t.declaredUnderHeld(sym) {
					t.reach(id)
				}
			}
			if len(t.queue) > 0 || t.open {
				continue
			}
			t.open = true
			for _, msg := range ctx.messages {
				if msg.Object == 0 {
					if err := t.message(msg); err != nil {
						return &HeldImageError{What: "image messages", Err: err}
					}
				}
			}
			continue
		}
		id := t.queue[0]
		t.queue = t.queue[1:]
		inst := ctx.instances[id]
		if err := t.object(inst); err != nil {
			return &HeldImageError{ID: inst.ID, Type: inst.Type, What: "image", Err: err}
		}
		if err := t.messagesTo(inst); err != nil {
			return &HeldImageError{ID: inst.ID, Type: inst.Type, What: "image messages", Err: err}
		}
	}
	t.finish()
	return nil
}

// declaredUnderHeld reports a usage declared under a type or behavior of an object
// reached, whose occurrence is the held graph's own.
func (t *imaging) declaredUnderHeld(sym *symbols.Symbol) bool {
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil && t.declared[owner] {
			return true
		}
	}
	return false
}

// object takes one object's state, its behaviors' included, reaching what it names.
func (t *imaging) object(inst *Instance) error {
	ctx := t.ctx
	if inst.owner != nil {
		t.reach(inst.owner.ID)
	}
	t.declared[inst.Type] = true
	for _, classifier := range inst.classifiers {
		t.declared[classifier] = true
	}
	obj := imagedObject{
		id: inst.ID, typ: inst.Type, classifiers: slices.Clone(inst.classifiers),
		explicit: inst.explicit, ownerFeature: inst.ownerFeature,
		life:      ctx.lives[inst.ID],
		ends:      slices.Clone(inst.Ends),
		anonymous: slices.Clone(inst.anonymous),
		keptAnon:  slices.Clone(inst.keptAnonymous),
	}
	if inst.owner != nil {
		obj.owner = inst.owner.ID
	}
	for i := range obj.ends {
		if err := t.value(obj.ends[i].Value); err != nil {
			return fmt.Errorf("connector end %s: %w", obj.ends[i].Name, err)
		}
	}
	for _, id := range obj.anonymous {
		t.reach(id)
	}
	index := make(map[*FeatureValue]int)
	for _, name := range slices.Sorted(maps.Keys(inst.FeatureValues)) {
		fv := inst.FeatureValues[name]
		if at, ok := index[fv]; ok {
			obj.features[at].names = append(obj.features[at].names, name)
			continue
		}
		if fv.changing {
			return fmt.Errorf("%w: feature %s is being written", ErrImageBound, name)
		}
		if err := t.value(fv.Value); err != nil {
			return fmt.Errorf("feature %s: %w", name, err)
		}
		if err := t.value(fv.Values); err != nil {
			return fmt.Errorf("feature %s: %w", name, err)
		}
		index[fv] = len(obj.features)
		f := imagedFeature{
			names: []string{name}, value: fv.Value, values: fv.Values,
			materialized: fv.Materialized, written: fv.Written, bindingDerived: fv.BindingDerived,
		}
		if fv.Feature != nil {
			f.feature = *fv.Feature
		}
		obj.features = append(obj.features, f)
	}
	for fv, id := range inst.keptConnectors {
		if at, ok := index[fv]; ok {
			obj.keptConn = append(obj.keptConn, keptConnector{feature: at, id: id})
		}
	}
	slices.SortFunc(obj.keptConn, func(a, b keptConnector) int { return a.feature - b.feature })
	t.img.objects = append(t.img.objects, obj)
	for _, b := range inst.behaviors {
		if err := t.behavior(b); err != nil {
			return fmt.Errorf("%s %s: %w", b.Kind, b.Name, err)
		}
	}
	return nil
}

// messagesTo takes the messages in flight bound for the object, reaching what they name.
func (t *imaging) messagesTo(inst *Instance) error {
	for _, msg := range t.ctx.messages {
		if msg.Object != inst.ID {
			continue
		}
		if err := t.message(msg); err != nil {
			return err
		}
	}
	return nil
}

// message checks that a message carries, reaching what it names.
func (t *imaging) message(msg Message) error {
	t.reach(msg.EventObject)
	t.reach(msg.PortID)
	if err := t.values(msg.Payload); err != nil {
		return fmt.Errorf("signal %s: %w", msg.SignalType, err)
	}
	if msg.Value != nil {
		if err := t.value(*msg.Value); err != nil {
			return fmt.Errorf("signal %s: %w", msg.SignalType, err)
		}
	}
	return nil
}

// run takes a run's bookkeeping once, however many executors share it, and
// answers its position; -1 for none.
func (t *imaging) run(state *runState) (int, error) {
	if state == nil {
		return -1, nil
	}
	if at, ok := t.runs[state]; ok {
		return at, nil
	}
	if len(state.calcUsageRuns) > 0 {
		return 0, fmt.Errorf("%w: calc usage evaluations open", ErrSnapshotMidRun)
	}
	run := imagedRun{steps: state.steps, elements: state.elements, notes: slices.Clone(state.notes)}
	if state.scheduler != nil {
		if state.scheduler.explore != nil {
			return 0, fmt.Errorf("%w: an exploration of the schedule under way", ErrImageBound)
		}
		if state.scheduler.pcg != nil {
			run.seeded = true
			run.generator = *state.scheduler.pcg
		}
	}
	at := len(t.img.runStates)
	t.runs[state] = at
	t.img.runStates = append(t.img.runStates, run)
	return at, nil
}

// finish takes what the context derives for the closure once it is complete: the
// feature-value edges within it, the derived objects it holds and the run driving the clock.
func (t *imaging) finish() {
	ctx := t.ctx
	img := t.img
	created := make(map[int64]int, len(ctx.created))
	for i, id := range ctx.created {
		created[id] = i
	}
	slices.SortFunc(img.objects, func(a, b imagedObject) int { return created[a.id] - created[b.id] })
	for i := range img.objects {
		inst := ctx.instances[img.objects[i].id]
		for j := range img.objects[i].features {
			fv := inst.FeatureValues[img.objects[i].features[j].names[0]]
			img.objects[i].features[j].dependents = t.edges(fv.dependents)
			img.objects[i].features[j].reads = t.edges(fv.reads)
		}
	}
	for sym, id := range ctx.occurrences {
		if img.held[id] {
			img.occurrences[sym] = id
		}
	}
	for key, id := range ctx.metadataObjects {
		if img.held[id] {
			img.metadataObjects[key] = id
		}
	}
	for key, id := range ctx.variantObjects {
		if img.held[id] && img.held[key.owner] {
			img.variantObjects[key] = id
		}
	}
	for key, variant := range ctx.selectedVariants {
		if img.held[key.owner] {
			img.selectedVariants[key] = variant
		}
	}
	if at, ok := t.runs[ctx.clockRun.state]; ok {
		img.clockRun = at
	}
	slices.SortStableFunc(img.behaviors, func(a, b imagedBehavior) int { return a.attached - b.attached })
	for _, msg := range ctx.messages {
		if msg.Object == 0 || img.held[msg.Object] {
			img.messages = append(img.messages, msg)
		}
	}
}

// edges names, of the feature values given, those the image holds.
func (t *imaging) edges(fvs []*FeatureValue) []imagedFeatureRef {
	var refs []imagedFeatureRef
	for _, fv := range fvs {
		if ref, ok := t.locate(fv); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

// locate finds a feature value among the imaged objects' by identity.
func (t *imaging) locate(fv *FeatureValue) (imagedFeatureRef, bool) {
	for _, obj := range t.img.objects {
		inst := t.ctx.instances[obj.id]
		for j, f := range obj.features {
			if inst.FeatureValues[f.names[0]] == fv {
				return imagedFeatureRef{object: obj.id, index: j}, true
			}
		}
	}
	return imagedFeatureRef{}, false
}

// Materialize gives dst objects of its own for the image: the same identities,
// types, owners, feature values, lifetimes and behavior executions, standing where
// the imaged ones stood, with dst's identity sequence advanced past them. It
// refuses, typed, an identity dst already holds (ErrImageIdentityTaken) and a clock
// already past the image's instant (ErrImageClock); it fails whole, leaving dst as
// it was, when anything of the image does not materialize.
func (img *HeldImage) Materialize(dst *Context) error {
	if dst.midRun() {
		return ErrSnapshotMidRun
	}
	for _, obj := range img.objects {
		if _, taken := dst.instances[obj.id]; taken {
			return &HeldImageError{ID: obj.id, Type: obj.typ, What: "materialize", Err: ErrImageIdentityTaken}
		}
	}
	if dst.clock.now > img.clock {
		return fmt.Errorf("%w: at t=%v, image at t=%v", ErrImageClock, dst.clock.now, img.clock)
	}
	m := &materializing{dst: dst, img: img, made: make(map[int64]*Instance, len(img.objects))}
	mark := len(dst.created)
	attached := len(dst.objectBehaviors)
	if err := m.run(); err != nil {
		dst.forgetBehaviorsFrom(attached)
		dst.abandonInstancesSince(mark)
		return err
	}
	return nil
}

// materializing builds one context's objects for an image.
type materializing struct {
	dst  *Context
	img  *HeldImage
	made map[int64]*Instance
	runs []*runState
}

// bring answers the object made here for an imaged identity.
func (m *materializing) bring(id int64) (*Instance, error) {
	inst, ok := m.made[id]
	if !ok {
		return nil, fmt.Errorf("%w: object #%d", ErrImageRoot, id)
	}
	return inst, nil
}

// value is v as a value of dst.
func (m *materializing) value(v Value) (Value, error) {
	return m.dst.Carry(v, m.bring)
}

// values is every value of the map as a value of dst, in a map of dst's own.
func (m *materializing) values(vals map[string]Value) (map[string]Value, error) {
	if vals == nil {
		return nil, nil
	}
	out := make(map[string]Value, len(vals))
	for name, v := range vals {
		carried, err := m.value(v)
		if err != nil {
			return nil, err
		}
		out[name] = carried
	}
	return out, nil
}

func (m *materializing) run() error {
	dst, img := m.dst, m.img
	for _, obj := range img.objects {
		inst := &Instance{
			ID: obj.id, Type: obj.typ, classifiers: slices.Clone(obj.classifiers),
			FeatureValues: make(map[string]*FeatureValue, len(obj.features)),
			explicit:      obj.explicit, ownerFeature: obj.ownerFeature,
			anonymous:     slices.Clone(obj.anonymous),
			keptAnonymous: slices.Clone(obj.keptAnon),
		}
		dst.registerInstance(inst)
		dst.claimID(obj.id)
		m.made[obj.id] = inst
	}
	for _, obj := range img.objects {
		if err := m.object(obj); err != nil {
			return &HeldImageError{ID: obj.id, Type: obj.typ, What: "materialize", Err: err}
		}
	}
	for _, obj := range img.objects {
		m.edges(obj)
	}
	dst.activations = max(dst.activations, img.activations)
	dst.runs = max(dst.runs, img.runs)
	dst.clock.now = img.clock
	for sym, id := range img.occurrences {
		dst.occurrences[sym] = id
	}
	for key, id := range img.metadataObjects {
		dst.metadataObjects[key] = id
	}
	for key, id := range img.variantObjects {
		dst.variantObjects[key] = id
	}
	for key, variant := range img.selectedVariants {
		dst.selectedVariants[key] = variant
	}
	for _, run := range img.runStates {
		m.runs = append(m.runs, m.runState(run))
	}
	if img.clockRun >= 0 {
		dst.clockRun.state = m.runs[img.clockRun]
	}
	for _, b := range img.behaviors {
		if err := m.behavior(b); err != nil {
			return &HeldImageError{ID: b.object, Type: m.made[b.object].Type, What: fmt.Sprintf("materialize %s %s", b.kind, b.name), Err: err}
		}
	}
	for _, msg := range img.messages {
		carried, err := m.message(msg)
		if err != nil {
			return &HeldImageError{ID: msg.Object, What: "materialize messages", Err: err}
		}
		dst.messages = append(dst.messages, carried)
	}
	return nil
}

// object fills one object made: its owner, lifetime, feature values and ends.
func (m *materializing) object(obj imagedObject) error {
	dst := m.dst
	inst := m.made[obj.id]
	if obj.owner != 0 {
		inst.owner = m.made[obj.owner]
	}
	dst.lives[obj.id] = obj.life
	for _, f := range obj.features {
		fv := &FeatureValue{
			Feature:      m.feature(inst, f.feature),
			Materialized: f.materialized, Written: f.written, BindingDerived: f.bindingDerived,
		}
		var err error
		if fv.Value, err = m.value(f.value); err != nil {
			return fmt.Errorf("feature %s: %w", f.names[0], err)
		}
		if fv.Values, err = m.value(f.values); err != nil {
			return fmt.Errorf("feature %s: %w", f.names[0], err)
		}
		for _, name := range f.names {
			inst.FeatureValues[name] = fv
		}
	}
	for _, kept := range obj.keptConn {
		inst.keepConnector(inst.FeatureValues[obj.features[kept.feature].names[0]], kept.id)
	}
	if obj.ends != nil {
		inst.Ends = make([]ConnectorEnd, len(obj.ends))
		for i, end := range obj.ends {
			v, err := m.value(end.Value)
			if err != nil {
				return fmt.Errorf("connector end %s: %w", end.Name, err)
			}
			inst.Ends[i] = ConnectorEnd{Name: end.Name, Value: v}
		}
	}
	return nil
}

// feature is dst's declaration of an imaged feature: the one of the object's types
// declaring the same symbol, so dst's own shape tables answer for it, else a copy.
func (m *materializing) feature(inst *Instance, f EffectiveFeature) *EffectiveFeature {
	if f.Symbol == nil && f.Name == "" {
		return nil
	}
	for _, typ := range inst.types() {
		features := m.dst.FeaturesOf(typ)
		for i := range features {
			if features[i].Symbol == f.Symbol && features[i].Name == f.Name && features[i].OwnerType == f.OwnerType {
				return &features[i]
			}
		}
	}
	copied := f
	return &copied
}

// edges links the feature values of one object made to the ones they read and
// that read them, within the image.
func (m *materializing) edges(obj imagedObject) {
	inst := m.made[obj.id]
	for _, f := range obj.features {
		fv := inst.FeatureValues[f.names[0]]
		for _, ref := range f.dependents {
			fv.dependents = append(fv.dependents, m.featureAt(ref))
		}
		for _, ref := range f.reads {
			fv.reads = append(fv.reads, m.featureAt(ref))
		}
	}
}

// featureAt is the feature value made for an imaged reference.
func (m *materializing) featureAt(ref imagedFeatureRef) *FeatureValue {
	at := slices.IndexFunc(m.img.objects, func(o imagedObject) bool { return o.id == ref.object })
	return m.made[ref.object].FeatureValues[m.img.objects[at].features[ref.index].names[0]]
}

// runState is a run of dst's own standing where an imaged run stood: what it spent
// and noted, and, where both schedules are seeded, the imaged generator's position.
func (m *materializing) runState(run imagedRun) *runState {
	state := m.dst.newRunState()
	state.steps, state.elements, state.notes = run.steps, run.elements, slices.Clone(run.notes)
	if run.seeded && state.scheduler != nil && state.scheduler.pcg != nil {
		*state.scheduler.pcg = run.generator
	}
	return state
}

// message is a message as dst carries it.
func (m *materializing) message(msg Message) (Message, error) {
	out := msg
	var err error
	if out.Payload, err = m.values(msg.Payload); err != nil {
		return Message{}, err
	}
	if msg.Value != nil {
		v, err := m.value(*msg.Value)
		if err != nil {
			return Message{}, err
		}
		out.Value = &v
	}
	return out, nil
}
