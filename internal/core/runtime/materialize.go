package runtime

import "github.com/Open-MBEE/OpenSysML/internal/core/symbols"

const (
	// maxMaterializeDepth bounds how deep a materialization walk descends into the
	// objects an object holds, as a feature value listing is bounded.
	maxMaterializeDepth = 8
	// maxMaterializeBudget bounds the walk as a whole, charged per feature value read and
	// per object a feature value holds: nesting multiplies, and reading a feature value materializes
	// the objects it holds, so breadth costs objects rather than only time.
	maxMaterializeBudget = 1000
)

// MaterializationErrors reads every feature value of an object, and of the objects its
// feature values hold, and returns what materializing them reported, in the order the
// feature values were read. Feature values are lazy, so a default that does not conform to its
// feature's multiplicity is only found by reading it: a caller reporting on an
// object it created calls this rather than leaving those diagnostics to whoever
// reads a feature value next. bounded is true when the walk did not read every feature value —
// its budget was spent, or nesting it does not descend into was elided — so
// what it did not reach is unreported rather than clean.
func (ctx *Context) MaterializationErrors(inst *Instance) (errs []error, bounded bool) {
	if inst == nil {
		return nil, false
	}
	w := &materializeWalk{
		ctx:     ctx,
		onPath:  map[*symbols.Symbol]bool{inst.Type: true},
		visited: map[int64]bool{inst.ID: true},
		read:    map[*FeatureValue]bool{},
		budget:  maxMaterializeBudget,
	}
	w.walk(inst, 0)
	return w.errs, w.bounded
}

// materializeWalk reads an object graph under the bounds a feature value listing uses:
// onPath holds the types being expanded above the current one, since a part
// containing its own kind materializes a fresh object per descent; depth bounds
// the descent and budget the walk as a whole. visited keeps an object two
// features hold from being reported twice, and read the one feature value a feature and
// the feature redefining it share.
type materializeWalk struct {
	ctx     *Context
	onPath  map[*symbols.Symbol]bool
	visited map[int64]bool
	read    map[*FeatureValue]bool
	budget  int
	bounded bool
	errs    []error
}

func (w *materializeWalk) walk(inst *Instance, depth int) {
	// A destroyed object holds no feature values to materialize any more.
	if w.ctx.checkNotDestroyed(inst) != nil {
		return
	}
	for _, of := range w.ctx.FeaturesOfObject(inst) {
		if w.budget <= 0 {
			w.bounded = true
			return
		}
		feat := of.Feature
		// A constraint or requirement a part carries holds a verdict about the
		// object rather than a value, so there is nothing to materialize.
		if holdsVerdict(feat) {
			continue
		}
		if held := w.ctx.CompositeTypeOf(feat); held != nil && (depth >= maxMaterializeDepth || w.onPath[held]) {
			// A part deeper than the walk descends, or one of a kind being expanded
			// above it, is elided: unchecked rather than clean.
			w.bounded = true
			continue
		}
		// A redefinition names the redefined feature again, and the two names read
		// one feature value, so reading it once reports what it holds once.
		if shared := inst.FeatureValues[of.Name]; shared != nil {
			if w.read[shared] {
				continue
			}
			w.read[shared] = true
		}
		w.budget--
		fv, err := inst.GetFeatureValue(w.ctx, of.Name)
		if err != nil {
			w.errs = append(w.errs, err)
			continue
		}
		nested := heldInstances(w.ctx, fv)
		w.budget -= len(nested)
		for _, held := range nested {
			if w.budget <= 0 {
				w.bounded = true
				return
			}
			if w.visited[held.ID] {
				continue
			}
			w.visited[held.ID] = true
			w.onPath[held.Type] = true
			w.walk(held, depth+1)
			delete(w.onPath, held.Type)
		}
	}
}

// holdsVerdict reports whether a feature is a condition about the object that
// carries it rather than a feature holding values.
func holdsVerdict(feat *EffectiveFeature) bool {
	if feat.Symbol == nil {
		return false
	}
	switch feat.Symbol.Kind {
	case symbols.SymbolConstraintUsage, symbols.SymbolRequirementUsage:
		return true
	default:
		return false
	}
}

// heldInstances returns the objects a feature value holds, one or a collection of them;
// an object standing for an unset value-typed feature reads as unset and is left out.
func heldInstances(ctx *Context, fv *FeatureValue) []*Instance {
	held := fv.HeldValue()
	values := []Value{held}
	switch held.Kind {
	case ValSequence:
		values = held.Sequence().Elements()
	case ValSet:
		values = held.Set().Elements()
	}

	var out []*Instance
	for _, val := range values {
		id, ok := val.Object()
		if !ok || ctx.HoldsNoValue(val) {
			continue
		}
		if nested, ok := ctx.Instance(id); ok {
			out = append(out, nested)
		}
	}
	return out
}
