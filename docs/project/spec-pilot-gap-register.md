# Spec-vs-Pilot Gap Register

## Overview

OpenSysML is validated largely by differential harnesses against the OMG SysML v2 pilot
implementation. Those harnesses find where OpenSysML *disagrees* with the pilot; they cannot find
where both fall short of, or read ambiguously, the KerML 1.0 and SysML v2 specification text.
The redefinition of a feature chain (`attribute :>> mid.leaf.value = 99.0;`) was such a case: both
tools accepted it and both silently gave it no effect, until the specification authors confirmed
that the chain's host feature is redefinable (fixed in #634).

This register lists every place found where OpenSysML's behaviour rests on "the pilot does this"
or on no recorded rationale at all, and where the pilot itself is known or suspected to fall
short of, or to interpret ambiguously, the specification. Each retained item carries the model
text, what each tool does, the clauses that bear on it, an assessment, and a question that could
be put to the specification authors. Items are ranked by modeler impact:
**silent wrong results > spurious errors > missed diagnostics > cosmetic**.

**Pilot:** [SysML v2 Pilot Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation)
release `2026-08`, commit `692170b71867353b8f90341e61556f49a5beb0e5` — the pin in
`scripts/pilot-pin.sh`. Where a source record observed an earlier release (`2026-05`, `2026-07`),
the item says so; those observations have not been re-run against `2026-08` for this register
unless stated.

**Specification text:** the clause numbers and quotations below were taken from the OMG
publications downloaded on 2026-09-27: KerML 1.0 at `https://www.omg.org/spec/KerML/1.0/PDF`,
whose title page reads **formal/2026-03-01**, and SysML v2.0 Part 1 (Language) at
`https://www.omg.org/spec/SysML/2.0/Language/PDF`, **formal/2026-03-02**. The KerML revision
requested for this audit, formal/2025-12-01, is not what that URL serves today and no copy exists
in the repository or the pilot pin (`scripts/download-pilot-grammars.sh` fetches the pilot's Xtext
grammars, not the OMG documents). Clause numbers are therefore those of formal/2026-03-01/-02; a
reader checking them against formal/2025-12-01 should expect the same numbering but must verify.
Every "the specification says" statement below is backed by text found in those two PDFs; every
"the pilot does" statement is either an observation recorded in the linked `docs/project/` record
or a reading of the pinned validator source, and is labelled as one or the other.

The register does not change any behaviour. Behaviour changes are follow-ups, each of which should
cite the item it closes.

## Summary table

The **Class** column is the audit's verdict on the item: **spec clear, both short** (the
specification says X, both tools do Y), **spec ambiguous** (needs an author opinion),
**pilot short, ours follows spec** (a pilot gap this register keeps because differential runs
alone would have accepted the pilot's silence), or **pilot-following justified** (a candidate that
turned out to rest on a clause after all — retained so it is not re-audited).

| # | Impact | Item | Pilot | OpenSysML | Class |
|---|---|---|---|---|---|
| 1 | Silent wrong result | Redefinition whose target is a feature chain (the motivating case) | accepts, no effect | redefines the chained feature (since #634) | spec clear (author-confirmed); pilot short |
| 2 | Silent wrong result | `Natural / Natural` evaluates to a `Rational` against a `Natural[1]` return declaration | evaluates `5/2` to `2.5` | types the operator `Rational`, as the pilot; the named function `NaturalFunctions::'/'` follows its declaration | spec ambiguous (library text vs. evaluator) |
| 3 | Silent wrong result | Two enabled branches of one choice (state machine or `decide`); which is taken | no executor; not exercised | first enabled in declaration order | spec silent |
| 4 | Silent wrong result | Filter conditions that read through a multi-hop feature chain, or a computed value, rooted in an unfeatured feature | evaluates them | reported as not evaluated and the filter not applied; fixed in #642 | spec clear; ours short |
| 5 | Silent wrong result | Repeated literal values bound to a unique multi-valued feature (`Integer[*] = (1, 1)`) | silent | reports the duplicate | spec clear; pilot short |
| 6 | Silent wrong result | Invocation leaving a required `in` parameter unbound (`F(1.0)` with two inputs) | silent | advisory only | spec ambiguous (no constraint stated) |
| 7 | Spurious error | Bare `import X::*;` without a visibility indicator | parse error | warning, then imports | pilot-following justified by the grammar; severity is ours |
| 8 | Spurious error | `send x to port` warned to use `via` | warning (`checkSendActionUsage`) | same warning | spec has no such constraint; pilot-following |
| 9 | Spurious error | `validateClassifierMultiplicityDomain` fired on a classifier reached through an alias | error on a valid model | not reported | spec clear; pilot short (filed upstream) |
| 10 | Spurious error | A calculation usage named as a value is the calculation, not its result (`n as String`) | warns | warns, as the pilot | spec ambiguous |
| 11 | Missed diagnostic | Eight control-node succession constraints (SysML §8.3.17) | unimplemented `TODO`s | reported | spec clear; pilot short |
| 12 | Missed diagnostic | Non-Boolean transition guard | accepted with the full library | rejected | spec clear; pilot short |
| 13 | Missed diagnostic | Indistinguishable memberships reported as a warning; repeated anonymous `perform a;` depends on bodies in the pilot | warning; silent for bodiless repeats | warning for every repeat | spec clear (a validation constraint), both tools under-report |
| 14 | Missed diagnostic | Constraints the pilot declares but never reports, absent from the published specification (`validateSubsettingPortionConformance`, `validateBindingConnectorArgumentTypeConformance`, `validatePartUsageType`, `validateItemUsageType`, …) | declared, unreported | not implemented | needs author opinion: are these normative? |
| 15 | Missed diagnostic | Lower-tier errors suppress later diagnostics the pilot still reports | reports secondary diagnostics over unresolved names | gates higher tiers | not a spec matter; recorded so it is not mistaken for one |
| 16 | Missed diagnostic | Body expression `{ … }` as a value typed `BooleanEvaluation` when its result is Boolean | so typed | as the pilot | spec ambiguous (implied typing of an expression body) |
| 17 | Cosmetic | Identifiers restricted to ASCII letters, digits and `_` | rejects non-ASCII basic names | same | pilot-following justified by KerML §8.2.2.3 |
| 18 | Cosmetic | Every reference subsetting after the first is reported | reports each extra | same | pilot-following justified by KerML §8.3.3.3 (at most one) |
| 19 | Cosmetic | `part p : ItemDef;` — a part typed only by a non-part definition | accepted | accepted | pilot-following justified: `parts` supplies `Part` through subsetting |
| 20 | Cosmetic | Alias identity: an `alias` is a name, not an element | resolves to the aliased element | fixed to match | pilot-following justified by KerML §8.3.2.4 (`Membership`) |

Grouped inventories follow the detailed items: [the `spec-compliance.md` rows whose
justification is the pilot or nothing](#inventory-a--spec-compliancemd-rows-justified-by-the-pilot-or-by-nothing),
[the code comments that rest on the pilot](#inventory-b--go-comments-that-rest-on-the-pilot),
and [the negative-corpus cases](#inventory-c--negative-corpus).

## Detailed items

### 1. Redefinition whose target is a feature chain

**Model text**

```sysml
part def Leaf { attribute value : Real = 1.0; }
part def Mid { part leaf : Leaf; }
part def Top { part mid : Mid; attribute :>> mid.leaf.value = 99.0; }
```

**Pilot:** parses the declaration and reports nothing; `mid.leaf.value` keeps `1.0`
(observation recorded in the #634 discussion; not a diagnostic difference, so no differential
harness saw it).

**OpenSysML:** the same until #634; since #634 the chain's host feature redefines `value` and the
evaluator answers `99.0`.

**Specification:** KerML §7.3.4.6 *Feature Chaining* — a feature chain in a feature declaration
"parses to a feature hosting the chain"; §8.3.3.3.4 `Feature`: `chainingFeature`, and
`deriveFeatureType`: "If the Feature has chainingFeatures, then the union also includes the types
of the last chainingFeature." §8.3.3.3.8 `Redefinition` places no restriction on a redefined
feature being chained. The reading that the host is redefinable was confirmed with the
specification authors (recorded in #634).

**Assessment:** spec clear, both tools were short; ours is fixed. Kept as item 1 because it is
the template for everything below: no rule in `spec-compliance.md` cited a clause for it, and the
implicit justification was the pilot's silence.

**Question for the authors:** none outstanding — closed by the confirmation recorded in #634.

### 2. `Natural / Natural` — the declared `Natural[1]` return against the pilot's `Rational` answer

**Model text**

```sysml
attribute a : ScalarValues::Natural = 5;
attribute b : ScalarValues::Natural = 2;
attribute q : ScalarValues::Natural = a / b;
```

**Pilot:** evaluates `a / b` to `LiteralRational 2.5` and reports nothing on `q`
(observation, pilot `2026-05`, recorded in [omg-issues.md](omg-issues.md#naturalfunctions-the-declared-natural-return-against-the-pilots-rational-answer)).
Its evaluator dispatches on the value's kind, so the division runs through
`RationalFunctions::'/'`.

**OpenSysML:** the type checker types `Natural/Natural` and `Integer/Integer` division as
`Rational` "as the reference evaluator types it" (`spec-compliance.md`, operator result types);
the runtime answers a Real, and a non-whole quotient bound to a `Natural`-typed feature is
reported rather than truncated. The function called by name, `NaturalFunctions::'/'(a, b)`,
follows its declaration and reports a non-whole quotient as an arithmetic-domain error.

**Specification:** KerML §9 Kernel Function Library, `NaturalFunctions`:
`function '/' specializes IntegerFunctions::'/' { in x: Natural[1]; in y: Natural[1]; return : Natural[1]; }`
(quoted from the vendored library, which is the normative library text). `IntegerFunctions::'/'`
returns `Rational[1]`. A specializing function's return that is `Natural` where the general
function's is `Rational` is well-formed as a redefinition; the text says nothing about how an
operator on two `Natural` operands is dispatched.

**Assessment:** spec ambiguous. The library declares a `Natural` result and the pilot never
produces one; OpenSysML follows the pilot for the operator and the declaration for the named
function, which is two answers to one question. A modeler binding `a / b` to a `Natural` attribute
gets a silently non-integral value from the pilot and an error from us.

**Question for the authors:** is `NaturalFunctions::'/'` meant to return `Rational[1]` (matching
`IntegerFunctions::'/'`), or is a conforming evaluator expected to truncate (or reject) a
non-whole quotient of two `Natural` operands written with the `/` operator?

### 3. Two enabled branches of one choice

**Model text** (the shape of fixture `state_choice_dynamic_conflict`)

```sysml
state def M {
    attribute level : ScalarValues::Integer; attribute route : ScalarValues::Integer;
    state idle; state low; state high;
    transition first idle accept Go do assign level := 8 then pick;   // pick is a choice
    transition first pick if level > 5 then low;                     // both guards hold
    transition first pick if level > 7 then high;
}
```

**Pilot:** the pinned pilot has no state-machine executor; no observation.

**OpenSysML:** reads the branches in declaration order and takes the first enabled one, recording
the choice; exploration reports the admissible set `{low, high}` (recorded as *Open: which
enabled branch is taken* in [behavior-semantic-oracle.md](behavior-semantic-oracle.md#two-branches-of-a-choice-enabled-by-the-data-the-incoming-effect-wrote-exactly-one-is-taken-which-one-is-open)).

**Specification:** `ControlPerformances.kerml` `DecisionPerformance::outgoingHBLink :
HappensBefore[1]` — exactly one branch follows; `TransitionPerformances.kerml` places the
segment's effect before what the segment leads to, so both guards are read after `level := 8`.
Nothing found in the library or in SysML §8.3.18 (`TransitionUsage`) ranks two branches whose
guards both hold. (SysML §8.3.17 `DecisionNode` has the same shape for actions.)

**Assessment:** spec silent. Any deterministic choice is an implementation policy; a model whose
guards overlap has an execution result that differs between conforming executors without any
diagnostic.

**Question for the authors:** when more than one outgoing transition of a choice (or succession
of a `decide` node) has a guard that evaluates `true`, is the choice implementation-defined, is
declaration order normative, or should a validator report the overlap?

### 4. Filter conditions read through a feature chain

**Model text**

```kerml
package P {
    feature root { feature n : ScalarValues::Integer = 1; feature m : ScalarValues::Integer = n + 1;
                   feature inner { feature k : ScalarValues::Integer = 3; } }
}
package Q { private import P::*[root.inner.k == 3]; }   // likewise [root.m == 2]
```

**Pilot:** evaluates the condition (recorded in the code comment `chainLimitation` of
`internal/semantic/semantics/filter.go` before #642: "the reference accepts chains rooted in a
feature with no featuring type").

**OpenSysML:** before #642, only a one-hop chain to a literal value (`root.n == 1`) was evaluated;
`root.inner.k` and `root.m` were reported as not evaluated and the filter was not applied. #642
evaluates every hop and the read feature's value expression; a chain through a feature with its
own value, a non-numeric/boolean terminal value, or a metaclass feature still reports the
limitation.

**Specification:** KerML §8.3.4.13.2 `ElementFilterMembership`,
`validateElementFilterMembershipConditionIsModelLevelEvaluable`: "The condition Expression must be
model-level evaluable." §8.3.4.8.5 `FeatureReferenceExpression::modelLevelEvaluable`: evaluable
if the referent "has no featuringTypes and, if it has a FeatureValue, the valueExpression is
model-level evaluable." §8.3.4.8.4 `FeatureChainExpression` is an `OperatorExpression` over its
argument, so a chain rooted in an unfeatured feature is evaluable when the rooted reference is.

**Assessment:** spec clear, ours is short and the pilot is right; the limitation is honest
(reported, not silent) but a filtered import with an unfiltered result is a wrong model. Listed
because the only rationale recorded is the pilot's acceptance, not the clause above.

**Question for the authors:** none needed for the rule; the follow-up is #642.

### 5. Repeated values bound to a unique feature

**Model text**

```sysml
attribute xs : ScalarValues::Integer[*] = (1, 1);
attribute os : Collections::OrderedSet { :>> elements = (1, 1, 2); }
```

**Pilot:** silent (observation, [pilot-differential.md](pilot-differential.md#value-uniqueness--only-ours-5)).

**OpenSysML:** reports the repeated value on the unique feature.

**Specification:** KerML §7.3.4.2: "The default is that the feature is unique"; §8.3.3.3.1
`+isUnique : Boolean = true`; `nonunique` is the one piece of concrete syntax that sets it
false. `Collections.kerml` declares `Collection::elements` `nonunique` and every redefinition
carries a note "Redefinition of 'elements' is unique by default", with no `nonunique` keyword —
the redefinition therefore *is* unique by the §7.3.4.2 default (the note is descriptive).
Recorded in [omg-issues.md](omg-issues.md#collectionsuniquecollectionelements-and-its-kin-unique-by-a-note-over-a-nonunique-root).

**Assessment:** spec clear, pilot short; kept because a differential run adjudicated it as
"only ours" and a maintainer could reasonably have read that as a spurious diagnostic. The
residual question is about the library: whether the `UniqueCollection` redefinition relying on the
default is intended, or whether `Collection::elements` being `nonunique` was meant to be inherited.

**Question for the authors:** is a value binding that repeats a value on a feature with
`isUnique = true` a well-formedness error (to be reported by a validator), an evaluation-time
matter, or neither?

### 6. Invocation leaving a required input parameter unbound

**Model text**

```sysml
calc def F { in x : Real; in y[1] : Real; return : Real = x + y; }
attribute f1 = F(1.0);
```

**Pilot:** the SysML and KerML validators report nothing; the evaluator forms and evaluates the
call (observation, pilot `2026-07`, [omg-issues.md](omg-issues.md#an-invocation-leaving-an-input-parameter-unbound-validates-clean-pilot-2026-07)).

**OpenSysML:** adjudicated the pilot's behaviour as the specification's reading and reports an
advisory only.

**Specification:** KerML §8.3.4.8.8 `InvocationExpression` constrains the arguments an
invocation *writes* (`validateInvocationExpressionParameterRedefinition`,
`validateInvocationExpressionNoDuplicateParameterRedefinition`) and states no constraint on
parameters left without an argument. A bare input parameter has the effective range `[0..*]`;
`y[1]` explicitly requires one value, so the *instance* described has a feature with no value
where one is required, but no constraint on the expression says so.

**Assessment:** spec ambiguous; both tools accept, and a modeler gets a result computed with an
unbound operand (the pilot) or an advisory (ours). This is the closest analogue to item 1: the
"validates clean" verdict rests on the pilot.

**Question for the authors:** is an `InvocationExpression` that leaves an input parameter of the
invoked type unbound — no argument, no default, lower bound 1 — intended to validate clean, and
if so what value does its result have?

### 7. Bare `import` without a visibility indicator

**Model text**

```sysml
package P {
    import Q::*;
}
```

**Pilot:** `mismatched input 'import' expecting '}'` — a parse error that also breaks the rest of
the file (observation, [pilot-differential.md](pilot-differential.md#only-the-pilot--candidate-gaps-139-sysml-side)).

**OpenSysML:** parses it, reports a warning, and imports as if `private`.

**Specification:** KerML §8.2.3.4.2 `Import = visibility = VisibilityIndicator 'import' ...`
with `VisibilityIndicator = 'public' | 'private' | 'protected'` — the assignment is not optional,
unlike the member prefix, where the indicator is. The SysML grammar (`SysML.xtext`
`ImportPrefix`) matches.

**Assessment:** pilot-following is justified by the grammar clause; the specification is not
ambiguous. What is ours alone is the recovery (warning rather than error), which
`pilot-differential.md` records as deliberate. Listed because the project record says the
question of mandatory visibility "is not settled", which the grammar text above settles.

**Question for the authors:** none about the grammar. A question worth putting is whether a
tool may recover a bare `import` as `private import` (the pilot's reading of the default
visibility for members) without being non-conforming.

### 8. `send … to port` warned to use `via`

**Model text**

```sysml
part def P { port p : Prt; action a { send Sig() to p; } }
```

**Pilot:** warns that a receiver that is a `PortUsage` should be given with `via`
(`SysMLValidator.checkSendActionUsage`; reading of the pinned source, and observed).

**OpenSysML:** `validateSendActionUsageReceiver` — the same warning, "the specification places no
such constraint, so the diagnostic is a warning as the reference's is" (`spec-compliance.md`).

**Specification:** SysML §8.3.17.15 `SendActionUsage`: `deriveSendActionUsageReceiverArgument`
("The receiverArgument of a SendActionUsage is its third argumentExpression"),
`validateSendActionParameters`; nothing in the constraint list distinguishes a port from any other
receiver. The `via` clause populates the *sender*/*receiver* pair per §7.16 textual notation, but
the text found does not forbid a port as a `to` target.

**Assessment:** the specification has no such constraint; the warning exists because the pilot's
does. It is advisory, so its cost is a spurious warning on a model the specification accepts.

**Question for the authors:** is `send x to <port>` (a `PortUsage` as receiver, rather than as
the `via` argument) meant to be discouraged or ill-formed, or is the pilot's warning a style
preference?

### 9. `validateClassifierMultiplicityDomain` fired through an alias

**Model text**

```kerml
classifier C [1];
package P { alias D for C; }
```

**Pilot:** reports "A classifier's multiplicity has no featuring type" on a valid model in which
the classifier is reached through an alias or reference membership (observation, shape recorded
in [validation-constraints.md](validation-constraints.md) and
[omg-issues.md](omg-issues.md#a-multiplicity-is-found-through-aliases-and-references-pilot-2026-07);
filed as Systems-Modeling/SysML-v2-Pilot-Implementation#802 and fixed upstream
on ST6RI-975 (commit `1563e068`, 2026-10-02; not yet included in a release).

**OpenSysML:** does not report it; the multiplicity is owned by `C`, which has no featuring type.

**Specification:** KerML §8.3.3.2.2 `Classifier`, `validateClassifierMultiplicityDomain`: the
`multiplicity` of a `Classifier`, if any, must have no `featuringType`. §8.3.3.1.10 `Type`:
`multiplicity` is the `Multiplicity` among the type's *owned* members, not one reached through an
alias or reference membership.

**Assessment:** spec clear, pilot short; OpenSysML declines to follow the pilot. Retained so that
the next differential run does not re-open the disagreement.

**Question for the authors:** none; this is a pilot defect already filed upstream.

### 10. A calculation usage named as a value is the calculation, not its result

**Model text**

```sysml
calc def Name { return : ScalarValues::String; }
part def P { calc n : Name; attribute s = n as ScalarValues::String; }
```

**Pilot:** warns that `n` does not conform to `String` (observation recorded in the code comment
in `internal/semantic/semantics/operator_conformance.go`).

**OpenSysML:** `featureResultTypes` treats `n` as the calculation, as the pilot does, and warns.

**Specification:** KerML §7.4.9.4 *Base Expressions* — a feature reference expression whose
referent is an `Expression` evaluates to the *result* of that expression when it is
model-level evaluable (§8.3.4.8.5); SysML §8.3.19.3 `CalculationUsage` is an `Expression`. Whether
naming a calculation usage as an operand denotes the calculation (a feature typed by a function)
or its result (its `result` parameter) is not stated in one place; the two readings give
different conformance verdicts.

**Assessment:** spec ambiguous. The consequence is a warning on a model that, under the other
reading, is well-typed.

**Question for the authors:** when a `calc` usage is named as an operand of a cast or an operator,
does the name denote the calculation feature (typed by the calculation definition) or its
result parameter?

### 11. Eight control-node succession constraints the pilot does not implement

**Model text**

```sysml
action def A {
    fork f; action a; action b;
    first a then f; first b then f;   // two incoming successions to a fork
}
```

**Pilot:** `SysMLValidator` declares nine `ControlNode` constants and reports only
`validateControlNodeOwningType`; the eight succession constraints are `TODO`s (reading of the
pinned source, recorded in [omg-issues.md](omg-issues.md#eight-control-node-succession-constraints-are-unimplemented-todos-pilot-2026-07)).

**OpenSysML:** reports each (`internal/check/passes/control_node.go`); the nine ours-only
negative-corpus cases (inventory C) are these.

**Specification:** SysML §8.3.17.2–§8.3.17.6 `ControlNode`, `DecisionNode`, `ForkNode`,
`JoinNode`, `MergeNode` — e.g. `validateForkNodeIncomingSuccessions` "A ForkNode must have at
most one incoming Succession", `validateJoinNodeOutgoingSuccessions`,
`validateMergeNodeOutgoingSuccessions`, `validateDecisionNodeIncomingSuccessions`, and the
outgoing/incoming counterparts.

**Assessment:** spec clear, pilot short, ours follows the specification. Retained because the
pilot's silence on these would have been indistinguishable from acceptance in a differential run.

**Question for the authors:** none.

### 12. Non-Boolean transition guard

**Model text**

```sysml
state def S { state a; state b; transition first a if "test" then b; }
```

**Pilot:** `checkTransitionFeatureMembership` implements the check and its own Xpect fixture
expects "Must be a Boolean expression." — but only with a reduced library; with the full library
loaded the guard is accepted (observation, pilot `2026-07`,
[omg-issues.md](omg-issues.md#a-non-boolean-transition-guard-is-accepted-with-the-full-library-loaded-pilot-2026-07)).

**OpenSysML:** rejects the guard.

**Specification:** SysML §8.3.18.8 `TransitionFeatureMembership`,
`validateTransitionFeatureMembershipGuardExpression`: a guard `transitionFeature` must be an
`Expression` whose `result` `specializesFromLibrary('ScalarValues::Boolean')`.

**Assessment:** spec clear, pilot short in its shipped configuration.

**Question for the authors:** none.

### 13. Indistinguishable memberships: severity, and anonymous performed actions

**Model text**

```sysml
part def A; part def A;
action def B { action a; perform a; perform a; }
```

**Pilot:** `Duplicate of other owned member name` as a **warning**; for `perform a; perform a;`
it warns only when the repeats have bodies (observation, pilot `2026-07`,
[adjudications.md](adjudications.md)).

**OpenSysML:** a warning for every repeat, bodies or not; `part def A; part def A;` is likewise
a warning.

**Specification:** KerML §8.3.2.4.5 `Namespace`, `validateNamespaceDistinguishibility` (so spelt in the PDF): "All
memberships of a Namespace must be distinguishable from each other." §8.3.2.4.3
`Membership::isDistinguishableFrom` compares `memberShortName`/`memberName`. §8.3.3.3.4
`Feature::effectiveName`: a feature with no declared name takes the effective name of its
`namingFeature()`; SysML §8.3.17.14 `PerformActionUsage::namingFeature` "is its performedAction".
Two `perform a;` therefore both have `memberName` `a`, whatever their bodies.

**Assessment:** spec clear on both counts. The rule is a `validate…` constraint — a violating model
is not well-formed — so a warning under-reports it; both tools do so (a warning was chosen so that
duplicated names in the training corpus do not fail its clean gate). The body-sensitivity is the
pilot's alone; ours follows the specification.

**Question for the authors:** is a namespace with indistinguishable memberships intended to be
rejected (an error) or is a tool free to report it as a warning and keep resolving? And for
two `perform a;` in one body, are the memberships indistinguishable regardless of whether the
usages have bodies?

### 14. Constraints the pilot declares that the published specification does not contain

**Model text**

```kerml
classifier C { portion feature p : C; }
classifier D :> C { feature q :> p; }   // subsets a portion feature, is not itself a portion
```

**Pilot:** `KerMLValidator`/`SysMLValidator` declare the constants and messages
`validateSubsettingPortionConformance` ("A feature subsetting a portion feature is a portion"),
`validateBindingConnectorArgumentTypeConformance`, `validatePartUsageType`,
`validateItemUsageType` and others listed under *declared, unreported* in
[validation-constraints.md](validation-constraints.md); none is reported by any `@Check`
(reading of the pinned source).

**OpenSysML:** does not implement them.

**Specification:** none of these names appears in KerML formal/2026-03-01 or SysML
formal/2026-03-02 (a search of both extracted texts). The neighbouring constraints that *do*
exist are cited in `validation-constraints.md`. `validatePartUsagePartDefinition` (SysML
§8.3.11.3) exists and is satisfied by `checkPartUsageSpecialization` for every part usage (item
19); `validatePartUsageType` does not.

**Assessment:** needs author opinion. Either these are constraints that were dropped from the
published text and the pilot's declarations are stale, or they are intended and the text is
missing them. Neither tool reports them, so a model violating one passes both.

**Question for the authors:** are `validateSubsettingPortionConformance` and
`validateBindingConnectorArgumentTypeConformance` (a feature subsetting a portion feature must be
a portion; a binding connector's ends must have conforming types) normative constraints that a
future revision will carry, or were they deliberately withdrawn?

### 15. Tier gating suppresses diagnostics the pilot still reports

**Model text**

```sysml
part def P { attribute a : Nowhere; attribute b : ScalarValues::Real = a; }
```

**Pilot:** reports the unresolved `Nowhere` and then type-checks the partially resolved model,
reporting `Bound features should have conforming types` on `b` (observation,
[pilot-differential.md](pilot-differential.md), the class of secondary diagnostics over an
unresolved reference).

**OpenSysML:** reports `Nowhere` and gates the type tier behind it (`AGENTS.md` §4), so `b` is not
reported until `a` resolves.

**Specification:** the specification defines well-formedness, not diagnostic ordering; no clause
bears on it.

**Assessment:** not a specification question. Listed so the difference is not mistaken for a
missing rule: the type-conformance rule exists on our side and fires once the lower tier is
clean.

**Question for the authors:** none.

### 16. `{ … }` written as a value is typed as an evaluation

**Model text**

```sysml
part def P { attribute cond : ScalarValues::Boolean = { true }; }
```

**Pilot:** types the body expression as a `BooleanEvaluation` when its result is Boolean
(observation recorded in the code comment `bodyExprType`, `internal/semantic/semantics/valuetype.go`).

**OpenSysML:** the same, "as the pilot reads it".

**Specification:** KerML §7.4.9.4 *Base Expressions*: an expression body `{ … }` "is an
expression whose result is that of its result expression"; §8.3.4.8 `Expression` implied
specialization is of `Performances::Evaluation`. The text found does not say the body expression
implicitly specializes `BooleanEvaluation` when its result is Boolean — that is the pilot's
implied-typing rule.

**Assessment:** spec ambiguous at the level of implied typing; the modeler-visible effect is a
conformance verdict on `cond`, which the two readings may differ on.

**Question for the authors:** does an expression body whose result is `Boolean` implicitly
specialize `Performances::BooleanEvaluation`, or only `Performances::Evaluation`?

### 17. ASCII-only basic names

**Model text**

```sysml
part def Größe;
```

**Pilot:** rejects; the name must be quoted `'Größe'` (observation, negative corpus).

**OpenSysML:** the same.

**Specification:** KerML §8.2.2.3 *Names*: `BASIC_NAME = BASIC_INITIAL_CHARACTER
BASIC_NAME_CHARACTER*`, `ALPHABETIC_CHARACTER = any character 'a' through 'z' or 'A' through 'Z'`;
`UNRESTRICTED_NAME` allows "any printable character other than backslash or single_quote"
between single quotes.

**Assessment:** pilot-following justified by clause; the code comment cites the pilot where it
could cite §8.2.2.3.

**Question for the authors:** none.

### 18. Every reference subsetting after the first is reported

**Model text**

```sysml
part def P { part a; part b; ref r ::> a ::> b; }
```

**Pilot / OpenSysML:** each `::>` after the first is reported (code comment in
`internal/check/passes/w8c_reference_subsetting.go`: "as the pilot does").

**Specification:** KerML §8.3.3.3.4 `Feature`: `ownedReferenceSubsetting : ReferenceSubsetting
[0..1]` — at most one. Which of several to report is a diagnostic-placement choice.

**Assessment:** pilot-following justified by the multiplicity; only the placement is the pilot's.

**Question for the authors:** none.

### 19. A part typed only by a non-part definition

**Model text**

```sysml
item def I; part p : I;
```

**Pilot / OpenSysML:** accepted (both).

**Specification:** SysML §8.3.11.3 `PartUsage`, `validatePartUsagePartDefinition`: "At least one
of the itemDefinitions of a PartUsage must be a PartDefinition." But `checkPartUsageSpecialization`
requires every `PartUsage` to specialize `Parts::parts`, whose type is `Parts::Part`, and KerML
§8.3.3.3.4 `deriveFeatureType` makes a feature's `type` "the union of the types of its typings and
the types of the Features it subsets", so `partDefinition` is never empty. §7.10's note says as
much: "every part usage is always directly or indirectly defined by at least one part definition,
implicitly if not explicitly."

**Assessment:** pilot-following justified by clause; the `validation-constraints.md` row that
marks the constraint "unobservable" is right for the reason above, and can cite it.

**Question for the authors:** none.

### 20. An alias is a name, not an element

**Model text**

```kerml
package test { class A; alias A_alias for A; }
```

**Pilot:** `test::A_alias` resolves to the element whose qualified name is `test::A`
(observation, [pilot-xpect.md](pilot-xpect.md)).

**OpenSysML:** answered `test::A_alias` until the alias was made a second membership of the
existing element; now the same as the pilot.

**Specification:** KerML §8.3.2.4.3 `Membership`: an alias declares a `Membership` whose
`memberElement` is the existing element and whose `memberName` is the alias; §7.2.5 *Namespaces*
(alias members).

**Assessment:** pilot-following justified by clause and fixed. Listed because the adjudication
record says "the pilot was right" without the clause.

**Question for the authors:** none.

## Inventory A — `spec-compliance.md` rows justified by the pilot or by nothing

Every row of `docs/project/spec-compliance.md` was classified as (a) citing a specification
clause, (b) citing the pilot or reference behaviour, or (c) citing neither. The extraction script
and its output are described under [How this list was produced](#how-this-list-was-produced-and-how-to-keep-it-current).
Of the rows examined, 442 cite a clause, 290 cite the pilot or reference (some also cite a
clause), and 217 cite neither. The (b) and (c) rows fall into the groups below; a row is
*semantic* when it decides what a model means or whether it is well-formed, and *implementation*
when it describes a surface the specification does not govern.

| Section of `spec-compliance.md` | (b) pilot-citing | (c) uncited | Nature | Disposition |
|---|---|---|---|---|
| Current Implementation Status | 2 | 3 | status prose | none needed |
| Detailed Semantic Compliance Map | 290 | 217 | **semantic** | see below |
| What We Don't Yet Support | 3 | 3 | limitations | none needed |
| Model Persistence and RDF Interchange | 4 | 25 | implementation (RDF vocabulary, round-trip) | not a spec matter; the SysML v2 API/RDF mapping is a separate OMG document not audited here |
| Source-Preserving Model Editing | 1 | 12 | implementation | none needed |
| gRPC Service Layer | 3 | 32 | implementation | none needed |
| Language Server | 1 | 4 | implementation | none needed |
| Analysis Engines | 1 | 4 | implementation | none needed |
| Constraint Solving | 8 | 50 | implementation (SMT encoding) | none needed; the encoding is an OpenSysML extension |

Within the *Detailed Semantic Compliance Map*, the pilot-citing rows are of four kinds:

1. **Rows whose rule cites a clause and whose *wording, severity or placement* cites the
   pilot** (the majority of the 290). These are conformance rows; the pilot reference is about the
   diagnostic text or where it is anchored. Nothing to adjudicate beyond items 13 and 18 above.
2. **Rows whose *rule itself* rests on the pilot** — the source of items 2, 8, 10 and 16, plus
   the rows grouped as "SysML Notation the Reference Accepts and We Reject — the ten classes",
   which record where we are *stricter* than the pilot with a grammar citation for each (no gap).
3. **Rows recording a pilot gap** ("the pinned pilot implements only …", "adjudicated as pilot
   gaps") — items 9, 11, 12, and the *declared, unreported* set of item 14.
4. **Rows on evaluation gating** ("matching the pilot's model-level-evaluable gating",
   `internal/semantic/semantics/eval.go`) — the gating is KerML §8.3.4.8 `modelLevelEvaluable`, so
   the citation should be the clause; behaviourally covered by item 4.

The 217 uncited semantic rows are of two kinds. Most are implementation-mapping rows (rule →
file:function → test → status) whose rule name *is* a `validate…`/`check…` constraint name from
the specification, so the name is the citation; this audit checked the names quoted in the
detailed items against the PDF text but did not verify every one of the 217 individually, which
is the first thing a re-run should do (search the extracted PDF text for each name). The rest are
OpenSysML-specific passes (multiplicity bound ordering, specialization cycles, unresolved
references) whose justification is well-formedness rather than a clause; none decides model
meaning.

## Inventory B — Go comments that rest on the pilot

Of the pilot-mentioning comments in non-test sources under `internal/` and `tools/`, those that
record a *behaviour* decision are:

| File | Decision | Item |
|---|---|---|
| `internal/semantic/semantics/operator_conformance.go` `featureResultTypes` | a calculation usage as an operand is the calculation, not its result | 10 |
| `internal/semantic/semantics/filter.go` `chainLimitation` | chains rooted in an unfeatured feature are evaluable (the reference does; we do not) | 4 |
| `internal/semantic/semantics/valuetype.go` `bodyExprType` | `{ … }` is a `BooleanEvaluation` when Boolean | 16 |
| `internal/semantic/semantics/eval.go` | unsupported-evaluation gating "matching the pilot's model-level-evaluable gating" | 4 (rule is KerML §8.3.4.8) |
| `internal/check/passes/w8c_reference_subsetting.go` | every extra reference subsetting reported | 18 |
| `internal/check/passes/w10b_usage_typing.go` | usage-typing diagnostics worded and placed as the pilot's | justified: SysML `/…Definition {redefines definition}` typing (e.g. `/occurrenceDefinition : Class`) |
| `internal/check/passes/w8d_connector_featuring.go` | connector-end featuring checks alongside the pilot's | the file cites its clauses; the pilot reference is for placement |
| `internal/check/passes/variant_owner.go` | variant-owner check | justified: cites SysML §7.20 alongside the pilot |
| `internal/check/passes/behavior/state_transition.go` | diagnostic wording from the pilot | wording only |

The remaining hits are rendering styles, DOT/diagram output, XMI and API shape, keyword lists
derived from the pilot grammars, and test-harness plumbing — implementation compatibility, not
model semantics. The full hit list with its per-hit classification is reproduced by the command
under [How this list was produced](#how-this-list-was-produced-and-how-to-keep-it-current).

## Inventory C — negative corpus

`tools/referee/reject/testdata/negative/` holds 306 cases; the committed baseline is
`bothReject 297, pilotOnlyRejects 0, oursOnlyRejects 9, bothAccept 0`. Every case carries a
rationale line, and every rationale cites either a specification clause or a grammar production
(the one that looked uncited, `grammar/g60-alias-keyword-as-name.sysml`, cites `AliasMember /
Identification, terminal ID`, i.e. KerML §8.2.2.3 — item 17's clause). **No case is rejected
because the pilot rejects it without a clause.** The nine `oursOnlyRejects` are the control-node
succession cases of item 11 — cases where the pilot accepts and the specification rejects — and
are the negative corpus's evidence for that item.

## How this list was produced, and how to keep it current

The register is an audit of four sources, each re-runnable from the repository root:

1. **`spec-compliance.md` rows.** Every table row and list item under each `##` section was
   classified by regular expression: (a) mentions `KerML`/`SysML` with a `§` or `8.x` clause number;
   (b) mentions `pilot`, `reference`, `Xpect`, `jupyter-sysml-kernel`, `SysMLValidator` or
   `KerMLValidator`; (c) neither. A row can be both (a) and (b). Re-run with a short script over
   `docs/project/spec-compliance.md` and compare the per-section counts in Inventory A; a count
   that moves means a row was added or reworded and should be classified into one of the four
   kinds above.
2. **Adjudication records.** `pilot-differential.md`, `pilot-corpora.md`, `pilot-xpect.md`,
   `pilot-rejection.md`, `pilot-execution-referee.md`, `adjudications.md`,
   `validation-constraints.md`, `omg-issues.md`, `behavior-semantic-oracle.md`,
   `training-examples.md`, `errata-overlay.md` and `tools/oracle/errata/errata.go` were searched
   for `pilot is right`, `follow(s|ing) the pilot`, `as the pilot`, `ambiguous`, `needs (an
   )?opinion`, `unclear`, `not settled`, `spec(ification)? is silent`, `Open:`, `Question, not a
   bug report`. Each hit was read in context; the errata registry had none (its entries carry
   clause citations).
3. **Go sources.** `rg -n -i 'pilot|spec is silent|not in the spec|ambiguous|unclear|TODO'
   internal tools --glob '!*_test.go'`, then each hit was classified as a behaviour decision,
   wording/placement, or implementation compatibility. Only the first kind is listed.
4. **Negative corpus.** Every case's rationale line was checked for a clause or grammar citation
   against the committed referee baseline (`tools/referee/reject`, see
   [pilot-rejection.md](pilot-rejection.md)).

**Specification text.** Download the two PDFs from the OMG URLs in the overview, extract text
(any PDF text extractor; this audit used `pypdf`), and grep for the constraint names quoted in an
item. Record the document numbers printed on the title pages; if they differ from the ones stated
above, re-verify every clause number before editing an item.

**When to add an item.** Any change that (a) adds a rule or a behaviour whose rationale in
`spec-compliance.md`, a code comment or a PR is "the pilot does this" without a clause, (b)
adjudicates a differential disagreement as "the pilot is right" or "ambiguous", or (c) declines
to evaluate or report something the specification requires, gets a row here with the six fields
of the detailed items. Close an item by linking the PR or the upstream answer that settles it, and
move it to a **Closed** section rather than deleting it, so that the next audit does not re-derive
it.

**Author questions.** The open questions are items 2, 3, 5, 6, 7, 8, 10, 13, 14 and 16. They are
drafted for a maintainer to raise with the specification authors (as with the nested-redefinition
question); nothing has been posted upstream from this register, and the register should be updated
with the answer when one arrives.
