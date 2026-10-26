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

## `check*` rows (208)

A `check*` constraint states an implied relationship — a specialization, subsetting or
redefinition of a library element, a binding connector, a type featuring — rather than a
diagnostic, so the probe asks whether the relationship is present. `bin/sysml -convert api-json`
writes no implied relationships (nothing carries `isImplied = true`), so the observable surface
is the model-level query `bin/sysml -e "<element> istype <library type>" <model>`: an untyped
usage (`part u_part;`) or a definition instance (`new P::D()`) is classified by a library type
only through the relationship the language implies, so a `true` answer is that relationship
and a `false` or undetermined answer its absence. The two models the queries run over are
`probes/triage/check-model.sysml` and `probes/triage/check-model.kerml`. The pinned pilot
validators report no `check*` constraint (none is a diagnostic), so the *Pilot* column is
n/a throughout; the pilot's own JSON export is the subject of a separate differential.

Four kinds of row cannot be observed this way and are marked (d) with the reason: a binding
connector, redefinition or type featuring (no surface emits them); a relationship whose target
is not a library element; a relationship the clause conditions on the owner or membership,
where `istype` cannot tell the conditional library feature (`Parts::subparts`) from the
unconditional one of the same type (`Parts::parts`); and an element that only exists inside a
behaviour body or an expression, where the model-level read performs it instead of classifying
it. A row observed `true` is recorded in the manifest against
`internal/semantic/semantics/implicit.go:Model.computeImplicitBases`, the table that supplies
the implied base of every definition and usage kind: faithful for the 29 rows whose clause
names a library *type* (`specializesFromLibrary('Parts::Part')`), approximate for the 14 rows
whose clause names a library *feature* (`specializesFromLibrary('Parts::parts')`). For those
14, `istype` against the base *definition* proves only that the usage is classified by the base
feature's type; `istype` against the base *usage* (`P::u_part istype Parts::parts`) and
`Model.Conforms(usage, baseFeature)` prove the subsetting the clause states. Both now hold:
`implicitBaseFeatures` makes the base usage an implied general beside the typing, and
`TestImplicitBaseUsageContributesThat` pins that the base usage contributes `that` *and* is a
supertype while `FeatureTypeSet` stays the definition's. The record of the gap this closed is
kept under Launch-ready gaps.

| # | Constraint | Metaclass | Clause | Ours | Pilot | Classification | Probe | In-flight overlap |
|---|---|---|---|---|---|---|---|---|
| 1 | `checkAcceptActionUsageReceiverBindingConnector` | AcceptActionUsage | sysml 8.3.17.2 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 2 | `checkAcceptActionUsageSpecialization` | AcceptActionUsage | sysml 8.3.17.2 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 3 | `checkAcceptActionUsageSubactionSpecialization` | AcceptActionUsage | sysml 8.3.17.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 4 | `checkAcceptActionUsageTriggerActionSpecialization` | AcceptActionUsage | sysml 8.3.17.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 5 | `checkActionDefinitionSpecialization` | ActionDefinition | sysml 8.3.17.3 | yes — `new P::A() istype Actions::Action` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 6 | `checkActionUsageOwnedActionSpecialization` | ActionUsage | sysml 8.3.17.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 7 | `checkActionUsageSpecialization` | ActionUsage | sysml 8.3.17.4 | not observable: `new P::Holder().act istype Actions::Action` → error: sysml: evaluation failed: uninitialized feature value: act | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 8 | `checkActionUsageStateActionRedefinition` | ActionUsage | sysml 8.3.17.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 9 | `checkActionUsageSubactionSpecialization` | ActionUsage | sysml 8.3.17.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 10 | `checkAllocationDefinitionSpecialization` | AllocationDefinition | sysml 8.3.15.2 | yes — `new P::ALD() istype Allocations::Allocation` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 11 | `checkAllocationUsageSpecialization` | AllocationUsage | sysml 8.3.15.3 | yes — `new P::Holder().alx istype Allocations::allocations` = true; `new P::Holder().alx istype Allocations::Allocation` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Allocations::allocations` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 12 | `checkAnalysisCaseDefinitionSpecialization` | AnalysisCaseDefinition | sysml 8.3.23.2 | yes — `new P::AN() istype AnalysisCases::AnalysisCase` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 13 | `checkAnalysisCaseUsageSpecialization` | AnalysisCaseUsage | sysml 8.3.23.3 | not observable: `new P::Holder().an istype AnalysisCases::AnalysisCase` → error: sysml: evaluation failed: no value: calc usage an computes output features (no output feature); read one of th | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 14 | `checkAnalysisCaseUsageSubAnalysisCaseSpecialization` | AnalysisCaseUsage | sysml 8.3.23.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 15 | `checkAssertConstraintUsageSpecialization` | AssertConstraintUsage | sysml 8.3.20.2 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 16 | `checkAssignmentActionUsageAccessedFeatureRedefinition` | AssignmentActionUsage | sysml 8.3.17.5 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 17 | `checkAssignmentActionUsageReferentRedefinition` | AssignmentActionUsage | sysml 8.3.17.5 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 18 | `checkAssignmentActionUsageSpecialization` | AssignmentActionUsage | sysml 8.3.17.5 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 19 | `checkAssignmentActionUsageStartingAtRedefinition` | AssignmentActionUsage | sysml 8.3.17.5 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 20 | `checkAssignmentActionUsageSubactionSpecialization` | AssignmentActionUsage | sysml 8.3.17.5 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 21 | `checkAssociationBinarySpecialization` | Association | kerml 8.3.4.4.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 22 | `checkAssociationSpecialization` | Association | kerml 8.3.4.4.2 | yes — `new K::As() istype Links::Link` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 23 | `checkAssociationStructureBinarySpecialization` | AssociationStructure | kerml 8.3.4.4.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 24 | `checkAssociationStructureSpecialization` | AssociationStructure | kerml 8.3.4.4.3 | yes — `new K::AS() istype Objects::LinkObject` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 25 | `checkAttributeUsageSpecialization` | AttributeUsage | sysml 8.3.7.3 | not observable: `new P::Holder().atr istype Base::DataValue` → error: sysml: evaluation failed: uninitialized feature value: atr | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 26 | `checkBehaviorSpecialization` | Behavior | kerml 8.3.4.6.2 | yes — `new K::Bh() istype Performances::Performance` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 27 | `checkBindingConnectorSpecialization` | BindingConnector | kerml 8.3.4.5.2 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 28 | `checkBooleanExpressionSpecialization` | BooleanExpression | kerml 8.3.4.7.2 | not observable: `new K::Oc().nbool istype Performances::BooleanEvaluation` → error: sysml: evaluation failed: object has no such feature: member nbool not found in instance | n/a (no diagnostic) | (d) not-observable: error: sysml: evaluation failed: object has no such feature: member nbool not found in instance | `probes/triage/check-model.kerml` | none |
| 29 | `checkCalculationDefinitionSpecialization` | CalculationDefinition | sysml 8.3.19.2 | yes — `new P::K() istype Calculations::Calculation` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 30 | `checkCalculationUsageSpecialization` | CalculationUsage | sysml 8.3.19.3 | not observable: `new P::Holder().kk istype Calculations::Calculation` → error: sysml: evaluation failed: no value: calc usage kk computes output features (no output feature); read one of th | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 31 | `checkCalculationUsageSubcalculationSpecialization` | CalculationUsage | sysml 8.3.19.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 32 | `checkCaseDefinitionSpecialization` | CaseDefinition | sysml 8.3.22.2 | yes — `new P::CS() istype Cases::Case` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 33 | `checkCaseUsageSpecialization` | CaseUsage | sysml 8.3.22.3 | yes — `P::u_case istype Cases::cases` = true; `P::u_case istype Cases::Case` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Cases::cases` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 34 | `checkCaseUsageSubcaseSpecialization` | CaseUsage | sysml 8.3.22.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 35 | `checkClassSpecialization` | Class | kerml 8.3.4.2.2 | yes — `new K::Cl() istype Occurrences::Occurrence` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 36 | `checkConcernDefinitionSpecialization` | ConcernDefinition | sysml 8.3.21.3 | yes — `new P::CO() istype Requirements::ConcernCheck` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 37 | `checkConcernUsageFramedConcernSpecialization` | ConcernUsage | sysml 8.3.21.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 38 | `checkConcernUsageSpecialization` | ConcernUsage | sysml 8.3.21.4 | yes — `P::u_concern istype Requirements::concernChecks` = true; `P::u_concern istype Requirements::ConcernCheck` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Requirements::concernChecks` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 39 | `checkConnectionDefinitionBinarySpecialization` | ConnectionDefinition | sysml 8.3.13.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 40 | `checkConnectionDefinitionSpecializations` | ConnectionDefinition | sysml 8.3.13.3 | yes — `new P::CN() istype Connections::Connection` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 41 | `checkConnectionUsageBinarySpecialization` | ConnectionUsage | sysml 8.3.13.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 42 | `checkConnectionUsageSpecialization` | ConnectionUsage | sysml 8.3.13.4 | yes — `new P::Holder().cnx istype Connections::connections` = true; `new P::Holder().cnx istype Connections::Connection` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Connections::connections` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 43 | `checkConnectorBinaryObjectSpecialization` | Connector | kerml 8.3.4.5.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 44 | `checkConnectorBinarySpecialization` | Connector | kerml 8.3.4.5.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 45 | `checkConnectorObjectSpecialization` | Connector | kerml 8.3.4.5.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 46 | `checkConnectorSpecialization` | Connector | kerml 8.3.4.5.3 | yes — `Model.Conforms(K::cx, Links::links)` and `Conforms(K::cx, Links::binaryLinks)` true (`istype` classifies a connector's value by its type only); `K::cx istype Links::Link` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Links::links` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.kerml` | none |
| 47 | `checkConnectorTypeFeaturing` | Connector | kerml 8.3.4.5.3 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 48 | `checkConstraintDefinitionSpecialization` | ConstraintDefinition | sysml 8.3.20.3 | yes — `new P::C() istype Constraints::ConstraintCheck` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 49 | `checkConstraintUsageCheckedConstraintSpecialization` | ConstraintUsage | sysml 8.3.20.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 50 | `checkConstraintUsageRequirementConstraintSpecialization` | ConstraintUsage | sysml 8.3.20.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 51 | `checkConstraintUsageSpecialization` | ConstraintUsage | sysml 8.3.20.4 | not observable: `new P::Holder().cc istype Constraints::ConstraintCheck` → error: sysml: evaluation failed: uninitialized feature value: cc | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 52 | `checkConstructorExpressionResultDefaultValueBindingConnector` | ConstructorExpression | kerml 8.3.4.8.3 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 53 | `checkConstructorExpressionResultFeatureRedefinition` | ConstructorExpression | kerml 8.3.4.8.3 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 54 | `checkConstructorExpressionResultSpecialization` | ConstructorExpression | kerml 8.3.4.8.3 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 55 | `checkConstructorExpressionSpecialization` | ConstructorExpression | kerml 8.3.4.8.3 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 56 | `checkControlNodeSpecialization` | ControlNode | sysml 8.3.17.6 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 57 | `checkDataTypeSpecialization` | DataType | kerml 8.3.4.1.2 | yes — `new K::DT() istype Base::DataValue` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 58 | `checkDecisionNodeOutgoingSuccessionSpecialization` | DecisionNode | sysml 8.3.17.7 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 59 | `checkDecisionNodeSpecialization` | DecisionNode | sysml 8.3.17.7 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 60 | `checkEventOccurrenceUsageSpecialization` | EventOccurrenceUsage | sysml 8.3.9.2 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 61 | `checkExhibitStateUsageSpecialization` | ExhibitStateUsage | sysml 8.3.18.2 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 62 | `checkExpressionResultBindingConnector` | Expression | kerml 8.3.4.7.3 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 63 | `checkExpressionSpecialization` | Expression | kerml 8.3.4.7.3 | not observable: `new K::Oc().nexpr istype Performances::Evaluation` → error: sysml: evaluation failed: object has no such feature: member nexpr not found in instance | n/a (no diagnostic) | (d) not-observable: error: sysml: evaluation failed: object has no such feature: member nexpr not found in instance | `probes/triage/check-model.kerml` | none |
| 64 | `checkExpressionTypeFeaturing` | Expression | kerml 8.3.4.7.3 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 65 | `checkFeatureChainExpressionResultSpecialization` | FeatureChainExpression | kerml 8.3.4.8.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 66 | `checkFeatureChainExpressionSourceTargetRedefinition` | FeatureChainExpression | kerml 8.3.4.8.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 67 | `checkFeatureChainExpressionTargetRedefinition` | FeatureChainExpression | kerml 8.3.4.8.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 68 | `checkFeatureCrossingSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 69 | `checkFeatureDataValueSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 70 | `checkFeatureEndRedefinition` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 71 | `checkFeatureEndSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 72 | `checkFeatureFeatureMembershipTypeFeaturing` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 73 | `checkFeatureFlowFeatureRedefinition` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 74 | `checkFeatureObjectSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 75 | `checkFeatureOccurrenceSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 76 | `checkFeatureOwnedCrossFeatureRedefinitionSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 77 | `checkFeatureOwnedCrossFeatureSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 78 | `checkFeatureOwnedCrossFeatureTypeFeaturing` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 79 | `checkFeatureParameterRedefinition` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 80 | `checkFeaturePortionSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 81 | `checkFeatureReferenceExpressionBindingConnector` | FeatureReferenceExpression | kerml 8.3.4.8.5 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 82 | `checkFeatureReferenceExpressionResultSpecialization` | FeatureReferenceExpression | kerml 8.3.4.8.5 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 83 | `checkFeatureResultRedefinition` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 84 | `checkFeatureSpecialization` | Feature | kerml 8.3.3.3.4 | not observable: `new K::Oc().nested istype Base::Anything` → error: sysml: evaluation failed: uninitialized feature value: nested | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.kerml` | none |
| 85 | `checkFeatureSubobjectSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 86 | `checkFeatureSuboccurrenceSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 87 | `checkFeatureValuationSpecialization` | Feature | kerml 8.3.3.3.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 88 | `checkFeatureValueBindingConnector` | FeatureValue | kerml 8.3.4.10.2 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 89 | `checkFlowDefinitionBinarySpecialization` | FlowDefinition | sysml 8.3.16.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 90 | `checkFlowDefinitionSpecialization` | FlowDefinition | sysml 8.3.16.2 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 91 | `checkFlowSpecialization` | Flow | kerml 8.3.4.9.2 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 92 | `checkFlowUsageFlowSpecialization` | FlowUsage | sysml 8.3.16.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 93 | `checkFlowUsageSpecialization` | FlowUsage | sysml 8.3.16.3 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 94 | `checkFlowWithEndsSpecialization` | Flow | kerml 8.3.4.9.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 95 | `checkForLoopActionUsageSpecialization` | ForLoopActionUsage | sysml 8.3.17.9 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 96 | `checkForLoopActionUsageSubactionSpecialization` | ForLoopActionUsage | sysml 8.3.17.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 97 | `checkForLoopActionUsageVarRedefinition` | ForLoopActionUsage | sysml 8.3.17.9 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 98 | `checkForkNodeSpecialization` | ForkNode | sysml 8.3.17.8 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 99 | `checkFunctionResultBindingConnector` | Function | kerml 8.3.4.7.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 100 | `checkFunctionSpecialization` | Function | kerml 8.3.4.7.4 | yes — `new K::Fn() istype Performances::Evaluation` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 101 | `checkIfActionUsageSpecialization` | IfActionUsage | sysml 8.3.17.10 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 102 | `checkIfActionUsageSubactionSpecialization` | IfActionUsage | sysml 8.3.17.10 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 103 | `checkIncludeUseCaseSpecialization` | IncludeUseCaseUsage | sysml 8.3.25.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 104 | `checkIndexExpressionResultSpecialization` | IndexExpression | kerml 8.3.4.8.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 105 | `checkInterfaceDefinitionBinarySpecialization` | InterfaceDefinition | sysml 8.3.14.2 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 106 | `checkInterfaceDefinitionSpecialization` | InterfaceDefinition | sysml 8.3.14.2 | yes — `new P::IF() istype Interfaces::Interface` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 107 | `checkInterfaceUsageBinarySpecialization` | InterfaceUsage | sysml 8.3.14.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 108 | `checkInterfaceUsageSpecialization` | InterfaceUsage | sysml 8.3.14.3 | yes — `new P::Holder().ifx istype Interfaces::interfaces` = true; `new P::Holder().ifx istype Interfaces::Interface` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Interfaces::interfaces` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 109 | `checkInvariantSpecialization` | Invariant | kerml 8.3.4.7.5 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 110 | `checkInvocationExpressionBehaviorBindingConnector` | InvocationExpression | kerml 8.3.4.8.8 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 111 | `checkInvocationExpressionBehaviorResultSpecialization` | InvocationExpression | kerml 8.3.4.8.8 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 112 | `checkInvocationExpressionDefaultValueBindingConnector` | InvocationExpression | kerml 8.3.4.8.8 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 113 | `checkInvocationExpressionSpecialization` | InvocationExpression | kerml 8.3.4.8.8 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 114 | `checkItemDefinitionSpecialization` | ItemDefinition | sysml 8.3.10.2 | yes — `new P::I() istype Items::Item` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 115 | `checkItemUsageSpecialization` | ItemUsage | sysml 8.3.10.3 | yes — `P::u_item istype Items::items` = true; `P::u_item istype Items::Item` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Items::items` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 116 | `checkItemUsageSubitemSpecialization` | ItemUsage | sysml 8.3.10.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 117 | `checkJoinNodeSpecialization` | JoinNode | sysml 8.3.17.11 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 118 | `checkLiteralBooleanSpecialization` | LiteralBoolean | kerml 8.3.4.8.9 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 119 | `checkLiteralExpressionSpecialization` | LiteralExpression | kerml 8.3.4.8.10 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 120 | `checkLiteralInfinitySpecialization` | LiteralInfinity | kerml 8.3.4.8.11 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 121 | `checkLiteralIntegerSpecialization` | LiteralInteger | kerml 8.3.4.8.12 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 122 | `checkLiteralRationalSpecialization` | LiteralRational | kerml 8.3.4.8.13 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 123 | `checkLiteralStringSpecialization` | LiteralString | kerml 8.3.4.8.14 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 124 | `checkMergeNodeIncomingSuccessionSpecialization` | MergeNode | sysml 8.3.17.13 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 125 | `checkMergeNodeSpecialization` | MergeNode | sysml 8.3.17.13 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 126 | `checkMetaclassSpecialization` | Metaclass | kerml 8.3.4.12.2 | yes — `new K::Mc() istype Metaobjects::Metaobject` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 127 | `checkMetadataAccessExpressionSpecialization` | MetadataAccessExpression | kerml 8.3.4.8.15 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 128 | `checkMetadataDefinitionSpecialization` | MetadataDefinition | sysml 8.3.27.2 | yes — `new P::MD() istype Metadata::MetadataItem` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 129 | `checkMetadataFeatureSemanticSpecialization` | MetadataFeature | kerml 8.3.4.12.3 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 130 | `checkMetadataFeatureSpecialization` | MetadataFeature | kerml 8.3.4.12.3 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 131 | `checkMetadataUsageSpecialization` | MetadataUsage | sysml 8.3.27.3 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 132 | `checkMultiplicityRangeExpressionTypeFeaturing` | MultiplicityRange | kerml 8.3.4.11.2 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 133 | `checkMultiplicitySpecialization` | Multiplicity | kerml 8.3.3.1.9 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 134 | `checkMultiplicityTypeFeaturing` | Multiplicity | kerml 8.3.3.1.9 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 135 | `checkNullExpressionSpecialization` | NullExpression | kerml 8.3.4.8.16 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 136 | `checkOccurrenceDefinitionIndividualSpecialization` | OccurrenceDefinition | sysml 8.3.9.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 137 | `checkOccurrenceDefinitionMultiplicitySpecialization` | OccurrenceDefinition | sysml 8.3.9.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 138 | `checkOccurrenceUsageSnapshotSpecialization` | OccurrenceUsage | sysml 8.3.9.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 139 | `checkOccurrenceUsageSpecialization` | OccurrenceUsage | sysml 8.3.9.4 | yes — `P::u_occurrence istype Occurrences::occurrences` = true; `P::u_occurrence istype Occurrences::Occurrence` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Occurrences::occurrences` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 140 | `checkOccurrenceUsageSuboccurrenceSpecialization` | OccurrenceUsage | sysml 8.3.9.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 141 | `checkOccurrenceUsageTimeSliceSpecialization` | OccurrenceUsage | sysml 8.3.9.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 142 | `checkPartDefinitionSpecialization` | PartDefinition | sysml 8.3.11.2 | yes — `new P::D() istype Parts::Part` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 143 | `checkPartUsageActorSpecialization` | PartUsage | sysml 8.3.11.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 144 | `checkPartUsageSpecialization` | PartUsage | sysml 8.3.11.3 | yes — `P::u_part istype Parts::parts` = true; `P::u_part istype Parts::Part` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Parts::parts` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 145 | `checkPartUsageStakeholderSpecialization` | PartUsage | sysml 8.3.11.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 146 | `checkPartUsageSubpartSpecialization` | PartUsage | sysml 8.3.11.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 147 | `checkPayloadFeatureRedefinition` | PayloadFeature | kerml 8.3.4.9.5 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 148 | `checkPerformActionUsageSpecialization` | PerformActionUsage | sysml 8.3.17.14 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 149 | `checkPortDefinitionSpecialization` | PortDefinition | sysml 8.3.12.5 | yes — `new P::PD() istype Ports::Port` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 150 | `checkPortUsageOwnedPortSpecialization` | PortUsage | sysml 8.3.12.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 151 | `checkPortUsageSpecialization` | PortUsage | sysml 8.3.12.6 | yes — `new P::Holder().q istype Ports::ports` = true; `new P::Holder().q istype Ports::Port` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Ports::ports` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 152 | `checkPortUsageSubportSpecialization` | PortUsage | sysml 8.3.12.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 153 | `checkPredicateSpecialization` | Predicate | kerml 8.3.4.7.6 | yes — `new K::Pr() istype Performances::BooleanEvaluation` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 154 | `checkRenderingDefinitionSpecialization` | RenderingDefinition | sysml 8.3.26.5 | yes — `new P::RD() istype Views::Rendering` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 155 | `checkRenderingUsageRedefinition` | RenderingUsage | sysml 8.3.26.6 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 156 | `checkRenderingUsageSpecialization` | RenderingUsage | sysml 8.3.26.6 | yes — `P::u_rendering istype Views::renderings` = true; `P::u_rendering istype Views::Rendering` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Views::renderings` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 157 | `checkRenderingUsageSubrenderingSpecialization` | RenderingUsage | sysml 8.3.26.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 158 | `checkRequirementDefinitionSpecialization` | RequirementDefinition | sysml 8.3.21.8 | yes — `new P::R() istype Requirements::RequirementCheck` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 159 | `checkRequirementUsageObjectiveRedefinition` | RequirementUsage | sysml 8.3.21.9 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 160 | `checkRequirementUsageRequirementVerificationSpecialization` | RequirementUsage | sysml 8.3.21.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 161 | `checkRequirementUsageSpecialization` | RequirementUsage | sysml 8.3.21.9 | not observable: `new P::Holder().rr istype Requirements::RequirementCheck` → error: sysml: evaluation failed: uninitialized feature value: rr | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 162 | `checkRequirementUsageSubrequirementSpecialization` | RequirementUsage | sysml 8.3.21.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 163 | `checkSatisfyRequirementUsageBindingConnector` | SatisfyRequirementUsage | sysml 8.3.21.10 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 164 | `checkSatisfyRequirementUsageSpecialization` | SatisfyRequirementUsage | sysml 8.3.21.10 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 165 | `checkSelectExpressionResultSpecialization` | SelectExpression | kerml 8.3.4.8.18 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 166 | `checkSendActionUsageSpecialization` | SendActionUsage | sysml 8.3.17.15 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 167 | `checkSendActionUsageSubactionSpecialization` | SendActionUsage | sysml 8.3.17.15 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 168 | `checkStateDefinitionSpecialization` | StateDefinition | sysml 8.3.18.5 | yes — `new P::S() istype States::StateAction` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 169 | `checkStateUsageExclusiveStateSpecialization` | StateUsage | sysml 8.3.18.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 170 | `checkStateUsageOwnedStateSpecialization` | StateUsage | sysml 8.3.18.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 171 | `checkStateUsageSpecialization` | StateUsage | sysml 8.3.18.6 | not observable: `new P::Holder().st istype States::StateAction` → error: sysml: evaluation failed: uninitialized feature value: st | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 172 | `checkStateUsageSubstateSpecialization` | StateUsage | sysml 8.3.18.6 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 173 | `checkStepEnclosedPerformanceSpecialization` | Step | kerml 8.3.4.6.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 174 | `checkStepOwnedPerformanceSpecialization` | Step | kerml 8.3.4.6.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 175 | `checkStepSpecialization` | Step | kerml 8.3.4.6.3 | not observable: `new K::Oc().nstep istype Performances::Performance` → error: sysml: evaluation failed: uninitialized feature value: nstep | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.kerml` | none |
| 176 | `checkStepSubperformanceSpecialization` | Step | kerml 8.3.4.6.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 177 | `checkStructureSpecialization` | Structure | kerml 8.3.4.3.2 | yes — `new K::St() istype Objects::Object` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 178 | `checkSuccessionFlowSpecialization` | SuccessionFlow | kerml 8.3.4.9.6 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 179 | `checkSuccessionFlowUsageSpecialization` | SuccessionFlowUsage | sysml 8.3.16.4 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 180 | `checkSuccessionSpecialization` | Succession | kerml 8.3.4.5.4 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 181 | `checkTerminateActionUsageSpecialization` | TerminateActionUsage | sysml 8.3.17.16 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 182 | `checkTerminateActionUsageSubactionSpecialization` | TerminateActionUsage | sysml 8.3.17.16 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 183 | `checkTransitionUsageActionSpecialization` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 184 | `checkTransitionUsagePayloadSpecialization` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 185 | `checkTransitionUsageSourceBindingConnector` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 186 | `checkTransitionUsageSpecialization` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 187 | `checkTransitionUsageStateSpecialization` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 188 | `checkTransitionUsageSuccessionBindingConnector` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 189 | `checkTransitionUsageSuccessionSourceSpecialization` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 190 | `checkTransitionUsageTransitionFeatureSpecialization` | TransitionUsage | sysml 8.3.18.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 191 | `checkTypeSpecialization` | Type | kerml 8.3.3.1.10 | yes — `new K::Cl() istype Base::Anything` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.kerml` | none |
| 192 | `checkUsageVariationDefinitionSpecialization` | Usage | sysml 8.3.6.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 193 | `checkUsageVariationUsageSpecialization` | Usage | sysml 8.3.6.4 | not observable | n/a (no diagnostic) | (d) not-observable: the implied relationship targets no library element (`specializes(instantiatedType)`, a chain of the model's own features), and the specialization graph has no CLI query | — | none |
| 194 | `checkUsageVariationUsageTypeFeaturing` | Usage | sysml 8.3.6.4 | not observable | n/a (no diagnostic) | (d) not-observable: a binding connector, redefinition or type featuring the language implies is not emitted by `-convert api-json` (which writes no implied relationships) and has no model-level query | — | none |
| 195 | `checkUseCaseDefinitionSpecialization` | UseCaseDefinition | sysml 8.3.25.3 | yes — `new P::UC() istype UseCases::UseCase` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 196 | `checkUseCaseUsageSpecialization` | UseCaseUsage | sysml 8.3.25.4 | yes — `P::u_usecase istype UseCases::useCases` = true; `P::u_usecase istype UseCases::UseCase` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `UseCases::useCases` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 197 | `checkUseCaseUsageSubUseCaseSpecialization` | UseCaseUsage | sysml 8.3.25.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 198 | `checkVerificationCaseSpecialization` | VerificationCaseDefinition | sysml 8.3.24.3 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 199 | `checkVerificationCaseUsageSpecialization` | VerificationCaseUsage | sysml 8.3.24.4 | not observable: `new P::Holder().vf istype VerificationCases::VerificationCase` → error: sysml: evaluation failed: no value: calc usage vf computes output features (no output feature); read one of th | n/a (no diagnostic) | (d) not-observable: the model-level read of the usage performs it or has no value to classify; the relationship is not exposed through `istype` or `-convert api-json` | `probes/triage/check-model.sysml` | none |
| 200 | `checkVerificationCaseUsageSubVerificationCaseSpecialization` | VerificationCaseUsage | sysml 8.3.24.4 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 201 | `checkViewDefinitionSpecialization` | ViewDefinition | sysml 8.3.26.7 | yes — `new P::V() istype Views::View` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 202 | `checkViewUsageSpecialization` | ViewUsage | sysml 8.3.26.11 | yes — `P::u_view istype Views::views` = true; `P::u_view istype Views::View` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Views::views` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 203 | `checkViewUsageSubviewSpecialization` | ViewUsage | sysml 8.3.26.11 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 204 | `checkViewpointDefinitionSpecialization` | ViewpointDefinition | sysml 8.3.26.8 | yes — `new P::VP() istype Views::ViewpointCheck` = true | n/a (no diagnostic) | (a) implemented-but-unmapped: the implied specialization is present; the row is now recorded faithful against `Model.computeImplicitBases` | `probes/triage/check-model.sysml` | none |
| 205 | `checkViewpointUsageSpecialization` | ViewpointUsage | sysml 8.3.26.9 | yes — `P::u_viewpoint istype Views::viewpointChecks` = true; `P::u_viewpoint istype Views::ViewpointCheck` = true (the base definition still types it) | n/a (no diagnostic) | (a) implemented: the usage directly specializes `Views::viewpointChecks` as an implied subsetting beside its typing, recorded faithful against `Model.implicitBaseFeatures`; conformance pinned by `internal/workspace/model/base_feature_conformance_test.go` | `probes/triage/check-model.sysml` | none |
| 206 | `checkViewpointUsageViewpointSatisfactionSpecialization` | ViewpointUsage | sysml 8.3.26.9 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |
| 207 | `checkWhileLoopActionUsageSpecialization` | WhileLoopActionUsage | sysml 8.3.17.19 | not observable | n/a (no diagnostic) | (d) not-observable: the element only exists inside a behaviour body or expression, where the model-level read performs it instead of classifying it | — | none |
| 208 | `checkWhileLoopActionUsageSubactionSpecialization` | WhileLoopActionUsage | sysml 8.3.17.19 | not observable | n/a (no diagnostic) | (d) not-observable: the clause conditions the relationship on the owner or membership, and `istype` cannot tell the conditional library feature (`subparts`, `substates`, …) from the unconditional one of the same type | — | none |

Summary: 43 rows are (a) implemented-but-unmapped — observed through `istype` and now recorded
faithful; 165 rows are (d) not-observable through the tree's surfaces (35 binding connectors,
redefinitions and type featurings; 18 relationships with no library target; 64 owner- or
membership-conditioned relationships; 36 elements inside behaviour bodies or expressions; 12
behaviour usages and valueless features whose model-level read performs or has no value). No
row is (b) or (c): no `check*` rule was found absent. Two observations: the formal document's
OCL for `checkViewpointDefinitionSpecialization` and `checkViewpointUsageSpecialization` names
`Views::Viewpoint`, which the pinned library does not define — it defines `Views::ViewpointCheck`,
against which both answers are `true`; and the API JSON form's silence on implied relationships
is what keeps the 165 rows unobservable, so an export that writes them (as the pilot's does)
would make this table checkable mechanically.

## `derive*` rows (235)

A `derive*` constraint defines a derived property. This table records only whether the API
JSON form (`bin/sysml -convert api-json`) emits the property at all, over the two models of the
`check*` step (`probes/triage/check-model.sysml`, `.kerml`), one conversion each: **yes** when
every element of the row's metaclass carries the property, **partial** when some do or when the
model has no element of that exact metaclass but the property appears on another, **no** when
no element of the metaclass carries it (the *Where* column says whether the model had elements
of that metaclass at all — a row whose metaclass is absent from the models is `no` only in the
sense that the property name appears nowhere in the export). The Turtle form maps the same
properties through the same writer, so it is not tabulated separately. Nothing here is a
diagnostic, so the pilot column is omitted; the pilot's own JSON export is the referee of the
API-JSON derived-property differential in progress, and #1232 adds owned-absence and
evaluator-derived properties to the export — both are the in-flight overlap of every row.

| # | Constraint | Metaclass | Clause | Property | API JSON | Where | In-flight overlap |
|---|---|---|---|---|---|---|---|
| 1 | `deriveAcceptActionUsagePayloadArgument` | AcceptActionUsage | sysml 8.3.17.2 | `payloadArgument` | no | no `AcceptActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 2 | `deriveAcceptActionUsagePayloadParameter` | AcceptActionUsage | sysml 8.3.17.2 | `payloadParameter` | no | no `AcceptActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 3 | `deriveAcceptActionUsageReceiverArgument` | AcceptActionUsage | sysml 8.3.17.2 | `receiverArgument` | no | no `AcceptActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 4 | `deriveActionDefinitionAction` | ActionDefinition | sysml 8.3.17.3 | `action` | no | 0/1 `ActionDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 5 | `deriveAllocationDefinitionAllocation` | AllocationDefinition | sysml 8.3.15.2 | `allocation` | no | 0/1 `AllocationDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 6 | `deriveAnalysisCaseDefinitionResultExpression` | AnalysisCaseDefinition | sysml 8.3.23.2 | `resultExpression` | no | 0/1 `AnalysisCaseDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 7 | `deriveAnalysisCaseUsageResultExpression` | AnalysisCaseUsage | sysml 8.3.23.3 | `resultExpression` | no | 0/2 `AnalysisCaseUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 8 | `deriveAnnotatingElementAnnotatedElement` | AnnotatingElement | kerml 8.3.2.3.2 | `annotatedElement` | no | no `AnnotatingElement` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 9 | `deriveAnnotatingElementAnnotation` | AnnotatingElement | kerml 8.3.2.3.2 | `annotation` | no | no `AnnotatingElement` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 10 | `deriveAnnotatingElementOwnedAnnotatingRelationship` | AnnotatingElement | kerml 8.3.2.3.2 | `ownedAnnotatingRelationship` | no | no `AnnotatingElement` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 11 | `deriveAnnotationAnnotatingElement` | Annotation | kerml 8.3.2.3.3 | `annotatingElement` | no | no `Annotation` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 12 | `deriveAnnotationOwnedAnnotatingElement` | Annotation | kerml 8.3.2.3.3 | `ownedAnnotatingElement` | no | no `Annotation` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 13 | `deriveAssertConstraintUsageAssertedConstraint` | AssertConstraintUsage | sysml 8.3.20.2 | `assertedConstraint` | no | no `AssertConstraintUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 14 | `deriveAssignmentActionUsageReferent` | AssignmentActionUsage | sysml 8.3.17.5 | `referent` | no | no `AssignmentActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 15 | `deriveAssignmentActionUsageValueExpression` | AssignmentActionUsage | sysml 8.3.17.5 | `valueExpression` | no | no `AssignmentActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 16 | `deriveAssignmentUsageTargetArgument` | AssignmentActionUsage | sysml 8.3.17.5 | `assignmentUsageTargetArgument` | no | no `AssignmentActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 17 | `deriveAssociationRelatedType` | Association | kerml 8.3.4.4.2 | `relatedType` | no | no `Association` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 18 | `deriveAssociationSourceType` | Association | kerml 8.3.4.4.2 | `sourceType` | no | no `Association` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 19 | `deriveAssociationTargetType` | Association | kerml 8.3.4.4.2 | `targetType` | no | no `Association` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 20 | `deriveBehaviorStep` | Behavior | kerml 8.3.4.6.2 | `step` | no | no `Behavior` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 21 | `deriveCalculationUsageCalculation` | CalculationDefinition | sysml 8.3.19.2 | `calculationUsageCalculation` | no | 0/1 `CalculationDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 22 | `deriveCaseDefinitionActorParameter` | CaseDefinition | sysml 8.3.22.2 | `actorParameter` | no | 0/1 `CaseDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 23 | `deriveCaseDefinitionObjectiveRequirement` | CaseDefinition | sysml 8.3.22.2 | `objectiveRequirement` | no | 0/1 `CaseDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 24 | `deriveCaseDefinitionSubjectParameter` | CaseDefinition | sysml 8.3.22.2 | `subjectParameter` | no | 0/1 `CaseDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 25 | `deriveCaseUsageActorParameter` | CaseUsage | sysml 8.3.22.3 | `actorParameter` | no | 0/1 `CaseUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 26 | `deriveCaseUsageObjectiveRequirement` | CaseUsage | sysml 8.3.22.3 | `objectiveRequirement` | no | 0/1 `CaseUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 27 | `deriveCaseUsageSubjectParameter` | CaseUsage | sysml 8.3.22.3 | `subjectParameter` | no | 0/1 `CaseUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 28 | `deriveClassifierOwnedSubclassification` | Classifier | kerml 8.3.3.2.2 | `ownedSubclassification` | no | no `Classifier` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 29 | `deriveConjugatedPortTypingPortDefinition` | ConjugatedPortTyping | sysml 8.3.12.3 | `portDefinition` | no | no `ConjugatedPortTyping` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 30 | `deriveConnectorDefaultFeaturingType` | Connector | kerml 8.3.4.5.3 | `defaultFeaturingType` | no | 0/1 `Connector` elements | API-JSON derived-property differential (in progress); #1232 |
| 31 | `deriveConnectorRelatedFeature` | Connector | kerml 8.3.4.5.3 | `relatedFeature` | yes | 1/1 `Connector` elements | API-JSON derived-property differential (in progress); #1232 |
| 32 | `deriveConnectorSourceFeature` | Connector | kerml 8.3.4.5.3 | `sourceFeature` | yes | 1/1 `Connector` elements | API-JSON derived-property differential (in progress); #1232 |
| 33 | `deriveConnectorTargetFeature` | Connector | kerml 8.3.4.5.3 | `targetFeature` | yes | 1/1 `Connector` elements | API-JSON derived-property differential (in progress); #1232 |
| 34 | `deriveConstructorExpressionArgument` | ConstructorExpression | kerml 8.3.4.8.3 | `argument` | no | no `ConstructorExpression` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 35 | `deriveDefinitionDirectedUsage` | Definition | sysml 8.3.6.2 | `directedUsage` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 36 | `deriveDefinitionOwnedAction` | Definition | sysml 8.3.6.2 | `ownedAction` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 37 | `deriveDefinitionOwnedAllocation` | Definition | sysml 8.3.6.2 | `ownedAllocation` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 38 | `deriveDefinitionOwnedAnalysisCase` | Definition | sysml 8.3.6.2 | `ownedAnalysisCase` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 39 | `deriveDefinitionOwnedAttribute` | Definition | sysml 8.3.6.2 | `ownedAttribute` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 40 | `deriveDefinitionOwnedCalculation` | Definition | sysml 8.3.6.2 | `ownedCalculation` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 41 | `deriveDefinitionOwnedCase` | Definition | sysml 8.3.6.2 | `ownedCase` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 42 | `deriveDefinitionOwnedConcern` | Definition | sysml 8.3.6.2 | `ownedConcern` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 43 | `deriveDefinitionOwnedConnection` | Definition | sysml 8.3.6.2 | `ownedConnection` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 44 | `deriveDefinitionOwnedConstraint` | Definition | sysml 8.3.6.2 | `ownedConstraint` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 45 | `deriveDefinitionOwnedEnumeration` | Definition | sysml 8.3.6.2 | `ownedEnumeration` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 46 | `deriveDefinitionOwnedFlow` | Definition | sysml 8.3.6.2 | `ownedFlow` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 47 | `deriveDefinitionOwnedInterface` | Definition | sysml 8.3.6.2 | `ownedInterface` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 48 | `deriveDefinitionOwnedItem` | Definition | sysml 8.3.6.2 | `ownedItem` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 49 | `deriveDefinitionOwnedMetadata` | Definition | sysml 8.3.6.2 | `ownedMetadata` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 50 | `deriveDefinitionOwnedOccurrence` | Definition | sysml 8.3.6.2 | `ownedOccurrence` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 51 | `deriveDefinitionOwnedPart` | Definition | sysml 8.3.6.2 | `ownedPart` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 52 | `deriveDefinitionOwnedPort` | Definition | sysml 8.3.6.2 | `ownedPort` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 53 | `deriveDefinitionOwnedReference` | Definition | sysml 8.3.6.2 | `ownedReference` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 54 | `deriveDefinitionOwnedRendering` | Definition | sysml 8.3.6.2 | `ownedRendering` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 55 | `deriveDefinitionOwnedRequirement` | Definition | sysml 8.3.6.2 | `ownedRequirement` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 56 | `deriveDefinitionOwnedState` | Definition | sysml 8.3.6.2 | `ownedState` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 57 | `deriveDefinitionOwnedTransition` | Definition | sysml 8.3.6.2 | `ownedTransition` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 58 | `deriveDefinitionOwnedUsage` | Definition | sysml 8.3.6.2 | `ownedUsage` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 59 | `deriveDefinitionOwnedUseCase` | Definition | sysml 8.3.6.2 | `ownedUseCase` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 60 | `deriveDefinitionOwnedVerificationCase` | Definition | sysml 8.3.6.2 | `ownedVerificationCase` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 61 | `deriveDefinitionOwnedView` | Definition | sysml 8.3.6.2 | `ownedView` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 62 | `deriveDefinitionOwnedViewpoint` | Definition | sysml 8.3.6.2 | `ownedViewpoint` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 63 | `deriveDefinitionUsage` | Definition | sysml 8.3.6.2 | `usage` | no | no `Definition` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 64 | `deriveDefinitionVariant` | Definition | sysml 8.3.6.2 | `variant` | partial | no `Definition` element in the model; present on EnumerationDefinition | API-JSON derived-property differential (in progress); #1232 |
| 65 | `deriveDefinitionVariantMembership` | Definition | sysml 8.3.6.2 | `variantMembership` | partial | no `Definition` element in the model; present on EnumerationDefinition | API-JSON derived-property differential (in progress); #1232 |
| 66 | `deriveElementDocumentation` | Element | kerml 8.3.2.1.2 | `documentation` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 67 | `deriveElementIsLibraryElement` | Element | kerml 8.3.2.1.2 | `isLibraryElement` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 68 | `deriveElementName` | Element | kerml 8.3.2.1.2 | `name` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 69 | `deriveElementOwnedAnnotation` | Element | kerml 8.3.2.1.2 | `ownedAnnotation` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 70 | `deriveElementOwnedElement` | Element | kerml 8.3.2.1.2 | `ownedElement` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 71 | `deriveElementOwner` | Element | kerml 8.3.2.1.2 | `owner` | partial | no `Element` element in the model; present on ActionDefinition, ActionUsage, AllocationDefinition, AllocationUsage | API-JSON derived-property differential (in progress); #1232 |
| 72 | `deriveElementQualifiedName` | Element | kerml 8.3.2.1.2 | `qualifiedName` | partial | no `Element` element in the model; present on ActionDefinition, ActionUsage, AllocationDefinition, AllocationUsage | API-JSON derived-property differential (in progress); #1232 |
| 73 | `deriveElementShortName` | Element | kerml 8.3.2.1.2 | `shortName` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 74 | `deriveElementTextualRepresentation` | Element | kerml 8.3.2.1.2 | `textualRepresentation` | no | no `Element` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 75 | `deriveEventOccurrenceUsageEventOccurrence` | EventOccurrenceUsage | sysml 8.3.9.2 | `eventOccurrence` | no | no `EventOccurrenceUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 76 | `deriveExpressionIsModelLevelEvaluable` | Expression | kerml 8.3.4.7.3 | `isModelLevelEvaluable` | no | 0/2 `Expression` elements | API-JSON derived-property differential (in progress); #1232 |
| 77 | `deriveExpressionResult` | Expression | kerml 8.3.4.7.3 | `result` | no | 0/2 `Expression` elements | API-JSON derived-property differential (in progress); #1232 |
| 78 | `deriveFeatureChainExpressionTargetFeature` | FeatureChainExpression | kerml 8.3.4.8.4 | `targetFeature` | partial | no `FeatureChainExpression` element in the model; present on AllocationUsage, ConnectionUsage, Connector, InterfaceUsage | API-JSON derived-property differential (in progress); #1232 |
| 79 | `deriveFeatureChainingFeature` | Feature | kerml 8.3.3.3.4 | `chainingFeature` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 80 | `deriveFeatureCrossFeature` | Feature | kerml 8.3.3.3.4 | `crossFeature` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 81 | `deriveFeatureFeatureTarget` | Feature | kerml 8.3.3.3.4 | `featureTarget` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 82 | `deriveFeatureFeaturingType` | Feature | kerml 8.3.3.3.4 | `featuringType` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 83 | `deriveFeatureOwnedCrossSubsetting` | Feature | kerml 8.3.3.3.4 | `ownedCrossSubsetting` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 84 | `deriveFeatureOwnedFeatureChaining` | Feature | kerml 8.3.3.3.4 | `ownedFeatureChaining` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 85 | `deriveFeatureOwnedFeatureInverting` | Feature | kerml 8.3.3.3.4 | `ownedFeatureInverting` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 86 | `deriveFeatureOwnedRedefinition` | Feature | kerml 8.3.3.3.4 | `ownedRedefinition` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 87 | `deriveFeatureOwnedReferenceSubsetting` | Feature | kerml 8.3.3.3.4 | `ownedReferenceSubsetting` | partial | no `Feature` element in the model; present on PortUsage, ReferenceUsage | API-JSON derived-property differential (in progress); #1232 |
| 88 | `deriveFeatureOwnedSubsetting` | Feature | kerml 8.3.3.3.4 | `ownedSubsetting` | partial | no `Feature` element in the model; present on PortUsage, ReferenceUsage | API-JSON derived-property differential (in progress); #1232 |
| 89 | `deriveFeatureOwnedTypeFeaturing` | Feature | kerml 8.3.3.3.4 | `ownedTypeFeaturing` | no | no `Feature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 90 | `deriveFeatureOwnedTyping` | Feature | kerml 8.3.3.3.4 | `ownedTyping` | partial | no `Feature` element in the model; present on AttributeUsage, Connector, PartUsage, PortUsage | API-JSON derived-property differential (in progress); #1232 |
| 91 | `deriveFeatureReferenceExpressionReferent` | FeatureReferenceExpression | kerml 8.3.4.8.5 | `referent` | no | no `FeatureReferenceExpression` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 92 | `deriveFeatureType` | Feature | kerml 8.3.3.3.4 | `type` | partial | no `Feature` element in the model; present on AttributeUsage, Connector, FeatureTyping, PartUsage | API-JSON derived-property differential (in progress); #1232 |
| 93 | `deriveFlowFlowEnd` | Flow | kerml 8.3.4.9.2 | `flowEnd` | no | no `Flow` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 94 | `deriveFlowPayloadFeature` | Flow | kerml 8.3.4.9.2 | `payloadFeature` | no | no `Flow` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 95 | `deriveFlowPayloadType` | Flow | kerml 8.3.4.9.2 | `payloadType` | no | no `Flow` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 96 | `deriveFlowSourceOutputFeature` | Flow | kerml 8.3.4.9.2 | `sourceOutputFeature` | no | no `Flow` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 97 | `deriveFlowTargetInputFeature` | Flow | kerml 8.3.4.9.2 | `targetInputFeature` | no | no `Flow` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 98 | `deriveForLoopActionUsageLoopVariable` | ForLoopActionUsage | sysml 8.3.17.9 | `loopVariable` | no | no `ForLoopActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 99 | `deriveForLoopActionUsageSeqArgument` | ForLoopActionUsage | sysml 8.3.17.9 | `seqArgument` | no | no `ForLoopActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 100 | `deriveFunctionResult` | Function | kerml 8.3.4.7.4 | `result` | no | 0/1 `Function` elements | API-JSON derived-property differential (in progress); #1232 |
| 101 | `deriveIfActionUsageElseAction` | IfActionUsage | sysml 8.3.17.10 | `elseAction` | no | no `IfActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 102 | `deriveIfActionUsageIfArgument` | IfActionUsage | sysml 8.3.17.10 | `ifArgument` | no | no `IfActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 103 | `deriveIfActionUsageThenAction` | IfActionUsage | sysml 8.3.17.10 | `thenAction` | no | no `IfActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 104 | `deriveInstantiationExpressionInstantiatedType` | InstantiationExpression | kerml 8.3.4.8.7 | `instantiatedType` | no | no `InstantiationExpression` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 105 | `deriveInvocationExpressionArgument` | InvocationExpression | kerml 8.3.4.8.8 | `argument` | no | no `InvocationExpression` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 106 | `deriveItemUsageItemDefinition` | ItemUsage | sysml 8.3.10.3 | `itemDefinition` | no | 0/2 `ItemUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 107 | `deriveLiteralExpressionIsModelLevelEvaluable` | LiteralExpression | kerml 8.3.4.8.10 | `isModelLevelEvaluable` | no | no `LiteralExpression` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 108 | `deriveLoopActionUsageBodyAction` | LoopActionUsage | sysml 8.3.17.12 | `bodyAction` | no | no `LoopActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 109 | `deriveMembershipImportImportedElement` | MembershipImport | kerml 8.3.2.4.4 | `importedElement` | no | no `MembershipImport` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 110 | `deriveMembershipMemberElementId` | Membership | kerml 8.3.2.4.3 | `memberElementId` | no | no `Membership` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 111 | `deriveMetadataAccessExpressionReferencdElement` | MetadataAccessExpression | kerml 8.3.4.8.15 | `referencdElement` | no | no `MetadataAccessExpression` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 112 | `deriveMetadataFeatureMetaclass` | MetadataFeature | kerml 8.3.4.12.3 | `metaclass` | no | no `MetadataFeature` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 113 | `deriveMultiplicityRangeBound` | MultiplicityRange | kerml 8.3.4.11.2 | `bound` | no | no `MultiplicityRange` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 114 | `deriveMultiplicityRangeLowerBound` | MultiplicityRange | kerml 8.3.4.11.2 | `lowerBound` | no | no `MultiplicityRange` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 115 | `deriveMultiplicityRangeUpperBound` | MultiplicityRange | kerml 8.3.4.11.2 | `upperBound` | no | no `MultiplicityRange` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 116 | `deriveNamespaceImportImportedElement` | NamespaceImport | kerml 8.3.2.4.6 | `importedElement` | no | no `NamespaceImport` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 117 | `deriveNamespaceImportedMembership` | Namespace | kerml 8.3.2.4.5 | `importedMembership` | no | 0/2 `Namespace` elements | API-JSON derived-property differential (in progress); #1232 |
| 118 | `deriveNamespaceMembers` | Namespace | kerml 8.3.2.4.5 | `members` | no | 0/2 `Namespace` elements | API-JSON derived-property differential (in progress); #1232 |
| 119 | `deriveNamespaceOwnedImport` | Namespace | kerml 8.3.2.4.5 | `ownedImport` | no | 0/2 `Namespace` elements | API-JSON derived-property differential (in progress); #1232 |
| 120 | `deriveNamespaceOwnedMember` | Namespace | kerml 8.3.2.4.5 | `ownedMember` | yes | 2/2 `Namespace` elements | API-JSON derived-property differential (in progress); #1232 |
| 121 | `deriveNamespaceOwnedMembership` | Namespace | kerml 8.3.2.4.5 | `ownedMembership` | yes | 2/2 `Namespace` elements | API-JSON derived-property differential (in progress); #1232 |
| 122 | `deriveOccurrenceUsageIndividualDefinition` | OccurrenceUsage | sysml 8.3.9.4 | `individualDefinition` | no | 0/4 `OccurrenceUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 123 | `deriveOwningMembershipOwnedMemberName` | OwningMembership | kerml 8.3.2.4.8 | `ownedMemberName` | no | 0/72 `OwningMembership` elements | API-JSON derived-property differential (in progress); #1232 |
| 124 | `deriveOwningMembershipOwnedMemberShortName` | OwningMembership | kerml 8.3.2.4.8 | `ownedMemberShortName` | no | 0/72 `OwningMembership` elements | API-JSON derived-property differential (in progress); #1232 |
| 125 | `deriveOwningNamespace` | Element | kerml 8.3.2.1.2 | `owningNamespace` | partial | no `Element` element in the model; present on ActionDefinition, ActionUsage, AllocationDefinition, AllocationUsage | API-JSON derived-property differential (in progress); #1232 |
| 126 | `derivePackageFilterCondition` | Package | kerml 8.3.4.13.4 | `filterCondition` | no | 0/2 `Package` elements | API-JSON derived-property differential (in progress); #1232 |
| 127 | `derivePartUsagePartDefinition` | PartUsage | sysml 8.3.11.3 | `partDefinition` | no | 0/4 `PartUsage` elements; derived reflectively (`Model.definitionTypedFeatures`, over `FeatureTypeSetWithImplied`): `part p : I;` has `partDefinition` = [Parts::Part], as the pilot's export states | API-JSON derived-property differential (in progress); #1232 (merged) |
| 128 | `derivePortDefinitionConjugatedPortDefinition` | PortDefinition | sysml 8.3.12.5 | `conjugatedPortDefinition` | yes | 1/1 `PortDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 129 | `deriveRelationshipRelatedElement` | Relationship | kerml 8.3.2.1.3 | `relatedElement` | partial | no `Relationship` element in the model; present on FeatureTyping, PortConjugation, ReferenceSubsetting | API-JSON derived-property differential (in progress); #1232 |
| 130 | `deriveRenderingDefinitionRendering` | RenderingDefinition | sysml 8.3.26.5 | `rendering` | no | 0/1 `RenderingDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 131 | `deriveRequirementConstraintMembershipReferencedConstraint` | RequirementConstraintMembership | sysml 8.3.21.7 | `referencedConstraint` | no | no `RequirementConstraintMembership` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 132 | `deriveRequirementDefinitionActorParameter` | RequirementDefinition | sysml 8.3.21.8 | `actorParameter` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 133 | `deriveRequirementDefinitionAssumedConstraint` | RequirementDefinition | sysml 8.3.21.8 | `assumedConstraint` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 134 | `deriveRequirementDefinitionFramedConcern` | RequirementDefinition | sysml 8.3.21.8 | `framedConcern` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 135 | `deriveRequirementDefinitionRequiredConstraint` | RequirementDefinition | sysml 8.3.21.8 | `requiredConstraint` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 136 | `deriveRequirementDefinitionStakeholderParameter` | RequirementDefinition | sysml 8.3.21.8 | `stakeholderParameter` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 137 | `deriveRequirementDefinitionSubjectParameter` | RequirementDefinition | sysml 8.3.21.8 | `subjectParameter` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 138 | `deriveRequirementDefinitionText` | RequirementDefinition | sysml 8.3.21.8 | `text` | no | 0/1 `RequirementDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 139 | `deriveRequirementUsageActorParameter` | RequirementUsage | sysml 8.3.21.9 | `actorParameter` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 140 | `deriveRequirementUsageAssumedConstraint` | RequirementUsage | sysml 8.3.21.9 | `assumedConstraint` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 141 | `deriveRequirementUsageFramedConcern` | RequirementUsage | sysml 8.3.21.9 | `framedConcern` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 142 | `deriveRequirementUsageRequiredConstraint` | RequirementUsage | sysml 8.3.21.9 | `requiredConstraint` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 143 | `deriveRequirementUsageStakeholderParameter` | RequirementUsage | sysml 8.3.21.9 | `stakeholderParameter` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 144 | `deriveRequirementUsageSubjectParameter` | RequirementUsage | sysml 8.3.21.9 | `subjectParameter` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 145 | `deriveRequirementUsageText` | RequirementUsage | sysml 8.3.21.9 | `text` | no | 0/2 `RequirementUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 146 | `deriveSatisfyRequirementUsageSatisfyingFeature` | SatisfyRequirementUsage | sysml 8.3.21.10 | `satisfyingFeature` | no | no `SatisfyRequirementUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 147 | `deriveSendActionUsagePayloadArgument` | SendActionUsage | sysml 8.3.17.15 | `payloadArgument` | no | no `SendActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 148 | `deriveSendActionUsageReceiverArgument` | SendActionUsage | sysml 8.3.17.15 | `receiverArgument` | no | no `SendActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 149 | `deriveSendActionUsageSenderArgument` | SendActionUsage | sysml 8.3.17.15 | `senderArgument` | no | no `SendActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 150 | `deriveStateDefinitionDoAction` | StateDefinition | sysml 8.3.18.5 | `doAction` | no | 0/1 `StateDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 151 | `deriveStateDefinitionEntryAction` | StateDefinition | sysml 8.3.18.5 | `entryAction` | no | 0/1 `StateDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 152 | `deriveStateDefinitionExitAction` | StateDefinition | sysml 8.3.18.5 | `exitAction` | no | 0/1 `StateDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 153 | `deriveStateDefinitionState` | StateDefinition | sysml 8.3.18.5 | `state` | no | 0/1 `StateDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 154 | `deriveStateUsageDoAction` | StateUsage | sysml 8.3.18.6 | `doAction` | no | 0/2 `StateUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 155 | `deriveStateUsageEntryAction` | StateUsage | sysml 8.3.18.6 | `entryAction` | no | 0/2 `StateUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 156 | `deriveStateUsageExitAction` | StateUsage | sysml 8.3.18.6 | `exitAction` | no | 0/2 `StateUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 157 | `deriveStepBehavior` | Step | kerml 8.3.4.6.3 | `behavior` | no | 0/2 `Step` elements | API-JSON derived-property differential (in progress); #1232 |
| 158 | `deriveTerminateActionUsageTerminatedOccurrenceArgument` | TerminateActionUsage | sysml 8.3.17.16 | `terminatedOccurrenceArgument` | no | no `TerminateActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 159 | `deriveTransitionUsageEffectAction` | TransitionUsage | sysml 8.3.18.9 | `effectAction` | no | no `TransitionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 160 | `deriveTransitionUsageGuardExpression` | TransitionUsage | sysml 8.3.18.9 | `guardExpression` | no | no `TransitionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 161 | `deriveTransitionUsageSource` | TransitionUsage | sysml 8.3.18.9 | `source` | partial | no `TransitionUsage` element in the model; present on FeatureTyping, PortConjugation, ReferenceSubsetting | API-JSON derived-property differential (in progress); #1232 |
| 162 | `deriveTransitionUsageSuccession` | TransitionUsage | sysml 8.3.18.9 | `succession` | no | no `TransitionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 163 | `deriveTransitionUsageTarget` | TransitionUsage | sysml 8.3.18.9 | `target` | partial | no `TransitionUsage` element in the model; present on FeatureTyping, PortConjugation, ReferenceSubsetting | API-JSON derived-property differential (in progress); #1232 |
| 164 | `deriveTransitionUsageTriggerAction` | TransitionUsage | sysml 8.3.18.9 | `triggerAction` | no | no `TransitionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 165 | `deriveTypeDifferencingType` | Type | kerml 8.3.3.1.10 | `differencingType` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 166 | `deriveTypeDirectedFeature` | Type | kerml 8.3.3.1.10 | `directedFeature` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 167 | `deriveTypeEndFeature` | Type | kerml 8.3.3.1.10 | `endFeature` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 168 | `deriveTypeFeature` | Type | kerml 8.3.3.1.10 | `feature` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 169 | `deriveTypeFeatureMembership` | Type | kerml 8.3.3.1.10 | `featureMembership` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 170 | `deriveTypeInheritedFeature` | Type | kerml 8.3.3.1.10 | `inheritedFeature` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 171 | `deriveTypeInheritedMembership` | Type | kerml 8.3.3.1.10 | `inheritedMembership` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 172 | `deriveTypeInput` | Type | kerml 8.3.3.1.10 | `input` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 173 | `deriveTypeIntersectingType` | Type | kerml 8.3.3.1.10 | `intersectingType` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 174 | `deriveTypeMultiplicity` | Type | kerml 8.3.3.1.10 | `multiplicity` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 175 | `deriveTypeOutput` | Type | kerml 8.3.3.1.10 | `output` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 176 | `deriveTypeOwnedConjugator` | Type | kerml 8.3.3.1.10 | `ownedConjugator` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 177 | `deriveTypeOwnedDifferencing` | Type | kerml 8.3.3.1.10 | `ownedDifferencing` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 178 | `deriveTypeOwnedDisjoining` | Type | kerml 8.3.3.1.10 | `ownedDisjoining` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 179 | `deriveTypeOwnedEndFeature` | Type | kerml 8.3.3.1.10 | `ownedEndFeature` | partial | no `Type` element in the model; present on AllocationUsage, ConnectionUsage, Connector, InterfaceUsage | API-JSON derived-property differential (in progress); #1232 |
| 180 | `deriveTypeOwnedFeature` | Type | kerml 8.3.3.1.10 | `ownedFeature` | partial | no `Type` element in the model; present on AllocationDefinition, AllocationUsage, AssociationUsage, ClassUsage | API-JSON derived-property differential (in progress); #1232 |
| 181 | `deriveTypeOwnedFeatureMembership` | Type | kerml 8.3.3.1.10 | `ownedFeatureMembership` | partial | no `Type` element in the model; present on AllocationDefinition, AllocationUsage, AssociationUsage, ClassUsage | API-JSON derived-property differential (in progress); #1232 |
| 182 | `deriveTypeOwnedIntersecting` | Type | kerml 8.3.3.1.10 | `ownedIntersecting` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 183 | `deriveTypeOwnedSpecialization` | Type | kerml 8.3.3.1.10 | `ownedSpecialization` | partial | no `Type` element in the model; present on AttributeUsage, Connector, PartUsage, PortUsage | API-JSON derived-property differential (in progress); #1232 |
| 184 | `deriveTypeOwnedUnioning` | Type | kerml 8.3.3.1.10 | `ownedUnioning` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 185 | `deriveTypeUnioningType` | Type | kerml 8.3.3.1.10 | `unioningType` | no | no `Type` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 186 | `deriveUsageDirectedUsage` | Usage | sysml 8.3.6.4 | `directedUsage` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 187 | `deriveUsageIsReference` | Usage | sysml 8.3.6.4 | `isReference` | partial | no `Usage` element in the model; present on ReferenceUsage | API-JSON derived-property differential (in progress); #1232 |
| 188 | `deriveUsageMayTimeVary` | Usage | sysml 8.3.6.4 | `mayTimeVary` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 189 | `deriveUsageNestedAction` | Usage | sysml 8.3.6.4 | `nestedAction` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 190 | `deriveUsageNestedAllocation` | Usage | sysml 8.3.6.4 | `nestedAllocation` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 191 | `deriveUsageNestedAnalysisCase` | Usage | sysml 8.3.6.4 | `nestedAnalysisCase` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 192 | `deriveUsageNestedAttribute` | Usage | sysml 8.3.6.4 | `nestedAttribute` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 193 | `deriveUsageNestedCalculation` | Usage | sysml 8.3.6.4 | `nestedCalculation` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 194 | `deriveUsageNestedCase` | Usage | sysml 8.3.6.4 | `nestedCase` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 195 | `deriveUsageNestedConcern` | Usage | sysml 8.3.6.4 | `nestedConcern` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 196 | `deriveUsageNestedConnection` | Usage | sysml 8.3.6.4 | `nestedConnection` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 197 | `deriveUsageNestedConstraint` | Usage | sysml 8.3.6.4 | `nestedConstraint` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 198 | `deriveUsageNestedEnumeration` | Usage | sysml 8.3.6.4 | `nestedEnumeration` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 199 | `deriveUsageNestedFlow` | Usage | sysml 8.3.6.4 | `nestedFlow` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 200 | `deriveUsageNestedInterface` | Usage | sysml 8.3.6.4 | `nestedInterface` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 201 | `deriveUsageNestedItem` | Usage | sysml 8.3.6.4 | `nestedItem` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 202 | `deriveUsageNestedMetadata` | Usage | sysml 8.3.6.4 | `nestedMetadata` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 203 | `deriveUsageNestedOccurrence` | Usage | sysml 8.3.6.4 | `nestedOccurrence` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 204 | `deriveUsageNestedPart` | Usage | sysml 8.3.6.4 | `nestedPart` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 205 | `deriveUsageNestedPort` | Usage | sysml 8.3.6.4 | `nestedPort` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 206 | `deriveUsageNestedReference` | Usage | sysml 8.3.6.4 | `nestedReference` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 207 | `deriveUsageNestedRendering` | Usage | sysml 8.3.6.4 | `nestedRendering` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 208 | `deriveUsageNestedRequirement` | Usage | sysml 8.3.6.4 | `nestedRequirement` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 209 | `deriveUsageNestedState` | Usage | sysml 8.3.6.4 | `nestedState` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 210 | `deriveUsageNestedTransition` | Usage | sysml 8.3.6.4 | `nestedTransition` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 211 | `deriveUsageNestedUsage` | Usage | sysml 8.3.6.4 | `nestedUsage` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 212 | `deriveUsageNestedUseCase` | Usage | sysml 8.3.6.4 | `nestedUseCase` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 213 | `deriveUsageNestedVerificationCase` | Usage | sysml 8.3.6.4 | `nestedVerificationCase` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 214 | `deriveUsageNestedView` | Usage | sysml 8.3.6.4 | `nestedView` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 215 | `deriveUsageNestedViewpoint` | Usage | sysml 8.3.6.4 | `nestedViewpoint` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 216 | `deriveUsageUsage` | Usage | sysml 8.3.6.4 | `usage` | no | no `Usage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 217 | `deriveUsageVariant` | Usage | sysml 8.3.6.4 | `variant` | partial | no `Usage` element in the model; present on EnumerationDefinition | API-JSON derived-property differential (in progress); #1232 |
| 218 | `deriveUsageVariantMembership` | Usage | sysml 8.3.6.4 | `variantMembership` | partial | no `Usage` element in the model; present on EnumerationDefinition | API-JSON derived-property differential (in progress); #1232 |
| 219 | `deriveUseCaseDefinitionIncludedUseCase` | UseCaseDefinition | sysml 8.3.25.3 | `includedUseCase` | no | 0/1 `UseCaseDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 220 | `deriveUseCaseUsageIncludedUseCase` | UseCaseUsage | sysml 8.3.25.4 | `includedUseCase` | no | 0/1 `UseCaseUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 221 | `deriveVerificationCaseDefinitionVerifiedRequirement` | VerificationCaseDefinition | sysml 8.3.24.3 | `verifiedRequirement` | no | 0/1 `VerificationCaseDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 222 | `deriveVerificationCaseUsageVerifiedRequirement` | VerificationCaseUsage | sysml 8.3.24.4 | `verifiedRequirement` | no | 0/2 `VerificationCaseUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 223 | `deriveVewRenderingMembershipReferencedRendering` | ViewRenderingMembership | sysml 8.3.26.10 | `vewRenderingMembershipReferencedRendering` | no | no `ViewRenderingMembership` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 224 | `deriveViewDefinitionSatisfiedViewpoint` | ViewDefinition | sysml 8.3.26.7 | `satisfiedViewpoint` | no | 0/1 `ViewDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 225 | `deriveViewDefinitionView` | ViewDefinition | sysml 8.3.26.7 | `view` | no | 0/1 `ViewDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 226 | `deriveViewDefinitionViewCondition` | ViewDefinition | sysml 8.3.26.7 | `viewCondition` | no | 0/1 `ViewDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 227 | `deriveViewDefinitionViewRendering` | ViewDefinition | sysml 8.3.26.7 | `viewRendering` | no | 0/1 `ViewDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 228 | `deriveViewUsageExposedElement` | ViewUsage | sysml 8.3.26.11 | `exposedElement` | no | 0/1 `ViewUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 229 | `deriveViewUsageSatisfiedViewpoint` | ViewUsage | sysml 8.3.26.11 | `satisfiedViewpoint` | no | 0/1 `ViewUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 230 | `deriveViewUsageViewCondition` | ViewUsage | sysml 8.3.26.11 | `viewCondition` | no | 0/1 `ViewUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 231 | `deriveViewUsageViewRendering` | ViewUsage | sysml 8.3.26.11 | `viewRendering` | no | 0/1 `ViewUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 232 | `deriveViewpointDefinitionViewpointStakeholder` | ViewpointDefinition | sysml 8.3.26.8 | `viewpointStakeholder` | no | 0/1 `ViewpointDefinition` elements | API-JSON derived-property differential (in progress); #1232 |
| 233 | `deriveViewpointUsageViewpointStakeholder` | ViewpointUsage | sysml 8.3.26.9 | `viewpointStakeholder` | no | 0/2 `ViewpointUsage` elements | API-JSON derived-property differential (in progress); #1232 |
| 234 | `deriveWhileLoopActionUsageUntilArgument` | WhileLoopActionUsage | sysml 8.3.17.19 | `untilArgument` | no | no `WhileLoopActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |
| 235 | `deriveWhileLoopActionUsageWhileArgument` | WhileLoopActionUsage | sysml 8.3.17.19 | `whileArgument` | no | no `WhileLoopActionUsage` element in the model; absent everywhere | API-JSON derived-property differential (in progress); #1232 |

Summary: 6 yes, 20 partial, 209 no. The export writes ownership, membership, typing and the
declared attributes an element states, and omits most derived properties; in particular it
omits `isAbstract` on a variation definition, `isSufficient` on a connection definition, and
every implied relationship, and it exports an `objective` as `isComposite = false`. These are
export observations for the differential, not validation gaps.

## Constraint names only the Beta 2 PDFs state (128)

A raw token scan of the two Beta 2 PDFs finds 128 `check*`/`derive*`/`validate*` names the
manifest does not record. Each is classified against the manifest's names and formal names:

| Class | Count | Meaning |
|---|---|---|
| truncated reference | 81 | the prose cites a constraint across a line break, so the token is a prefix of a recorded name |
| misspelling or variant | 34 | within edit distance of a recorded name (`checkFeatureParameterRefinition`, `validateOccurranceUsageIndividualDefinition`, `checkPartUsageSupartSpecialization`, …) |
| kind-swapped reference | 1 | the prose names a `validate*` rule as `check*` or the reverse |
| stated and recorded | 3 | already a manifest name or formal name |
| unmatched | 9 | no recorded name within reach: `checkCondition`, `checkConstraints`, `checkLevel` are prose tokens, not constraints; `checkFeatureEndIsConstaint` is a misspelled, kind-swapped reference to `validateFeatureEndIsConstant`; `checkActionUsageAnalysisAction`, `checkConnectorEndRedefinition`, `checkFeatureEndRedefinitionSpecialization`, `checkFeatureValueExpressionTypeFeaturing` and `checkVerificationCaseUsageSubAnalysisCaseSpecialization` are Beta 2 names for implied relationships the formal documents state under another name (`checkActionUsageOwnedActionSpecialization`, `checkFeatureEndRedefinition`, `checkFeatureOwnedCrossFeatureRedefinitionSpecialization`, `checkExpressionTypeFeaturing`, `checkVerificationCaseUsageSubVerificationCaseSpecialization`); all are `check*` rows whose observability is the one recorded for the formal name above |

No name in the list is a `validate*` rule the pilot genuinely lacks: every `validate*` token is a
misspelling or variant of a recorded name. The full mapping:

| Token | Class | Recorded name |
|---|---|---|
| `checkAcceptActionUsage` | truncated reference | `checkAcceptActionUsageSpecialization (+3 more)` |
| `checkAcceptActionUsageSubaction` | truncated reference | `checkAcceptActionUsageSubactionSpecialization` |
| `checkAcceptActionUsageTriggerAction` | truncated reference | `checkAcceptActionUsageTriggerActionSpecialization` |
| `checkActionUsageAnalysisAction` | unmatched | `checkActionUsageSpecialization` |
| `checkActionUsageAnalysisActionSpecialization` | misspelling or variant | `checkActionUsageOwnedActionSpecialization` |
| `checkActionUsageStateAction` | truncated reference | `checkActionUsageStateActionRedefinition` |
| `checkAnalysisCaseUsageSubAnalysisCase` | truncated reference | `checkAnalysisCaseUsageSubAnalysisCaseSpecialization` |
| `checkAssignmentActionReferentRedefinition` | misspelling or variant | `checkAssignmentActionUsageReferentRedefinition` |
| `checkAssignmentActionUsage` | truncated reference | `checkAssignmentActionUsageSpecialization (+4 more)` |
| `checkAssignmentActionUsageSubaction` | misspelling or variant | `checkAssignmentActionUsageSpecialization` |
| `checkAssociation` | truncated reference | `checkAssociationSpecialization (+3 more)` |
| `checkAssociationBinary` | truncated reference | `checkAssociationBinarySpecialization` |
| `checkAssociationStructure` | truncated reference | `checkAssociationStructureSpecialization (+1 more)` |
| `checkAttributeUsageDataTypeSpecialization` | misspelling or variant | `checkAttributeUsageSpecialization` |
| `checkBindingConnector` | truncated reference | `checkBindingConnectorSpecialization` |
| `checkBooleanExpression` | truncated reference | `checkBooleanExpressionSpecialization` |
| `checkCalculationUsageSubcalculation` | misspelling or variant | `checkCalculationUsageSpecialization` |
| `checkClassifierSpecialization` | misspelling or variant | `checkClassSpecialization` |
| `checkConcernUsageFramedConcern` | truncated reference | `checkConcernUsageFramedConcernSpecialization` |
| `checkCondition` | unmatched | — |
| `checkConnectionDefinitionSpecialization` | misspelling or variant | `checkConnectionDefinitionSpecializations` |
| `checkConnectorBinary` | truncated reference | `checkConnectorBinarySpecialization (+1 more)` |
| `checkConnectorBinaryObject` | truncated reference | `checkConnectorBinaryObjectSpecialization` |
| `checkConnectorEndRedefinition` | unmatched | `checkFeatureEndRedefinition` |
| `checkConnectorObject` | truncated reference | `checkConnectorObjectSpecialization` |
| `checkConstraintUsageCheckedConstraint` | truncated reference | `checkConstraintUsageCheckedConstraintSpecialization` |
| `checkConstraintUsageRequirementConstraint` | misspelling or variant | `checkConstraintUsageRequirementConstraintSpecialization` |
| `checkConstraints` | unmatched | — |
| `checkConstructorExpression` | truncated reference | `checkConstructorExpressionSpecialization (+3 more)` |
| `checkConstructorExpressionResultFeatureRedefnition` | misspelling or variant | `checkConstructorExpressionResultFeatureRedefinition` |
| `checkDecisionNodeOutgoingSuccession` | truncated reference | `checkDecisionNodeOutgoingSuccessionSpecialization` |
| `checkExpression` | truncated reference | `checkExpressionTypeFeaturing (+2 more)` |
| `checkExpressionFeaturingType` | misspelling or variant | `checkExpressionTypeFeaturing` |
| `checkExpressionResult` | truncated reference | `checkExpressionResultBindingConnector` |
| `checkFeatureChainExpression` | truncated reference | `checkFeatureChainExpressionTargetRedefinition (+2 more)` |
| `checkFeatureCrossing` | truncated reference | `checkFeatureCrossingSpecialization` |
| `checkFeatureDataValue` | truncated reference | `checkFeatureDataValueSpecialization` |
| `checkFeatureEnd` | truncated reference | `checkFeatureEndRedefinition (+1 more)` |
| `checkFeatureEndIsConstaint` | unmatched | `validateFeatureEndIsConstant` |
| `checkFeatureEndIsConstant` | kind-swapped reference | `validateFeatureEndIsConstant` |
| `checkFeatureEndRedefinitionSpecialization` | unmatched | `checkFeatureOwnedCrossFeatureRedefinitionSpecialization` |
| `checkFeatureFeatureMembership` | truncated reference | `checkFeatureFeatureMembershipTypeFeaturing` |
| `checkFeatureFlowFeature` | truncated reference | `checkFeatureFlowFeatureRedefinition` |
| `checkFeatureObject` | truncated reference | `checkFeatureObjectSpecialization` |
| `checkFeatureOccurrence` | truncated reference | `checkFeatureOccurrenceSpecialization` |
| `checkFeatureOwnedCrossFeature` | truncated reference | `checkFeatureOwnedCrossFeatureTypeFeaturing (+2 more)` |
| `checkFeatureParameter` | truncated reference | `checkFeatureParameterRedefinition` |
| `checkFeatureParameterRefinition` | misspelling or variant | `checkFeatureParameterRedefinition` |
| `checkFeatureReference` | truncated reference | `checkFeatureReferenceExpressionBindingConnector (+1 more)` |
| `checkFeatureResult` | truncated reference | `checkFeatureResultRedefinition` |
| `checkFeatureSubobject` | truncated reference | `checkFeatureSubobjectSpecialization` |
| `checkFeatureSuboccurrence` | truncated reference | `checkFeatureSuboccurrenceSpecialization` |
| `checkFeatureValuation` | truncated reference | `checkFeatureValuationSpecialization` |
| `checkFeatureValue` | truncated reference | `checkFeatureValueBindingConnector` |
| `checkFeatureValueExpressionTypeFeaturing` | unmatched | `checkExpressionTypeFeaturing` |
| `checkFlowBinarySpecialization` | misspelling or variant | `checkFlowSpecialization` |
| `checkFlowUsageFlowSpecification` | misspelling or variant | `checkFlowUsageFlowSpecialization` |
| `checkFlowUsageSpecification` | misspelling or variant | `checkFlowUsageSpecialization` |
| `checkForLoopActionUsageSubaction` | truncated reference | `checkForLoopActionUsageSubactionSpecialization` |
| `checkFunctionResult` | truncated reference | `checkFunctionResultBindingConnector` |
| `checkIncludeUseCaseUsageSpecialization` | misspelling or variant | `checkIncludeUseCaseSpecialization` |
| `checkIndexExpressionResult` | truncated reference | `checkIndexExpressionResultSpecialization` |
| `checkInvocationExpression` | truncated reference | `checkInvocationExpressionSpecialization (+3 more)` |
| `checkInvocationExpressionResultSpecialization` | misspelling or variant | `checkInvocationExpressionSpecialization` |
| `checkLevel` | unmatched | — |
| `checkLiteralBoolean` | truncated reference | `checkLiteralBooleanSpecialization` |
| `checkLiteralExpression` | truncated reference | `checkLiteralExpressionSpecialization` |
| `checkLiteralInfinity` | truncated reference | `checkLiteralInfinitySpecialization` |
| `checkLiteralInteger` | truncated reference | `checkLiteralIntegerSpecialization` |
| `checkLiteralRational` | truncated reference | `checkLiteralRationalSpecialization` |
| `checkLiteralString` | truncated reference | `checkLiteralStringSpecialization` |
| `checkMergeNodeIncomingSuccession` | truncated reference | `checkMergeNodeIncomingSuccessionSpecialization` |
| `checkMetadataAccess` | truncated reference | `checkMetadataAccessExpressionSpecialization` |
| `checkMetadataFeature` | truncated reference | `checkMetadataFeatureSpecialization (+1 more)` |
| `checkMetadataFeatureSemantic` | truncated reference | `checkMetadataFeatureSemanticSpecialization` |
| `checkMultiplicity` | truncated reference | `checkMultiplicityTypeFeaturing (+2 more)` |
| `checkNullExpression` | truncated reference | `checkNullExpressionSpecialization` |
| `checkOccurrenceDefinitionIndividual` | truncated reference | `checkOccurrenceDefinitionIndividualSpecialization` |
| `checkOccurrenceDefinitionMultiplicity` | truncated reference | `checkOccurrenceDefinitionMultiplicitySpecialization` |
| `checkOccurrenceDefinitionSpecialization` | misspelling or variant | `checkOccurrenceDefinitionIndividualSpecialization` |
| `checkOccurrenceUsageSnapshot` | truncated reference | `checkOccurrenceUsageSnapshotSpecialization` |
| `checkOccurrenceUsageSuboccurrence` | truncated reference | `checkOccurrenceUsageSuboccurrenceSpecialization` |
| `checkOccurrenceUsageTimeSlice` | truncated reference | `checkOccurrenceUsageTimeSliceSpecialization` |
| `checkPartUsageSupartSpecialization` | misspelling or variant | `checkPartUsageSubpartSpecialization` |
| `checkPayloadFeature` | truncated reference | `checkPayloadFeatureRedefinition` |
| `checkRenderingUsage` | truncated reference | `checkRenderingUsageRedefinition (+2 more)` |
| `checkRenderingUsageSubrendering` | truncated reference | `checkRenderingUsageSubrenderingSpecialization` |
| `checkRequirementUsage` | truncated reference | `checkRequirementUsageSpecialization (+3 more)` |
| `checkRequirementUsageRequirement` | truncated reference | `checkRequirementUsageRequirementVerificationSpecialization` |
| `checkRequirementUsageSubrequirement` | truncated reference | `checkRequirementUsageSubrequirementSpecialization` |
| `checkSatisfyRequirement` | truncated reference | `checkSatisfyRequirementUsageSpecialization (+1 more)` |
| `checkSelectExpressionResult` | truncated reference | `checkSelectExpressionResultSpecialization` |
| `checkSendActionUsageSubaction` | truncated reference | `checkSendActionUsageSubactionSpecialization` |
| `checkStateUsageExclusiveState` | truncated reference | `checkStateUsageExclusiveStateSpecialization` |
| `checkStepEnclosedPerformance` | truncated reference | `checkStepEnclosedPerformanceSpecialization` |
| `checkStepOwnedPerformance` | truncated reference | `checkStepOwnedPerformanceSpecialization` |
| `checkStepSubperformance` | truncated reference | `checkStepSubperformanceSpecialization` |
| `checkSuccession` | truncated reference | `checkSuccessionSpecialization (+2 more)` |
| `checkSuccessionFlow` | truncated reference | `checkSuccessionFlowSpecialization (+1 more)` |
| `checkSuccessionFlowUsage` | truncated reference | `checkSuccessionFlowUsageSpecialization` |
| `checkTerminateActionUsageSubaction` | truncated reference | `checkTerminateActionUsageSubactionSpecialization` |
| `checkTransitionUsage` | truncated reference | `checkTransitionUsageSpecialization (+7 more)` |
| `checkTransitionUsageSource` | truncated reference | `checkTransitionUsageSourceBindingConnector` |
| `checkTransitionUsageSuccessionSource` | truncated reference | `checkTransitionUsageSuccessionSourceSpecialization` |
| `checkTransitionUsageTransitionFeature` | truncated reference | `checkTransitionUsageTransitionFeatureSpecialization` |
| `checkTriggerInvocationExpressionSpecialization` | misspelling or variant | `checkInvocationExpressionSpecialization` |
| `checkUsageVariationUsage` | truncated reference | `checkUsageVariationUsageTypeFeaturing (+1 more)` |
| `checkUseCaseUsageSubAnalysisCaseSpecialization` | misspelling or variant | `checkAnalysisCaseUsageSubAnalysisCaseSpecialization` |
| `checkVariationUsageTypeFeaturing` | misspelling or variant | `checkUsageVariationUsageTypeFeaturing` |
| `checkVerificationCaseDefinitionSpecialization` | misspelling or variant | `checkVerificationCaseSpecialization` |
| `checkVerificationCaseUsageSubAnalysisCaseSpecialization` | unmatched | `checkVerificationCaseUsageSubVerificationCaseSpecialization` |
| `checkVerificationCaseUsageSubVerification` | truncated reference | `checkVerificationCaseUsageSubVerificationCaseSpecialization` |
| `checkViewpointUsageViewpointSatisfaction` | misspelling or variant | `checkViewpointUsageViewpointSatisfactionSpecialization` |
| `checkWhileLoopActionUsageSubaction` | truncated reference | `checkWhileLoopActionUsageSubactionSpecialization` |
| `deriveElementOwningNamespace` | misspelling or variant | `deriveOwningNamespace` |
| `deriveMetadataAccessExpressionReferencedElement` | misspelling or variant | `deriveMetadataAccessExpressionReferencdElement` |
| `validateAttributeDefinitionFeature` | misspelling or variant | `validateAttributeDefinitionFeatures` |
| `validateAttributeUsageFeature` | misspelling or variant | `validateAttributeUsageFeatures` |
| `validateDecisionNodeOutcomingSuccessions` | misspelling or variant | `validateDecisionNodeOutgoingSuccessions` |
| `validateDefinitionVariationOwnedFeatureMembership` | stated and recorded | `validateDefinitionVariationOwnedFeatureMembership` |
| `validateForkNodeOutgoingSuccessions` | misspelling or variant | `validateJoinNodeOutgoingSuccessions` |
| `validateJoinNodeIncomingSuccessions` | misspelling or variant | `validateForkNodeIncomingSuccessions` |
| `validateOccurranceUsageIndividualDefinition` | misspelling or variant | `validateOccurrenceUsageIndividualDefinition` |
| `validateOccurrenceUsageIsIndividual` | misspelling or variant | `validateOccurrenceUsageIndividualUsage` |
| `validatePortDefinitionNestedUsagesNotComposite` | misspelling or variant | `validatePortDefinitionOwnedUsagesNotComposite` |
| `validateUsageOwningType` | misspelling or variant | `validateUsageType` |
| `validateUsageVariationOwnedFeatureMembership` | stated and recorded | `validateUsageVariationOwnedFeatureMembership` |
| `validateViewDefinitionOnlyOneViewRendering` | stated and recorded | `validateViewDefinitionOnlyOneViewRendering` |

## Observations: where the two tools differ on a probe

Neither item is a rule this record changes; both are recorded so the difference is on file.

- `validatePartUsagePartDefinition` (Beta 2 §8.3.11.3, `partDefinition->notEmpty()`): on
  `package P { item def I; part p : I; }` and on `item def ID; part def P { part p : ID; }`
  this implementation reported `part-usage-part-definition` and the pinned pilot reports nothing.
  Re-read against the derivation, the report was stricter than the specification, not only than
  the pilot: `partDefinition` filters KerML `Feature::type` (§8.3.3.3.4), which takes in the
  implied `Parts::parts` subsetting of `checkPartUsageSpecialization`, and validation constraints
  presume the semantic constraints hold (§8.3.1), so every part usage is typed by `Parts::Part`
  (§7.10). The rule was withdrawn; the census row is deliberate and the derivation is reflective
  `partDefinition` (`semantics.Model.FeatureTypeSetWithImplied`).
- `[3..1]` (`validateMultiplicityRangeBounds.violating.kerml`): this implementation reports
  `multiplicity-range` (lower bound exceeds upper bound), the pilot reports nothing. Beta 2
  names no constraint for a lower bound above the upper bound; the report rests on the
  semantics of a multiplicity range (KerML §7.3.3), not on a named constraint, and is stricter
  than the pilot.
- `var end a : A` (`validateFeatureEndIsConstant.violating.kerml`): the Beta 2 grammar admits
  only `const` before `end`; the pilot rejects the text as a syntax error, our parser accepts it
  and reports `variable-feature-owner`. A lenience of the parser, not a constraint verdict.

## Launch-ready gaps

None from the `validate*` rows (every rule the Beta 2 documents state for these 88 names is
either reported already or cannot be violated in the textual notation), none from the `derive*`
rows (export observations, not validation rules) and none from the Beta 2-only names (every
`validate*` token is a misspelling or variant of a recorded name). One item from the `check*`
rows, since closed:

### Usages subset the base library usage of their kind (closed)

- **Rows (14, now faithful):** `checkAllocationUsageSpecialization`,
  `checkCaseUsageSpecialization`, `checkConcernUsageSpecialization`,
  `checkConnectionUsageSpecialization`, `checkConnectorSpecialization` (KerML),
  `checkInterfaceUsageSpecialization`, `checkItemUsageSpecialization`,
  `checkOccurrenceUsageSpecialization`, `checkPartUsageSpecialization`,
  `checkPortUsageSpecialization`, `checkRenderingUsageSpecialization`,
  `checkUseCaseUsageSpecialization`, `checkViewUsageSpecialization`,
  `checkViewpointUsageSpecialization`.
- **Specification:** each clause in the table above (e.g. SysML v2.1 Beta 2 §8.3.11.3,
  `checkPartUsageSpecialization`: "A PartUsage must directly or indirectly specialize the
  PartUsage `Parts::parts`", OCL `specializesFromLibrary('Parts::parts')`; KerML 1.1 Beta 2
  §8.3.4.5.3, `checkConnectorSpecialization`: `specializesFromLibrary('Links::links')`). The
  stated relationship is a subsetting of the base *feature*; SysML v2.1 Beta 2 §7.6 is where
  that subsetting makes `that` visible in a usage body.
- **Behaviour before:** `internal/semantic/semantics/implicit.go:Model.kindBaseDefinitionFQNs`
  replaced the base feature by its types, so the implied supertype of `part u_part;` was
  `Parts::Part` only; the base usage contributed its members without being a supertype, pinned
  by `TestImplicitBaseUsageContributesThat` (`Conforms(p, things)` false). The pin dated from the
  member-only `Base::things` edge that first made `that` resolve, and from the later feature-base
  design that kept the base feature out of `DirectSupertypes` so that effective typing stayed
  the definition's; neither recorded a reading of the clause under which the usage must not
  conform to the base usage.
- **Behaviour now:** `Model.implicitBaseFeatures` appends the base usage of the kind (or
  `Base::things` for a usage of no kind, `Links::binaryLinks` for a two-ended connector) to
  `DirectSupertypes` as an implied Subsetting, suppressed only when a declared chain already
  reaches it. `FeatureTypeSet`, parameter and role redefinition, the inherited-name-conflict
  walk and the runtime's shape derivation skip the implied base features, so types,
  multiplicities, `self`/`that` and classification are as before; conformance gains the base
  usage. `Model.DeclaredSupertypes` names the generals less the implied ones.
- **Probe:** `probes/triage/check-model.sysml` / `.kerml`. Ours before → after:
  `P::u_part istype Parts::parts` false → true, `P::u_part istype Parts::Part` true → true,
  `P::u_part istype Base::things` false → true; the same flip for `u_case`/`Cases::cases`,
  `u_concern`/`Requirements::concernChecks`, `u_item`/`Items::items`,
  `u_occurrence`/`Occurrences::occurrences`, `u_rendering`/`Views::renderings`,
  `u_usecase`/`UseCases::useCases`, `u_view`/`Views::views`,
  `u_viewpoint`/`Views::viewpointChecks`, `new P::Holder().alx`/`Allocations::allocations`,
  `.cnx`/`Connections::connections`, `.ifx`/`Interfaces::interfaces`, `.q`/`Ports::ports`.
  `K::cx` conforms to `Links::links` and `Links::binaryLinks` at model level
  (`internal/workspace/model/base_feature_conformance_test.go`); `istype` classifies a KerML
  connector's value by its type only, so it stays `Links::Link` there. No diagnostic is
  expected: this is an implied relationship, not a validation rule.
- **Pilot verdict:** both probe models are clean under the pinned validators (no diagnostic),
  as before; neither validator reports a `check*` constraint.
- **Tests:** `internal/workspace/model/base_feature_conformance_test.go`
  (`TestUsageSubsetsTheBaseUsageOfItsKind`, every row plus a typed usage, an individual
  occurrence, a plain usage and the KerML connector forms),
  `internal/semantic/semantics/implicit_test.go:TestImplicitBaseUsageContributesThat`,
  `internal/workspace/model/implicit_typing_test.go:TestImplicitUsageBaseTypes`.
