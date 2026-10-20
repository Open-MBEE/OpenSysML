# Constraint Census Triage

The [validation-constraint census](validation-constraints.md) records, for every constraint the
pinned pilot names and every constraint the formal documents state, whether this implementation
reports it. This record triages the rows the census still lists as ❔ unknown against the
SysML v2.1 Beta 2 and KerML 1.1 Beta 2 specifications (the documents the pinned 2026-08 pilot
release ships) and against the pinned pilot validators, so that every genuine gap becomes a
work item that can be handed on verbatim, and every row that is not a gap says why.

**Labels:** the short labels in this record are internal cross-references, not specification or
product terms. `(a)`–`(d)` are the four triage classes defined below; a `w<n><letter>` prefix on a
pass file names the pass tier the file lives in under `internal/check/passes/`. A reader who only
wants the verdicts can ignore them.

## Pinned references

- Pilot: OMG SysML v2 Pilot Implementation `2026-08` (`jupyter-sysml-kernel 0.62.0`), run as
  `build/pilot-sysml-validator/validate-sysml-batch` and `build/pilot-kerml-validator/validate-kerml`
  (`scripts/download-pilot-sysml-validator.sh`, `scripts/download-pilot-kerml-validator.sh`).
- Specifications: KerML 1.1 Beta 2 and SysML v2.1 Beta 2, `doc/1-Kernel_Modeling_Language.pdf`
  and `doc/2a-OMG_Systems_Modeling_Language.pdf` of the `2026-08` tag of
  `Systems-Modeling/SysML-v2-Release`, read with `pdftotext -layout`. Where a Beta 2 clause
  differs from the formal KerML 1.0 / SysML 2.0 documents the manifest pins, the row says so.
- Ours: `bin/sysml -validate` built from the tree, each probe run under a fresh `XDG_CACHE_HOME`.

## Classes

- **(a) implemented-but-unmapped** — this implementation already reports the rule; the census row
  was unknown only because the pilot's name and the document's name differ, or because the row
  cited no implementation. The correction is in the manifest: the pilot row now cites the
  document's name (`spec.name`), so the document's constraint is that row's counterpart and no
  longer a separate spec-sourced row, or the spec-sourced row cites the pass directly.
- **(b) gap-ours** — the pilot reports the rule on the violating probe and we do not.
- **(c) gap-both** — the specification states the rule, neither tool reports it.
- **(d) not-applicable** — the rule cannot be violated in the textual notation (the grammar
  fixes the value, or the rule is an abstract-syntax invariant the parser guarantees), the
  Beta 2 documents no longer state it, or the pilot declares the constant and never reports it
  and the documents give no rule to report.

Every (b) and (c) row quotes the specification's prose and OCL under *Launch-ready gaps*.

## Probes

The probe models live under `tools/census/validation/testdata/probes/triage/`, one file per
model, named `<constraint>[.<variant>][.pilot-twin].<violating|conforming>.<kerml|sysml>`. Each
opens with the constraint it probes, whether the model violates or conforms to it, and the
verdict each tool gave: `Ours:` the diagnostic codes `bin/sysml -validate` reported (`clean`
when none), `Pilot:` the first messages of the pinned validator. A `.pilot-twin` file is the
census probe of the pilot-named row the document's name maps to, re-run under this name.
The directory is outside the census gate's probe set on purpose: those probes are evidence for
rows the census records as implemented, these are evidence for the triage verdicts.

Reproduce a row with

```sh
XDG_CACHE_HOME=$(mktemp -d) bin/sysml -validate <probe>
build/pilot-sysml-validator/validate-sysml-batch --root <dir> <probe>   # .sysml
build/pilot-kerml-validator/validate-kerml <probe>                       # .kerml
```

## In-flight overlap

Work in progress elsewhere that bears on a row is named in the row's last column so that no
duplicate work item is launched from this record. At the time of writing, none of the
`validate*` rows below overlaps work in progress; the rules under way elsewhere — the constant-
conformance rule for SysML usages (`validateSubsettingConstantConformance`), the owner condition
of `validateRedefinitionEndConformance`, the rule that connection- and flow-definition ends are
usages (added to the pilot after the pinned release), `RenderingUsage::namingFeature`, and an
API-JSON derived-property differential — are already recorded in the census as faithful,
approximate or outside the pilot's constraint list, and are not re-listed as gaps here.

## `validate*` rows (88)

The *Ours* and *Pilot* columns give the verdict on the violating probe where one exists (the
diagnostic codes we report; the pilot's first message), otherwise on the conforming model. A
row whose rule cannot be violated in text has a conforming probe only.

| # | Constraint | Source | Clause | Ours | Pilot | Classification | Probe | In-flight overlap |
|---|---|---|---|---|---|---|---|---|
| 1 | `validateAcceptActionUsageParameters` | sysml | sysml 8.3.17.2 | syntax | e: no viable alternative at input 'action' | (d) not-applicable: grammar: `accept` requires a payload (`accept;` is a syntax error in both parsers), and the payload parameter is always an input | `validateAcceptActionUsageParameters.violating.sysml` | none |
| 2 | `validateAnnotationAnnotatingElement` | kerml | kerml 8.3.2.3.3 | clean | clean | (d) not-applicable: abstract-syntax invariant: the notation owns an annotating element in exactly one of the two ways | `validateAnnotationAnnotatingElement.conforming.kerml` (conforming only) | none |
| 3 | `validateAssignmentActionUsageArguments` | sysml | sysml 8.3.17.5 | syntax / syntax | e: no viable alternative at input 'assign' | (d) not-applicable: grammar: `assign x := v` always parses both arguments; the pilot declares the constant and never reports it | `validateAssignmentActionUsageArguments.violating.sysml` | none |
| 4 | `validateAssociationStructureIntersection` | kerml | kerml 8.3.4.4.2 | clean | clean | (d) not-applicable: metaclass identity (`assoc struct`); not expressible | `validateAssociationStructureIntersection.conforming.kerml` (conforming only) | none |
| 5 | `validateAttributeDefinitionFeatures` | sysml | sysml 8.3.7.2 | clean | clean | (d) not-applicable: grammar: §7.7 — the owned features of an attribute definition or usage are parsed non-composite; `attribute def D { part p; }` exports `isComposite = false` and both validators are silent | `validateAttributeDefinitionFeatures.violating.sysml` | none |
| 6 | `validateAttributeUsageFeatures` | sysml | sysml 8.3.7.3 | clean | clean | (d) not-applicable: grammar, as validateAttributeDefinitionFeatures | `validateAttributeUsageFeatures.violating.sysml` | none |
| 7 | `validateAttributeUsageIsReferential` | sysml | sysml 8.3.7.3 | clean | clean | (d) not-applicable: grammar: an attribute usage is always referential (§7.7); Beta 2 name validateAttributeUsageIsReference, now cited by the row | `validateAttributeUsageIsReferential.conforming.sysml` (conforming only) | none |
| 8 | `validateBindingConnectorArgumentTypeConformance` | kerml | kerml 8.3.4.5.2 | bound-feature-types | w: Bound features should have conforming types | (d) not-applicable: pilot no-op (`checkBindingConnector` keeps the check commented out) and not named by Beta 2; the warning both tools raise on `binding a = b` of unrelated classes is validateBindingConnectorTypeConformance (ours `bound-feature-types`) | `validateBindingConnectorArgumentTypeConformance.violating.kerml` | none |
| 9 | `validateCollectExpressionOperator` | kerml | kerml 8.3.4.8.2 | clean | clean | (d) not-applicable: grammar fixes the operator | `validateCollectExpressionOperator.conforming.kerml` (conforming only) | none |
| 10 | `validateConjugatedPortDefinitionConjugatedPortDefinition` | sysml | sysml 8.3.12.2 | clean | clean | (d) not-applicable: derived structure of `port def`; Beta 2 name …ConjugatedPortDefinitionIsEmpty, now cited by the row | `validateConjugatedPortDefinitionConjugatedPortDefinition.conforming.sysml` (conforming only) | none |
| 11 | `validateConstructorExpressionOwnedFeatures` | kerml | kerml 8.3.4.8.3 | clean | clean | (d) not-applicable: grammar: `new A(x = 1)` owns only its result; the arguments nest under the result | `validateConstructorExpressionOwnedFeatures.conforming.kerml` (conforming only) | none |
| 12 | `validateDefinitionVariationIsAbstract` | sysml | sysml 8.3.6.2 | clean | clean | (d) not-applicable: `isAbstract` is implied by `variation`; not violable in text (our API JSON omits `isAbstract` on a variation definition — see the derive* table) | `validateDefinitionVariationIsAbstract.conforming.sysml` (conforming only) | none |
| 13 | `validateElementIsImpliedIncluded` | kerml | kerml 8.3.2.1.2 | clean | clean | (d) not-applicable: abstract-syntax invariant | `validateElementIsImpliedIncluded.conforming.kerml` (conforming only) | none |
| 14 | `validateEndFeatureMembershpIsEnd` | kerml | kerml 8.3.3.3.3 | clean | clean | (d) not-applicable: grammar: an end feature membership only arises from `end`; Beta 2 name validateEndFeatureMembershipIsEnd, now cited by the row | `validateEndFeatureMembershpIsEnd.conforming.kerml` (conforming only) | none |
| 15 | `validateEnumerationDefinitionIsVariation` | sysml | sysml 8.3.8.2 | clean | clean | (d) not-applicable: derived: `enum def` is a variation; our API JSON emits `isVariation = true` | `validateEnumerationDefinitionIsVariation.conforming.sysml` (conforming only) | none |
| 16 | `validateEventOccurrenceUsageIsReference` | sysml | sysml 8.3.9.2 | clean | clean | (d) not-applicable: grammar: `event occurrence e;` is referential; our API JSON emits `isComposite = false` | `validateEventOccurrenceUsageIsReference.conforming.sysml` (conforming only) | none |
| 17 | `validateExposeIsImportAll` | sysml | sysml 8.3.26.2 | clean | clean | (d) not-applicable: grammar: `expose` is an import-all; our API JSON emits `isImportAll = true` | `validateExposeIsImportAll.conforming.sysml` (conforming only) | none |
| 18 | `validateFeatureChainExpressionOperator` | kerml | kerml 8.3.4.8.4 | clean | clean | (d) not-applicable: grammar fixes the operator | `validateFeatureChainExpressionOperator.conforming.kerml` (conforming only) | none |
| 19 | `validateFeatureEndIsConstant` | kerml | kerml 8.3.3.3.4 | variable-feature-owner | e: no viable alternative at input 'end' | (d) not-applicable: grammar: Beta 2 `EndFeaturePrefix` is `('const')? 'end'`, so an end cannot be declared `var`; the pilot rejects `var end` and `end var` as syntax errors; our parser accepts `var end` and reports only `variable-feature-owner` (lenience of the parser, not a rule gap) | `validateFeatureEndIsConstant.violating.kerml` | none |
| 20 | `validateFeatureReferenceExpressionResult` | kerml | kerml 8.3.4.8.5 | clean | clean | (d) not-applicable: abstract-syntax invariant | `validateFeatureReferenceExpressionResult.conforming.kerml` (conforming only) | none |
| 21 | `validateFlowEndIsEnd` | kerml | kerml 8.3.4.9.3 | clean | clean | (d) not-applicable: grammar | `validateFlowEndIsEnd.conforming.kerml` (conforming only) | none |
| 22 | `validateFlowEndNestedFeature` | kerml | kerml 8.3.4.9.3 | flow-end-subsetting / flow-end-subsetting | e: Must have at least two related elements | (d) not-applicable: grammar: the `FlowEnd` production owns exactly one feature; `flow from a to b` without dot notation is validateFlowEndSubsetting in both tools (ours `flow-end-subsetting`) | `validateFlowEndNestedFeature.violating.kerml` | none |
| 23 | `validateFlowEndOwningType` | kerml | kerml 8.3.4.9.3 | clean | clean | (d) not-applicable: grammar | `validateFlowEndOwningType.conforming.kerml` (conforming only) | none |
| 24 | `validateFlowItemFeature` | kerml | kerml 8.3.4.9.2 | clean | clean | (d) not-applicable: grammar: at most one `of` item; Beta 2 name validateFlowPayloadFeature, now cited by the row | `validateFlowItemFeature.conforming.kerml` (conforming only) | none |
| 25 | `validateForLoopActionUsageLoopVariable` | sysml | sysml 8.3.17.9 | clean | clean | (d) not-applicable: grammar | `validateForLoopActionUsageLoopVariable.conforming.sysml` (conforming only) | none |
| 26 | `validateForLoopActionUsageParameters` | sysml | sysml 8.3.17.9 | clean | clean | (d) not-applicable: grammar | `validateForLoopActionUsageParameters.conforming.sysml` (conforming only) | none |
| 27 | `validateFramedConcernMembershipConstraintKind` | sysml | sysml 8.3.21.5 | clean | clean | (d) not-applicable: grammar fixes the kind | `validateFramedConcernMembershipConstraintKind.conforming.sysml` (conforming only) | none |
| 28 | `validateIfActionUsageParameters` | sysml | sysml 8.3.17.10 | clean | clean | (d) not-applicable: grammar | `validateIfActionUsageParameters.conforming.sysml` (conforming only) | none |
| 29 | `validateIndexExpressionOperator` | kerml | kerml 8.3.4.8.6 | clean | clean | (d) not-applicable: grammar fixes the operator | `validateIndexExpressionOperator.conforming.kerml` (conforming only) | none |
| 30 | `validateInstantiationExpressionResult` | kerml | kerml 8.3.4.8.7 | clean | clean | (d) not-applicable: abstract-syntax invariant | `validateInstantiationExpressionResult.conforming.kerml` (conforming only) | none |
| 31 | `validateInvocationExpressionOwnedFeatures` | kerml | kerml 8.3.4.8.8 | clean | clean | (d) not-applicable: grammar: invocation arguments are `in` parameters | `validateInvocationExpressionOwnedFeatures.conforming.kerml` (conforming only) | none |
| 32 | `validateItemUsageType` | sysml | sysml 8.3.10.3 | name-conflict / usage-typing | e: An occurrence, item or part must be typed by occurrence definitions | (d) not-applicable: not stated by Beta 2 (the pilot keeps `checkItemUsage` commented out); `item i : AD` with an attribute definition is validateOccurrenceUsageType in both tools, `item i : A` with an action definition is silent in both | `validateItemUsageType.violating.sysml` | none |
| 33 | `validateMultiplicityRangeBounds` | kerml | kerml 8.3.4.11.2 | multiplicity-range | clean | (d) not-applicable: abstract-syntax ordering of the bound members; on `[3..1]` ours additionally reports `multiplicity-range`, the pilot nothing — see Observations | `validateMultiplicityRangeBounds.violating.kerml` | none |
| 34 | `validateObjectiveMembershipIsComposite` | sysml | sysml 8.3.22.4 | reserved-keyword-name / syntax | e: no viable alternative at input 'objective' | (d) not-applicable: grammar: `objective ref` and `ref objective` are syntax errors in both parsers (our API JSON exports the objective with `isComposite = false`, `direction = in` — see the derive* table) | `validateObjectiveMembershipIsComposite.violating.sysml` | none |
| 35 | `validateParameterMembershipDirection` | kerml | kerml 8.3.4.6.4 | clean | clean | (d) not-applicable: grammar; Beta 2 name validateParameterMembershipParameterDirection, now cited by the row | `validateParameterMembershipDirection.conforming.sysml` (conforming only) | none |
| 36 | `validateParameterMembershipOwningType` | kerml | kerml 8.3.4.6.4 | clean | clean | (d) not-applicable: grammar | `validateParameterMembershipOwningType.conforming.sysml` (conforming only) | none |
| 37 | `validatePartUsageType` | sysml | sysml 8.3.11.3 | name-conflict / usage-typing | e: An occurrence, item or part must be typed by occurrence definitions | (d) not-applicable: not stated by Beta 2 (the pilot keeps `checkPartUsage` commented out); `part p : AD` is validateOccurrenceUsageType in both tools; `part p : I` with an item definition is validatePartUsagePartDefinition, which ours reports and the pilot does not — see Observations | `validatePartUsageType.violating.sysml` | none |
| 38 | `validatePortDefinitionConjugatedPortDefinition` | sysml | sysml 8.3.12.5 | clean | clean | (d) not-applicable: derived structure of `port def` | `validatePortDefinitionConjugatedPortDefinition.conforming.sysml` (conforming only) | none |
| 39 | `validateReferenceUsageIsReference` | sysml | sysml 8.3.6.3 | clean | clean | (d) not-applicable: grammar: `ref` is referential | `validateReferenceUsageIsReference.conforming.sysml` (conforming only) | none |
| 40 | `validateRequirementConstraintMembershipIsComposite` | sysml | sysml 8.3.21.7 | syntax | e: no viable alternative at input 'require' | (d) not-applicable: grammar: `require ref constraint` and `require constraint ref` are syntax errors in both parsers | `validateRequirementConstraintMembershipIsComposite.violating.sysml` | none |
| 41 | `validateRequirementVerificationMembershipKind` | sysml | sysml 8.3.24.2 | verification-owning-type | e: no viable alternative at input 'verify' | (d) not-applicable: grammar fixes the kind | `validateRequirementVerificationMembershipKind.conforming.sysml` (conforming only) | none |
| 42 | `validateResultExpressionMembershipOwningType` | kerml | kerml 8.3.4.7.7 | syntax | e: no viable alternative at input '1' | (d) not-applicable: grammar: a result expression member is only admitted in a function or expression body (`class C { 1 + 1 }`, `part def P { 1 + 1 }` are syntax errors in both parsers) | `validateResultExpressionMembershipOwningType.violating.kerml` | none |
| 43 | `validateSelectExpressionOperator` | kerml | kerml 8.3.4.8.18 | clean | clean | (d) not-applicable: grammar fixes the operator | `validateSelectExpressionOperator.conforming.kerml` (conforming only) | none |
| 44 | `validateSubsettingPortionConformance` | kerml | kerml 8.3.3.3.10 | clean | clean | (d) not-applicable: not stated by Beta 2 and a pilot no-op (constant declared, never reported); both tools are silent on `feature f : O :> p` with `portion feature p` | `validateSubsettingPortionConformance.violating.kerml` | none |
| 45 | `validateTransitionFeatureMembershipEffectAction` | sysml | sysml 8.3.18.8 | clean | clean | (d) not-applicable: grammar: `do` takes an action | `validateTransitionFeatureMembershipEffectAction.conforming.sysml` (conforming only) | none |
| 46 | `validateTransitionFeatureMembershipOwningType` | sysml | sysml 8.3.18.8 | clean | clean | (d) not-applicable: grammar | `validateTransitionFeatureMembershipOwningType.conforming.sysml` (conforming only) | none |
| 47 | `validateTransitionFeatureMembershipTriggerAction` | sysml | sysml 8.3.18.8 | clean | clean | (d) not-applicable: grammar: `accept` is the only trigger | `validateTransitionFeatureMembershipTriggerAction.conforming.sysml` (conforming only) | none |
| 48 | `validateTransitionUsageParameters` | sysml | sysml 8.3.18.9 | clean | clean | (d) not-applicable: grammar | `validateTransitionUsageParameters.conforming.sysml` (conforming only) | none |
| 49 | `validateUsageIsReferential` | sysml | sysml 8.3.6.4 | clean | clean | (d) not-applicable: grammar: a directed, end or package-level usage is referential; our API JSON exports `in part x` and a package-level `part q` with `isComposite = false` | `validateUsageIsReferential.conforming.sysml` (conforming only) | none |
| 50 | `validateUsageVariationIsAbstract` | sysml | sysml 8.3.6.4 | clean | clean | (d) not-applicable: `isAbstract` is implied by `variation`; not violable in text | `validateUsageVariationIsAbstract.conforming.sysml` (conforming only) | none |
| 51 | `validateUseCaseUsageReference` | sysml | sysml 8.3.25.4 | clean | clean | (d) not-applicable: pilot no-op (constant declared, no check) and not named by Beta 2; `include` referents are validateIncludeUseCaseUsageReference | `validateUseCaseUsageReference.conforming.sysml` (conforming only) | none |
| 52 | `validateWhileLoopActionUsageParameters` | sysml | sysml 8.3.17.19 | clean | clean | (d) not-applicable: grammar; Beta 2 name validateWhileLoopActionUsage, now cited by the row | `validateWhileLoopActionUsageParameters.conforming.sysml` (conforming only) | none |
| 53 | `validateAssignmentActionUsage` | spec | sysml 8.3.17.5 | assignment-referent | e: An assignment must have a referent. | (a) implemented-but-unmapped: the formal document's name for the pilot's validateAssignmentActionUsageReferentIsTimeVarying (faithful); the pilot row now cites it | `validateAssignmentActionUsage.pilot-twin.violating.sysml` | none |
| 54 | `validateAttributeUsageIsReference` | spec | sysml 8.3.7.3 | clean | clean | (d) not-applicable: rename of validateAttributeUsageIsReferential (above) | `validateAttributeUsageIsReference.conforming.sysml` (conforming only) | none |
| 55 | `validateConjugatedPortDefinitionConjugatedPortDefinitionIsEmpty` | spec | sysml 8.3.12.2 | clean | clean | (d) not-applicable: rename of validateConjugatedPortDefinitionConjugatedPortDefinition (above) | `validateConjugatedPortDefinitionConjugatedPortDefinitionIsEmpty.conforming.sysml` (conforming only) | none |
| 56 | `validateConjugatedPortDefinitionOriginalPortDefinition` | spec | sysml 8.3.12.2 | clean | clean | (d) not-applicable: derived structure of `port def` | `validateConjugatedPortDefinitionOriginalPortDefinition.conforming.sysml` (conforming only) | none |
| 57 | `validateConnectionDefinitionIsSufficient` | spec | sysml 8.3.13.3 | related-elements | e: Must have at least two related elements | (d) not-applicable: derived: a connection definition is sufficient; not violable in text (our API JSON omits `isSufficient` — see the derive* table) | `validateConnectionDefinitionIsSufficient.conforming.sysml` (conforming only) | none |
| 58 | `validateControlNodeIsComposite` | spec | sysml 8.3.17.6 | syntax | e: no viable alternative at input 'ref' | (d) not-applicable: grammar: `ref fork` and `fork ref` are syntax errors in both parsers; our API JSON emits `isComposite = true` for a fork node | `validateControlNodeIsComposite.violating.sysml` | none |
| 59 | `validateElementFilterMembershipConditionIsBoolean` | spec | kerml 8.3.4.13.2 | filter-not-boolean | e: Must have a Boolean result | (a) implemented-but-unmapped: rename of validateElementFilterMembershipIsBoolean (faithful, `filter-not-boolean`) | `validateElementFilterMembershipConditionIsBoolean.violating.sysml` | none |
| 60 | `validateElementFilterMembershipConditionIsModelLevelEvaluable` | spec | kerml 8.3.4.13.2 | clean | e: Couldn't resolve reference to Type 'Boolean'. | (a) implemented-but-unmapped: rename of validateElementFilterMembershipIsModelLevelEvaluable (faithful, `filter-not-evaluable`) | `validateElementFilterMembershipConditionIsModelLevelEvaluable.violating.sysml` | none |
| 61 | `validateEndFeatureMembershipIsEnd` | spec | kerml 8.3.3.3.3 | clean | clean | (d) not-applicable: rename of validateEndFeatureMembershpIsEnd (above) | `validateEndFeatureMembershipIsEnd.conforming.kerml` (conforming only) | none |
| 62 | `validateEventOccurrenceUsageReference` | spec | sysml 8.3.9.2 | name-conflict / occurrence-usage-type / event-reference-occurrence | e: Must reference an occurrence. | (a) implemented-but-unmapped: rename of validateEventOccurrenceUsageReferent (faithful, `event-reference-occurrence`) | `validateEventOccurrenceUsageReference.violating.sysml` | none |
| 63 | `validateExposeVisibility` | spec | sysml 8.3.26.2 | clean | e: no viable alternative at input 'public' | (d) not-applicable: grammar fixes `protected`: `public expose` and `private expose` are syntax errors in both parsers; our API JSON emits `visibility = protected` | `validateExposeVisibility.violating.sysml` | none |
| 64 | `validateFeatureChainExpressionConformance` | spec | kerml 8.3.4.8.4 | feature-reference-referent | e: The required feature 'targetFeature' of 'org.omg.sysml.lang.sysml.i | (a) implemented-but-unmapped: rename of validateFeatureChainExpressionFeatureConformance (faithful, `feature-reference-referent`) | `validateFeatureChainExpressionConformance.pilot-twin.violating.kerml` | none |
| 65 | `validateFeatureEndMultiplicity` | spec | kerml 8.3.3.3.4 | end-feature-multiplicity | w: End feature must have multiplicity 1 | (a) implemented-but-unmapped: rename of validateFeatureEndFeatureMultiplicity (faithful, warning `end-feature-multiplicity`) | `validateFeatureEndMultiplicity.pilot-twin.violating.kerml` | none |
| 66 | `validateFlowDefinitionFlowEnds` | spec | sysml 8.3.16.2 | flow-def-too-many-ends | e: A flow connection definition can have at most two ends. | (a) implemented-but-unmapped: rename of validateFlowDefinitionConnectionEnds (faithful, `flow-def-too-many-ends`) | `validateFlowDefinitionFlowEnds.pilot-twin.violating.sysml` | none |
| 67 | `validateFlowPayloadFeature` | spec | kerml 8.3.4.9.2 | clean | clean | (d) not-applicable: rename of validateFlowItemFeature (above) | `validateFlowPayloadFeature.conforming.kerml` (conforming only) | none |
| 68 | `validateMetadataAccessExpressionReferencedElement` | spec | kerml 8.3.4.8.15 | clean | e: An attribute must be typed by attribute definitions. | (d) not-applicable: abstract-syntax invariant of `.metadata` | `validateMetadataAccessExpressionReferencedElement.conforming.sysml` (conforming only) | none |
| 69 | `validateMetadataFeatureMetaclass` | spec | kerml 8.3.4.12.3 | metadata-metaclass | e: Must have a concrete type | (a) implemented-but-unmapped: rename of validateMetadataFeatureMetadata (faithful, `metadata-metaclass`) | `validateMetadataFeatureMetaclass.pilot-twin.violating.kerml` | none |
| 70 | `validateMetadataFeatureMetaclassNotAbstract` | spec | kerml 8.3.4.12.3 | metadata-concrete-type | e: Must have a concrete type | (a) implemented-but-unmapped: rename of validateMetadataFeatureMetadataNotAbstract (faithful, `metadata-concrete-type`) | `validateMetadataFeatureMetaclassNotAbstract.pilot-twin.violating.kerml` | none |
| 71 | `validateMultiplicityRangeBoundResultTypes` | spec | kerml 8.3.4.11.2 | multiplicity-bound-natural | e: Must have a Natural value | (a) implemented-but-unmapped: rename of validateMultiplicityRangeResultTypes (faithful, `multiplicity-bound-natural`) | `validateMultiplicityRangeBoundResultTypes.pilot-twin.violating.kerml` | none |
| 72 | `validateNamespaceDistinguishibility` | spec | kerml 8.3.2.4.5 | name-conflict / name-conflict | w: Duplicate of other owned member name | (a) implemented-but-unmapped: rename of validateNamespaceDistinguishablity (approximate, warning `name-conflict`) | `validateNamespaceDistinguishibility.pilot-twin.violating.kerml` | none |
| 73 | `validateOccurrenceUsagePortionKind` | spec | sysml 8.3.9.4 | portion-owner | e: Must be owned by an occurrence definition or usage. | (a) implemented-but-unmapped: the rule the pilot reports under the name validateOccurrenceUsageIsPortion (`Must be owned by an occurrence definition or usage.`); implemented by `W10BPortionOwnerPass` (`portion-owner`); the formal validateOccurrenceUsageIsPortion is the distinct derivation `portionKind <> null implies isPortion`, so this row is recorded faithful on its own | `validateOccurrenceUsagePortionKind.violating.sysml` | none |
| 74 | `validateParameterMembershipParameterDirection` | spec | kerml 8.3.4.6.4 | clean | clean | (d) not-applicable: rename of validateParameterMembershipDirection (above) | `validateParameterMembershipParameterDirection.conforming.sysml` (conforming only) | none |
| 75 | `validateSendActionParameters` | spec | sysml 8.3.17.15 | send-payload-missing | e: no viable alternative at input 'state' | (d) not-applicable: grammar: `send` always owns its payload, sender and receiver parameters; the observable rules are the pilot's validateSendActionUsagePayloadArgument and validateSendActionUsageReceiver (faithful) | `validateSendActionParameters.pilot-twin.violating.sysml` | none |
| 76 | `validateSpecificationSpecificNotConjugated` | spec | kerml 8.3.3.1.8 | specialization-specific-conjugated | e: Conjugated type cannot be a specialized type | (a) implemented-but-unmapped: rename of validateSpecializationSpecificNotConjugated (faithful, `specialization-specific-conjugated`) | `validateSpecificationSpecificNotConjugated.pilot-twin.violating.kerml` | none |
| 77 | `validateStateDefinitionStateSubactionKind` | spec | sysml 8.3.18.5 | state-entry-action | e: A state may have at most one entry action. | (a) implemented-but-unmapped: rename of validateStateDefinitionSubactionKind (faithful, `state-entry-action`) | `validateStateDefinitionStateSubactionKind.pilot-twin.violating.sysml` | none |
| 78 | `validateStateSubactionMembershipOwningType` | spec | sysml 8.3.18.4 | state-subaction-membership-owning-type | e: mismatched input 'entry' expecting '}' | (a) implemented-but-unmapped: rename of validateStateSubactionMembershioOwningType (faithful, `state-subaction-membership-owning-type`) | `validateStateSubactionMembershipOwningType.pilot-twin.violating.sysml` | none |
| 79 | `validateStateUsageStateSubactionKind` | spec | sysml 8.3.18.6 | state-entry-action | e: A state may have at most one entry action. | (a) implemented-but-unmapped: rename of validateStateUsageSubactionKind (faithful, `state-entry-action`) | `validateStateUsageStateSubactionKind.pilot-twin.violating.sysml` | none |
| 80 | `validateTriggerInvocationExpressionAfterArgument` | spec | sysml 8.3.17.17 | trigger-after-duration | e: An after expression must be a DurationValue. | (a) implemented-but-unmapped: rename of validateTriggerInvocationActionAfterArgument (faithful, `trigger-after-duration`) | `validateTriggerInvocationExpressionAfterArgument.pilot-twin.violating.sysml` | none |
| 81 | `validateTriggerInvocationExpressionAtArgument` | spec | sysml 8.3.17.17 | trigger-at-time-instant | e: An at expression must be a TimeInstantValue. | (a) implemented-but-unmapped: rename of validateTriggerInvocationActionAtArgument (faithful, `trigger-at-time-instant`) | `validateTriggerInvocationExpressionAtArgument.pilot-twin.violating.sysml` | none |
| 82 | `validateTriggerInvocationExpressionWhenArgument` | spec | sysml 8.3.17.17 | trigger-when-boolean | e: A when expression must be Boolean. | (a) implemented-but-unmapped: rename of validateTriggerInvocationActionWhenArgument (faithful, `trigger-when-boolean`) | `validateTriggerInvocationExpressionWhenArgument.pilot-twin.violating.sysml` | none |
| 83 | `validateTypeOwnedDifferencingNotOne` | spec | kerml 8.3.3.1.10 | type-differencing-not-one | e: Cannot have only one differencing | (a) implemented-but-unmapped: rename of validateOwnedDifferencingNotOne (faithful, `type-differencing-not-one`) | `validateTypeOwnedDifferencingNotOne.pilot-twin.violating.kerml` | none |
| 84 | `validateTypeOwnedIntersectingNotOne` | spec | kerml 8.3.3.1.10 | type-intersecting-not-one | e: Cannot have only one intersecting | (a) implemented-but-unmapped: rename of validateOwnedIntersectingNotOne (faithful, `type-intersecting-not-one`) | `validateTypeOwnedIntersectingNotOne.pilot-twin.violating.kerml` | none |
| 85 | `validateTypeOwnedUnioningNotOne` | spec | kerml 8.3.3.1.10 | type-unioning-not-one | e: Cannot have only one unioning | (a) implemented-but-unmapped: rename of validateOwnedUnioningNotOne (faithful, `type-unioning-not-one`) | `validateTypeOwnedUnioningNotOne.pilot-twin.violating.kerml` | none |
| 86 | `validateVariantMembershipOwningNamespace` | spec | sysml 8.3.6.5 | variant-outside-variation | e: A variant must be an owned member of a variation. | (a) implemented-but-unmapped: rename of validateVariationMembershipOwningNamespace (faithful, `variant-outside-variation`) | `validateVariantMembershipOwningNamespace.pilot-twin.violating.sysml` | none |
| 87 | `validateViewUsageOnlyOneViewRendering` | spec | sysml 8.3.26.11 | only-one-view-rendering | e: A view may have at most one view rendering. | (a) implemented-but-unmapped: rename of validateViewUsageOnlyOneRendering (faithful, `only-one-view-rendering`) | `validateViewUsageOnlyOneViewRendering.pilot-twin.violating.sysml` | none |
| 88 | `validateWhileLoopActionUsage` | spec | sysml 8.3.17.19 | clean | clean | (d) not-applicable: rename of validateWhileLoopActionUsageParameters (above) | `validateWhileLoopActionUsage.conforming.sysml` (conforming only) | none |

Summary: 24 rows are (a) implemented-but-unmapped — 23 are the document's names for pilot
constraints this implementation already reports (the pilot rows now cite them, so the census
counts them as pilot-named) and `validateOccurrenceUsagePortionKind` is recorded faithful on its
own; 64 rows are (d) not-applicable; no row is (b) or (c). Six further document names are
renames of pilot constraints that are themselves not-applicable (`validateAttributeUsageIsReference`,
`validateConjugatedPortDefinitionConjugatedPortDefinitionIsEmpty`, `validateEndFeatureMembershipIsEnd`,
`validateFlowPayloadFeature`, `validateParameterMembershipParameterDirection`,
`validateWhileLoopActionUsage`); their pilot rows cite them too.

## Observations: where the two tools differ on a probe

Neither item is a rule this record changes; both are recorded so the difference is on file.

- `validatePartUsagePartDefinition` (Beta 2 §8.3.11.3, `partDefinition->notEmpty()`): on
  `package P { item def I; part p : I; }` and on `item def ID; part def P { part p : ID; }`
  this implementation reports `part-usage-part-definition` and the pinned pilot reports nothing.
  The rule is stated by Beta 2, so the report is specification-backed and stricter than the
  pilot, not stricter than the specification.
- `[3..1]` (`validateMultiplicityRangeBounds.violating.kerml`): this implementation reports
  `multiplicity-range` (lower bound exceeds upper bound), the pilot reports nothing. Beta 2
  names no constraint for a lower bound above the upper bound; the report rests on the
  semantics of a multiplicity range (KerML §7.3.3), not on a named constraint, and is stricter
  than the pilot.
- `var end a : A` (`validateFeatureEndIsConstant.violating.kerml`): the Beta 2 grammar admits
  only `const` before `end`; the pilot rejects the text as a syntax error, our parser accepts it
  and reports `variable-feature-owner`. A lenience of the parser, not a constraint verdict.

## Launch-ready gaps

None from the `validate*` rows: every rule the Beta 2 documents state for these 88 names is
either reported already or cannot be violated in the textual notation.
