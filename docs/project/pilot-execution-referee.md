# The pilot as a referee for *behavioral* conformance

`cmd/pilot-diff` uses the pinned OMG pilot as an external referee for parsing, resolution and
validation. This document answers a different question: **how far does the pinned pilot's
*execution* surface reach, and which of the behavior rows in
[spec compliance](spec-compliance.md) can it adjudicate?**

The answer is narrow, and deliberately stated as such. The pinned artifact evaluates
*expressions* over a model's declarations. It does not execute actions, does not run state
machines, and has no notion of a step, a token or a trace. Three of the four behavior areas
are therefore **out of its reach**, and no amount of harness work changes that.

Pin: tag `2026-08`, artifact `jupyter-sysml-kernel 0.62.0` (`scripts/pilot-pin.sh`). Every
command below was run against the shaded jar that `scripts/download-pilot-validator.sh`
unpacks, at
`build/pilot-validator/target/sysml-download/sysml/jupyter-sysml-kernel-0.62.0-all.jar`.

## Capability map

| Behavior area | Verdict | Why |
|---------------|---------|-----|
| **Expression evaluation** | **Can adjudicate** — for model-level expressions | `SysMLInteractive.eval` (the `%eval` magic) evaluates literals, operators, library functions, `calc` invocations and feature *default values*, headlessly, deterministically. This is a genuine second opinion, and `cmd/pilot-exec-diff` uses it |
| **Action / token-flow execution** | **Cannot speak to it** | No interpreter exists in the artifact. `%eval` refuses an action def or usage as a target |
| **State-machine execution** | **Cannot speak to it** | Same: no state execution, no transition firing, no trace output. `%eval` refuses a state def or an `exhibit`ed state |
| **Classifier behaviors (`exhibit`/`perform`)** | **Cannot speak to it** for execution; **can corroborate** the *declared* value of a performed action's `out` parameter | `%eval` reads `machine.p.n` as the model-level value of the parameter's default expression, which is not an execution: no performer object exists, nothing is stepped |
| *(cross-cutting)* **Scope of an expression in a behavior body** | **Can corroborate only** | The pilot resolves names for a *model-level* evaluation of a declaration written in a behavior body. It says nothing about frames, shadowing or the values a running body writes — the substance of those rows |

## Evidence

### The artifact's whole execution surface is expression evaluation

The magics in the pinned jar:

```
$ unzip -Z1 build/pilot-validator/.../jupyter-sysml-kernel-0.62.0-all.jar \
    | grep -i 'jupyter/kernel/magic/[A-Za-z]*\.class'
org/omg/sysml/jupyter/kernel/magic/Load.class
org/omg/sysml/jupyter/kernel/magic/Projects.class
org/omg/sysml/jupyter/kernel/magic/View.class
org/omg/sysml/jupyter/kernel/magic/Viz.class
org/omg/sysml/jupyter/kernel/magic/Show.class
org/omg/sysml/jupyter/kernel/magic/Publish.class
org/omg/sysml/jupyter/kernel/magic/Repo.class
org/omg/sysml/jupyter/kernel/magic/Eval.class
org/omg/sysml/jupyter/kernel/magic/Help.class
org/omg/sysml/jupyter/kernel/magic/MyMagicParser.class
org/omg/sysml/jupyter/kernel/magic/Export.class
org/omg/sysml/jupyter/kernel/magic/Listing.class
```

`Eval` is the only one that computes anything: `Load`/`Projects`/`Repo`/`Publish` talk to a
model repository, `Viz`/`View`/`Show`/`Listing`/`Export` render or serialize. The kernel's own
help text for it, verbatim from `SysMLInteractiveHelp`:

```
Usage: %eval [--target=<NAME>] <EXPR>
Print the results of evaluating <EXPR> on the target given by <NAME>, which must be fully qualified.
If a target is not given, then evaluate <EXPR> in global scope.
```

The `org/omg/sysml/execution/` package in the jar contains exactly one thing —
`execution/expressions/`, an `ExpressionEvaluator` plus library function implementations
(`SizeFunction`, `SelectFunction`, `SumFunction`, the trig functions, …). There is no
interpreter, simulator, scheduler, token or trace class under `org/omg/sysml/` at all; a
search for those names in the jar returns only Guava/ICU/Xtext infrastructure. Action and
state semantics are present in the artifact **as a metamodel and as adapters**
(`ActionUsageImpl`, `StateUsageImpl`, `TransitionUsageImpl`, `ExhibitStateUsageAdapter`,
`plantuml/VStateMachine` for *drawing* a machine) — representation and rendering, not
execution.

### It runs headlessly

`scripts/pilot-evaluator/EvalSysML.java` drives `SysMLInteractive` directly — the same class
the kernel drives — with no notebook and no kernel protocol:
`SysMLInteractive.createInstance()`, `loadLibrary(dir)`, `process(modelText)` per model, then
`eval(expr, target, List.of())` per case. `scripts/download-pilot-evaluator.sh` compiles it
against the pinned jar and writes the launcher `build/pilot-evaluator/eval-sysml`. So the
expression surface is usable as a referee exactly the way the two validators are.

### What it emits

Output is **plain text, one line per resulting value**, each line a metamodel node kind, the
value, and the element's UUID:

```
== case int
LiteralInteger 7 (fb98f88c-9172-45bc-8ed8-b78fe546719b)
== end int
== case seqattr
LiteralInteger 3 (938afbeb-0cea-48ec-9c99-b5eadc1ab9b9)
LiteralInteger 1 (390142ba-ef95-4e34-9f78-8381a269be31)
LiteralInteger 2 (1b8b3471-f721-4eab-a377-6d6da070fb36)
== end seqattr
```

(`== case`/`== end` are our driver's framing; the lines between them are the pilot's own
output, verbatim.) There is **no machine-readable protocol** for `eval` — no JSON, no exit
code per case. A sequence is a run of value lines in order; an empty result is *zero* lines,
which is the same rendering an unevaluable expression gets (`1 / 0` produces no lines and no
diagnostic). Diagnostics come back as `ERROR:…` / `WARNING:…` lines in the same stream.

An expression the evaluator cannot reduce comes back as the **unevaluated node**, which is how
quantities appear:

```
== case quant
OperatorExpression [ (3009c3ad-7f3b-4665-a111-d99b4e256305)
== end quant
== case quantcalc
OperatorExpression + (e2642709-e92b-484f-bb0c-03331ad7ca9e)
== end quantcalc
```

`Probe::q` is `3.0 [SI::kg]` and `Probe::Quant()` is `3.0 [SI::kg] + 1.0 [SI::kg]`; OpenSysML
answers `3.00 [SI::kg]` and `4.00 [SI::kg]`. The pilot is not disagreeing — it is not
evaluating. Unit-carrying values are therefore out of the referee's reach, and
`cmd/pilot-exec-diff` buckets them `pilot-unevaluated` rather than as a disagreement.

It is **deterministic**: two runs of the same cases in separate JVMs differ only in the UUIDs.

```
$ diff <(sed -E 's/\([0-9a-f-]{36}\)//' run1.txt) <(sed -E 's/\([0-9a-f-]{36}\)//' run2.txt)
IDENTICAL
```

### What it accepts

A model is `process`ed as text into an accumulating session, as a notebook cell is. **A model
with any error registers nothing**: while `Probe` had one unresolved function name, every
later case failed with `Couldn't resolve reference to Element 'Probe::n'` — a whole-file
verdict, not a per-case one. The harness reports the model block so this cannot be mistaken
for a semantic disagreement.

A `--target` must be a **namespace**. A package works; a part, an action, a state or a feature
does not:

```
== case action-def         (target -, expr Behave::Bump)
ERROR:Must be a valid feature
== case action-usage       (target -, expr Behave::Flow::a)
ERROR:Must be an accessible feature (use dot notation for nesting)
== case state-def          (target -, expr Behave::Modes)
ERROR:Must be a valid feature
== case exhibit            (target -, expr Behave::machine::s)
ERROR:Must be an accessible feature (use dot notation for nesting)
== case perform            (target -, expr Behave::machine::p)
ERROR:Must be an accessible feature (use dot notation for nesting)
== case part-attr          (target Behave::cc, expr count)
ERROR:Must be an accessible feature (use dot notation for nesting)
```

Naming a behavior is how one would ask for its execution, and that is precisely what is
refused. Nothing in the surface takes a behavior and runs it.

Feature *values* are readable through dot notation, and this is the corroboration the surface
does offer:

```
== case dot-part-attr      (expr Behave::cc.count)
LiteralInteger 0 (d36e4833-0c8d-480c-b7e4-3de1b0f94d6d)
== case dot-perform-out    (expr Behave::machine.p.n)
LiteralInteger 1 (1a672f09-0b90-4137-a358-c8f5ea36164d)
```

`Behave::Bump` declares `out n : Integer = c.count + 1`, so `1` is the default expression
evaluated over declarations — no object was materialized and no action ran.

## What this means for the 23 behavior rows

- **Action (all rows), State Machine (all rows), Classifier Behaviors (execution rows).**
  Out of reach. The two named deliberate deviations — concurrent fork branches writing a
  shared feature space in step order, and variant selection not being ordering-sensitive —
  **cannot be adjudicated by the pinned artifact**, because ordering is a property of an
  execution the artifact never performs. Their status stays self-assessed against our golden
  traces; this document is not evidence for or against them.
- **Scope of an expression in a behavior body.** Corroboration only, and only for the
  model-level slice: whether a name written in a behavior body resolves at all, and what a
  declaration's default expression evaluates to. Frames, shadowing and values written by a
  running body are unobservable to the pilot.
- **Expression Evaluation rows.** Genuinely adjudicable, within the limits below; this is what
  `cmd/pilot-exec-diff` compares.

No compliance row's status flag is changed on the strength of this work.

## Limits of the comparison (read before trusting a bucket)

- **Reals are compared to two decimal places**, a tolerance of the harness itself (both sides
  are rounded in `cmd/pilot-exec-diff`, `roundedReal`). A divergence below 2dp is invisible to
  this harness. It is no longer a display limit — we now print `1.0 / 3.0` as
  `0.3333333333333333`, the same digits the pilot reports as `LiteralRational`, so the
  tolerance can be tightened on its own once the buckets it moves are adjudicated.
- **Integer vs Rational is reported, never normalized away.** `2 ** 40` gives the pilot
  `LiteralRational 1.099511627776E12` and gives us `1099511627776`; the harness buckets that
  `kind-only`, not `agree`.
- **Quantities are out of reach** (see above), as is anything else the pilot returns as an
  unevaluated node.
- **Silence is its own state, and it is ambiguous.** The pilot prints *nothing at all* both for
  an expression that legitimately yields the empty sequence and for one it silently gives up on
  (`1 / 0` produces no lines and no diagnostic), so it cannot distinguish "no value" from
  "declined to evaluate". Those cases bucket `pilot-silent` rather than being read as agreement
  with our empty sequence or as our error.
- **Collection order** is compared as order; a same-multiset/different-order result is its own
  bucket, so a real ordering difference is never hidden by sorting.
- **Scalar vs one-element sequence is unobservable.** We print `[2]` where the pilot prints a
  single value line, and its rendering has no way to distinguish the two (in the notation every
  value is a sequence), so single-element sequences are unwrapped on both sides. A genuine
  scalar/singleton difference, if one exists, would be invisible here.
- **UUID identity is the only thing normalized away** on the pilot side.

Run it with `go run ./cmd/pilot-exec-diff` after `./scripts/download-pilot-evaluator.sh`; with the
execution artifact absent it prints a provisioning instruction, exits 0 and writes nothing, so
`cmd/pilot-diff` and its committed baseline are untouched. The bucket counts below are as measured
when this record was last updated and are not the current baseline — `go run ./cmd/pilot-exec-diff`
prints the current ones. State of the 262 committed cases, the original 32, the 62 the
expression round added (one of them, `intdiv`, since moved to `integer_quotient.cases`), the 14 of
`value_classification.cases`, the 3 of `contextual_names.cases`, the 14 of `rational_terms.cases`,
the 5 the empty-aggregate and subsetting round added to `w6d_expr_depth.cases` the 12 of
`tensor_quantities.cases`, the 9 of `coordinate_frames.cases`, the 7 of `cast_expressions.cases`,
the 27 of `scalar_classification.cases`, the 24 of `literal_types.cases`, the 23 of
`enumeration_classification.cases`, the 24 of `metadata_access.cases` and the 6 of
`extent_expressions.cases`:

```
agree: 158 · kind-only: 1 · order-only: 0 · disagree: 17
pilot-unevaluated: 63 · pilot-silent: 11 · pilot-error: 2 · ours-error: 2 · both-error: 8
nondeterministic: 0
```

The six `extent_expressions.cases` probe `all T` (KerML 1.0 §7.4.9.2, §8.2.5.8.1
`ExtentExpression`, `BaseFunctions::'all'`), added with the evaluation they were meant to referee
and could not: the pilot does not evaluate the operator, as Table 5 of §8.2.5.8.1 foretells by
marking `all` not model-level evaluable. The four extents — `all Size`
over an enumeration, `all Wheel` and `all Car` over definitions a package-level `part car : Car`
instantiates, `all Boat` over one nothing instantiates — come back as the unevaluated
`OperatorExpression all` and land in `pilot-unevaluated`, where we answer the three literals, the
two wheels `car` holds, `car`, and the empty sequence. The two counts are the `disagree` cases
`extent-variation-count` and `extent-uninstantiated-count`: `size(all Gearbox)` over a variation
with two variants and `size(all Boat)` over a definition with no instance both answer `1` from the
pilot, against our `2` and `0`. The `1` is the size of the one unevaluated node `size` was handed,
not a count of instances — no reading of the extent gives a type with no instances the same size
as one with two — so neither is a verdict against us. The extent semantics are self-assessed in
the extent row of [spec-compliance.md](spec-compliance.md).

The run is deterministic: two runs into separate output directories differ only in the pilot's
element UUIDs, and agree line for line once those are stripped.

The fourteen `metadata_access.cases` referee `x.metadata` and the `meta` cast, added with the
evaluation they referee, and all fourteen agree. They were run *before* the runtime changed, to
settle what `.metadata` answers: KerML 1.0 §8.3.4.8.15 states that the result of a
MetadataAccessExpression is the metadata annotations of the referenced element followed by one
reflective metaobject of the element's own metaclass, and the pilot (`0.61.0`) reads it the same
way — `seatBelt.metadata->size()` on a part annotated once is `2` and `chassis.metadata->size()`
on a part nothing annotates is `1`, where the runtime then answered `1` and `0` (the two
`disagree` of that run; every `meta` case was `ours-error`, the operator refused). The pilot and
the spec text agree, so the runtime now appends the reflective metaobject and the two committed
`.metadata` conformance fixtures moved with it: `metadata_access_annotations` expects the
two annotation objects then `meta(test::seatBelt : SysML::Systems::PartUsage)` (and the metaobject
alone for the element nothing annotates, where it expected `()`), and
`metadata_access_textual_order` expects its four annotations then the metaobject. The `meta`
cases fix the cast as `x.metadata as T` (§7.4.9.2): `(seatBelt meta KerML::Feature)->size()` is
`1` (the metaobject alone — a `Safety` annotation is not a `Feature`), `(seatBelt meta
Safety)->size()` `1` with `.level` `4`, `(chassis meta SysML::PartDefinition)->size()` `0` for a
part usage and `(chassis meta SysML::PartUsage)->size()` `1`; the reflective features read
alike on both sides — `.name` `"seatBelt"`, `.declaredName` `"chassis"`, `.qualifiedName`
`"Meta::chassis"`, `.ownedFeature->size()` `2` (the nested part and the attribute),
`(Vehicle meta SysML::PartDefinition).declaredName` `"Vehicle"`, `.isAbstract` `false` — and
`(chassis meta KerML::Feature) === (chassis meta KerML::Type)` is `true` on both, so a
metaobject's identity is the element's whatever metaclass it is cast to. Each case reads a
model-level attribute bound to the expression because the pilot resolves no library name
(`KerML::Feature`, `SysML::PartUsage`) inside a bare `%eval`.

Five more `metadata_access.cases` referee `Element::documentation`, declared
`Documentation[0..*]` in `KerML.kerml`, so a documentation comment reads back as a metaobject
whose own `Comment::body` and `Comment::locale` are the strings. Three agree:
`(Wheel meta KerML::Element).documentation->size()` is `1` for a part definition with one
`doc` comment and `0` for one without, and `.documentation.owner.declaredName` is `"Wheel"`.
`.documentation.body` is the one new `disagree`, and it is a rendering artefact of the pilot,
not a semantic difference: the pilot prints `LiteralString Turns.  (<uuid>)`, a body with a
trailing space before its two-space id separator, which normalizes to `Turns. ` against the
runtime's `"Turns."`: the pilot keeps the blank before `*/`, the runtime reads the body the way
`Element::documentation` and LSP hover always have (`lexer.CommentBody`, delimiters and
margin off), so the runtime's answer stands and the referee's normalizer is left honest rather
than taught to trim. `.documentation.qualifiedName` is `pilot-silent`: the pilot prints nothing
for it, and the runtime answers `()`, which is what `Element::qualifiedName` derives to for an
element that declares no name (KerML 1.0 §8.3.2.1 Elements — a `doc` comment names nothing);
the empty pilot line and the empty sequence are the same reading, but the referee cannot tell
the pilot's "no value" from "declined", so the bucket is left as measured.

Five more `metadata_access.cases` referee the two KerML declarations a SysML model holds that
are neither types nor features — a `dependency` and a textual representation (`rep … language
"Java" /* … */`) — which `.metadata` classifies as `KerML::Dependency` and
`KerML::TextualRepresentation`. Four agree: `relies.metadata->size()` is `1` (the metaobject
alone) and `(relies meta KerML::Dependency)->size()` `1`, `.client.declaredName` is
`"seatBelt"` (`Dependency::client` redefines `Relationship::source`, the `from` side), and
`chassis::asJava.metadata->size()` is `1`. `.representedElement.declaredName` is the ninth
`disagree`, adjudicated ours: `KerML.kerml` declares `TextualRepresentation::representedElement :
Element[1..1] subsets owner redefines annotatedElement` (KerML 1.0 §8.3.2.2.12, the element the
representation is of, always its owner), so the runtime answers `"chassis"`, the owning part;
the pilot answers `"asJava"`, the representation's own name — it reads `representedElement` as
the representation itself, while its `.owner.declaredName` is `"chassis"` and its
`.documentation.annotatedElement` prints nothing, so its `annotatedElement` redefinitions do
not derive to the owner the library says they subset. The case reads `.representedElement`
rather than `.language` because `language` is a keyword to the pilot's expression parser (`no
viable alternative at input 'language'`), which would fail the whole model.

The twelve `tensor_quantities.cases` probe `TensorCalculations` over a 2×2 stress tensor built
by `TensorCalculations::'['` on a model-declared `TensorMeasurementReference`, added with the
tensor quantity value they were meant to referee and could not: the pilot answers every one —
the construction (`InvocationExpression [`), `dimensions` and `flattenedSize` (the `Feature
dimensions` itself), `#` (`IndexExpression #`), `+` (`OperatorExpression +`), the scalar
multiplications, the zero and unit predicates, `tensorTensorMult` and `VectorCalculations::outer`,
qualified at the prompt or as an attribute's value — with the unevaluated node, so all twelve
land in `pilot-unevaluated`. The semantics are therefore self-assessed against the vendored
declarations, in [spec-compliance.md](spec-compliance.md) (*Structured values*) and
[omg-issues.md](omg-issues.md#vectorcalculationsouter--a-vectorquantityvalue-return-for-an-order-two-product).

The nine `coordinate_frames.cases` probe a coordinate frame and a measurement scale as
values, added with the implementation they were meant to referee and could not: the pilot
answers the Annex A frame `spatialCF` with the unevaluated `AttributeUsage spatialCF`,
`spatialCF / s` and `velocityCF / s` with `OperatorExpression /`, `(1.0, 2.0, 3.0) [spatialCF]`
with `OperatorExpression [`, its `mRef` with `AttributeUsage mRef`, `transform(...)` with
`InvocationExpression transform`, `ConvertQuantity` to and from `SI::'°C_abs'` with
`InvocationExpression ConvertQuantity` and `Time::UTC` with the usage itself, so all nine land in
`pilot-unevaluated` and the semantics are self-assessed against the library text
(`spec-compliance.md`, *Structured values*; the readings the text leaves open are drafted in
[omg-issues.md](omg-issues.md)).

The fourteen `rational_terms.cases` probe `RationalFunctions::rat`, `numer` and `denom`, added
with the implementation they were meant to referee and could not: the pilot answers every call
— `rat(1, 3)`, `rat(1, 0)`, `numer(0.1)`, `denom(1.0 / 3.0)`, `numer(2)`, qualified at the prompt
or as an attribute's value — with the unevaluated `InvocationExpression rat`/`numer`/`denom`, so
thirteen land in `pilot-unevaluated` and only `quotient-by-operator` (`6 / 4`, `1.5`) agrees. The
semantics are therefore self-assessed, in
[exact-rational-evaluation.md](exact-rational-evaluation.md#rationalfunctionsrat-numer-and-denom-over-a-binary64-rational).

The fourteen `value_classification.cases` all agree, and they were added with the fixes they
referee: `x @ T` with a value subject is a classification test like `x istype T` — `a : Integer =
3` answers `a @ Integer` and `a @ Real` `true` and `a @ String` `false`, `car : Car` answers `car
@ Vehicle` `true` — where before the runtime judged every `@` against the subject's metadata
annotations and reported a scalar as classifying no element. The two operators part over a
collection (KerML 1.0 §7.4.9.2): `istype` holds when every value is classified, `@` when at least
one is, so `mixed : Real[*] = (1, 2.5, 3)` answers `mixed istype Integer` `false` and `mixed @
Integer` `true`, and `()` answers `@ Integer` `false` (and `istype Integer` `true`, the
`empty-istype-integer` of `scalar_classification.cases`); the pilot's `IsTypeFunction` tests every
value and its `AtFunction` any, and both sides agree on all of them.
A feature declared `Integer[0..1]` with no value is not probed: the pilot evaluates the bare
feature reference to the feature itself, one element whose type is `Integer`, and answers `none @
Integer` `true` and `none @ String` `false`, where the runtime holds the empty collection and
answers `@` `false` — pinned by `conformance/value_classification_shared_rule.sysml`. `x @
Safety` with a metadata type keeps the metadata reading, which the pilot does not share (its `@`
classifies the value alone, so it answers `false`); no committed case probes it, since the corpus
was written model-level and the annotation forms are pinned by the runtime conformance fixtures
instead.

The seven `cast_expressions.cases` probe `x as T`, added with the evaluation they referee. Two
agree: `n as Real` on `n : Integer = 7` answers `7` on both sides, and `(1, 2.5, 3) as Integer`
answers `(1, 3)` on both — the cast selects element-wise and converts nothing. Three are
`pilot-silent`: `n as Natural`, `2.5 as Integer` and `4.0 as Integer` draw no output at all from
the pilot, so its reading of a cast is unobservable here; we answer the empty sequence for both
`2.5 as Integer` and `4.0 as Integer`, and the typed `ErrUndecidedClassification` for `n as
Natural`, because a cast keeps exactly the values `istype` affirms and the pilot's `istype`
verdicts below fix those. The two part cases, `car as Vehicle` and `car as Car`, land in
`pilot-unevaluated`: the pilot answers with the unevaluated `PartUsage car`, which names the same
value we select but is not an evaluation of the cast.

The 27 `scalar_classification.cases` referee the rule the cast borrows — which ScalarValues types
a scalar value is of — through `istype` and `hastype`, which the pilot does evaluate. 26 agree,
and together they fix the rule as the one KerML states: a value is of the type its representation
states and of that type's supertypes, whatever number it holds. An integer is an `Integer`
(`n hastype Rational` false, `n istype Rational` true); a finite real is a `Rational`, whole or not
(`w : Real = 4.0` answers `w istype Integer` false, `w hastype Rational` true and `w hastype Real`
false — KerML 1.0 §8.4.4.9.2: only the rational subset of the reals has a finite literal, so a
`LiteralRational`'s result is classified in `Rational`); a quotient is what `IntegerFunctions::'/'`
returns, a `Rational` (§9.4.11.1), so `6 / 3 istype Integer` and `(7 / 2) hastype Real` are false
and `(7 / 2) hastype Rational` true; an integer written to a `Rational` feature stays the `Integer`
it is (`rat : Rational = 4` answers `rat hastype Integer` true, `rat hastype Rational` false); `*`
is a `Positive` (§8.4.4.9.2) and the empty sequence is of every type. `n as Rational` keeps the
integer `4` and `r as Real` the rational `2.5`, converting neither. The one `disagree` is
`natural-feature-istype-natural`: `nat : Natural = 7` answers `nat istype Natural` `true` here and
`false` from the pilot, which reads the literal's type alone — the same evaluator answers `nat
hastype Integer` `true`. The values of a feature are instances of all its types (§8.3.3.3.4
Feature, `type`), so the feature's typing is a type its value is of and the verdict stays ours;
no evaluation produces a value whose own type is `Natural`, which is why `hastype Natural` is false
on both sides and why a bare `7 istype Natural` is false on both (`w6d:istype-int-natural`).

The 23 `enumeration_classification.cases` referee classification against an enumeration, added
with the rule they referee: an enumeration's enumerated values are the only instances of it
(SysML v2 §8.3.7 EnumerationDefinition: "An EnumerationDefinition is an AttributeDefinition all
of whose instances are given by an explicit list of enumeratedValues"), so whether a value is of
`enum def Level :> Integer { low = 1; high = 3; }` is decided by equality with the enumerated
values where the shared classification rule (`classifyValue`) would otherwise leave a scalar
against a narrower type undecided — `3 istype Level` is `true`, `2 istype Level` `false`, `3 as
Level` the value `Level::high` (printed `3`), `2 as Level` `()`, and `held : Level = three` is
admitted while `= 2` is the write-conformance refusal. `hastype` keeps reading the value's own
type alone (KerML 1.0 §7.4.9.2, "directly"): a bare `3` is an `Integer` and no `Level`, and a
`Level` literal — written `Level::high`, held by `lvl : Level`, or produced by `3 as Level` — is a
`Level` and not directly an `Integer`, which is only a supertype. A plain `enum def Color { red;
green; blue; }` classifies by identity with its literals, and `5 as Even` on a plain subtype
stays undecided. Twelve agree, eight disagree and three are `pilot-silent`, per case:

| Case | Pilot | Ours | Read |
|---|---|---|---|
| `two-istype-level`, `three-hastype-level`, `three-hastype-integer`, `high-istype-integer`, `high-eq-three`, `high-plus-one`, `cast-hastype-level`, `red-istype-color`, `red-hastype-color`, `c-hastype-color`, `three-istype-color`, `red-istype-level` | as ours | `false`, `false`, `true`, `true`, `true`, `4`, `true`, `true`, `true`, `true`, `false`, `false` | **Agree.** A bare `3` is directly an `Integer` and no `Level`; a `Level` literal is an `Integer` by specialization and equals and computes as its value; `(3 as Level) hastype Level` is `true` on both sides; a plain enumeration's literal is of its enumeration alone and a scalar is of no plain enumeration |
| `three-istype-level`, `three-at-level` | `false` | `true` | **Ours.** The pilot's `IsTypeFunction`/`AtFunction` compare the literal's type (`Integer`) against `Level` by specialization alone and never consult the enumerated values, so they answer `false` for a value §8.3.7 makes an instance of `Level`. The same evaluator answers `two-istype-level` `false` for the right reason and the wrong one at once; the two verdicts cannot both come from the extent |
| `high-istype-level`, `high-hastype-level`, `high-hastype-integer`, `lvl-hastype-level`, `lvl-hastype-integer`, `held-hastype-level` | `false`, `false`, `true`, `false`, `true`, `false` | `true`, `true`, `false`, `true`, `false`, `true` | **Ours.** The pilot folds a scalar-valued enumeration literal to the `LiteralInteger 3` it is assigned and classifies that — so `Level::high istype Level` is `false` from the pilot, which no reading of §8.3.7 (the enumerated values are the instances) or of `hastype` (KerML 1.0 §7.4.9.2) allows, and contradicts its own `cast-hastype-level` and `red-hastype-color` answers. The runtime keeps the literal's identity on its scalar value (`runtime.Value.EnumerationLiteral`), so a literal is directly of its enumeration and only indirectly an `Integer` |
| `three-as-level`, `two-as-level`, `five-as-even` | no output | `3`, `()`, `ErrUndecidedClassification` | **Unrefereeable.** As for `cast_expressions.cases`, a cast draws no output from the pilot, so its reading is unobservable; ours follows the `istype` verdicts above, and `5 as Even` keeps the undecided refusal the plain-subtype row pins |

The 24 `literal_types.cases` all agree, and they pin the rule the runtime's two typing paths now
share: a literal's own type is its `ScalarValues` definition, found in the library and not by its
simple name in the evaluating scope. `scalar_classification.cases` could not see the difference
because its model imports `ScalarValues::*`. In a package that imports nothing from
`ScalarValues`, `2 istype ScalarValues::Integer` and `2.5 istype ScalarValues::Real` are `true`
(the direct-type path once failed both, unable to determine the direct type `"Integer"`), `2
istype ScalarValues::Real` is `true` and `2 hastype ScalarValues::Real` and `2 istype
ScalarValues::Natural` are `false`; `2.5` `hastype ScalarValues::Rational` and not `Real`, as the
paragraph above states. Beside a model's own `attribute def Integer` (and `Real`, `Boolean`,
`String`), the written `istype Integer` still resolves to the type the scope sees, so `2 istype
Integer` and `2 hastype Integer` are `false` while `2 istype ScalarValues::Integer` is `true` —
where the direct-type path once took the model's `Integer` for the literal's type.

The three `contextual_names.cases` all agree, and they were added with the parser fix they
referee: `chain` is the feature chain modifier only when a name follows it, so `attribute chain =
1;` declares a feature named `chain` and `chain + 1` reads it — where before the parser took the
word for the modifier, declared an anonymous attribute and left `chain` unresolved.

A later round of runtime fixes moved five of the six `ours-error` cases to `agree`
(`dot-perform-out`, `dot-machine-attr`, `w6d:inherited-value-no-body`,
`w6d:inherited-value-template`, `w6d:vector-elements`) and `bump-out-target` from
`both-error` to `pilot-error`: we now answer `n = 1` on `--target=Behave::Bump`, which the pilot
refuses outright, so the remaining error is the pilot's alone.

These counts were lost once and regained. When the expression type checker
(`passes/typecheck_expr.go`) landed, 20 cases moved from `agree` (19) and `kind-only` (1) to
`ours-error` — `agree: 37 · ours-error: 21` — without a single referee case changing. All 20 draw on
`expr_values.sysml`, and the checker refused that whole model on one declaration, `calc def IntDiv {
return : Integer = 7 / 2; }`, with `cannot bind Rational value to a feature typed by Integer`. The
first fix was in the checker: a binding was refused only when the value's type and the feature's
were disjoint (`String` to `Integer`, `Boolean` to `Real`) or when the value was a literal whose
type is exact (`2.5` or `-3` to `Natural`), and a quotient, a call or a `Real` feature bound to an
`Integer` was left to evaluation, which then judged the value by its magnitude — `4 / 2`, evaluating
to `2.0`, was held by an `Integer` feature and `7 / 2` was not. After that fix the count read
`agree: 56 · ours-error: 1`; each of the 20 returned to the bucket it held before, and `intdiv`
alone was `ours-error`, refused at run time.

That by-magnitude reading is superseded. The `scalar_classification.cases` above fix what a scalar
value is of, and a feature write is the same judgement: a feature value and a calculation argument
are bindings (KerML 1.0 §7.4.9: a `BindingConnector` requires the same values at both ends), and
the values of a feature must be instances of all its types (§8.3.3.3.4), so a feature holds a value
exactly when `istype` would affirm the feature's type of it. A quotient is a `Rational` whatever it
divides (§9.4.11.1), a finite real is a `Rational` whatever number it holds (§8.4.4.9.2), and neither
is an `Integer`: `attribute whole : Integer = 4 / 2` and an `Integer` parameter fed the real `2.0`
are now the typed `ErrTypeMismatch` that `7 / 2` and `3.5` already were, and a model that means the
whole number converts with `RationalFunctions::ToInteger`, `RealFunctions::ToInteger` or
`IntegerFunctions::ToNatural` — the library functions that convert — or declares the feature
`Rational` or `Real`. One shared classification (`runtime/classification.go` `classifyValue`)
answers `as`, `istype`, `hastype`, `@` and `write_conformance.go` `valueConforms`, so no two of
them can judge the same value and type differently. The static checker follows the same rule
where the value's type is settled at its spelling: a literal's type is exact, and a quotient's is
`Rational` however whole or signed, so `Integer = 7 / 2`, `Integer = -(4 / 2)`, `Natural = i / 2`
and an argument `add(4 / 2)` to an `Integer` parameter are refused where they are written
(`passes/typecheck_expr.go` `bindable`, `isQuotient`); a call or a `Real` feature bound to an
`Integer` is still left to evaluation, since their static type only bounds their values (a `Real`
feature may hold an integer, as `rat : Rational = 4` shows). Nothing is truncated: a sequence
index that evaluates to `2.0` names the second element and one that evaluates to `1.5` names none,
as before.

The fixtures that relied on the by-magnitude write were re-adjudicated one by one rather than
relaxed: an `Integer` or `Natural` feature or parameter that a quotient or a whole real reached
is declared `Rational` or `Real` where the model computes such a value (the REPL, gRPC and LSP
fixtures, the compiled-calculation and choice-point fixtures; the disposal-robot demo computes
no such value and runs unchanged), converts with
`ToInteger` where it means the integer (`value_conformance_test.go`, the checker's own tests), or
pins the typed error where the write is the point of the fixture (`runtime/robustness_test.go`, the
coordinate-frame failure modes, `calc_cast_scalar_values`). Each movement is cited in the
feature-write conformance row of [spec-compliance.md](spec-compliance.md).

`intdiv` is now refused by the checker rather than at run time — `cannot bind Rational value to a
feature typed by Integer` where the pilot answers `3.5` — and is kept in a model of its own,
`integer_quotient.cases`, so the static refusal leaves the other `expr_values` probes evaluable.
The pilot's answer is not a reading of the specification we differ on: its evaluator computes the
result expression and does not check what the result parameter is typed by, so it would answer
`3.5` for `return : String = 7 / 2` too. Reporting the binding is stricter than the reference, not
different from it, and the case stays as written because it probes exactly that.

The one `kind-only` is `2 ** 40` (above). The `pilot-error`, `pilot-unevaluated` and `pilot-silent`
buckets — 71 cases, a third of the corpus — are the pilot's limits rather than
disagreements, which is the central finding of this page restated as a count.

Of the five `disagree` outside `enumeration_classification.cases` and `extent_expressions.cases`,
the first is `w6d:complex-is-zero-qualified`: the pilot answers `false` for
`ComplexFunctions::isZero(rect(0.0, 0.0))` where we answer `true`. It is not a value verdict against
us — the pilot's `re`/`im` have no evaluable body, and the same run answers `false` for
`isZero(rect(3.0, 4.0))` too, so its result folds against unevaluated operands rather than deciding
zero. Read it as unrefereeable, and see the `ComplexFunctions` row of
[spec-compliance.md](spec-compliance.md) for the adjudication.

The remaining three are the subsetting-membership cases added with the fix they were meant to
referee, and they are unrefereeable in the same way. `w6d:subsetting-defaulted-count` asks
`size(subsystem)` of a `part subsystem : Sub[*] default null;` that `part a : Sub :> subsystem;`
and `part b : Sub :> subsystem;` subset: we answer `2`, the pilot `0`. Its `0` is the folded
default, not a count of members — `w6d:subsetting-two-count` drops the default and the pilot
answers `1` for the same two parts, and `w6d:subsetting-none-count` answers `1` for a collection
nothing subsets (we answer `2` and `0`). A count that is `1` for two members and for none is the
static evaluator reading the feature as one value, so it decides nothing about which objects a
subsetted collection holds; `w6d:subsetting-rollup` agrees only because `Sub::mass` defaults to
`1.0` and one part subsets. The two empty-aggregate cases of the same round land in
`pilot-unevaluated`: `sum(qsNone)` over an empty `MassValue[*]` is `InvocationExpression sum` and
`3.0 [SI::kg] + sum(qsNone)` is `OperatorExpression +` where we answer `0 [kg]` and `3.0 [SI::kg]`,
so the kind of an empty quantity sum, like every other unit-carrying value, has no reference
verdict. See the aggregate and subsetting rows of [spec-compliance.md](spec-compliance.md).

The dedicated Complex value moved no case between buckets — the counts above are as remeasured
after it — but it changed one answer inside `pilot-unevaluated`: `w6d:complex-mul-re`,
`re(rect(0.0, 1.0) * rect(0.0, 1.0))`, now answers `-1.0` where we reported `operator '*' is not
defined for a sequence and a sequence`. A Complex was two Reals, so the unqualified `*` saw two
sequences; it is one value now (`runtime.ValComplex`), and an arithmetic operator over a Complex
operand computes as the `ComplexFunctions` declaration of the same name. The pilot answers no value
for the case, so the bucket does not move. Overload selection by argument type changed two more
answers inside `pilot-unevaluated` without moving them: `w6d:complex-abs` answers `5.0` and
`w6d:complex-is-zero` answers `true`, where both reported that `NumericalFunctions::abs`/`isZero`
require a numeric value — the unqualified name now selects the `ComplexFunctions` declaration its
argument fits (the `ComplexFunctions` row of [spec-compliance.md](spec-compliance.md)). The pilot
answers the unevaluated `InvocationExpression` for both, so it referees neither.

The two remaining `ours-error` cases are adjudicated divergences:

| Case | Ours | Read |
|---|---|---|
| `w6d:held-undeclared-multi` | `multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1` | **Deliberately ours.** `attribute xs = (1.0, 2.0)` declares no multiplicity, so the assumed `1..1` makes the default a violation (KerML 1.0 §7.4.5, and the multiplicity row of [spec-compliance.md](spec-compliance.md)); the pilot returns both values. An adjudicated divergence, not a defect |
| `intdiv` | `integer_quotient.sysml:6:42: error: cannot bind Rational value to a feature typed by Integer` | **Deliberately ours.** `calc def IntDiv { return : Integer = 7 / 2; }` binds a `Rational` — what `IntegerFunctions::'/'` returns, KerML 1.0 §9.4.11.1 — to an `Integer` result parameter, and a feature's values must be instances of its types (§8.3.3.3.4, §7.4.9; the feature-write conformance row of [spec-compliance.md](spec-compliance.md)). The pilot answers `3.5`, checking nothing against the parameter's type. Stricter than the reference, not a defect in either |

The cases that were `ours-error` before that round, and what closed them:

| Case | Was | Closed by |
|---|---|---|
| `dot-perform-out`, `dot-machine-attr` | `usage machine: performed action p of machine: bind c: unresolved reference: cc` | A name that denotes one object — an occurrence, or a structured attribute usage holding features rather than a value — evaluates to that object (`runtime.Context.namesOneObject`), so a binding in a performed action's body resolves the package-level sibling it names |
| `w6d:inherited-value-no-body`, `w6d:inherited-value-template` | `unresolved reference: template`, `usage W6D::template has no value` | The same classification: `template` is a structured value, so `plain.cost` reads its features and `template.v` reads the redefined one, with the features it does not redefine keeping their inherited values |
| `w6d:vector-elements` | `cannot chain through non-instance member` | A numerical vector is the sequence of its elements, so `elements` of one is that same sequence (`runtime.isNumericVector`) rather than an element-wise member lookup |

## Findings to carry forward

1. **`Behave::machine.p.n` — a `perform action` binding's scope, closed by those runtime fixes.**
   Both tools validate the model clean and both now answer the same value:

   ```
   $ ./bin/sysml -e "Behave::machine.p.n" behavior.sysml
   ✓ Behave::machine.p.n
     = 1
   ```

   against the pilot's `LiteralInteger 1`, on `perform action p : Bump { in c = cc; }` inside
   `part machine`, where `cc` is a sibling member of the enclosing package. The binding resolves
   because a name denoting one object evaluates to that object; conformance case
   `perform_action_binding_package_sibling`.
2. **The pilot's evaluation context is narrower than ours, so the scope rows get no referee.**
   With `--target=Behave::Flow` the pilot cannot see the enclosing package's members
   (`cc.count` → `Couldn't resolve reference to Element 'cc'`) where we answer `0`; and
   `--target=Behave::Bump` with `n` is refused outright (`Must be an accessible feature`) where we
   answer `1`. Both are limits of the pilot's `%eval` context rather than statements about scope
   semantics — which is exactly why the "scope of an expression in a behavior body" rows get
   corroboration only, and why these two cases are bucketed `pilot-error` rather than as our
   disagreement.

## What a real behavioral referee would take

Nothing short of a second *executing* implementation. Options, in order of cost:

1. **A pilot version that executes.** The OMG pilot's execution work lives outside this
   artifact; adopting it means moving off the pin, and the pin is what makes the static
   differential meaningful. It would have to be a *second*, separately pinned artifact.
2. **A different tool** (e.g. an fUML/Alf-based executor, or SysIDE's runtime if it grows one),
   which brings its own conformance question: a disagreement then needs adjudication against
   the specification anyway.
3. **Specification-derived traces.** Hand-adjudicated traces from KerML `Performances`/
   `Occurrences` and the Systems Library, reviewed as evidence in their own right. Slower per
   row, but it is the only route that answers the ordering questions the deviations raise.
