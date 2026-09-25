package runtime

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// RandomFunctionsFQN names the OpenSysML library of random functions.
const RandomFunctionsFQN = "RandomFunctions"

// registerRandomFunctions registers the OpenSysML RandomFunctions library
// (internal/workspace/libs/stdlib/OpenSysML Libraries/RandomFunctions.kerml): each call
// is a draw from the run's modeled stream, recorded for the trace and the witness.
func registerRandomFunctions() {
	registerContextFunction(RandomFunctionsFQN+"::uniform", []string{"lo", "hi"}, drawUniform)
	registerContextFunction(RandomFunctionsFQN+"::uniformInteger", []string{"lo", "hi"}, drawUniformInteger, integerDomain, integerDomain)
	registerContextFunction(RandomFunctionsFQN+"::triangular", []string{"lo", "mode", "hi"}, drawTriangular)
	registerContextFunction(RandomFunctionsFQN+"::normal", []string{"mean", "sd"}, drawNormal)
}

// drawCall spells the call a draw records: the function's own name over its
// arguments as evaluated, `uniform(1, 80)`.
func drawCall(name string, args []semantics.Value) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+len("::"):]
	}
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = formatDrawn(arg)
	}
	return name + "(" + strings.Join(parts, ", ") + ")"
}

// drawUniform is RandomFunctions::uniform: a Real uniform on [lo, hi), lo at most
// hi; the fixed policies yield lo, hi (the bound, though no random draw reaches it) and their midpoint.
func drawUniform(ctx *Context, name string, args []semantics.Value) (semantics.Value, error) {
	lo, hi := asReal(args[0]), asReal(args[1])
	if err := finiteBounds(name, args); err != nil {
		return semantics.Value{}, err
	}
	if lo > hi {
		return semantics.Value{}, fmt.Errorf("%w: uniform(%s, %s): lo exceeds hi", ErrRandomDomain, formatDrawn(args[0]), formatDrawn(args[1]))
	}
	return ctx.draw(drawCall(name, args), distribution{
		draw: func(rng *rand.Rand) semantics.Value {
			// Rounding at the top of a wide range may land on hi; the interval excludes it.
			return drawnReal(math.Min(between(lo, hi, rng.Float64()), math.Nextafter(hi, lo)))
		},
		admits: realHalfOpen(lo, hi),
		fixed:  realPoints(lo, hi, between(lo, hi, 0.5)),
	})
}

// drawUniformInteger is RandomFunctions::uniformInteger: an Integer uniform on
// [lo, hi], both ends included, lo at most hi; the span is counted unsigned so
// the whole Integer range stays exact. The average is the midpoint, a half
// rounded toward hi.
func drawUniformInteger(ctx *Context, name string, args []semantics.Value) (semantics.Value, error) {
	lo, hi := args[0].Int, args[1].Int
	if lo > hi {
		return semantics.Value{}, fmt.Errorf("%w: uniformInteger(%d, %d): lo exceeds hi", ErrRandomDomain, lo, hi)
	}
	span := unsignedInt(hi) - unsignedInt(lo)
	return ctx.draw(drawCall(name, args), distribution{
		draw: func(rng *rand.Rand) semantics.Value {
			return semantics.Value{Kind: semantics.ValInt, Int: signedInt(unsignedInt(lo) + drawOffset(rng, span))}
		},
		admits: func(v semantics.Value) bool { return v.Kind == semantics.ValInt && lo <= v.Int && v.Int <= hi },
		fixed: func(policy DrawPolicy) (semantics.Value, bool) {
			var n int64
			switch policy {
			case DrawMin:
				n = lo
			case DrawMax:
				n = hi
			case DrawAverage:
				n = signedInt(unsignedInt(lo) + span/2 + span%2)
			default:
				return semantics.Value{}, false
			}
			return semantics.Value{Kind: semantics.ValInt, Int: n}, true
		},
	})
}

// drawTriangular is RandomFunctions::triangular: a Real on [lo, hi] densest at
// mode, lo at most mode at most hi and lo below hi, drawn by the inverse of its
// distribution function; its average is the distribution's mean (lo + mode + hi) / 3.
func drawTriangular(ctx *Context, name string, args []semantics.Value) (semantics.Value, error) {
	lo, mode, hi := asReal(args[0]), asReal(args[1]), asReal(args[2])
	if err := finiteBounds(name, args); err != nil {
		return semantics.Value{}, err
	}
	if !(lo <= mode && mode <= hi && lo < hi) {
		return semantics.Value{}, fmt.Errorf("%w: triangular(%s, %s, %s): needs lo <= mode <= hi with lo < hi",
			ErrRandomDomain, formatDrawn(args[0]), formatDrawn(args[1]), formatDrawn(args[2]))
	}
	return ctx.draw(drawCall(name, args), distribution{
		draw: func(rng *rand.Rand) semantics.Value {
			u, cut := rng.Float64(), fractionOf(lo, mode, hi)
			if u < cut {
				return drawnReal(between(lo, hi, math.Sqrt(u*cut)))
			}
			return drawnReal(between(lo, hi, 1-math.Sqrt((1-u)*(1-cut))))
		},
		admits: realWithin(lo, hi),
		fixed:  realPoints(lo, hi, lo/3+mode/3+hi/3),
	})
}

// drawNormal is RandomFunctions::normal: a finite Real about mean with sd >= 0
// (zero draws mean); a tail overflowing to infinity is drawn again. Its average is
// mean; with sd > 0 it has no least or greatest value, so `min` and `max` refuse it.
func drawNormal(ctx *Context, name string, args []semantics.Value) (semantics.Value, error) {
	mean, sd := asReal(args[0]), asReal(args[1])
	if err := finiteBounds(name, args); err != nil {
		return semantics.Value{}, err
	}
	if sd < 0 {
		return semantics.Value{}, fmt.Errorf("%w: normal(%s, %s): sd is negative", ErrRandomDomain, formatDrawn(args[0]), formatDrawn(args[1]))
	}
	return ctx.draw(drawCall(name, args), distribution{
		draw: func(rng *rand.Rand) semantics.Value {
			for {
				if x := mean + sd*rng.NormFloat64(); !math.IsInf(x, 0) {
					return drawnReal(x)
				}
			}
		},
		admits: func(v semantics.Value) bool {
			if sd == 0 {
				return v.Kind == semantics.ValReal && v.Real == mean
			}
			return v.Kind == semantics.ValReal && !math.IsInf(v.Real, 0) && !math.IsNaN(v.Real)
		},
		fixed: func(policy DrawPolicy) (semantics.Value, bool) {
			if policy == DrawAverage || sd == 0 {
				return drawnReal(mean), true
			}
			return semantics.Value{}, false
		},
	})
}

// finiteBounds refuses a distribution parameter that is infinite or not a number:
// no distribution is bounded by it.
func finiteBounds(name string, args []semantics.Value) error {
	for _, arg := range args {
		if x := asReal(arg); math.IsInf(x, 0) || math.IsNaN(x) {
			return fmt.Errorf("%w: %s: %s is not a finite number", ErrRandomDomain, drawCall(name, args), formatDrawn(arg))
		}
	}
	return nil
}

// drawnReal is x as a Real value.
func drawnReal(x float64) semantics.Value {
	return semantics.Value{Kind: semantics.ValReal, Real: x}
}

// between is the point the fraction t in [0, 1] of the way from lo to hi, clamped
// to [lo, hi]; the span hi-lo is formed only where it is finite.
func between(lo, hi, t float64) float64 {
	x := lo*(1-t) + hi*t
	if span := hi - lo; !math.IsInf(span, 0) {
		x = lo + t*span
	}
	return math.Max(lo, math.Min(x, hi))
}

// fractionOf is where x in [lo, hi] lies between lo and hi, in [0, 1], for lo < hi;
// halving keeps a span too wide for a float finite.
func fractionOf(lo, x, hi float64) float64 {
	if span := hi - lo; !math.IsInf(span, 0) {
		return (x - lo) / span
	}
	return (x/2 - lo/2) / (hi/2 - lo/2)
}

// realWithin admits a Real on [lo, hi]: what a bounded distribution can draw.
func realWithin(lo, hi float64) func(v semantics.Value) bool {
	return func(v semantics.Value) bool { return v.Kind == semantics.ValReal && lo <= v.Real && v.Real <= hi }
}

// realHalfOpen admits a Real on [lo, hi), or lo alone when lo == hi: what uniform can draw.
func realHalfOpen(lo, hi float64) func(v semantics.Value) bool {
	return func(v semantics.Value) bool {
		return v.Kind == semantics.ValReal && lo <= v.Real && (v.Real < hi || v.Real == lo)
	}
}
