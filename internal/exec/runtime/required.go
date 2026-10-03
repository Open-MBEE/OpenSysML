package runtime

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// eagerLowerBound is the most members a lower bound has made with its collection; past
// it, the members beyond the first are held as required members, each made when reached.
const eagerLowerBound int64 = 1000

// requiredMembers are the members a collection holds past the ones made, to make up its
// lower bound: count objects of typ held by owner's feature (a namespace usage when owner
// is 0), identities first to first+count-1 reserved for them in order.
type requiredMembers struct {
	// ctx is the context the identities are reserved in.
	ctx     *Context
	typ     *symbols.Symbol
	owner   int64
	feature string
	first   int64
	count   int64
	// reached is the activation the population was reached in, which each member was reached in too.
	reached int64
}

// at is the i-th member, by its reserved identity.
func (r *requiredMembers) at(i int64) Value {
	return Value{Kind: ValInstance, Instance: r.first + i}
}

// holds reports whether id is reserved for one of the members.
func (r *requiredMembers) holds(id int64) bool {
	return id >= r.first && id-r.first < r.count
}

// requiredTail is the sequence val is when it ends in required members, else nil.
func requiredTail(val Value) *Sequence {
	if val.Kind != ValSequence {
		return nil
	}
	if seq := val.Sequence(); seq != nil && seq.required != nil {
		return seq
	}
	return nil
}

// withRequired is the sequence of made elements followed by count members of typ that
// owner's feature holds, their identities reserved now and their objects made when reached.
func (ctx *Context) withRequired(made []Value, typ *symbols.Symbol, owner *Instance, feature string, count int64) (*Sequence, error) {
	first := ctx.ids.next
	if count <= 0 || count > math.MaxInt64-first || count > int64(math.MaxInt-len(made)) {
		return nil, fmt.Errorf("%w: %d required values exceed the identities a context can reserve", ErrElementLimitExceeded, count)
	}
	first = ctx.allocateID()
	ctx.claimID(first + count - 1)
	r := &requiredMembers{ctx: ctx, typ: typ, feature: feature, first: first, count: count, reached: ctx.activations}
	if owner != nil {
		r.owner = owner.ID
	}
	ctx.registerRequired(r)
	return &Sequence{elements: made, required: r}, nil
}

// registerRequired records r, so its identities are reached as the members they name.
func (ctx *Context) registerRequired(r *requiredMembers) {
	i, found := slices.BinarySearchFunc(ctx.required, r.first, func(e *requiredMembers, id int64) int {
		return cmp.Compare(e.first, id)
	})
	if found {
		return
	}
	ctx.required = slices.Insert(ctx.required, i, r)
	ctx.claimID(r.first + r.count - 1)
	ctx.noteProbeUndo(func() {
		ctx.required = slices.DeleteFunc(ctx.required, func(e *requiredMembers) bool { return e == r })
	})
}

// requiredOf is the population id is reserved in, while the object holding it lives.
func (ctx *Context) requiredOf(id int64) (*requiredMembers, bool) {
	i := sort.Search(len(ctx.required), func(i int) bool { return ctx.required[i].first > id }) - 1
	if i < 0 || !ctx.required[i].holds(id) {
		return nil, false
	}
	r := ctx.required[i]
	if r.owner != 0 {
		if _, live := ctx.instances[r.owner]; !live {
			return nil, false
		}
	}
	return r, true
}

// requiredMember is the member of r with identity id, made now if it was not before, as
// the eager fill would have made it: held by r's owner, alive with it, reached with the rest.
func (ctx *Context) requiredMember(r *requiredMembers, id int64) (*Instance, error) {
	if inst, ok := ctx.instances[id]; ok {
		return inst, nil
	}
	var owner *Instance
	if r.owner != 0 {
		owner = ctx.instances[r.owner]
	}
	mark := len(ctx.created)
	inst, err := ctx.materialize(r.typ, id, owner, r.feature)
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	if fv := ctx.droppedFrom(r, owner); fv != nil {
		inst.owner, inst.ownerFeature = ctx.otherHomeOf(inst, fv)
	}
	if l, ok := ctx.lives[inst.ID]; ok {
		if l.began == l.reached {
			l.began = r.reached
		}
		l.reached = r.reached
		ctx.lives[inst.ID] = l
	}
	return inst, nil
}

// droppedFrom is the feature of owner a write took r's members out of, as releaseDropped
// released the made ones from it; nil while the feature still holds them.
func (ctx *Context) droppedFrom(r *requiredMembers, owner *Instance) *FeatureValue {
	if owner == nil {
		return nil
	}
	fv := owner.FeatureValues[r.feature]
	if fv == nil || !fv.Materialized {
		return nil
	}
	if seq := requiredTail(fv.HeldValue()); seq != nil && seq.required.first == r.first && seq.required.count == r.count {
		return nil
	}
	return fv
}

// reachedObject is the object id names, a required member made now; ok is false for an
// identity the run holds no object under.
func (ctx *Context) reachedObject(id int64) (inst *Instance, ok bool, err error) {
	if inst, ok := ctx.instances[id]; ok {
		return inst, true, nil
	}
	r, ok := ctx.requiredOf(id)
	if !ok {
		return nil, false, nil
	}
	inst, err = ctx.requiredMember(r, id)
	return inst, err == nil, err
}

// heldInFull is val with every required member it ends in made, the work of reaching
// every value: each is charged to the element budget, past which it is refused.
func (ctx *Context) heldInFull(val Value) (Value, error) {
	seq := requiredTail(val)
	if seq == nil {
		return val, nil
	}
	if err := ctx.makeRequired(seq); err != nil {
		return Value{}, err
	}
	return NewSequenceValue(&Sequence{elements: seq.Elements(), elementUnit: seq.elementUnit}), nil
}

// makeRequired makes every required member seq ends in.
func (ctx *Context) makeRequired(seq *Sequence) error {
	r := seq.required
	release := ctx.elementScope()
	defer release()
	if err := ctx.chargeElements(int64(seq.Size())); err != nil {
		return fmt.Errorf("reading all %d values of %s: %w", seq.Size(), ctx.requiredHolder(r), err)
	}
	mark := len(ctx.created)
	for i := int64(0); i < r.count; i++ {
		if _, err := ctx.requiredMember(r, r.first+i); err != nil {
			ctx.abandonInstancesSince(mark)
			return fmt.Errorf("reading all %d values of %s: %w", seq.Size(), ctx.requiredHolder(r), err)
		}
	}
	return nil
}

// requiredHolder names the feature or usage holding r, for an error.
func (ctx *Context) requiredHolder(r *requiredMembers) string {
	if r.owner == 0 {
		return symbolText(r.typ)
	}
	return fmt.Sprintf("feature %q", r.feature)
}

// requiredAt is the element of seq at index (0-based), its object made if it is a
// required member not made yet; the rest are left as they are.
func (ctx *Context) requiredAt(seq *Sequence, index int) (Value, error) {
	val, err := seq.At(index)
	if err != nil || index < len(seq.elements) {
		return val, err
	}
	if _, err := ctx.requiredMember(seq.required, val.Instance); err != nil {
		return Value{}, err
	}
	return val, nil
}

// madeRequired is the members of r already made, in order.
func (ctx *Context) madeRequired(r *requiredMembers) []*Instance {
	var out []*Instance
	if int64(len(ctx.instances)) < r.count {
		for id, inst := range ctx.instances {
			if r.holds(id) {
				out = append(out, inst)
			}
		}
		slices.SortFunc(out, func(a, b *Instance) int { return cmp.Compare(a.ID, b.ID) })
		return out
	}
	for i := int64(0); i < r.count; i++ {
		if inst, ok := ctx.instances[r.first+i]; ok {
			out = append(out, inst)
		}
	}
	return out
}

// standingElements is what val holds as it stands: a required member only once it is made.
func standingElements(val Value) []Value {
	seq := requiredTail(val)
	if seq == nil {
		return elementsOf(val)
	}
	out := slices.Clone(seq.elements)
	for _, inst := range seq.required.ctx.madeRequired(seq.required) {
		out = append(out, Value{Kind: ValInstance, Instance: inst.ID})
	}
	return out
}

// listedElements is what val holds but its required members, which no write names.
func listedElements(val Value) []Value {
	if seq := requiredTail(val); seq != nil {
		return seq.elements
	}
	return elementsOf(val)
}

// heldUpTo is the first limit values val holds, each required member among them made;
// cut is true when val holds more.
func (ctx *Context) heldUpTo(val Value, limit int) (out []Value, cut bool, err error) {
	seq := requiredTail(val)
	if seq == nil {
		elements := elementsOf(val)
		if len(elements) > limit {
			return elements[:max(limit, 0)], true, nil
		}
		return elements, false, nil
	}
	n := min(seq.Size(), max(limit, 0))
	out = make([]Value, 0, n)
	for i := range n {
		elem, err := ctx.requiredAt(seq, i)
		if err != nil {
			return out, true, err
		}
		out = append(out, elem)
	}
	return out, seq.Size() > n, nil
}

// sameRequired reports whether two sequences end in the one population.
func sameRequired(a, b *Sequence) bool {
	if a.required == nil || b.required == nil {
		return a.required == b.required
	}
	return a.required.first == b.required.first && a.required.count == b.required.count && a.required.typ == b.required.typ
}

// formatRequired spells the required members seq ends in, by the range of their identities.
func formatRequired(seq *Sequence) string {
	r := seq.required
	return fmt.Sprintf("%d of %s #%d..#%d", r.count, symbolText(r.typ), r.first, r.first+r.count-1)
}

// HeldElements is what a collection value holds, in order — a lazily held member named by
// the identity reserved for it, whose object Instance makes — or the value itself when it
// is no collection. One holding more than the element budget is ErrElementLimitExceeded.
func (ctx *Context) HeldElements(val Value) ([]Value, error) {
	seq := requiredTail(val)
	if seq != nil && ctx == nil {
		ctx = seq.required.ctx
	}
	if seq != nil && int64(seq.Size()) > ctx.maxElements {
		return nil, fmt.Errorf("listing all %d values of %s: %w (%d elements; raise %s to allow more)",
			seq.Size(), ctx.requiredHolder(seq.required), ErrElementLimitExceeded, ctx.maxElements, MaxElementsEnvVar)
	}
	return elementsOf(val), nil
}

// memberPosition is the 0-based position of id in a sequence ending in required members, or -1.
func memberPosition(seq *Sequence, id int64) int {
	if i := slices.IndexFunc(seq.elements, func(v Value) bool { return v.Kind == ValInstance && v.Instance == id }); i >= 0 {
		return i
	}
	if seq.required.holds(id) {
		return len(seq.elements) + int(id-seq.required.first)
	}
	return -1
}

// holdsMember reports whether val is, or holds, the object id.
func holdsMember(val Value, id int64) bool {
	if seq := requiredTail(val); seq != nil {
		return memberPosition(seq, id) >= 0
	}
	return slices.Contains(heldObjects(val), id)
}

// carriedRequired is r as a population of ctx under the same identities, typ its members'
// type there: r itself, the one ctx already reserved them for, or a copy reserved now. An
// identity in the range ctx holds an object under, of those made outside keep, is ErrImageIdentityTaken.
func (ctx *Context) carriedRequired(r *requiredMembers, typ *symbols.Symbol, keep map[int64]bool) (*requiredMembers, error) {
	if r.ctx == ctx && r.typ == typ {
		return r, nil
	}
	if prior, ok := ctx.requiredOf(r.first); ok && prior.first == r.first && prior.count == r.count && prior.typ == typ {
		return prior, nil
	}
	i := sort.Search(len(ctx.required), func(i int) bool { return ctx.required[i].first+ctx.required[i].count > r.first })
	if i < len(ctx.required) && ctx.required[i].first < r.first+r.count {
		return nil, fmt.Errorf("%w: identities #%d to #%d are reserved for other members", ErrImageIdentityTaken, r.first, r.first+r.count-1)
	}
	for _, inst := range ctx.madeRequired(r) {
		if !keep[inst.ID] {
			return nil, fmt.Errorf("%w: object #%d", ErrImageIdentityTaken, inst.ID)
		}
	}
	carried := &requiredMembers{ctx: ctx, typ: typ, owner: r.owner, feature: r.feature, first: r.first, count: r.count, reached: r.reached}
	ctx.registerRequired(carried)
	return carried, nil
}

// ElementCount is how many values val holds: a collection's elements, required members
// counted made or not; none for null; one for any other value.
func ElementCount(val Value) int64 {
	return elementCount(&val)
}

// ElementAt is the value at the 0-based index of a collection, a required member's object
// made once it is reached; any other value is its only element.
func (ctx *Context) ElementAt(val Value, index int) (Value, error) {
	if seq := requiredTail(val); seq != nil {
		return ctx.requiredAt(seq, index)
	}
	elements := elementsOf(val)
	if index < 0 || index >= len(elements) {
		return Value{}, fmt.Errorf("%w: index %d is outside 0..%d", ErrIndexOutOfRange, index, len(elements)-1)
	}
	return elements[index], nil
}

// MadeElements is what a collection holds of values already there, each with its 0-based
// position: every element, but of the required members only those already made.
func (ctx *Context) MadeElements(val Value) ([]int, []Value) {
	seq := requiredTail(val)
	if seq == nil {
		elements := elementsOf(val)
		positions := make([]int, len(elements))
		for i := range positions {
			positions[i] = i
		}
		return positions, elements
	}
	elements := standingElements(val)
	positions := make([]int, len(elements))
	for i, elem := range elements {
		if i < len(seq.elements) {
			positions[i] = i
		} else {
			positions[i] = len(seq.elements) + int(elem.Instance-seq.required.first)
		}
	}
	return positions, elements
}
