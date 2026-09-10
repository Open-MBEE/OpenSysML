- **Function values compile natively.** `sysml -compile` (C and Go) now accepts a calc that
  takes an `in calc` parameter, a calc def, a calc usage with an unsupplied input or a compiled
  library function (`RealFunctions::sqrt`, `floor`) passed for one, `f(a)` and `f(v = a)` in the
  body — positionally, by name, passed on to another calc and through recursion — and
  `SampledFunctions::Sample(f, xs)` bound to a `SampledFunction` attribute or read by `Domain` and
  `Range`. The compiler fixes each function value at compile time and compiles the callee once
  per distinct binding, so `f(a)` is a direct call and computes, prints and fails exactly as the
  interpreter's invocation does (`Apply(Recip, 0.0)` divides by zero, `Sample` reports its first
  failing element, a null domain samples to `[]`). What a compile-time value cannot express keeps
  a typed refusal naming the construct: a function value returned, stored, compared, chosen by an
  `if` at run time or handed to a value parameter, a calc owned by a part or declared in a
  behavior body (its value closes over that object or run), an entry calc's own `in calc`
  parameter (a program cannot take one on its command line), a control operation such as
  `collect` passed as a function, and a `SampledFunction` used as anything but the operand of
  `Domain` or `Range`. `docs/project/native-compilation.md` states the representation and its
  trade-off.
