# The `~` operator: adjudicated abstract-only and closed

KerML's expression notation has four unary operators: `+`, `-`, `not` and `~`. The first three
evaluate — `-x` and `+x` on every numeric kind and quantity, `not b` on a Boolean — and `~` was
the one operator of the whole notation the runtime refused with no external verdict behind the
refusal: `unsupported operator: '~': bitwise complement is declared by no function library the
runtime applies`. The [compliance mapping](spec-compliance.md) carried it as *not implemented*,
the honest flag for "we have not decided", and the [roadmap](roadmap.md#track-x--expression-forms-the-evaluator-did-not-reach)
lists the expression forms the evaluator did not reach. This record decides it.

The question is whether `~` has a meaning a runtime can give it — two's-complement on `Integer`
and `Natural` (`~x == -x - 1`), negation on `Boolean`, or anything else — or whether the operator
is abstract-only in KerML 1.0, so that an implementation that evaluated it would be inventing a
semantics no reference agrees on. It is settled against the specification text, the bundled Kernel
Function Library, the OMG corpora, the pinned pilot implementation and the runtime as it stands.
The answer is that **`~` is abstract-only**: the specification says so in as many words, no
library gives it a definition, no model in any corpus writes it, and the pilot declines to
evaluate it. The item is **closed** with the refusal kept and the one behavior the specification
does ask of a tool — a warning when the operator is used — added.

**Specifications cited.** KerML 1.0 (formal/2026-03-01), the document the compliance mapping
cites; library text is quoted from the bundled copies under `internal/workspace/libs/stdlib/`.
Corpus paths are those `scripts/download-training-examples.sh` and
`scripts/download-pilot-corpora.sh` create under `examples/`, at the `2026-08` pin of
`scripts/pilot-pin.sh`; the pilot is the `0.62.0` artifact the same pin names.

## What the specification says

KerML 1.0 §7.4.9.2 (Operator Expressions) introduces the unary operators as "the numerical
operators `+` and `-` and the logical operator `not`" — `~` is not named there at all, though the
concrete syntax admits it: §8.2.5.8.1 gives

```
UnaryOperator = '+' | '-' | '~' | 'not'
```

and then, in the same clause, the one sentence that decides this record (numbered rule 3 under
the `OperatorExpression` productions):

> The unary operator symbol `~` maps to the library Function `DataFunctions::'~'`, as shown in
> Table 5. This abstract Function may be given a concrete definition in a domain-specific
> Function library, but no default definition is provided in the Kernel Functions Library. If no
> domain-specific definition is available, a tool should give a warning if this operator is used.

Table 5 (Operator Mapping) has one row for it, and it is the only operator row of the table whose
description is not a meaning:

| Operator | Library Function | Description | Model-Level Evaluable? |
|---|---|---|---|
| `~` | `DataFunctions::'~'` | Undefined | No |

Every other row names what the operator does — "Logical not", "Addition", "Range construction" —
and every other data-function row is model-level evaluable. `~` is neither. Table 6 (Operator
Precedence) places it with the other unary operators, which is a statement about parsing, not
meaning. Nothing in §8.4.4.9 (Expression Evaluation semantics) gives it a semantics; the
`DataFunctions` evaluation rules are stated per concrete library function, and there is none.

The one other operator the specification treats the same way is `[`: Table 5's row for it reads
`BaseFunctions::'['`, "Undefined", "No" (§8.2.5.8.2), and the pilot warns on every `x[i]` in
KerML with `Use #(...) for indexing`. The compliance mapping already carries that as a faithful
warning (`bracket-operator`); `~` is its sibling.

## What the library declares

The Kernel Function Library (KerML 1.0 §9.4.3, §9.4.4) declares the function twice and defines it
nowhere:

```kerml
// DataFunctions.kerml
abstract function '~' { in x: DataValue[1]; return : DataValue[1]; }

// ScalarFunctions.kerml
abstract function '~' specializes DataFunctions::'~' { in x: ScalarValue[1]; return : ScalarValue[1]; }
```

§9.4.3.1 says what the package is for: "the abstract base Functions corresponding to all the
unary and binary operators in the KerML expression notation that might be defined on various
kinds of DataValues". For every other operator the chain continues into a concrete function:
`ScalarFunctions::'not'` is specialized by `BooleanFunctions::'not'`, `ScalarFunctions::'-'` by
`IntegerFunctions::'-'`, `NaturalFunctions::'-'`, `RationalFunctions::'-'`, `RealFunctions::'-'`
and `ComplexFunctions::'-'`. For `'~'` the search over the bundled library
(`rg "'~'" 'internal/workspace/libs/stdlib'`) finds exactly the two abstract declarations above:
`IntegerFunctions`, `NaturalFunctions`, `RationalFunctions`, `RealFunctions`, `BooleanFunctions`,
`StringFunctions` and `ComplexFunctions` declare no `'~'`. The signature says the *shape* — one
data value in, one out, the same kinds as `'not'` — and nothing about the value.

So the two readings a runtime might reach for have no library behind them. Two's complement on
`Integer` would make `~5` equal `-6`, but `Integer` is unbounded in KerML (§8.4.4.9.2: the
literal is a mathematical integer, and the bundled `IntegerFunctions` defines no width); the
identity `~x == -x - 1` is the two's-complement one and nothing in the library states it. Negation
on `Boolean` would make `~` a synonym of `not`, which `BooleanFunctions` already defines under its
own name; a library that wanted `~` to mean it would have declared `function '~' specializes
ScalarFunctions::'~' { in x: Boolean[1]; return : Boolean[1]; }` beside `'not'`, and has not.

## What the corpora write

Searched: the OMG training corpus (`examples/sysml-v2-training`, 100 files) and the three pilot
corpora (`examples/pilot-corpora/kerml-examples`, `sysml-examples`, `sysml-validation`, 213
files), at the pinned revisions. Twenty files contain a `~`; every one of them is the conjugation
notation — `port p : ~Def`, `feature g ~ B::f`, `type Conjugate4 ~ Conjugate1`,
`conjugation c2 conjugate Conjugate2 ~ Original` — or a comment. No model in any corpus writes a
unary `~` in an expression. The operator has no user; a tool that gave it a value would be judged
by no example.

## What the pilot evaluates

The pinned pilot expression evaluator (`build/pilot-evaluator/eval-sysml`, built by
`scripts/download-pilot-evaluator.sh` from the `0.62.0` artifact) was run through the execution
referee (`cmd/pilot-exec-diff`) over a probe model whose attributes are typed by fully qualified
scalar types, which both sides load without a diagnostic:

```sysml
package BitNot {
    attribute five : ScalarValues::Integer = 5;
    attribute nat : ScalarValues::Natural = 5;
    attribute yes : ScalarValues::Boolean = true;
    attribute half : ScalarValues::Real = 1.5;
    attribute cFive : ScalarValues::Integer = ~5;
    attribute cTrue : ScalarValues::Boolean = ~true;
    attribute cNegOne : ScalarValues::Integer = ~(-1);
    attribute cHalf : ScalarValues::Real = ~1.5;
    attribute cNat : ScalarValues::Integer = ~nat;
    attribute cZero : ScalarValues::Integer = ~0;
}
```

Seventeen cases: the literals `~5`, `~true`, `~(-1)`, `~1.5`, `~0` evaluated directly; the six
attributes read by qualified name; `~five`, `~yes`, `~nat`, `~half` evaluated in `BitNot`; and
two controls, `not yes` and `-five`. The pilot's answer to every `~` case, verbatim, is the
unevaluated expression node — for example

```
lit-five: pilot-unevaluated
  expression: ~5
  raw pilot:
OperatorExpression ~ (961e7fae-fc34-4e2a-8cea-4a37ac74cbe0)
```

and the same `OperatorExpression ~ (<uuid>)` for `~true`, `~(-1)`, `~1.5`, `~0`, for each of the
six attributes (reading `BitNot::cFive` returns its bound expression unfolded) and for the four
evaluated in `BitNot`. The two controls fold: `not yes` is `LiteralBoolean false`, `-five` is
`LiteralInteger -5`, and the runtime agrees on both. Bucket counts for that run — a snapshot of
one probing round, not the current baseline: agree 2, pilot-unevaluated 15, disagree 0. The pilot raises no diagnostic on the model — no
"undefined operator" warning, no type error on `~true` or `~1.5` — so the specification's *should
give a warning* is one-sided here, as the mapping notes for other warnings the pilot does not
raise.

The pilot therefore has no function behind `~` either: it neither gives a value nor rejects the
expression; it leaves it as written. That is exactly what Table 5's "No" under *Model-Level
Evaluable?* predicts, and it is the same treatment the pilot gives `all T`, the other operator of
Table 5 it does not fold.

## What OpenSysML does today

- **Parsing and typing.** `~x` parses as `ast.OperatorExpr` with `ast.OpBitNot`
  (`parser/expr.go`), and the static typer gives it `DataValue` as its result type
  (`semantics/valuetype.go` `operatorResultFQN`), which is what `DataFunctions::'~'` declares.
  Both stay: the notation is KerML, and the typer reads the library's declaration.
- **Evaluation.** `runtime/eval.go` `unimplementedOperators` refuses `OpBitNot` with a typed
  `ErrUnsupportedOperator` naming the reason; the two library functions are registered by name
  and signature in `runtime/library_unevaluable.go`, so `DataFunctions::'~'(5)` written as a call
  reports the same reason rather than "unknown function". The solver reports it outside its
  subset (`solve/translate.go`, "bitwise negation is outside the subset"). All three are pinned by
  `eval_operator_test.go:TestUnimplementedOperatorReportsWhy`,
  `library_functions_test.go:TestUnevaluableLibraryFunctionsNameThemselves` and
  `invoke_calc_body_test.go:TestUnevaluableResultIsNotReportedAsMissing`.
- **Checking.** Until this record, nothing warned. A dedicated pass now reports every use of the
  operator, in a `.kerml` or a `.sysml` document, in a value, a condition, a filter or a
  multiplicity bound, as a warning under the code `undefined-operator`
  (`passes/undefined_operator.go` `UndefinedOperatorPass`, walking the tree with `ast.Inspect`):
  `operator '~' invokes DataFunctions::'~', which the Kernel Function Library declares abstract
  and leaves undefined; no library the runtime applies defines it, so the expression has no
  value`. It is a warning in strict conformance too, since the specification asks for a warning,
  not a rejection — the notation is standard KerML. It runs at the syntax tier because the
  written operator is all it reads: an unresolved name elsewhere in the document gates the type
  tier but never this warning.
  `undefined_operator_test.go:TestUndefinedOperatorWarns` pins it, and
  `TestUndefinedOperatorSurvivesUnresolvedReference` pins the tier placement.

## Options

**A — Give `~` a value: two's complement on `Integer`/`Natural`, negation on `Boolean`.** Rejected.
No reference states it: the specification calls the operator undefined, the library defines no
function, the pilot folds nothing, and no corpus model would exercise it. An implementation that
answered `-6` for `~5` would be asserting a semantics that a later domain library specializing
`DataFunctions::'~'` is entitled to contradict — the specification reserves the meaning for such
a library. The identity `~x == -x - 1` is also a fixed-width one; on KerML's unbounded `Integer`
it is a choice, not a consequence.

**B — Keep the typed refusal, add the warning the specification asks for, record the reading.**
Chosen. The refusal already names the reason; the warning is the one behavior §8.2.5.8.1 asks of
a tool, and it is the same shape as the `bracket-operator` warning the mapping carries for the
other undefined operator.

**C — Dispatch `~` to a concrete domain-specific specialization of `DataFunctions::'~'` when a
model declares one.** Not done, and recorded as the limitation the closure leaves. The
specification permits such a library, but the runtime has no operator-to-user-function dispatch
for *any* operator — `+` on a user type does not look for a user `'+'` either — so adding it for
`~` alone would be a special case with no other user. If a model that needs it appears, the item
is "operator dispatch to concrete specializations of the abstract data functions", a general
mechanism, and the warning should then be suppressed where such a definition is in scope, as the
specification's "if no domain-specific definition is available" conditions it.

## Conclusion

`~` is abstract-only in KerML 1.0. The compliance row moves from ❌ *not implemented* to ⛔
*deliberate divergence*, stated as: the runtime refuses the operator with a typed error naming
`DataFunctions::'~'`, the checker warns on its use, and no value is given because none is
defined. What that leaves is option C, a general mechanism, not a gap in this operator.

## Sources

- KerML v1.0 (formal/2026-03-01): §7.4.9.2 Operator Expressions (the unary operators named);
  §8.2.5.8.1 Operator Expressions concrete syntax (`UnaryOperator`, rule 3 on `~`, Table 5
  Operator Mapping, Table 6 Operator Precedence); §8.2.5.8.2 (`[` in Table 5); §8.4.4.9
  Expression Evaluation semantics; §9.4.3 Data Functions, §9.4.4 Scalar Functions — read as the
  bundled `internal/workspace/libs/stdlib/Kernel Libraries/Kernel Function Library/
  DataFunctions.kerml`, `ScalarFunctions.kerml`, `BooleanFunctions.kerml`,
  `IntegerFunctions.kerml`, `NaturalFunctions.kerml`, `RealFunctions.kerml`.
- The pinned pilot implementation, `2026-08` / artifact `0.62.0` (`scripts/pilot-pin.sh`),
  through `scripts/download-pilot-evaluator.sh` and `tools/referee/exec`
  ([pilot-execution-referee.md](pilot-execution-referee.md)).
- The OMG corpora at the same pin ([pilot-corpora.md](pilot-corpora.md),
  [training-examples.md](training-examples.md)).
