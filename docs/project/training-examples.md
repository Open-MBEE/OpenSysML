# Training Examples Status

## Overview

**Source:** [SysML-v2-Pilot-Implementation](https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) training examples  
**Download:** https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation/tree/master/sysml/src/training  
**Status:** 100/100 files parse and resolve cleanly (0 semantic errors)  
**Errors**: none recorded  
**Gate**: the per-file error counts are recorded in `internal/core/model/testdata/training_examples_expected.txt`, so `TestTrainingExamplesSemanticErrors` fails when a file regresses *or* improves without updating the list (`-update-training` regenerates it)  

Defects found in OMG material *outside* this corpus — a wrong declaration in a vendored
specification library rather than a model using it — are recorded in
[omg-issues.md](omg-issues.md), so there is one place to look for both.

These training examples are from the official OMG pilot implementation and are not vendored here. Run `./scripts/download-training-examples.sh` to fetch the pinned (`2026-07`) copy into `examples/sysml-v2-training/`; the tests that read it skip while it is absent.

CI is not allowed that skip. `.github/workflows/pr.yml` runs the download script before the
suite and exports `OPENSYSML_REQUIRE_TRAINING_CORPUS=1`, under which a missing or empty
corpus fails the gate instead of skipping it, and a dedicated `Corpus gate` step prints the
`N/100 training files clean` line and fails on a `--- SKIP`. The corpus directory is cached
between runs, keyed on the download script, but the cache cannot mask an absent corpus:
the presence check and the required-corpus env var both run after the restore.

---

## Adjudicating a change in the expectations

The expectation file is a snapshot of this implementation's behavior, not an
oracle: regenerating it with `-update-training` records whatever the code now
reports, so a regression re-baselines just as quietly as a fix. Every entry that
changes must therefore be judged against the OMG model that produced it, and the
verdict recorded below, before the new count is committed:

- **A file that got cleaner** is only an improvement if the references it used to
  report now resolve to the right declarations. Confirm that — a file also goes
  clean when a construct stops being parsed or checked at all.
- **A file that reports more** is a regression until shown otherwise. New
  diagnostics that are false positives stay recorded, with the gap named here.
- **A count that moves without a code change in that area** usually means the
  harness moved. Both cases found so far were harness artifacts, not semantics.

### Verdicts for the 2026-08 re-pin (71/100)

The recorded counts had been generated seven PRs earlier and were re-adjudicated
file by file:

**Real fixes made while adjudicating**

| Finding | Verdict |
|---|---|
| `variant attribute diameterSmall = 70[mm];` reported `expected '{' or ';' after declaration` (`Variation Definitions`) | Regression in keyword-name parsing: the prefix `variant` took the kind keyword `attribute` as its name. Fixed in `parser.parseDefUsage`. |
| `ambiguous reference: SI::min` (`Local Clock Example`) | False positive: `SI` declares `<min> minute` *and* re-exports `min` through `public import`. A namespace's own member now shadows a wildcard re-export (`symbols.Index.LookupQualified`). |
| `Requirement Groups`, both `Car Mass Rollup` files, four errors in `Variation Configuration` | Harness artifact: the gate diagnosed each file immediately after opening it, so a cross-file `private import 'Requirement Usages'::*` failed purely because that file sorts later. The gate now opens the whole corpus first. |

**Genuinely cleaner (verified, not silently unchecked)**

`Interaction Example-1`, `Interaction Realization-1`, `Interaction Realization-2`
and six of the eight errors in `Message Payload Example` were `unresolved
reference: setSpeedSent` and friends. `event occurrence setSpeedSent;` now keeps
its name (PR #17) and indexes as an occurrence usage, so those references resolve
to the declarations they name.

**False positives that stay recorded (resolver gaps, now reachable)**

Resolving names inside behavior bodies (PR #8) exposed these; each is a wrong
diagnostic on a well-formed model, so the count is pinned and the gap named:

| File | Diagnostic | Gap |
|---|---|---|
| `Time Constraints` | `unresolved member: done` | Inherited occurrence features are not members of a state usage: an untyped `state normal;` has no implicit typing to `States::StateAction`. |
| `Message Payload Example` | `unresolved reference: fuelCommand` (2×) | The payload feature a message declares in its `of` clause is not registered. (Fixed — see the message-payload re-pin below.) |
| `Action Performance Example`, `Allocation Usage Example`, `Conditional Succession Example-1` | `unresolved member: focus`/`generateTorque`/`isWellFocused` | Same missing implicit typing: features of the stdlib base type of an untyped usage are not members of it. |

### Verdicts for the inherited- and body-local-feature re-pin (80/100)

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `Calculation Usages-1` | 7 unresolved `a`/`v`/`x` | Real fix: `return a;` declares the calc's return parameter, which the parser had read as a reference to one. The names now resolve to those parameters. |
| `Trade Study Analysis Example` | 10 errors, now 1 | Real fix, same cause plus inherited members of a calc usage (`power`, `mass`, `efficiency`, `cost`). The remaining error is `alternative`, a genuine typo in the OMG file (`alternatives`). |
| `Variation Usages`, `Variation Configuration` | `engine::'4cylEngine'` etc. | Real fix: a qualified-name segment now reaches inherited members, and `part redefines engine` no longer resolves its own redefinition target to itself. |
| `Parts Example-1`, `Parts Example-2` | `unresolved reference: cyl` (2× each) | Real fix, same redefinition-shadowing cause: `part redefines eng { part redefines cyl[4]; }` resolves `cyl` through `Engine`. |
| `Use Case Usage Example` | 6 unresolved actors | Real fix: `'provide transportation'::driver` reaches the actors the use case usage inherits from its definition. |
| `Control Structures Example` | `unresolved reference: charging` | Real fix: a loop owns the scope its body declares into, so its `until` condition sees `charging`. |
| `Assignment Example` | `unresolved reference: dynamics` (2×) | Real fix, same scope: an action declared in a `for` body is visible to the `assign` steps in that body. |
| `Analysis Case Definition Example` | `unresolved reference: i` (8×) | Real fix: a body expression's parameters (`->forAll {in i: Positive; ...}`) are in scope in its result. |

Each verdict is locked by a focused test in
`internal/core/model/inherited_scope_resolve_test.go`, including the negative
cases (a redefinition of an undeclared name, and body-local names referenced
from outside their body, both still report).

### Verdicts for the implicit-usage-typing re-pin (81/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `31. Constraints/Time Constraints` | 1 × `unresolved member: done` | Real fix: `state normal;` is now implicitly typed by `States::StateAction`, which declares `done`, so `TimeOf(normal.done)` resolves to that declaration. The negative counterpart (`normal.notAMember`) still reports — see `internal/core/model/implicit_typing_test.go`. |

**Still recorded, and why implicit typing alone does not fix them**

| File | Diagnostic | Remaining gap |
|---|---|---|
| `Conditional Succession Example-1` | `unresolved member: isWellFocused` | Implicit *redefinition*: `out item image;` inside `action focus : Focus` refines `Focus::image` (typed `Image`). Untyped usages that shadow an inherited feature deliberately got no implicit base, so the type came from nowhere. (Fixed in the 88/100 re-pin below.) |
| `Action Performance Example`, `Allocation Usage Example` | `unresolved member: focus`/`shoot`/`generateTorque` | The members come from `perform action takePhoto references takePicture;` and `perform providePower.generateTorque;`: a `references` edge and the feature a `perform` statement contributes, neither of which is a generalization. **Fixed since — see the reference-subsetting verdicts below.** |
| `Time Slice and Snapshot Example`, `Individuals and Time Slices` | `unresolved reference: start`/`done` | Not OMG bugs after all — see the implicit definition-base re-pin below. |

### Verdicts for the reference-subsetting re-pin (81 → 83; 87 once `main`'s message-payload and satisfy-reference fixes merged in)

Two entries went clean and one reports more; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `18. Action Performance/Action Performance Example` | 2 × `unresolved member: focus`/`shoot` | Real fix: `perform action takePhoto references takePicture;` relates `takePhoto` to `takePicture` by a reference subsetting (SysML 7.17.6), which contributes the referenced action's members. `takePhoto.focus` now resolves to `takePicture::focus`. The negative counterpart (a member the referenced action does not declare) still reports — see `internal/core/semantics/reference_test.go` and `internal/core/model/perform_reference_test.go`. |
| `38. Allocation/Allocation Usage Example` | 2 × `unresolved member: generateTorque` | Real fix, two causes: `perform providePower.generateTorque;` names its feature after the feature it references (KerML `Feature::effectiveName`), so `torqueGenerator.generateTorque` names a declaration; and `allocate torqueGenerator to powerTrain` is an anonymous binary allocation, whose first name is a connector end rather than the usage's own name. |
| `32. Requirements/Requirement Satisfaction` | 2, then unchanged | Same fix, in a file that was already recorded: `perform 'provide power'.'generate torque'` resolves now. Its two remaining errors were unrelated and are cleared separately by the satisfy-reference verdicts below. |

**A file that reports more, adjudicated**

| File | Was | Now | Verdict |
|---|---|---|---|
| `34. Verification/Verification Case Usage Example` | 3 × `unresolved reference: testVehicle`/`massMeasured` | 6 × `individual cannot specialize partDef` / `... cannot be typed by individualDef` | The three name-resolution false positives are fixed by this change: `perform vehicleMassTest;` used to shadow the verification usage it performs with an empty feature, so `vehicleMassTest.collectData` and the redefinitions under it resolved to nothing. With the name-resolution tier clean, the type tier runs on this file for the first time (tiers are skipped after a lower tier errors) and reports six pre-existing false positives about individuals: `individual def TestSystem :> MassVerificationSystem;` and `individual testSystem : TestSystem` are well-formed (SysML 7.9.5), and the kind tables in `passes/typecheck.go` do not yet accept an individual definition specializing an occurrence definition. Recorded, not fixed, to keep this change scoped; see `docs/project/spec-compliance.md`. |

### Verdicts for the message-payload re-pin (82/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `27. Occurrences/Message Payload Example` | 2 × `unresolved reference: fuelCommand` | Real fix: `message m of fuelCommand : FuelCommand` *declares* the payload feature. The parser built that declaration but then overwrote `Usage.Members` with the body members, so it was never registered, and the `of` name was resolved as a reference in the enclosing scope. The declaration is now a member of the message (`FlowEnds.PayloadDecl`) and the `of` name resolves to it, which also makes `fuelCommandMessage.fuelCommand` reach the payload. |

The payload *reference* form (`flow f of Fuel from a to b`) is unchanged and
still resolves outward, with the negative case (`of` naming nothing) still
reporting — see `internal/core/model/flow_payload_resolve_test.go`.

### Verdicts for the satisfy-reference re-pin (85/100)

Three entries drifted, all of them the same false positive; every other file kept
its exact count.

**Spec basis.** `SatisfyRequirementUsage` (SysML v2 §7.21.4, abstract syntax
§8.3.21.10; concrete syntax in the pilot `SysML.xtext`) is:

```
SatisfyRequirementUsage :
    OccurrenceUsagePrefix 'assert'? ( isNegated ?= 'not' )? 'satisfy'
    ( ownedRelationship += OwnedReferenceSubsetting FeatureSpecializationPart?
    | RequirementUsageKeyword UsageDeclaration?
    )
    ValuePart? ( 'by' ownedRelationship += SatisfactionSubjectMember )? RequirementBody
;
```

Without the `requirement` keyword the name after `satisfy` is an
**OwnedReferenceSubsetting** — a `ReferenceSubsetting` (a `Subsetting`) whose
`referencedFeature` must be a `Feature`, i.e. a **usage**, never a definition.
`satisfy <requirementDef>` is in fact the ill-formed direction.

The abstract syntax makes this normative — `SatisfyRequirementUsage` carries the
constraint

```
ownedReferenceSubsetting <> null implies
    referencedFeatureTarget().oclIsKindOf(RequirementUsage)
```

so the referenced element must be a `RequirementUsage`. `ViewpointUsage` and
`ConcernUsage` both specialize `RequirementUsage` (`SysML.ecore`:
`ViewpointUsage eSuperTypes="#//RequirementUsage"`), so
`satisfy <viewpointUsage>` inside a `view def` is equally legal.

**Verdict: type-checker false positive.** The parser encoded the reference as a
`FeatureTyping` (`RelTyping`), so the type tier demanded a definition. It now
encodes it as `RelSubsets`, and the type tier requires the target to be a
requirement usage (including viewpoint and concern usages).

| File | Was | Verdict |
|---|---|---|
| `32. Requirements/Requirement Satisfaction` | 2 × `type must be a definition, found requirementUsage` | False positive: `satisfy vehicleSpecification by vehicle_design;` references the requirement usages declared in `Requirement Groups`. Legal per the grammar above. |
| `33. Analysis/Analysis Case Usage Example` | 1 × `type must be a definition, found requirementUsage` | False positive: `satisfy vehicleFuelEconomyRequirements by vehicle_c1;` references the `requirement` usage declared in the same part. |
| `42. Views/Views Example` | 1 × `type must be a definition, found viewpointUsage` | False positive: `satisfy 'system structure perspective';` references the `viewpoint` usage in `Viewpoint Example`; a viewpoint usage is a requirement usage. |

The checking is narrowed, not dropped: `satisfy <non-requirement usage>` still
reports (`satisfy target must be a requirement usage, found ...`), locked by
`TestTypeCheckSatisfyNonRequirementUsageError` alongside the two positive cases in
`internal/core/passes/typecheck_test.go`, and the parse shape is pinned by
`internal/core/parser/testdata/parse/satisfy_reference.golden`.

### Verdicts for the implicit-parameter-redefinition re-pin (88/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `16. Conditional Succession/Conditional Succession Example-1` | 1 × `unresolved member: isWellFocused` | Real fix: `out item image;` inside `action focus : Focus` is the second parameter of a step, so it implicitly redefines `Focus::image` (KerML 7.4.7.3, SysML v2 7.17.2 — the match is by *position*, not by name) and takes its type `Image`. `focus.image.isWellFocused` now resolves to `Image::isWellFocused`, the declaration the OMG model means. The negative counterpart (`focus.image.notAMember`) still reports — see `internal/core/model/implicit_typing_test.go` `TestImplicitRedefinitionSuppliesInheritedMembers`. |

**Deliberate test change**

`internal/core/model/implicit_typing_test.go` `TestParameterRedefinitionAccompaniesTheImplicitBase`
pinned the previous behavior of a *name*-based rule: any usage whose name matched
a feature its owner inherits was left with no implicit base at all, on the
assumption that an implicit redefinition would later supply the type. The
specification has no such name-based rule — implicit redefinition applies to the
parameters of behaviors and steps by position (KerML 7.4.7.2/7.4.7.3), to
connection and association ends by position (SysML v2 7.13.2), and to result
parameters as results (SysML v2 7.19.2), while a nested usage that merely shares
a name with an inherited feature is a *name conflict* to be resolved by an
explicit redefinition (SysML v2 7.6.1, KerML 7.3.2.1). The test therefore now
pins the parameter case (the parameter takes the redefined parameter's type),
and the new `TestLikeNamedUsageIsNotAnImplicitRedefinition` pins the other side:
a like-named undirected usage keeps the standard library base of its kind
instead of being silently treated as a redefinition. We still do not diagnose
the name conflict itself; that gap is recorded in `docs/project/spec-compliance.md`.

### Verdicts for the individual-definition re-pin (89/100, 90/100 with the import-in-definition-body fix below)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `34. Verification/Verification Case Usage Example` | 3 × `individual cannot specialize partDef`, 1 × `attribute cannot be typed by individualDef`, 2 × `part cannot be typed by individualDef` | Real fix: all six were false positives from the kind tables in `passes/typecheck.go`. `individual def X` is an occurrence definition — it is equivalent to `individual occurrence def X` and individuates the definition it specializes (SysML v2 7.9.4, abstract syntax 8.3.9.3: `individual` is `OccurrenceDefinition::isIndividual`, not a metaclass of its own) — so `individual def TestSystem :> MassVerificationSystem;` and the two `:> Vehicle` declarations are well formed, and a usage may be typed by an individual definition wherever it may be typed by an occurrence definition (`individual testSystem : TestSystem`, `in individual :>> testVehicle : TestVehicle1`). |

The checking is narrowed, not dropped. An occurrence definition still cannot
specialize a data type — `Occurrences::Occurrence` is disjoint with
`Base::DataValues` (SysML v2 8.4.5.1) — so `individual def Bad :> SomeAttributeDef;`
still reports `individual cannot specialize attributeDef (kind mismatch)`, and a
usage kind that rejects an occurrence definition still rejects an individual
definition (`port p : SomeIndividualDef`). Both negatives, and the positive
cases including the corpus file's shape, are locked by
`internal/core/passes/typecheck_individuals_test.go`.

Two of the six messages named a usage kind the declaration does not have:
`individual testSystem : TestSystem` was checked as an *attribute* usage and
`in individual :>> testVehicle` as a *part* usage, because the parser drops the
`individual` modifier (`parser/behavior.go`, `parser/defusage.go` record it but
the AST has no field for it) and falls back to the default kind instead of the
occurrence kind that 7.9.4 prescribes for a declaration with no kind keyword.
That is a separate gap: it does not change the verdict here, since typing by an
individual definition is legal for those kinds too, but it is why the pinned
messages read the way they did.

### Verdicts for the connector-end-redefinition re-pin (93/100)

Three entries drifted, all in the same direction; every other file kept its
exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `09. Connections/Connections Example` | 2 × `unresolved reference: bead`, `mountingRim` | False positives: `connection : PressureSeat connect bead references t.bead to mountingRim references w.rim;` *declares* those two names as the connection's own ends, which redefine `PressureSeat::bead` and `PressureSeat::mountingRim` by position (SysML v2 7.13.2). They were being resolved as references to features of the enclosing part, where nothing declares them. |
| `11. Interfaces/Interface Example` | 2 × `unresolved reference: supplierPort`, `consumerPort` | False positives: same rule for an interface usage, whose ends are bound with `::>` rather than `references` (SysML v2 7.14.2); the names declare the ends inherited from `FuelInterface`. |
| `13. Flows/Flow Interface Example` | 2 × `unresolved reference: supplierPort`, `consumerPort` | False positives: identical to the above; the flows the interface definition declares between the two ends do not change how the usage's `connect` clause names them. |

Making the name-resolution tier clean in the latter two files exposed the type
tier behind it, which reported `attribute cannot be typed by portDef` for the
definitions' own ends (`end supplierPort : FuelOutPort;`). That too was a false
positive: an `end` feature is a plain KerML feature typed by the feature it
connects, not an `AttributeUsage`, so the usage-kind taxonomy does not constrain
its type (SysML v2 7.14.2). The kind check now skips end features.

### Verdicts for the subsetting-conformance and flow-ends re-pin (95 → 97/100)

Two entries drifted, both from over-strict constraint checks; every other file
kept its exact count. Both files were adjudicated against the OMG source model
and the specifications, and in both the OMG example is well formed.

#### `41. Language Extension/Model Library Example` — subsetting type conformance

```sysml
abstract occurrence def Situation;
abstract occurrence situations : Situation[*] nonunique;
abstract occurrence def Cause { attribute probability : Real; }
abstract occurrence causes : Cause[*] nonunique :> situations;
```

We reported `causes (typed by Cause) subsets situations (typed by Situation):
types do not conform` (and the same for `failures`/`Failure`), because `Cause`
does not specialize `Situation`.

**Spec basis.** KerML 1.0 (formal/2026-03-01), `Subsetting`, §8.3.3.3.10, states the
rule this check was meant to enforce:

> "To support this the domain of the `subsettingFeature` must be the same or
> specialize (at least indirectly) the domain of the `subsettedFeature` (via
> `Specialization`), and the co-domain (intersection of the types) of the
> `subsettingFeature` must specialize the co-domain of the `subsettedFeature`."

The co-domain is the **intersection of the types**, and KerML §8.3.3.3.4
(`Feature`, attribute `/type`) says where those types come from:

> "Types that restrict the values of this Feature, such that the values must be
> instances of all the types. The types of a Feature are derived from its
> `typings` and the types of its `subsettings`."

So the types of `causes` are `Cause` *and* `Situation`, and its co-domain
`Cause ∩ Situation` specializes `Situation` by construction — the rule is
satisfied whatever the declared typing is. KerML §7.3.4.4 says the same thing in
prose:

> "A subsetting feature can restrict aspects of the subsetted feature, otherwise
> it will, by default, have the same properties as the subsetted feature. In
> particular, a subsetting feature can constrain its featured types to be
> specializations of those of the subsetted feature and add additional feature
> types."

The three normative constraints KerML lists on `Subsetting`
(`validateSubsettingConstantConformance`, `validateSubsettingFeaturingTypes`,
`validateSubsettingUniquenessConformance`) are about constancy, accessibility and
uniqueness; none of them requires the declared type of the subsetting feature to
conform to the type of the subsetted feature.

**Verdict: our checker was over-strict.** A subsetting feature *adds* types; it
does not have to specialize them. `checkTypingConformance` in
`internal/core/passes/constraint.go` could therefore never report a true
positive, and it is removed. The corresponding conformance rule for
*redefinition* (`checkRedefinition`) is untouched. `subsetting-multiplicity`,
which implements a rule KerML does state (§7.3.4.4: a subsetting feature "can
also restrict the multiplicity of its subsetted feature"), is untouched as well.

#### `27. Occurrences/Interaction Example-2` — a message with no ends

```sysml
message setSpeedMessage of SetSpeed;
then message sensedSpeedMessage of SensedSpeed;
message fuelCommandMessage of FuelCommand;
```

We reported `flow setSpeedMessage must declare both a source and a target end`
(3×).

**Spec basis.** SysML v2 1.0 (formal/2026-03-02) §8.2.2.16 makes the ends
optional in both flow and message declarations:

```
MessageDeclaration : FlowUsage =
    UsageDeclaration ValuePart?
    ( 'of' ownedRelationship += FlowPayloadFeatureMember )?
    ( 'from' ownedRelationship += MessageEventMember
      'to' ownedRelationship += MessageEventMember )?
  | ownedRelationship += MessageEventMember 'to'
    ownedRelationship += MessageEventMember
```

and §8.4.12.2 (Flow Usages) makes end-less-ness a *requirement* for a message:

> "For a FlowUsage to be considered a message, it must not have any owned
> flowEnds. Therefore, such a FlowUsage should only be defined by
> FlowDefinitions that are abstract and have no flowEnds."

The same clause notes that even `message m : M of i : I from evt1 to evt2;` "is
parsed as an abstract FlowUsage, but without any flowEnds" — `evt1`/`evt2` become
`in` parameters redefining `Message::sourceEvent`/`targetEvent`. The abstract
syntax agrees: the only end-related constraint on `FlowUsage` (§8.3.16.3,
`checkFlowUsageFlowSpecialization`) is conditioned on
`ownedEndFeatures->notEmpty()`, and §8.4.12.1 states that "An abstract
FlowDefinition may have less than two flowEnds." This is exactly the shape of the
OMG example, whose events are supplied by the participants
(`event setSpeedMessage.sourceEvent;` inside `ref part driver`).

**Verdict: our checker was over-strict.** The check fired on any flow with a
non-nil `FlowEnds`, and a payload-only `of <payload>` clause allocates one, so a
message that declares only a payload looked like a flow with a missing end.
`checkConnectorEnds` no longer looks at flow ends at all. The narrower rule it
would otherwise keep — a flow that names one end but not the other — is
unreachable at the constraint tier: `parseFlowTo` records `expected 'to' between
flow ends` whenever the `to` clause is missing, and `Registry.Run` skips every
pass above the level that errored, so a half-declared flow never reaches
`ConstraintPass`. That case is pinned where it actually surfaces, as the
`flow_source_without_target` entry of the parser's `TestNegative` suite.

#### Implied corpus ceiling

Both verdicts are "the checker is wrong", so the set of pinned OMG bugs is
unchanged at three files (`start`/`done` in
`Time Slice and Snapshot Example` and `Individuals and Time Slices`, and
`alternative` vs `alternatives` in `Trade Study Analysis Example`). The corpus
ceiling is therefore **97/100**, and the two files between 95 and 97 are our
own remaining false positives.

> Amended at 97/100: `alternative` is no longer among the pinned OMG bugs. The
> `:>> alternative` in `Trade Study Analysis Example` now matches `alternatives`
> through implicit redefinition, so the file's one remaining error is a false
> positive of ours (`objective cannot be typed by requirementDef`). The
> `start`/`done` files were pinned as OMG bugs here, putting the ceiling at
> **98/100**; that verdict was later overturned and the corpus is 100/100.

---

### Verdicts for the objective-typing re-pin (97/100 → 98/100)

One entry drifted; every other file kept its exact count. This reaches the
**98/100** ceiling implied above: the only errors left are the two pinned
`start`/`done` OMG bugs.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `33. Analysis/Trade Study Analysis Example` | 1 × `objective cannot be typed by requirementDef (kind mismatch)` | Real fix: the checker was wrong. `objective : MaximizeObjective;` names `requirement def MaximizeObjective :> TradeStudyObjective` (`Domain Libraries/Analysis/TradeStudies.sysml:98`), and an `objective` *is* a requirement — an `ObjectiveMembership`'s `ownedObjectiveRequirement` is a `RequirementUsage` (SysML v2 §8.3.22.4, mirrored by `derived ref item objectiveRequirement : RequirementUsage[0..1]` in `Systems Library/SysML.sysml:77`). The kind check still runs on the usage; it now consults the requirement-definition kinds instead of the structural ones, and an objective typed by anything else (a `part def`, an `action def`) still reports. |

**What changed**

`compatibleTyping` in `internal/core/passes/typecheck.go` had one row covering
`subject` and `objective` together, accepting the structural definition kinds for
both. The two memberships are not the same shape, so the row was split:

- `objective` accepts a `RequirementDefinition` or one of its specializations
  (`ConcernDefinition`, `ViewpointDefinition`, via the new `isRequirementDefKind`)
  and nothing else. This is a *narrowing*: `objective o : Vehicle` (a `part def`)
  was accepted before and now reports.
- `subject` keeps the structural kinds. A `SubjectMembership`'s
  `ownedSubjectParameter` is an unconstrained `Usage` (SysML v2 §8.3.21), and the
  stdlib types subjects with structural definitions throughout
  (`subject subj : View[1]` in `Systems Library/Views.sysml:61`), so the row that
  was right for `subject` stays as it was.

Both directions are locked by unit tests in
`internal/core/passes/typecheck_kinds_test.go`
(`TestTypeCheckObjectiveTypedByRequirementDefOK`,
`TestTypeCheckObjectiveTypedByConcernDefOK`,
`TestTypeCheckObjectiveTypedByPartDefError`,
`TestTypeCheckObjectiveTypedByActionDefError`).

> **Superseded in part.** The `subject` half of this verdict was wrong, and the
> corpus could not show it because most subjects were never checked at all. See
> the subject re-pin below.

---

### Verdicts for the subject usage-kind re-pin (98/100, no count change)

No expectation entry moved, so `training_examples_expected.txt` is untouched.
This section records a rule change that *would* have moved one had it not been
corrected, which is the case the adjudication process exists for.

**What was wrong**

A `subject` reached the usage-kind rules only when it happened to parse as an
`ast.Usage`. The requirement-specific body path produces an `ast.SubjectMember`
instead, and the type checker walked only usages — so whether a subject was
checked depended on which keyword opened the body:

```sysml
requirement def R { subject s : A; }              // not checked
requirement def R { attribute x; subject s : A; } // checked
requirement R     { subject s : A; }              // never checked (a requirement
                                                  // usage always takes that path)
```

**What the corpus then showed**

Checking every subject turned `32. Requirements/Requirement Definitions` red with
two errors, and they were *ours*, not the model's:

| File | New errors | Verdict |
|---|---|---|
| `32. Requirements/Requirement Definitions` | `subject cannot be typed by portDef`, `subject cannot be typed by actionDef` | False positive. The file declares `port def ClutchPort;` and `action def GenerateTorque;` and subjects them directly (`subject clutchPort: ClutchPort;`, `subject generateTorque: GenerateTorque;`). A `SubjectMembership`'s `ownedSubjectParameter` is an unconstrained `Usage` (SysML v2 §8.3.21) — there is no kind restriction to violate. |

**What changed**

`compatibleTyping` accepted only `partDef | attributeDef | itemDef |
occurrenceDef | individualDef` for a `subject`. That list contradicted the
comment directly above it and the OMG's own models, and it survived because the
subjects that would have failed it escaped the check. `subject` now accepts any
definition kind; a subject typed by a *usage* is still an error, since that is
the general typing rule rather than a kind rule.

This retires `TestTypeCheckSubjectTypedByRequirementDefError`, which asserted
that a `requirement def` may not type a subject. It had no basis in §8.3.21 and
contradicted the sentence above it; the replacement,
`TestTypeCheckSubjectTypedByAnyDefKindOK`, pins the kinds the corpus and the
stdlib actually use.

---

### Verdicts for the import-in-definition-body re-pin (89/100)

One entry drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `34. Verification/Verification Case Definition Example` | 3 × (`unresolved reference: VerdictKind` ×2, `unresolved reference: PassIf`) | Real fix: `verification def VehicleMassTest` opens its body with `private import VerificationCases::*;`, and the three names it declares to be missing (`VerdictKind` on the `evaluateData` output and on the `def`-level `return`, and `PassIf` in the `evaluateData` body) all resolve to `VerificationCases`. The prior verdict below — "imports must be at package level, not inside a verification def" — was wrong: in KerML an `Import` is a `Relationship` whose `importOwningNamespace` is *any* `Namespace` (KerML 7.2.5.4 Imports; abstract syntax 8.3.2.4.2 Import / 8.3.2.4.6 NamespaceImport), and a definition body is a `Namespace` because a `Definition`/`Usage` is a `Type` and every `Type` is a `Namespace` (SysML v2 7.5.1 Namespaces, 7.5.3 Imports, 7.6 Definition and Usage). A `NamespaceImport` therefore imports the visible (public) memberships of `VerificationCases` into the `VehicleMassTest` body, and — through the ordinary parent-scope walk — into the nested `evaluateData` action, exactly where the OMG model uses them. |

**What changed**

`internal/core/resolve/unqualified.go` `importsOf` only harvested imports from
`*ast.Package`, `*ast.Namespace`, and `*ast.RootNamespace`, so an import declared
inside a definition or usage body was never consulted during name resolution. It
now also harvests imports from `*ast.Definition` and `*ast.Usage` bodies. The
existing membership/inheritance-then-import ordering (`walkUnqualifiedHiding`) and
import visibility (`visibleThroughImport`, `import all`) are unchanged, and a
`private import` in a definition body still does not leak to importers of that
definition because an imported name is not an *owned* member and is not
re-surfaced by a `NamespaceImport` of the outer definition. Covered by
`internal/core/resolve/imports_test.go`
(`TestImportInDefinitionBodyVisibleInBody`,
`TestImportInDefinitionBodyVisibleInNestedBody`,
`TestImportInPackageBodyVisibleInNestedDefinition`,
`TestImportInDefinitionBodyDoesNotLeakToImporter`).

---

### Verdicts for the metadata and user-keyword re-pin (95/100)

Two entries drifted; every other file kept its exact count.

**Genuinely cleaner (verified, not silently unchecked)**

| File | Was | Verdict |
|---|---|---|
| `39. Metadata/Metadata Example-1` | 2 × `unresolved reference: annotatedElement` | Real fix: a metadata definition is a kind of `Metadata::MetadataItem`, which specializes `Metaobjects::Metaobject` (SysML v2 §7.27.2, §9.2.21; [KerML, 9.2.17]), so `:> annotatedElement : SysML::PartDefinition` redefines the inherited feature. Two parts were missing: the implicit base for the metadata kind, and the specialization edges of *cached* library symbols, which were persisted but never restored, hiding `MetadataItem :> Metaobject`. `annotatedElement` now resolves to `Metaobjects::Metaobject::annotatedElement`. |
| `41. Language Extension/User Keyword Example` | 2 × `unresolved reference: probability` / `severity` | Real fix: `#cause` and `#failure` name metadata definitions specializing `Metaobjects::SemanticMetadata`, so the usages they prefix implicitly subset the `baseType` those definitions bind — `causes : Cause` and `failures : Failure` from `Model Library Example` (SysML v2 §7.27.3, §7.27.4). `probability` and `severity` are members of `Cause`/`Failure` and now resolve there. The parser also dropped a keyword-only usage declaration (`#cause 'battery old' { ... }`, §7.27.4), re-reading it as an enumeration literal without its prefix; it is now parsed as the usage it declares. |

`41. Language Extension/Model Library Example` keeps its 2 errors: `causes`
subsets `situations` while being typed by `Cause` rather than `Situation`. That
is a separate conformance question about subsetting with a specialized type and
is not touched here.

---

## Re-pin for implicit definition bases (98 → 100)

The last two recorded files went clean, and the "OMG bug" verdict standing
against them since the first baseline was **wrong**.

| File | Was | Verdict |
|---|---|---|
| `27. Occurrences/Time Slice and Snapshot Example` | 2 × `unresolved reference: start`/`done` | Real fix: `Systems Library/Items.sysml:33` declares `item start :>> startShot` and `item done :>> endShot`, and `Parts.sysml:28` redefines both, so `snapshot sale = start` names a library feature. Only *behavior* definitions had an implicit base, so no `part def`/`item def` body inherited it; `implicitDefinitionBases` now covers every definition kind. |
| `28. Individuals/Individuals and Time Slices` | 2 × the same | Same fix. It then exposed a second false positive: `individual item def Alice :> Person` names a *part* definition, which the kind table rejected on an exact-kind match. A part definition **is** an item definition (SysML v2 §8.3.9.2), so specialization now follows the definition taxonomy — see `defKindsComparable` in `passes/typecheck.go`. |

The corpus therefore has no pinned OMG bugs left and no known false positives:
`training_examples_expected.txt` lists no files.

### Type System Limitations (0 errors)

None recorded: the `objective`/`requirement def` kind-table gap that stood here
was fixed (see the objective-typing re-pin above).

### Resolved Historically ✅

- `objective cannot be typed by requirementDef` (`33. Analysis/Trade Study Analysis Example`): the kind table now types an `objective` with a requirement definition, as `ObjectiveMembership` requires; the file is clean.

- `alternative` (`33. Analysis/Trade Study Analysis Example`): the file's `:>> alternative` is now matched against `alternatives` through implicit redefinition, so the former "OMG typo" verdict no longer applies to this file; the one error it still emits is the objective kind-table gap above.
- Connection- and interface-usage end names (`09. Connections`, `11. Interfaces`, `13. Flows`, 6 errors): fixed by implicit redefinition of connector ends by position and by resolving a connector end's reference target in the enclosing scope (see the connector-end-redefinition re-pin above).
- `annotatedElement` and user-keyword typing (`39. Metadata`, `41. Language Extension`, 4 errors): fixed by inheriting metadata features and applying semantic metadata keywords (see the metadata and user-keyword re-pin above).
- `X subsets Y: types do not conform` (`41. Language Extension/Model Library Example`, 2 errors): withdrawn — subsetting intersects types rather than requiring conformance (see the subsetting-conformance and flow-ends re-pin above).
- `flow X must declare both a source and a target end` (`27. Occurrences/Interaction Example-2`, 3 errors): withdrawn for flows that declare no ends at all, which is how a message is written (same re-pin).
- `individual cannot specialize partDef` / `... cannot be typed by individualDef` (`34. Verification/Verification Case Usage Example`, 6 errors): fixed by treating an individual definition as an occurrence definition (SysML v2 7.9.5).
- `VerdictKind`, `PassIf` (`34. Verification/Verification Case Definition Example`, 3 errors): fixed by consulting imports owned by a definition/usage body during name resolution. The former verdict — "imports must be at package level, not inside a verification def" — was wrong: the file's `private import VerificationCases::*;` sits inside the `verification def VehicleMassTest` body, which is a legitimate place for an import, because a definition body is a `Namespace` and an `Import`'s `importOwningNamespace` may be any `Namespace` (KerML 7.2.5.4; SysML v2 7.5.3, 7.6).
- `localClock`, `payload` (4 errors): fixed in 8304f03, c683bc8 by resolving features inherited from parent definitions (Part → Item → Occurrence, Flow → Message → Transfer).
- Named argument resolution: fixed in ff70654 (named args did not resolve parameter names).

---

## Training Example Compliance

| Category | Pass | Fail | Pass Rate |
|----------|------|------|-----------|
| **All Examples** | 100 | 0 | 100% |

---

## Remaining Work for Full Training Example Support

### Priority 1: Pedagogical Documentation
- Mark which examples are intentionally incomplete
- Provide "complete" versions of pedagogical examples for testing

---

## Testing

To run training example analysis:

```bash
go test -run TestTrainingExamplesSemanticErrors ./internal/core/model -v
```

This generates error frequency analysis and per-file diagnostics.

The gate indexes the standard library from an empty semantic cache — it points
`XDG_CACHE_HOME` at a temporary directory — so it reports the same numbers on a
fresh machine as on one whose `~/.cache/sysml-ls` is already populated.
`TestCorpusGatesCacheStateIndependent` runs each of the four OMG roots twice over
one cache directory and fails if any file's diagnostics differ between the two.

---

## Conclusion

**Implementation Status**: Core behavioral semantics complete (671/671 execution conformance cases passing).

**Training Example Status**: 100/100 clean. The last two files were false positives of ours, not
OMG bugs: `start` and `done` are declared by `Items::Item` and redefined by `Parts::Part`, and every
definition body now inherits the features its kind implies (see the re-pin below).

A clean training corpus means these 100 files produce no diagnostic — it is not evidence about
execution semantics, which this corpus does not run, nor about the notation these files happen not
to use. See [spec compliance](spec-compliance.md) for what is and is not claimed.
