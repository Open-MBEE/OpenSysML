package Model {
    package Signals {
        constraint def 'Sine Wave' {
            in attribute sine_wave : ScalarValues::Real[1];
            in attribute t : ScalarValues::Real[1];
            sine_wave == TrigFunctions::sin(t)
        }
        constraint def 'Damped Amplitude' {
            in attribute amplitude : ScalarValues::Real[1];
            in attribute x : ScalarValues::Real[1];
            in attribute y : ScalarValues::Real[1];
            in attribute t : ScalarValues::Real[1];
            amplitude == RealFunctions::sqrt(x ** 2 + y ** 2) * OpenSysMLMathFunctions::exp(-RealFunctions::abs(t)) and RealFunctions::floor(t) <= RealFunctions::max(x, y)
        }
        constraint def Clamp {
            in attribute clamped : ScalarValues::Real[1];
            in attribute x : ScalarValues::Real[1];
            in attribute phase : ScalarValues::Real[1];
            /* not migrated: Constraint 'dotted' {English} clamped = Math.max(x, 0) — the construct "Math.max(" is outside the translated subset: a call is not English; the subset reads names, literals, comparisons, not/and/or and the functions abs, acos, arccos, arcsin, arctan, asin, atan, atan2, ceil, ceiling, cos, cot, exp, floor, ln, log, max, min, pow, round, sin, sqrt, tan */
            /* not migrated: Constraint 'arity' {English} clamped = sin(x, phase) — the call "sin" is not in the translated function table: sin takes 1 argument(s), not 2 */
            /* not migrated: Constraint 'words' {English} clamped = sin phase — the name "sin phase" resolves to nothing readable: nothing visible from Signals::Clamp is called sin phase */
        }
    }
}
