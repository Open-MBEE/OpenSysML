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
| 13 | Missed diagnostic | Indistinguishable memberships reported as a warning; repeated anonymous `perform a;` depends on bodies in the pilot | warning; silent for bodiless repeats | warning for every repeat by default, error under strict conformance | spec clear (a validation constraint); the pilot under-reports, ours does by default |
| 14 | Missed diagnostic | Constraints the pilot declares but never reports, absent from the published specification (`validateSubsettingPortionConformance`, `validateBindingConnectorArgumentTypeConformance`, `validatePartUsageType`, `validateItemUsageType`, …) | declared, unreported | not implemented | needs author opinion: are these normative? |
| 15 | Missed diagnostic | Lower-tier errors suppress later diagnostics the pilot still reports | reports secondary diagnostics over unresolved names | gates higher tiers | not a spec matter; recorded so it is not mistaken for one |
| 16 | Missed diagnostic | Body expression `{ … }` as a value typed `BooleanEvaluation` when its result is Boolean | so typed | as the pilot | spec ambiguous (implied typing of an expression body) |
| 17 | Cosmetic | Identifiers restricted to ASCII letters, digits and `_` | rejects non-ASCII basic names | same | pilot-following justified by KerML §8.2.2.3 |
| 18 | Cosmetic | Every reference subsetting after the first is reported | reports each extra | same | pilot-following justified by KerML §8.3.3.3 (at most one) |
| 19 | Cosmetic | `part p : ItemDef;` — a part typed only by a non-part definition | accepted | accepted | pilot-following justified: `parts` supplies `Part` through subsetting |
| 20 | Cosmetic | Alias identity: an `alias` is a name, not an element | resolves to the aliased element | fixed to match | pilot-following justified by KerML §8.3.2.4 (`Membership`) |
| 21 | Spurious error | A succession whose ends are qualified names (`first r::move then r::grip;` at package level) | `Must be an accessible feature (use dot notation for nesting)` at each end; the same ends on a `connect` are accepted | accepted, featured by the ends' innermost common featuring type, and executed | spec clear (KerML §8.3.4.5.3); pilot short |
| 22 | Spurious error | A KerML metaclass as the type of a SysML metadata usage (`metadata m : KerML::Classifier;`), the case KERML-90 was resolved to admit | three errors | one error | spec ambiguous: the text both tools follow does not achieve the KERML-90 resolution |
| 23 | Cosmetic | The implied subsetting of `outgoingHBLink` by a decision node's outgoing successions and of `incomingHBLink` by a merge node's incoming ones (SYSML21-306) | adds both, from `DecisionPerformance` where the OCL names `MergePerformance` | adds neither; its checks and execution do not consult them | spec defect (OCL, filed); ours short |
| 24 | Spurious error | The default `[1..1]` of a usage (SysML §7.6.3, non-normative: SYSML21-185) | by metaclass; any owned subsetting of a type-owned feature withholds it, but a subsetting of a package-owned feature does not | the same | spec ambiguous (prose only); both follow the pilot's rule for a package-owned target |
| 25 | Cosmetic | The declaration production of a case usage (SYSML2-783) | `ActionUsageDeclaration` | the same syntax | no observable difference |
| 26 | — | Five validation constraints the pilot source marks `TODO` | implemented under the `TODO` | implemented | no difference; the `TODO`s are stale |
| 27 | Spurious error | An explicitly declared subject or return parameter of a variation usage | accepted: every `ParameterMembership` is exempt | rejected: only objectives are exempt | spec clear (`validateUsageVariationOwnedFeatureMembership`), both short for objectives; pilot short for parameters |
| 28 | Cosmetic | A negative model-level-evaluable multiplicity bound (KERML-199) | rejected, by a `-2` marker the OCL does not have | rejected | spec OCL short of its prose; both follow the prose |

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
```sysml
part def A; attribute def A;
```

**Pilot:** `Duplicate of other owned member name` as a **warning**; for `perform a; perform a;`
it warns only when the repeats have bodies (observation, pilot `2026-07`,
[adjudications.md](adjudications.md)). On `part def A; attribute def A;`, and on the KerML
`class A; datatype A;`, it also warns `Duplicate of other owned member name`, twice: its
`Membership_isDistinguishableFrom_InvocationDelegate.java` compares the names only and carries
`// TODO: Add member element metaclass check` (pilot `2026-08`). The same omission accounts for
55 of the 76 `Duplicate of inherited member name` warnings its Xpect suite declares beside a
typing error — for example `'self' from Action, Part` at `ActionUsage_invalid.sysml.xt:40`,
where `Actions::Action::self` is an `ActionUsage` and `Parts::Part::self` a `ReferenceUsage` —
and for the `'p' from A2` warning of `RedefinitionDiamond_Invalid.sysml.xt` and
`RedefinitionDiamond1_invalid.sysml.xt`, where `part p` (a `PartUsage`) meets the inherited
`p :>> p` (a `ReferenceUsage`).

**OpenSysML:** a warning for every repeat, bodies or not; `part def A; part def A;` is likewise
a warning. Under strict conformance (`-strict`, `%strict`, `strictConformance`,
`strict_conformance`) every such finding — `Duplicate of other owned member name`, `Duplicate of
owned member name` (an alias), `Duplicate of other alias name`, the short-name variant, and
`Duplicate of inherited member name` for an owned member repeating an inherited one, a name
inherited from two supertypes, and the library-base variant of both — is an error; the finding
and its wording are those of the default mode. `part def A; attribute def A;` and `class A; datatype A;` are clean, as are the 55
diamonds and the two `RedefinitionDiamond` members above: two memberships whose member elements'
metaclasses conform in neither direction are distinguishable whatever their names
(`resolve.Resolver.DistinguishableByMetaclass`). Same or specializing metaclasses
(`part def A; item def A;`, `class A; class A;`, the Part/Port and DataValue/Occurrence `self`
diamonds) still warn, and so does a pair where either metaclass is unknown or an alias target
does not resolve. The Xpect harness builds each fixture from the library files its `XPECT_SETUP`
names, which never include `SysML.sysml` or `KerML.kerml`, so no metaclass is known there and
those rows still agree in [pilot-xpect.md](pilot-xpect.md).

**Specification:** KerML §8.3.2.4.5 `Namespace`, `validateNamespaceDistinguishibility` (so spelt in the PDF): "All
memberships of a Namespace must be distinguishable from each other." §8.3.2.4.3
`Membership::isDistinguishableFrom` compares `memberShortName`/`memberName` only where the
member elements' metaclasses are related: its first disjunct,
`not (memberElement.oclKindOf(other.memberElement.oclType()) or other.memberElement.oclKindOf(memberElement.oclType()))`,
makes two memberships distinguishable whatever their names when neither member element's
metaclass is the other's or a specialization of it. §8.3.3.3.4
`Feature::effectiveName`: a feature with no declared name takes the effective name of its
`namingFeature()`; SysML §8.3.17.14 `PerformActionUsage::namingFeature` "is its performedAction".
Two `perform a;` therefore both have `memberName` `a`, whatever their bodies.

**Assessment:** spec clear on both counts. The rule is a `validate…` constraint — a violating model
is not well-formed — so a warning under-reports it. The pilot does so; OpenSysML does so by
default (a warning was chosen so that duplicated names in the training corpus do not fail its
clean gate, and so that the pilot differential compares like with like) and reports the error
the specification calls for under strict conformance, the mode that judges a model as conforming
SysML v2. Inherited memberships are in scope: `Type::inheritedMembership` "subsets membership"
and is "included in the derived union for the memberships of the Type" (§8.3.3.1.10), §7.3.2.1
spells it out ("The member names of all inherited memberships must be distinct from each other
and from the member names of all owned memberships"), and a library supertype's members are
inherited like any other, so the resolver's and the library-base pass's inherited findings are
escalated alike. Imported collisions are not: §7.2.5.4 hides them, so the namespace keeps no
indistinguishable membership and `Duplicate of imported member name` stays a warning in every
mode (see [Imported memberships](#imported-memberships) below). The body-sensitivity is the
pilot's alone; ours follows the specification. On the metaclass clause the specification is
clear and the pilot is short: it does not implement the clause, and ours does.

**Question for the authors:** for two `perform a;` in one body, are the memberships
indistinguishable regardless of whether the usages have bodies?

#### Imported memberships

**Model text**

```sysml
package A { part def Engine; }
package B { part def Engine; }
package C {
    private import A::*;
    private import B::*;
    part e : Engine;
}
```

**Pilot:** silent. `KerMLValidator.checkNamespace` compares each *owned* membership with the
namespace's memberships and, for a type, with its inherited ones; two memberships that both
arrive by import are never compared (pilot `2026-08`, reading of `KerMLValidator.class` and
`NamespaceAdapter.getImportedMembership`). `e` is typed by `A::Engine`. For contrast, the Rust
sysml-toolkit (v0.10.2) reports `error: ambiguous reference 'Engine' resolves to multiple
memberships` at `part e : Engine;` — a use-site error the specification does not provide for.

**OpenSysML:** `warning: Duplicate of imported member name 'Engine': A::Engine (import A::*),
B::Engine (import B::*)` on `B::*` — once per colliding name per importing namespace, on the
import that brings the later membership, naming every colliding member and the import each came
through (`resolve.Resolver.checkImportedNames`). Resolution is unchanged: `Engine` in `C` is
`A::Engine`, the first matching membership. The same clauses as for owned names apply — short
names count as names, and two members whose metaclasses conform in neither direction
(`part def Engine` beside `attribute def Engine`) are distinguishable — and:

- *One membership reached twice is one membership.* `import A::*; import Q::*;` where `Q`
  publicly re-imports `A` surfaces one `Membership` of `A::Engine` through two imports and does
  not warn (`visibleMemberships` is an `OrderedSet(Membership)`).
- *Two memberships of one element do not warn either:* an alias beside the element it names
  (`package B { alias Engine for A::Engine; }`) and a membership import beside a wildcard that
  also surfaces the member (`import A::Engine; import A::*;`). Read literally, §8.3.2.4.3 makes
  these indistinguishable — they are distinct `Membership`s with equal names and one metaclass —
  but whichever membership `resolveLocal` takes first, the name denotes the same element, so the
  model has nothing to fix; the owned check treats an alias beside its target the same way
  (`sameElement`), and the pilot reports neither. An alias under another name is no such case:
  `alias Spare for A::Engine` binds `Spare`, and `A::Engine` reached after it still collides with a
  third `Engine` under its own name.
- *An imported name a type inherits takes part:* `Type::membership` is owned, imported and
  inherited memberships together, and `inheritedMemberships` removes only redefined features, so
  `part def Child :> Base { private import A::*; }` where both `Base` and `A` declare a `part x`
  warns on the import (`A::x (import A::*), Base::x (inherited)`); resolution keeps the inherited
  member, which it reaches first. An owned member of that name hides both and silences the
  warning, as it does for two imports. The pilot never compares imported with inherited
  memberships either.
- *An imported name an owned member hides takes no part:* `Namespace::importedMemberships(excluded)`
  excludes a membership whose names an owned membership repeats, so `part def Engine;` declared in
  `C` silences both imports.
- *A repeat among the imported namespace's own members is that namespace's duplicate,* reported
  where it is declared (or, for one package declared in two documents of a workspace, not at all),
  never at the importer. Only a package's declarations are one namespace this way; two types of
  one qualified name (`part def P { part x; } part def P { part x; }`) are two namespaces, and
  the `x` of each collides at an `import A::**` that reaches both.
- *Each declaration of a package carries its own imports.* The resolver reads a package declared
  in two documents as one namespace for its members but resolves the body of each declaration
  against the imports that declaration writes, so `package P { private import A::*; }` in one
  document and `package P { private import B::*; }` in another never bring `A::Engine` and
  `B::Engine` into one body, and no warning is reported for the pair. The pilot reads the two
  declarations as two namespaces.
- *Library content is left out,* as the inherited check leaves out library supertypes: the
  standard library is not the model's to fix. This is load-bearing — read with the library
  included, `import ISQ::*` alone brings eight colliding pairs, each a quantity ISO 80000
  defines in two parts: from `ISQElectromagnetism` (IEC 80000-6) and `ISQAtomicNuclear`
  (ISO 80000-10) `MagneticDipoleMomentValue`, `MagneticDipoleMomentUnit`,
  `CartesianMagneticDipoleMoment3dVector`, `CartesianMagneticDipoleMoment3dCoordinateFrame`,
  `magneticDipoleMoment` and `cartesianMagneticDipoleMoment3dVector` (two different
  quantities, L^3·M·T^-2·I^-1 and L^2·I); from `ISQSpaceTime` (ISO 80000-3) and
  `ISQCondensedMatter` (ISO 80000-12) `CartesianDisplacement3dVector` and
  `cartesianDisplacement3dVector`. Under §7.2.5.4 (next paragraph) none of the eight is a
  member of `ISQ`; a first-match reader binds the Electromagnetism or SpaceTime one, and the
  library's own `SI.sysml` trips over that at lines 233 and 303, which the declared errata
  overlay qualifies ([omg-issues.md](omg-issues.md), "Defects in the vendored quantity
  libraries"). `import NumericalFunctions::*; import DataFunctions::*;` brings sixteen more
  (`'+'`, `'*'`, `'=='`, …), the operator overloads invocation selects among by argument type.
  Model-vs-library pairs are likewise not reported.

**What the specification makes of an imported collision.** The warning's name says
"indistinguishable", but KerML does not leave two imported memberships of one name standing to
be judged by `validateNamespaceDistinguishibility`: §7.2.5.4 — "if the member name or member
short name of any imported membership conflicts with the name of any owned member, *or with the
name of any visible membership from any other imported namespace*, then the conflicting
membership is hidden and is not included in the set of imported memberships of the importing
namespace" — and §8.3.2.4.5 `Namespace::importedMemberships(excluded)` ("excluding Memberships
that have distinguishability collisions with each other or with any ownedMembership") remove
both. The namespace is well-formed; the name resolves to nothing in it, so `resolveLocal` walks
on to the outer scopes and an unqualified `Engine` in `C` is unresolved. Neither tool does
that: the pilot's `NamespaceImportAdapter.importMemberships` adds every visible membership of
the imported namespace, hiding by owned names only, so `C::Engine` resolves to the first import's
member; OpenSysML resolves the same way (`lookupImports`, in owned-import order), deliberately,
so that the two implementations agree on what every reference binds to. The warning is what
marks the deviation: it is reported on exactly the memberships §7.2.5.4 hides, where the spec
would leave the reference dangling and both tools bind it. A model that wants the spec's
outcome qualifies the name.
- *Private imports count:* visibility governs what `C` re-exports, not what it imports.
- *Overload sets are not exempt.* Two `calc def pick` reached through `import A::*; import B::*;`
  are indistinguishable memberships like two owned `pick` are, which already warn
  (`Duplicate of other owned member name`); invocation overload selection still chooses among
  them ([spec-compliance.md](spec-compliance.md), "Invocation overload selection"). The overload
  suites' diagnostic helpers set this warning aside, as they are about the selection.

Over the OMG corpora this reports 238 warnings, every one a same-metaclass pair under the
rule above: `13a-Model Containment.sysml` (six names — `Engine`, `Transmission`, `ClutchPort`,
`DrivePwrPort`, `EngineToTransmissionInterface`, `vehicle1_c1` — from
`'2a-Parts Interconnection'::*` and `'8-Requirements'::*`), `4a-Functional Allocation.sysml`
(`Definitions` and `Usages` from `'2a-Parts Interconnection'::*` and
`'3a-Function-based Behavior-1'::*`), the kerml-examples `Imports.kerml` (`P::A` beside `Q::A`,
and `Q::D` beside `Q::Q1::D` under `import Q::**`), `AHFNorwayTopics.sysml` (two names under
`AHFCoreLib::**`), and the Annex A vehicle model, where a recursive `import … ::**` of
`VehicleConfiguration_b` surfaces, besides the three `vehicle_b` (`PartsTree::vehicle_b`,
`DiscreteInteractions::CruiseControl1::vehicle_b`, `…CruiseControl2::vehicle_b`), forty-odd
nested feature names — `vehicle_b::mass` beside `vehicle_b::fuelTank::mass`,
`rearWheel1::diameter` beside `rearWheel2::diameter`, the requirement short name `'1'` of
`vehicleMassRequirement` beside that of `engineMassRequirement` — at each of the four importing
namespaces; `VehiclePartDef::Vehicle::*` beside `FuelTankPartDef::FuelTank::*` brings two `mass`.
Recursive imports reach that far because `Namespace::visibleMemberships(…, isRecursive = true)`
recurses into every owned member that is a `Namespace`, types and features included; the pilot's
`NamespaceUtil.getVisibleMembershipsFor` does the same. The pilot is silent on all 238, which the
differential records as only-ours rows ([pilot-differential.md](pilot-differential.md)); the Xpect
suite declares none of them and no agreeing row moves.

**Specification:** KerML §8.3.2.4.5 `Namespace::membership` is "all Memberships in this
Namespace, including (at least) the union of `ownedMemberships` and `importedMemberships`", and
`validateNamespaceDistinguishibility` ranges over `membership`. §8.3.2.4.5 `resolveLocal` and
`resolveVisible` select the memberships whose `memberShortName` or `memberName` match and take
`->first()`, so resolution never fails on multiplicity. §8.3.2.4.3 `Membership::isDistinguishableFrom`
as above. §8.3.2.4.4 `Import::importedMemberships`, `Namespace::importedMemberships(excluded)`
("excluding … a Membership whose … name is the same as an ownedMembership's").

**Assessment:** spec clear, both implementations short in the same way: §7.2.5.4 hides a name
two imports bring, so the reference does not resolve; the pilot binds the first import's member
and reports nothing, and OpenSysML binds the same member and warns at the import that brought
the collision. The toolkit's use-site error has the spec's outcome (the name is unresolved) but
reports it as an ambiguity at the reference rather than a hidden name at the namespace. The
exclusions listed above are the adjudicated readings.

**Questions for the authors:** two memberships of one element — an alias beside the element, or a
membership import beside a wildcard that surfaces it — are indistinguishable by the letter of
§8.3.2.4.3; is a namespace holding them intended to be ill-formed? And is the pilot's
first-import binding of a name §7.2.5.4 hides intended, given that the Quantities and Units
library itself relies on it (`SI.sysml`:233 and 303, unqualified `MagneticDipoleMomentUnit`
under `import ISQ::*`), and that the eight `ISQ` names above are unreachable through `ISQ` by
the letter of the clause?

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

### 21. A succession whose ends are qualified names

**Model text**

```sysml
package R {
    part def Robot { perform action move; perform action grip; }
    part r : Robot;
    first r::move then r::grip;
}
```

**Pilot:** `2026-08` (`jupyter-sysml-kernel` 0.62.0) reports
`nsq.sysml:4:11: error: Must be an accessible feature (use dot notation for nesting)` and the
same at `4:24`. The same two ends on `connect r::move to r::grip;` validate clean. The
`spec-compliance.md` example `first part1::action1 then requirement1;` draws the same error at
its first end. The dot-chain form `first b.g then b.m;` validates clean.

**OpenSysML:** accepts the succession. It is featured by `Robot`, the innermost common
featuring type of `Robot::move` and `Robot::grip`, and orders `grip` after `move` on every
`Robot` it runs ([behavior-semantic-oracle.md](behavior-semantic-oracle.md#a-succession-outside-a-behavior-body-orders-the-performances-it-relates-wherever-they-run)).
Ends with no common featuring type (`first p1::a then p2::b` across two package-level parts)
are rejected with the pilot's message, and the pilot rejects them too.

**Specification:**
- KerML §8.3.4.5.3 `deriveConnectorDefaultFeaturingType`: "the innermost common direct or
  indirect featuringType of the relatedFeatures".
- `checkConnectorTypeFeaturing`: each related feature has each featuring type of the
  connector as a direct or indirect featuring type.
- §8.4.4.6.1 and Table 11 note 2: an implied TypeFeaturing to the `defaultFeaturingType` is
  added when the connector has no owning type, no owned TypeFeaturing, and a non-null
  default.
- A Succession is a Connector (§8.3.4.5.4), and SysML §8.4.9.4 applies the same to
  `SuccessionAsUsage`.

Nothing in these clauses treats a succession's ends differently from a connector's.

**Assessment:** pilot short, ours follows the specification. The pilot accepts the connector
form, which suggests that its succession check does not consult the default featuring type.
That suggestion comes from comparing behavior, not from reading the pilot's source.

**Question for the authors:** is a package-owned succession whose ends name members of one
type, by qualified name, intended to be well formed by the implied TypeFeaturing of KerML
§8.4.4.6.1, as the same connector is?

### 22. A KerML metaclass as the type of a SysML metadata usage (KERML-90)

**Model text**

```sysml
package P { part def A { metadata m : KerML::Classifier; } }
```

with, for comparison, the KerML spelling `classifier A { metadata m : KerML::Classifier; }` and
`package P { metadata def MC :> KerML::Classifier; part def A { metadata m : MC; } }`.

**Pilot:** `2026-08` (`jupyter-sysml-kernel` 0.62.0), `validate-sysml-batch` (observation):

```
kerml90.sysml:1:26: error: A metadata usage must be typed by one metadata definition.
kerml90.sysml:1:26: error: Must have exactly one metaclass
kerml90.sysml:1:26: error: Must have a concrete type
```

The KerML spelling (`validate-kerml`) and the `metadata def MC` spelling validate clean. In the
pinned source (reading), `KerMLValidator.checkMetadataFeature` opens with

```
// TODO: Submit new issue to revise this to actually fix the problem KERML-90 was trying to address.
// validateMetadataFeatureMetaclass
if (mf.type.filter(Metaclass).size() != 1) {
```

and checks `validateMetadataFeatureMetaclassNotAbstract` as `mf.type.exists[abstract]`, over every
type rather than the metaclass; `SysMLValidator.checkMetadataUsage` adds
`checkOneType(usg, Metaclass, …)`, which requires `FeatureUtil.getAllTypesOf` to return exactly
one type. The model has two: `KerML::Classifier` and the abstract `Metadata::MetadataItem`, so
all three checks fail.

**OpenSysML:**

```
kerml90.sysml:1:26: error: A metadata usage must be typed by one metadata definition.
sysml: kerml90.sysml did not analyse cleanly; no check was made
```

The error is the type tier's (`internal/check/passes/typecheck.go`, with the message from
`pilotTypingMessage` in `internal/check/passes/w10b_usage_typing.go`): a SysML `metadata` usage
must be typed by a metadata definition (`usageWantsDefKind`), and `KerML::Classifier` is a KerML
metaclass. The error stops the later tiers, so the two KerML-side messages are not
reached. The KerML spelling and the `metadata def MC` spelling are `no errors`.

**Specification:**
- KERML-90 (<https://issues.omg.org/issues/KERML-90>, closed in KerML 1.0b2) reported that
  `MetadataFeature::metaclass`, a `[0..1]` redefinition of `type`, left a metadata usage typed by
  a KerML metaclass two types, because SysML `checkMetadataUsageSpecialization` adds
  `Metadata::MetadataItem`. The resolution changed `metaclass` to a subset of `type`: KerML
  §8.3.4.12.3 `MetadataFeature`, "/metaclass : Metaclass [0..1] {subsets type}".
- The same clause keeps `validateMetadataFeatureMetaclass`: "A MetadataFeature must have exactly
  one type that is a Metaclass." (`type->selectByKind(Metaclass).size() = 1`), and
  `validateMetadataFeatureMetaclassNotAbstract`: `not metaclass.isAbstract`.
- SysML §8.3.27.3 `MetadataUsage`, `checkMetadataUsageSpecialization`: "A MetadataUsage must
  directly or indirectly specialize the base MetadataUsage Metadata::metadataItems from the
  Systems Model Library." `metadataItems` is typed by `Metadata::MetadataItem`, declared
  `abstract metadata def MetadataItem :> Metaobject, Item`; a metadata definition is a
  Metaclass. The same clause declares `/metadataDefinition : Metaclass [0..1] {redefines
  itemDefinition, metaclass}`.

The constraint that rejects the model is therefore KerML `validateMetadataFeatureMetaclass`,
inherited by `MetadataUsage`: `checkMetadataUsageSpecialization` makes `MetadataItem` a type
of `m`, `KerML::Classifier` does not specialize it, and both are Metaclasses, so
`selectByKind(Metaclass)` has size 2. KERML-90 removed the `[0..1]` conflict on `metaclass`
but not this count, because the second type is itself a Metaclass. In KerML the implied
supertype is `Metaobjects::metaobjects`, typed by `Metaobjects::Metaobject`, which
`KerML::Classifier` specializes, so one type remains. SysML 2.0 Part 1 states no constraint
named `validateMetadataUsageType`, the pilot's first message.

**Assessment:** both tools follow the published text, and the text does not achieve what
KERML-90 was resolved to allow. This is a specification question, not an implementation gap. If
the text were revised to admit the model, OpenSysML would have to change its type-tier rule as
well as its metaclass count; the pilot would have to change its two counts.

**Question for the authors:** is a KerML metaclass that does not specialize
`Metadata::MetadataItem` intended to type a SysML metadata usage? If so, should
`validateMetadataFeatureMetaclass` discount the type implied by `checkMetadataUsageSpecialization`,
or should that constraint not apply to such a usage? If not, should SysML say so (for instance by
typing `metadataDefinition` by `MetadataDefinition`)? Drafted in
[omg-issues.md](omg-issues.md#proposed-specification-issue-a-kerml-metaclass-cannot-type-a-sysml-metadata-usage-kerml-90-follow-up).

### 23. Decision-node outgoing and merge-node incoming successions are not given their implied subsetting (SYSML21-306)

**Model text**

```sysml
package P {
    action def A {
        attribute x : ScalarValues::Integer;
        first start;
        then decide d;
            if x > 0 then a;
            if x <= 0 then b;
        action a;
        action b;
        then merge m;
        first a then m;
        first b then m;
    }
}
```

**Pilot:** validates clean (observation, `2026-08`). Reading: `SuccessionAsUsageAdapter.addDefaultGeneralType`
calls `addDecisionNodeOutgoingSuccessionSpecialization` and
`addMergeNodeIncomingSuccessionSpecialization`, which add an implied specialization of the chain
from the node to `ControlPerformances::DecisionPerformance::outgoingHBLink` and
`ControlPerformances::MergePerformance::incomingHBLink` (`ImplicitGeneralizationMap`, keys
`decision` and `merge`). The first carries

```
 * TODO: Update checkDecisionNodeOutgoingSuccessionSpecialization
 *
 * OCL refers to MergePerformance::outgoingHBLink rather than DecisionPerformance::outgoingHBLink.
 * See SYSML21-306
```

**OpenSysML:** validates clean. It adds neither implied subsetting:
`rg -n 'outgoingHBLink|incomingHBLink' internal` finds only the library declarations and their
comments (`ControlPerformances.kerml` lines 22–48, `Actions.sysml` lines 303–317) and one comment in
`internal/exec/runtime/state_route.go`. The control-node constraints
(`internal/check/passes/behavior/control_node.go`, `ControlNodeSuccessionPass`) count a node's
successions and read their declared end multiplicities; execution takes exactly one outgoing
succession of a decision procedurally (`internal/exec/runtime/state_route.go`, `followOut`, citing
`DecisionPerformance::outgoingHBLink: HappensBefore[1]`). Neither consults the subsetting.

**Specification:**
- SysML §8.3.17.7 `DecisionNode`, `checkDecisionNodeOutgoingSuccessionSpecialization`: "All
  outgoing Successions from a DecisionNode must subset the inherited outgoingHBLink feature of the
  DecisionNode", with OCL
  `resolveGlobal('ControlPerformances::MergePerformance::outgoingHBLink')`. `MergePerformance`
  declares `incomingHBLink`, not `outgoingHBLink`; `DecisionPerformance` declares
  `outgoingHBLink` (`ControlPerformances.kerml`).
- §8.3.17.13 `MergeNode`, `checkMergeNodeIncomingSuccessionSpecialization`: "All incoming
  Successions to a MergeNode must subset the inherited incomingHBLink feature of the MergeNode."
- §8.4.13.4 Control Nodes repeats the mix-up in prose: "checkDecisionNodeOutgoingSuccessionSpecialization
  requires that any incoming Succession to a MergeNode specialize the Feature
  DecisionPerformance::outgoingHBLink".
- SYSML21-306 (<https://issues.omg.org/issues/SYSML21-306>, open) reports the OCL's qualified name.

**Assessment:** a specification defect, filed; the pilot implements the evident intent. OpenSysML
does not materialise either implied relationship, which is an OpenSysML limitation: a model's
specializations as OpenSysML reports them (reflective queries, exported models) lack the two
subsettings the pilot adds. Validation and execution do not depend on them.

**Question for the authors:** none beyond SYSML21-306. The §8.4.13.4 sentence should be corrected
with it.

### 24. Default multiplicity of a usage (SYSML21-185)

**Model text:** each probe declares a usage without a multiplicity in a definition `D` and
redefines it with `[0..*]` in a specialization, so that the redefinition warnings
(`validateRedefinitionMultiplicityConformance`, `validateSubsettingMultiplicityConformance`)
appear exactly when the original took the default `[1..1]`:

```sysml
package P { enum def E { enum a; enum b; } part def D { enum e : E; } part def F :> D { enum :>> e [0..*]; } }
```

**Pilot:** reading. `ImplicitGeneralizationMap`:

```
// TODO: Update SysML specification to formalize default multiplicities.
// See SYSML21-185
put(MultiplicityImpl.class, "feature", "Base::exactlyOne");
```

The default is added when `isAddMultiplicity()` holds. `AttributeUsageAdapter`,
`ItemUsageAdapter` (and so every part usage) and `PortUsageAdapter` return
`UsageAdapter.isAddDefaultMultiplicity()`; `ConnectionUsageAdapter` returns `isEnd()`:

```java
return target.isEnd() ||
       target.getOwningType() != null &&
       target.getOwnedSubsetting().stream().
            map(Subsetting::getSubsettedFeature).
            filter(f->f != null).
            map(FeatureUtil::getBasicFeatureOf).
            noneMatch(f->f != null && f.getOwningType() != null);
```

`getOwnedSubsetting` returns every owned `Subsetting`, which includes `Redefinition`,
`ReferenceSubsetting` and `CrossSubsetting` (KerML §8.3.3.3.8–§8.3.3.3.10). `getBasicFeatureOf`
is the last chaining feature of a feature chain and the feature itself otherwise.

**OpenSysML:** `semantics.ImplicitMultiplicityApplies` (`internal/semantic/semantics/multiplicity.go`)
requires a usage owned by a type (`featureOwnedByType`: not owned by a package or namespace),
whose metaclass is an attribute usage (including an enumeration usage), an item usage (including
part, view, rendering, actor and stakeholder usages) or a port usage, and with no `RelSubsets`,
`RelRedefines`, `RelReferences` or `RelCrosses` relationship whose target, read to the last
feature of a chain, is owned by a type. Connection usages and their subclasses, metadata usages,
subjects and objectives take no default. An end feature takes `[1..1]`
separately (`internal/check/passes/multiplicity_conformance.go`, `conformanceMultiplicity`,
`declaresEndFeature`).

Observations, `2026-08` and this tree, each model in a file of its own (`D`, `E`, `F` as above):

| Case | Model (inside `package P { … }`) | Pilot | OpenSysML |
|---|---|---|---|
| attribute, part, port, `ref part`, in a part or attribute definition | `part def D { part x; } part def E :> D { part :>> x [0..*]; }` (and the other keywords) | both warnings | both warnings |
| occurrence | `part def D { occurrence x; } part def E :> D { occurrence :>> x [0..*]; }` | none | none |
| enumeration usage | `enum def E { enum a; enum b; } part def D { enum e : E; } part def F :> D { enum :>> e [0..*]; }` | both warnings at `1:98` | both warnings at `1:98` |
| view usage | `view def V; part def D { view v : V; } part def F :> D { view :>> v [0..*]; }` | both warnings at `1:79` | both warnings at `1:79` |
| rendering usage | `rendering def R; part def D { rendering r : R; } part def F :> D { rendering :>> r [0..*]; }` | both warnings at `1:94` | both warnings at `1:94` |
| actor, stakeholder | `part def U; requirement def R { subject s : U; actor a : U; stakeholder k : U; } requirement def R2 :> R { subject :>> s; actor :>> a [0..*]; stakeholder :>> k [0..*]; }` | both warnings at `1:145` and `1:171` | both warnings at `1:145` and `1:171` |
| subsetting a type-owned feature | `part def D { part a [0..*]; part b :> a; } part def E :> D { part :>> b [0..*]; }` | none | none |
| redefining a type-owned feature | `part def D { part a; } part def E :> D { part :>> a; } part def F :> E { part :>> a [0..*]; }` | none | none |
| reference-subsetting a type-owned feature | `part def D { part a [0..*]; part b ::> a; } part def E :> D { part :>> b [0..*]; }` | none | none |
| subsetting a chain ending in a type-owned feature | `part p { part a [0..*]; } part def D { part b :> p.a; } part def E :> D { part :>> b [0..*]; }` | none | none |
| subsetting a package-owned feature | `part q [0..*]; part def D { part b :> q; } part def E :> D { part :>> b [0..*]; }` | both warnings at `1:83` | both warnings at `1:83` |
| end feature | `connection def C { end part a; end part b; } connection def C2 :> C { end part :>> a [0..*]; end part :>> b; }` | `End feature must have multiplicity 1` at `1:83`; the upper-bound warning at `1:96` | the same two, the first with OpenSysML's explanation appended |

"Both warnings" is `warning: Redefining feature should not have smaller multiplicity lower bound`
and `warning: Subsetting/redefining feature should not have larger multiplicity upper bound` at
the redefining feature.

**Specification:** SysML §7.6.3 Usages (non-normative): "a tighter default of [1..1] is implicitly
declared for the usage if all of the following conditions hold: 1. The usage is an attribute usage,
an item usage (including a part usage, except if it is a connection usage), or a port usage. 2. The
usage is owned by a definition or another usage (not a package). 3. The usage does not have any
explicit owned subsettings or owned redefinitions." An `EnumerationUsage` is an `AttributeUsage`
(§8.3.8.3), `ViewUsage` and `RenderingUsage` are `PartUsage`s (§8.3.26.11, §8.3.26.6), and actors
and stakeholders are `PartUsage`s owned through `ActorMembership`/`StakeholderMembership`. The end
default is separate, §7.13.2: "If a multiplicity is not explicitly declared for an end feature,
then a default of 1..1 is implicitly declared for it (regardless of the usual conditions for
default multiplicity given in 7.6.3)". SYSML21-185
(<https://issues.omg.org/issues/SYSML21-185>, open) records that Clause 8 states none of this
normatively.

**Assessment:** the two tools agree on every case above. Both select the default by usage
metaclass, as condition 1 does, and both count every owned subsetting (`:>`, `:>>`, `::>` and
cross subsetting) as condition 3 does, with one exception: both keep the default when every
subsetted feature is owned by no type, where condition 3 withholds it for any explicit owned
subsetting. Both follow the pilot's rule rather than the prose there, a question of which rule
is intended.

**Question for the authors:** when SYSML21-185 formalizes the default, is condition 3 meant
literally (any owned subsetting or redefinition withholds it), or only a subsetting of a feature
that has a featuring type, as the pilot implements?

### 25. The declaration production of a case usage (SYSML2-783)

**Model text**

```sysml
package P {
    case def K;
    case k : K;
    case k2 : K :> k;
    case k3 [1] : K = k;
}
```

**Pilot:** validates clean (observation). `SysML.xtext` (reading):

```
// TODO: Correct erroneous use of ConstraintUsageDeclaration for CaseUsage from resolution of SYSML2-783.

CaseUsage returns SysML::CaseUsage :
	OccurrenceUsagePrefix CaseUsageKeyword ActionUsageDeclaration CaseBody
;
```

**OpenSysML:** validates clean (`case` is parsed as a usage declaration in
`internal/syntax/parser/defusage.go`).

**Specification:** SysML §8.2.2.22 `CaseUsage = OccurrenceUsagePrefix 'case' ConstraintUsageDeclaration CaseBody`.
§8.2.2.20 `ConstraintUsageDeclaration : ConstraintUsage = UsageDeclaration ValuePart?` and
§8.2.2.17.2 `ActionUsageDeclaration : ActionUsage = UsageDeclaration ValuePart?`. SYSML2-783
(<https://issues.omg.org/issues/SYSML2-783>, closed in SysML 2.0b2) revised the notation
productions.

**Assessment:** the two productions have the same right-hand side; only the declared target
metaclass differs, and the `CaseUsage` rule creates the element. No observable difference. The
pilot's comment says the published production, not its own, is in error.

### 26. Validation constraints marked TODO in the pilot source

The pilot source marks five constraints `TODO` and implements each immediately after the
comment; OpenSysML implements all five. The eight control-node constraints the pilot marks
`TODO: Check … (?)` (`SysMLValidator.xtend`, `checkControlNode` and its siblings) and does not
implement are item 11.

| Constraint | Pilot `TODO` (`KerMLValidator.xtend`) | OpenSysML |
|---|---|---|
| `validateRedefinitionMultiplicityConformance` | `// TODO: Add validateRedefinitionMultiplicityConformance`, then the lower-bound check | `internal/check/passes/multiplicity_conformance.go`, `constraintChecker.checkMultiplicityConformance` |
| `validateSubsettingMultiplicityConformance` | `// TODO: Add validateSubsettingMultiplicityConformance`, then the upper-bound check | the same function |
| `validateBindingConnectorTypeConformance` | `// TODO: Add validateBindingConnectorTypeConformance`, then `//Binding type conformance` | `internal/check/passes/w9c_bound_feature_types.go`, `W9CBoundFeatureTypesPass.Run` |
| `validateFlowEndSubsetting` | `// TODO: Add validateFlowEndSubsetting? validateFlowEndImplicitSubsetting?`, then `getSubsettedNotRedefinedFeaturesOf(flowEnd).isEmpty` | `internal/check/passes/w8d_flow_end.go`, `W8DFlowEndPass.Run` |
| `validateOperatorExpressionCastConformance` | `// TODO: Add validateOperatorExpressionCastConformance`, then the `as` check | `internal/check/passes/typecheck_expr.go`, `exprChecker.checkCast` |

The models are the census probes under `tools/census/validation/testdata/probes/`, each a
package `P`; pilot `validate-kerml`, `2026-08`, and this tree (observations):

- `class A { feature x [1..2]; } class B specializes A { feature y [0..2] redefines x; }` —
  both: `5:54: warning: Redefining feature should not have smaller multiplicity lower bound`.
- `class A { feature x [0..2]; } class B specializes A { feature y [0..5] subsets x; }` — both:
  `5:52: warning: Subsetting/redefining feature should not have larger multiplicity upper bound`.
- `class C { feature x : ScalarValues::String; feature y : ScalarValues::Integer; binding x = y; }`
  — both: `4:82: warning: Bound features should have conforming types`.
- `class A { out feature o : ScalarValues::Integer; } class B { in feature i : ScalarValues::Integer; }
  class C { feature a : A; feature b : B; flow from A::o to B::i; }` — both:
  `6:43: error: Must have at least two related elements`, then at `6:53` and again at `6:61`
  `error: Cannot identify flow end (use dot notation)` and
  `error: Must be an accessible feature (use dot notation for nesting)`.
- `feature s : ScalarValues::String; feature n = s as ScalarValues::Integer;` — pilot:
  `5:15: warning: Cast argument should have conforming types`; OpenSysML: `5:15: warning: cast
  argument is typed by String, unrelated to the target Integer: neither type specializes the
  other, so the cast selects no value`.

**Assessment:** no difference in verdict. Every one of the five `TODO`s is stale, not only the
cast's. The census rows for these constraints are in
[validation-constraints.md](validation-constraints.md).

### 27. Variation parameters and stakeholder specialization

**Model text**

```sysml
package P {
    calc def C;
    variation calc vc : C { variant calc c1 : C; }
    variation requirement vr { variant requirement r1; }
    variation case vk { variant case k1; }
}
```

and, one per file, `variation calc vp { in p; variant calc c2; }`,
`part def X; variation requirement vr { subject s : X; variant requirement r1; }`,
`variation calc vc { return r : ScalarValues::Real; variant calc c1; }`,
`variation case vk { objective o; variant case k1; }`; and
`part def X; requirement def R { subject y : X; stakeholder s : X; }` against
`part def X; part def Y { stakeholder s : X; }`.

**Pilot:** reading. `SysMLValidator.checkUsage`, for `validateUsageVariationOwnedFeatureMembership`:

```
// NOTE: Need to allow parameters and objectives because they are currently physically inserted by transform implementation.
// TODO: Add allowance of parameters and objectives in variations to spec? Or remove when possible?
if (!(mem instanceof ParameterMembership || mem instanceof ObjectiveMembership)) {
```

`PartUsageAdapter.isRequirementStakeholder` adds the implied specialization of
`Requirements::RequirementCheck::stakeholders` only when the stakeholder's owning type is a
requirement definition or usage, noting that "checkPartUsageStakeholderSpecialization OCL doesn't
explicitly require the owningType to be a RequirmentDefinition or RequirementUsage".

Observations, `2026-08`, and this tree:

| Model | Pilot | OpenSysML |
|---|---|---|
| the three variations above | clean | clean |
| `in p` in a variation calc | `2:25: error: An owned usage of a variation must be a variant.` | the same |
| `subject s : X` in a variation requirement | clean | `1:52: error: An owned usage of a variation must be a variant.` |
| `return r` in a variation calc | clean | `1:33: error: An owned usage of a variation must be a variant.` |
| `objective o` in a variation case | clean | clean |
| `stakeholder` in a requirement definition | clean | clean |
| `stakeholder` in a part definition | `3:18: error: mismatched input 'stakeholder' expecting '}'` and `4:1: error: extraneous input '}' expecting EOF` | `3:18: error: 'stakeholder' declares a stakeholder of a requirement and is only allowed in a requirement body; move it into the requirement it belongs to` |

**OpenSysML:** `internal/check/passes/w8d_variability.go`, `checkMembers`, exempts an objective,
a metadata usage and, in an enumeration, an enumerated value, and reports a subject member and
every other non-variant usage. A stakeholder outside a requirement body is a parser diagnostic
(`internal/syntax/parser/defusage.go`).

**Specification:** SysML §8.3.6.4 `Usage`, `validateUsageVariationOwnedFeatureMembership`: "If a
Usage is a variation, then it must not have any ownedFeatureMemberships."
(`isVariation implies ownedFeatureMembership->isEmpty()`); §8.3.6.2 states the same for a
definition. A `ParameterMembership` and an `ObjectiveMembership` are `FeatureMembership`s. SysML
§8.3.11.3 `PartUsage`, `checkPartUsageStakeholderSpecialization`: "If a PartUsage is owned via a
StakeholderMembership, then it must directly or indirectly specialize either
Requirements::RequirementCheck::stakeholders."

**Assessment:** the parameters a variation gets without writing them, and the stakeholder
restriction, make no observable difference: the inserted parameters draw nothing in either tool,
and both grammars admit `stakeholder` only in a requirement body, where the pilot's condition
always holds. The parameter exemption does differ for a parameter written in the model: the pilot
exempts every `ParameterMembership`, so an explicit subject or return parameter is accepted,
while OpenSysML exempts only objectives and rejects it. By the OCL both are rejected, and so is
the objective both tools accept. Neither tool is fully faithful.

**Question for the authors:** the pilot's own: should a variation be allowed parameters and an
objective? If it should, `validateUsageVariationOwnedFeatureMembership` and
`validateDefinitionVariationOwnedFeatureMembership` need to exempt them. If not, should the
inserted ones be inserted at all?

### 28. Multiplicity bound non-negativity (KERML-199)

Two corrections the pilot maintainers have made or ruled on are already recorded, and only the
second needs a register entry:

- Systems-Modeling/SysML-v2-Pilot-Implementation#802, `Type::multiplicity` reached through
  alias and reference memberships, fixed upstream on ST6RI-975, is item 9 and
  [omg-issues.md](omg-issues.md#a-multiplicity-is-found-through-aliases-and-references-pilot-2026-07).
- Systems-Modeling/SysML-v2-Pilot-Implementation#803 is recorded in
  [omg-issues.md](omg-issues.md#a-bound-naming-a-package-level-feature-is-rejected-whatever-its-type-pilot-2026-07)
  and [pilot-differential.md](pilot-differential.md#multiplicity-bound-result-types-round). What
  follows is the OCL point behind it.

**Model text**

```kerml
package P { feature k : ScalarValues::Integer = -1; feature d [k]; }
```

and `package P { feature n : ScalarValues::Natural = 2; feature e [n]; }`.

**Pilot:** `validate-kerml`, `2026-08` (observation):
`kerml199.kerml:1:64: error: Must have a Natural value`; the second model is clean. Reading:
`KerMLValidator.checkMultiplicityRange`:

```
// TODO: Correct validateMultiplicityBoundResults OCL from KERML-199.
// validateMultiplicityRangeBoundResultTypes
for (b: mult.bound) {
    if (if (b.isModelLevelEvaluable) mult.valueOf(b) == -2 else !b.isInteger) {
```

and `MultiplicityRange_valueOf_InvocationDelegate` returns `-2` "to represent a "null" result".

**OpenSysML:** the same error at `1:64`; the second model is `no errors`.
`internal/check/passes/w8c_multiplicity_bounds.go`, `multiplicityBoundsChecker.checkBound`: an
evaluable bound must fold to a non-negative integer or `*`; any other bound must have an
Integer-conforming result type.

**Specification:** KerML 1.1 Beta 2 §8.3.4.11.2 `MultiplicityRange`,
`validateMultiplicityRangeBoundResultTypes`; the clause has the same number in KerML 1.0
(formal/2026-03-01), from which this quotation is taken: "The results of the bound Expression(s)
of a MultiplicityRange must be typed by ScalarValues::Intger from the Kernel Data Types Library.
If a bound is model-level evaluable, then it must evaluate to a non-negative value." (so spelt),
with OCL

```
bound->forAll(b |
    b.result.specializesFromLibrary('ScalarValues::Integer') and
    let value : UnlimitedNatural = valueOf(b) in
    value <> null implies value >= 0
)
```

The same clause's `valueOf` returns `null` for a bound that is not model-level evaluable, whose
evaluation is not a single element, or whose `LiteralInteger` value is negative (`if value >= 0
then value else null`), so `value >= 0` holds whenever `value <> null` and the clause cannot
fail. KERML-199 (<https://issues.omg.org/issues/KERML-199>, closed) introduced the
non-negativity sentence.

**Assessment:** the OCL is short of its prose. Both tools implement the prose, the pilot by
reading `valueOf`'s `null` as its `-2` marker. No difference.

**Question for the authors:** should `validateMultiplicityRangeBoundResultTypes` test the
evaluated bound directly, for example `bound.isModelLevelEvaluable implies valueOf(b) <> null`,
so that its OCL rejects a negative bound as its prose does?

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

The register is an audit of five sources, each re-runnable from the repository root:

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

5. **Pilot source.** The pinned tag, cloned sparsely (`org.omg.kerml.xtext`, `org.omg.sysml.xtext`,
   `org.omg.sysml.logic`), was searched with `rg -n 'TODO|NOTE:|KERML-[0-9]+|SYSML2-[0-9]+|SYSML21-[0-9]+'`
   for the places where the pilot's own source admits a departure from the specification or cites
   an OMG issue. Each hit was read against the OpenSysML code and, where a model shows it, both
   validators were run. Items 22–28 and the membership-distinguishability entry in
   [omg-issues.md](omg-issues.md#defects-in-the-pilot-implementation) come from this sweep.

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

**Author questions.** The open questions are items 2, 3, 5, 6, 7, 8, 10, 13, 14, 16, 22, 24, 27 and 28. They are
drafted for a maintainer to raise with the specification authors (as with the nested-redefinition
question); nothing has been posted upstream from this register, and the register should be updated
with the answer when one arrives.
