### OpenSysML Math Extension Library (non-normative)

The OMG Kernel Function Library declares no exponential, no logarithm, no
two-argument arctangent, no ceiling and no truncating Integer quotient: `RealFunctions` has
`sqrt`/`floor`/`round`/`abs`/`max`/`min`/`'**'`/`'^'`, `TrigFunctions` has
`sin`/`cos`/`tan`/`cot`/`arcsin`/`arccos`/`arctan`, `IntegerFunctions::'/'` answers a
Rational, and that is all. The vendored OMG files stay byte-identical, so the missing signatures are
declared in a clearly non-normative OpenSysML extension instead:
`internal/workspace/libs/stdlib/OpenSysML Libraries/OpenSysMLMathFunctions.kerml`. It
is bundled by the same `embed.FS` as the vendored tree and enters the same
gates — the current bundled-library gate reports 111/111 clean, including this, the
`DocumentQueries`, `IdentityMetadata`, `DiagramLayout`, `OOSEM`, `MOSA`, `StateSpaceIntegration`,
`Stochastic`, `RandomFunctions`, `Simulation`, `AnalysisRecords`, `MigrationMetadata`,
`OpenSysMLRenderings`, `StateMachines`, `SysMLValidation` and `StateActivity` extensions. It is OpenSysML code under
Apache 2.0, not OMG code under EPL-2.0; `internal/workspace/libs/stdlib/NOTICE` carves
the subdirectory out of the OMG notice.

**Reachability.** A model writes `import OpenSysMLMathFunctions::*;` (or calls
`OpenSysMLMathFunctions::exp(x)` qualified); both resolve like any other library
package, with no diagnostic. A *bare* `exp(x)` with no import is reported
`unresolved reference: exp` and **does not evaluate**: the call fails with the
same unresolved-reference error, naming `OpenSysMLMathFunctions::exp` as the
declaration an import would make visible, rather than being answered by a
declaration the model never made visible. A bare `sqrt(x)` is governed by the
same rule — the OMG function libraries are not implicitly imported either.

| Semantic Rule | Implementation | Test Case | Status |
|--------------|----------------|-----------|--------|
| `exp(x)` — e raised to the power x | `runtime/library_functions.go` (`math.Exp`) | `TestLibraryFunctionValues` | 🧩 Extension (OpenSysML library) |
| `ln(x)` — natural logarithm, defined for `x > 0.0` | `runtime/library_functions.go` `naturalLog` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors` | 🧩 Extension (OpenSysML library) |
| `log(x, base)` — logarithm to an explicit base, so base 10 and base e are never confused; base 10 and base 2 use `math.Log10`/`math.Log2`, which are exact where the ratio of logarithms is not | `runtime/library_functions.go` `logToBase` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors` | 🧩 Extension (OpenSysML library) |
| `atan2(y, x)` — full-quadrant angle, parameters ordered as in IEEE 754 and `math.Atan2` | `runtime/library_functions.go` `atan2Real` | `TestLibraryFunctionValues`, `TestLibraryFunctionAtan2NamedArguments` | 🧩 Extension (OpenSysML library) |
| `ceiling(x)` — the least Integer not less than `x`, the counterpart of `RealFunctions::floor`: `ceiling(2.1)` is `3`, `ceiling(-2.9)` is `-2`, `ceiling(-9223372036854775808.0)` is the least Integer (which `-floor(-x)` cannot reach, its negation being 2⁶³) and a whole Real beyond the `int64` range is the exact Integer it is (`ceiling(1.0e20)` is `100000000000000000000`), never a wrapped `int64`; a non-number is `ErrTypeMismatch`. This is what the v1 migration writes for a script's `Math.ceil` | `runtime/library_functions.go` `ceilingToInteger` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors`, `TestRuntimeRobustnessCeiling` (`ceiling_beyond_int64`, `ceiling_of_an_infinity`, `ceiling_of_the_least_integer`, `ceiling_of_a_boolean`); conformance `calc_integer_ceiling`; `migrate/opaque_migration_test.go:TestTranslatedRoundingsGiveExactIntegers` | 🧩 Extension (OpenSysML library) |
| `quotient(x, y)` — the Integer quotient of two Integers truncated toward zero, so `quotient(x, y) * y + x % y == x` for every `y` other than `0`: `quotient(7, 2)` is `3`, `quotient(-7, 2)` is `-3`, `quotient(7, -2)` is `-3`, exact over the whole Integer range where `/` answers the rounded binary64 (`quotient(27021597764222979, 3)` is `9007199254740993`, which `/` cannot hold); a Real operand is `ErrTypeMismatch`. This is the quotient a Java or C `/` computes over two whole numbers, and what the v1 migration writes for a Java body's `/` | `runtime/library_functions.go` `integerQuotient` | `TestLibraryFunctionValues`, `TestLibraryFunctionErrors`; conformance `calc_integer_quotient` | 🧩 Extension (OpenSysML library) |
| `quotient(x, 0)` is `ErrDivisionByZero`; `quotient(-9223372036854775808, -1)` is `9223372036854775808` (2⁶³, beyond `int64` but an Integer), never a wrapped `-9223372036854775808`, and operands beyond `int64` divide exactly | `runtime/library_functions.go` `integerQuotient` | `TestLibraryFunctionErrors`, `TestRuntimeRobustnessIntegerQuotient` (`quotient_by_zero`, `quotient_of_the_least_int64_by_minus_one`, `quotient_of_big_integers`, `quotient_of_a_real`) | 🧩 Extension (OpenSysML library) |
| `ln(0.0)`, `ln(-1.0)`, `log(x, 1.0)`, `log(-1.0, 10.0)`, `atan2(0.0, 0.0)` report a domain error; `exp` beyond the Real range reports an overflow | `runtime/library_functions.go` | `TestLibraryFunctionErrors`, `TestRuntimeRobustnessLibraryFunctions/extension_library_function_outside_its_domain` | 🧩 Extension (OpenSysML library) |
| The shipped declarations and the registered implementations cannot drift (names, parameter names, parameter order) | `runtime/library_functions.go` registry | `TestOpenSysMLMathFunctionsMatchTheShippedDeclarations` | 🧩 Extension (OpenSysML library) |
| Evaluable from a `calc def` body under `import OpenSysMLMathFunctions::*;` | `runtime/invoke_calc.go` | `calc_opensysml_math_functions.sysml` + golden trace | 🧩 Extension (OpenSysML library) |
| An unqualified call the model imports no declaration of fails with the unresolved-reference error the checker reports, naming the qualified declaration an import would make visible, so the diagnostic and the behavior agree instead of contradicting each other | `runtime/eval.go` `invocationTarget`, `unresolvedInvocation` | `runtime/library_functions_test.go:TestLibraryFunctionUnqualifiedNames`, `TestRuntimeRobustnessCalcInvocation/calc_calls_an_unimported_extension_function`, `lsp/library_invocation_test.go:TestPublishDiagnosticsKeepsExtensionFunctionImportGated` | 🧩 Extension (OpenSysML library) |
| The function listing covers every function the build implements, each with the package an import must name for its unqualified name to resolve, so a working function is advertised neither as unsupported nor as callable bare | `runtime/builtin_names.go` `Builtin.Package`, `Builtins`, read by `repl/discover.go` `doBuiltins` and `repl/complete.go` | `runtime/library_functions_test.go:TestBuiltinsListEveryFunctionWithItsPackage`, `repl/discover_test.go:TestBuiltinsListsAnExtensionFunctionWithItsImport` | 🧩 Extension (OpenSysML library) |
