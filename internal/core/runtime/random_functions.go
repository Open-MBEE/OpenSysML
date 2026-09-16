package runtime

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// RandomFunctionsFQN names the OpenSysML library of random functions.
const RandomFunctionsFQN = "RandomFunctions"

// registerRandomFunctions registers the OpenSysML RandomFunctions library
// (internal/core/libs/stdlib/OpenSysML Libraries/RandomFunctions.kerml): each call
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

// drawUniform is RandomFunctions::uniform: a Real uniform on [lo, hi), lo at most hi.
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
			return drawnReal(math.Min(lo+rng.Float64()*(hi-lo), math.Nextafter(hi, lo)))
		},
		admits: realHalfOpen(lo, hi),
	})
}

// drawUniformInteger is RandomFunctions::uniformInteger: an Integer uniform on
// [lo, hi], both ends included, lo at most hi; the span is counted unsigned so
// the whole Integer range stays exact.
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
	})
}

// drawTriangular is RandomFunctions::triangular: a Real on [lo, hi] densest at
// mode, lo at most mode at most hi and lo below hi, drawn by the inverse of its
// distribution function.
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
			u := rng.Float64()
			if cut := (mode - lo) / (hi - lo); u < cut {
				return drawnReal(lo + math.Sqrt(u*(hi-lo)*(mode-lo)))
			}
			return drawnReal(hi - math.Sqrt((1-u)*(hi-lo)*(hi-mode)))
		},
		admits: realWithin(lo, hi),
	})
}

// drawNormal is RandomFunctions::normal: a Real normal about mean with standard
// deviation sd, which is not negative; a zero sd draws mean.
func drawNormal(ctx *Context, name string, args []semantics.Value) (semantics.Value, error) {
	mean, sd := asReal(args[0]), asReal(args[1])
	if err := finiteBounds(name, args); err != nil {
		return semantics.Value{}, err
	}
	if sd < 0 {
		return semantics.Value{}, fmt.Errorf("%w: normal(%s, %s): sd is negative", ErrRandomDomain, formatDrawn(args[0]), formatDrawn(args[1]))
	}
	return ctx.draw(drawCall(name, args), distribution{
		draw: func(rng *rand.Rand) semantics.Value { return drawnReal(mean + sd*rng.NormFloat64()) },
		admits: func(v semantics.Value) bool {
			if sd == 0 {
				return v.Kind == semantics.ValReal && v.Real == mean
			}
			return v.Kind == semantics.ValReal && !math.IsInf(v.Real, 0) && !math.IsNaN(v.Real)
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
