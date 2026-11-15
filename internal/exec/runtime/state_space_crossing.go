package runtime

import (
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// A zero crossing is located inside a step: the run previews its guards at the
// quarters of the step, bisects the first crossing and takes its next sample there.

// crossingQuarters is how many equal parts of the span left a preview samples.
const crossingQuarters = 4

// crossingDipBits bounds the search for a guard's nearest approach to zero between
// two samples: 2^-crossingDipBits of the window, a smooth minimum's resolution.
const crossingDipBits = 26

// guardPreview is the guards the run would hold span past its latest sample.
type guardPreview struct {
	span   float64
	guards []float64
}

// locateCrossing sets run.cut to the span to the first zero crossing before the
// step's end, zero where the preview finds none or fails: the step then runs whole.
func (e *ActionExecutor) locateCrossing(run *stateSpaceRun) {
	before, lead := run.before, run.lead
	run.cut, run.before, run.lead = 0, nil, 0
	if len(run.dyn.Crossings) == 0 || !run.settled() {
		return
	}
	state, err := e.stateVector()
	if err != nil {
		return
	}
	input, err := e.inputVector()
	if err != nil {
		return
	}
	preview := func(span float64) ([]float64, bool) { return e.previewGuards(run, state, input, span) }

	rest := run.step - run.into
	var samples []guardPreview
	now := 0
	if before != nil {
		samples, now = append(samples, guardPreview{-lead, before}), 1
	}
	samples = append(samples, guardPreview{0, run.guards})
	for i := 1; i <= crossingQuarters; i++ {
		span := rest
		if i < crossingQuarters {
			span = rest * float64(i) / crossingQuarters
		}
		guards, ok := preview(span)
		if !ok {
			return
		}
		samples = append(samples, guardPreview{span, guards})
	}
	last := len(samples) - 1
	// Past the step's end the preview extrapolates, only to see a guard turn back there.
	if guards, ok := preview(rest * (crossingQuarters + 1) / crossingQuarters); ok {
		samples = append(samples, guardPreview{rest * (crossingQuarters + 1) / crossingQuarters, guards})
	}

	cut, short := rest, 0.0
	for i := range run.dyn.Crossings {
		search := crossingSearch{
			guard: func(span float64) (float64, bool) {
				guards, ok := preview(span)
				if !ok {
					return 0, false
				}
				return guards[i], true
			},
			limit: rest,
		}
		spans, guards := make([]float64, len(samples)), make([]float64, len(samples))
		for j, sample := range samples {
			spans[j], guards[j] = sample.span, sample.guards[i]
		}
		lo, at, found, ok := search.first(spans, guards, now, run.sides[i])
		if !ok {
			return
		}
		if found && at < cut {
			cut, short = at, lo
		}
	}
	if cut < rest {
		// The guards just short of the crossing stand as the sample before it, so a
		// guard that left zero unseen and is back at it has crossed.
		if short > 0 {
			guards, ok := preview(short)
			if !ok {
				return
			}
			run.hold(guards)
		}
		run.cut = cut
		return
	}
	// The last preview inside the step stands as the sample before the step's end,
	// so a guard resting at zero that left it unseen and is back at the end has crossed.
	run.before, run.lead = samples[last-1].guards, rest-samples[last-1].span
	run.hold(samples[last-1].guards)
}

// previewGuards is every guard span past the run's latest sample, over the state
// the integrator reaches there; false where one cannot be had or is not finite.
func (e *ActionExecutor) previewGuards(run *stateSpaceRun, state Value, input *Value, span float64) ([]float64, bool) {
	defer e.ctx.beginProbe()()
	next, err := e.nextState(run, state, input, span)
	if err != nil || e.checkFinite(next) != nil {
		return nil, false
	}
	if _, err := e.holdFeature(lower.StateSpaceFeature, next); err != nil {
		return nil, false
	}
	if run.dyn.Time != nil {
		if _, err := e.holdFeature(lower.TimeFeature, e.instantValue(run.after(span))); err != nil {
			return nil, false
		}
	}
	output, err := e.invokeProtocolCalc(run.dyn.OutputCalc, lower.GetOutputCalc, input, next, nil)
	if err != nil {
		return nil, false
	}
	if _, err := e.checkVector(lower.OutputFeature, output); err != nil {
		return nil, false
	}
	if _, err := e.holdFeature(lower.OutputFeature, output); err != nil {
		return nil, false
	}
	guards := make([]float64, len(run.dyn.Crossings))
	for i, crossing := range run.dyn.Crossings {
		guard, err := e.evalGuard(crossing)
		if err != nil || math.IsNaN(guard) || math.IsInf(guard, 0) {
			return nil, false
		}
		guards[i] = guard
	}
	return guards, true
}

// crossingSearch looks for one guard's first crossing among previewed samples:
// guard evaluates it at a span, and limit is the span the search stops at.
type crossingSearch struct {
	guard func(span float64) (float64, bool)
	limit float64
}

// first is the span of the guard's first crossing after sample now and the span just
// short of it. side is the last nonzero guard up to now; ok is false on a failed guard.
func (s crossingSearch) first(spans, guards []float64, now int, side float64) (short, at float64, found, ok bool) {
	for i := now; i < len(spans)-1; i++ {
		if guards[i] != 0 {
			side = guards[i]
		} else if returns(side, guards[i+1]) && spans[i] < s.limit {
			// Resting at zero, then back on the side it arrived from: it may have
			// left for the other side in between, and crossed again coming back.
			hi := math.Min(spans[i+1], s.limit)
			flip, other, found, ok := s.dip(spans[i], hi, side, true)
			if !ok {
				return 0, 0, false, false
			}
			if found && hi == spans[i+1] {
				return s.bisect(flip, other, hi)
			}
			if found {
				break
			}
		}
		if i > 0 && dips(guards[i-1], guards[i], guards[i+1]) && math.Max(spans[i-1], 0) < s.limit {
			lo := math.Max(spans[i-1], 0)
			flip, _, found, ok := s.dip(lo, math.Min(spans[i+1], s.limit), guards[i], false)
			if !ok {
				return 0, 0, false, false
			}
			if found {
				// The window's start holds the sign of its middle sample.
				return s.bisect(lo, guards[i], flip)
			}
		}
		if spans[i] >= s.limit || spans[i+1] > s.limit {
			break
		}
		if crosses(guards[i], guards[i+1]) {
			return s.bisect(spans[i], guards[i], spans[i+1])
		}
	}
	return 0, 0, false, true
}

// dips reports three consecutive samples of one sign whose middle one is nearest
// zero: between its neighbours the guard turned back, and may have reached zero.
func dips(before, at, after float64) bool {
	if before == 0 || at == 0 || after == 0 ||
		math.Signbit(before) != math.Signbit(at) || math.Signbit(after) != math.Signbit(at) {
		return false
	}
	before, at, after = math.Abs(before), math.Abs(at), math.Abs(after)
	return at <= before && at <= after && (at < before || at < after)
}

// returns reports a guard back on the side it was on before it reached zero.
func returns(side, guard float64) bool {
	return side != 0 && guard != 0 && math.Signbit(side) == math.Signbit(guard)
}

// dip descends by golden section from lo to hi towards the side opposite was, and
// stops at the first span where the guard has crossed, or, leaving a zero, changed side.
func (s crossingSearch) dip(lo, hi, was float64, leaving bool) (flip, guard float64, found, ok bool) {
	const ratio = 0.6180339887498949 // (√5 − 1) / 2
	// depth is how far the guard stands on was's side: negative once it has crossed.
	depth := func(guard float64) float64 { return math.Copysign(1, was) * guard }
	crossed := func(guard float64) bool {
		if leaving {
			return depth(guard) < 0
		}
		return crosses(was, guard)
	}
	width := hi - lo
	left, right := hi-float64(ratio*(hi-lo)), lo+float64(ratio*(hi-lo))
	atLeft, ok := s.guard(left)
	if !ok {
		return 0, 0, false, false
	}
	if crossed(atLeft) {
		return left, atLeft, true, true
	}
	atRight, ok := s.guard(right)
	if !ok {
		return 0, 0, false, false
	}
	if crossed(atRight) {
		return right, atRight, true, true
	}
	for hi-lo > math.Ldexp(width, -crossingDipBits) {
		var probe float64
		if depth(atLeft) < depth(atRight) {
			hi, right, atRight = right, left, atLeft
			left = hi - float64(ratio*(hi-lo))
			probe = left
		} else {
			lo, left, atLeft = left, right, atRight
			right = lo + float64(ratio*(hi-lo))
			probe = right
		}
		if probe <= lo || probe >= hi {
			break
		}
		at, ok := s.guard(probe)
		if !ok {
			return 0, 0, false, false
		}
		if crossed(at) {
			return probe, at, true, true
		}
		if probe == left {
			atLeft = at
		} else {
			atRight = at
		}
	}
	return 0, 0, false, true
}

// bisect narrows a crossing between lo, where the guard is was, and hi, where it
// has crossed, to two adjacent float64 spans, the earlier and the later.
func (s crossingSearch) bisect(lo, was, hi float64) (short, at float64, found, ok bool) {
	for {
		mid := lo + (hi-lo)/2
		if mid <= lo || mid >= hi {
			return lo, hi, true, true
		}
		guard, ok := s.guard(mid)
		if !ok {
			return 0, 0, false, false
		}
		if crosses(was, guard) {
			hi = mid
		} else {
			lo, was = mid, guard
		}
	}
}
