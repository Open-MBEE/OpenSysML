package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

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
	return ctx.materializationErrorsWithin(inst, maxMaterializeBudget)
}

// materializationErrorsWithin is MaterializationErrors under a budget of its own.
func (ctx *Context) materializationErrorsWithin(inst *Instance, budget int) (errs []error, bounded bool) {
	w := &materializeWalk{
		ctx:        ctx,
		root:       inst,
		onPath:     map[*symbols.Symbol]bool{inst.Type: true},
		visited:    map[int64]bool{inst.ID: true},
		restricted: map[int64]bool{},
		read:       map[*FeatureValue]bool{},
		reported:   map[featureValueRef]bool{},
		budget:     budget,
	}
	w.walk(inst, 0, false)
	return w.errs, w.bounded
}

// materializeWalk reads an object graph under the bounds a feature value listing uses:
// onPath holds the types being expanded above the current one, since a part
// containing its own kind materializes a fresh object per descent; depth bounds
// the descent and budget the walk as a whole. visited keeps an object two
// features hold from being reported twice, read the one feature value a feature and
// the feature redefining it share, and reported the feature values whose failure is recorded.
type materializeWalk struct {
	ctx     *Context
	root    *Instance
	onPath  map[*symbols.Symbol]bool
	visited map[int64]bool
	// restricted holds the objects read under unperformed only, which a full read may still reach.
	restricted map[int64]bool
	read       map[*FeatureValue]bool
	reported   map[featureValueRef]bool
	budget     int
	bounded    bool
	errs       []error
}

// walk reads inst's feature values; under unperformed (a performance nothing performs, and
// what it holds) only the values its own declarations bind are read, not what its run binds.
func (w *materializeWalk) walk(inst *Instance, depth int, unperformed bool) {
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
		if unperformed && (!w.ctx.valueBinds(feat) || w.ctx.model.semantics.IsConnectorObjectUsage(feat.Symbol)) {
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
			w.report(err)
			continue
		}
		values, cut, err := w.ctx.heldUpTo(fv.HeldValue(), w.budget+1)
		if err != nil {
			w.errs = append(w.errs, err)
		}
		w.bounded = w.bounded || cut
		held := unperformed || w.ctx.unperformedUsage(inst, feat)
		// Each object held is charged as it is reached, so every one the budget admits
		// is walked and only those past it are cut.
		for _, child := range instancesOf(w.ctx, values) {
			if w.budget <= 0 {
				w.bounded = true
				return
			}
			w.budget--
			// An object read whole is done; one read under unperformed is read whole
			// when a feature of the structure holds it later.
			if w.visited[child.ID] || (held && w.restricted[child.ID]) {
				continue
			}
			if held {
				w.restricted[child.ID] = true
			} else {
				w.visited[child.ID] = true
			}
			w.onPath[child.Type] = true
			w.walk(child, depth+1, held)
			delete(w.onPath, child.Type)
		}
	}
}

// unperformedUsage reports an action or state usage of inst that no behavior of the
// object performs or exhibits: neither under the usage itself, nor under a member
// redefining it, nor under a `perform`/`exhibit` whose bindings name it. A step of a
// running action runs as a frame, not as a behavior of the object, so its identity
// is read as an unperformed one: for the values its declarations bind.
func (ctx *Context) unperformedUsage(inst *Instance, feat *EffectiveFeature) bool {
	sym := feat.Symbol
	if sym == nil || sym.Decl == nil || (sym.Kind != symbols.SymbolActionUsage && sym.Kind != symbols.SymbolStateUsage) {
		return false
	}
	for _, b := range inst.behaviors {
		if b.member == nil {
			continue
		}
		if b.member == sym || b.member.Decl == sym.Decl || declaresAny(b.bindings, sym) ||
			declaresAny(ctx.redefinedFeatures(b.member, inst.Type), sym) {
			return false
		}
	}
	return true
}

// report records a feature value's failure once, as the innermost feature value that failed;
// a value the model leaves open (a partial binding, an unset feature read) is no finding.
func (w *materializeWalk) report(err error) {
	var undetermined *undeterminedBindingError
	var noValue *NoValueError
	if errors.As(err, &undetermined) || errors.As(err, &noValue) {
		return
	}
	failed := featureValueFailures(err)
	if len(failed) == 0 {
		w.errs = append(w.errs, err)
		return
	}
	for _, fve := range failed {
		if w.reported[fve.ref()] {
			return
		}
	}
	root := failed[len(failed)-1]
	for _, fve := range failed {
		w.reported[fve.ref()] = true
	}
	for _, fve := range failed {
		if fve.ref() == root.ref() {
			w.errs = append(w.errs, w.located(fve))
			return
		}
	}
}

// located prefixes a failure below the walked object with the usages walked to its holder,
// which the error names itself when it materializes a usage.
func (w *materializeWalk) located(fve *FeatureValueError) error {
	var path []string
	for o := fve.Instance; o != w.root; o = o.owner {
		if o == nil || o.owner == nil {
			return fve
		}
		switch {
		case o == fve.Instance && usageSymbol(o.Type):
			// The error names the usage the holder materializes.
		case usageSymbol(o.Type):
			path = append(path, o.Type.Name)
		default:
			path = append(path, o.ownerFeature)
		}
	}
	if len(path) == 0 {
		return fve
	}
	slices.Reverse(path)
	return fmt.Errorf("in %s: %w", strings.Join(path, "."), fve)
}

// ref identifies the feature value that failed.
func (e *FeatureValueError) ref() featureValueRef {
	return featureValueRef{instance: e.Instance.ID, feature: e.Feature}
}

// featureValueFailures lists the feature values that failed along err's cause chain, outermost first.
func featureValueFailures(err error) []*FeatureValueError {
	switch e := err.(type) {
	case *FeatureValueError:
		if e.Instance == nil {
			return featureValueFailures(e.Err)
		}
		return append([]*FeatureValueError{e}, featureValueFailures(e.Err)...)
	case interface{ Unwrap() error }:
		return featureValueFailures(e.Unwrap())
	case interface{ Unwrap() []error }:
		for _, inner := range e.Unwrap() {
			if failed := featureValueFailures(inner); len(failed) > 0 {
				return failed
			}
		}
	}
	return nil
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

// heldInstances returns the objects a feature value holds, one or a collection of them,
// as they stand; an object standing for an unset value-typed feature reads as unset and is left out.
func heldInstances(ctx *Context, fv *FeatureValue) []*Instance {
	return instancesOf(ctx, standingElements(fv.HeldValue()))
}

// instancesOf is the objects among values; one standing for an unset value-typed feature is left out.
func instancesOf(ctx *Context, values []Value) []*Instance {
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
