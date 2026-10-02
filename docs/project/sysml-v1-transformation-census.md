# SysML v1 to v2 transformation census

OMG's SysML v1 to v2 transformation model names every mapping it applies — one class per
mapping, grouped by the v1 metamodel package it migrates. This page is the census of those
mapping classes against OpenSysML's migrator: each row ties one OMG mapping to the code that
carries it out, or records why it does not.

**Run:** `go run -C tools ./cmd/transformation-census` (rewrites the generated blocks below from the
baseline); `go run -C tools ./cmd/transformation-census -check` (the gate); `-update` re-extracts the
mapping list from the pinned model keeping every recorded verdict; `-measure` recomputes the scope
counts.
**Baseline:** [sysml-v1-transformation-census-baseline.json](sysml-v1-transformation-census-baseline.json)
**Model:** provisioned by `./scripts/download-sysml-v1tov2.sh` into `build/sysml-v1tov2/`, pinned by
`scripts/sysml-v1tov2-pin.sh`; `-check` compares against it when present and `-check -require-xmi`
fails when absent.

<!-- census:begin source -->
**Source:** OMG SysML v1 to v2 transformation model, document `ptc/25-04-10`, `SysMLv1Tov2.xmi` (`sha256:093359439fb62cb3a9b1e89fd850ac4ac0b4bbe135b4b54f7861492722cc8459`) — 38 packages, 910 classes, 783 mapping classes, 1084 OCL bodies.
<!-- census:end source -->

<!-- census:begin summary -->
**Census:** 783 mapping classes — 0 ✅ faithful, 0 ⚠️ approximate, 0 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 783 ❔ unknown.

| Package | Mappings | ✅ | ⚠️ | ❌ | ⛔ | 🚧 | ❔ |
|---|---|---|---|---|---|---|---|
| CommonMappings | 18 | 0 | 0 | 0 | 0 | 0 | 18 |
| SysMLv1::Activities | 17 | 0 | 0 | 0 | 0 | 0 | 17 |
| SysMLv1::Allocations | 21 | 0 | 0 | 0 | 0 | 0 | 21 |
| SysMLv1::Blocks | 14 | 0 | 0 | 0 | 0 | 0 | 14 |
| SysMLv1::ConstraintBlocks | 2 | 0 | 0 | 0 | 0 | 0 | 2 |
| SysMLv1::ModelElements | 63 | 0 | 0 | 0 | 0 | 0 | 63 |
| SysMLv1::Ports&Flows | 20 | 0 | 0 | 0 | 0 | 0 | 20 |
| SysMLv1::Requirements | 49 | 0 | 0 | 0 | 0 | 0 | 49 |
| UML4SysML::Actions::AcceptEventActions | 25 | 0 | 0 | 0 | 0 | 0 | 25 |
| UML4SysML::Actions::Actions | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| UML4SysML::Actions::InvocationActions | 40 | 0 | 0 | 0 | 0 | 0 | 40 |
| UML4SysML::Actions::LinkActions | 7 | 0 | 0 | 0 | 0 | 0 | 7 |
| UML4SysML::Actions::ObjectActions | 42 | 0 | 0 | 0 | 0 | 0 | 42 |
| UML4SysML::Actions::OtherActions | 2 | 0 | 0 | 0 | 0 | 0 | 2 |
| UML4SysML::Actions::StructuralFeatureActions | 34 | 0 | 0 | 0 | 0 | 0 | 34 |
| UML4SysML::Actions::StructuredActions | 3 | 0 | 0 | 0 | 0 | 0 | 3 |
| UML4SysML::Actions::VariableActions | 31 | 0 | 0 | 0 | 0 | 0 | 31 |
| UML4SysML::Activities | 68 | 0 | 0 | 0 | 0 | 0 | 68 |
| UML4SysML::Classification | 48 | 0 | 0 | 0 | 0 | 0 | 48 |
| UML4SysML::CommonBehavior | 57 | 0 | 0 | 0 | 0 | 0 | 57 |
| UML4SysML::CommonStructure | 18 | 0 | 0 | 0 | 0 | 0 | 18 |
| UML4SysML::InformationFlows | 9 | 0 | 0 | 0 | 0 | 0 | 9 |
| UML4SysML::Interactions | 19 | 0 | 0 | 0 | 0 | 0 | 19 |
| UML4SysML::Packages | 34 | 0 | 0 | 0 | 0 | 0 | 34 |
| UML4SysML::SimpleClassifiers | 22 | 0 | 0 | 0 | 0 | 0 | 22 |
| UML4SysML::StateMachines | 26 | 0 | 0 | 0 | 0 | 0 | 26 |
| UML4SysML::StructuredClassifiers | 39 | 0 | 0 | 0 | 0 | 0 | 39 |
| UML4SysML::UseCases | 14 | 0 | 0 | 0 | 0 | 0 | 14 |
| UML4SysML::Values | 33 | 0 | 0 | 0 | 0 | 0 | 33 |
| **Total** | 783 | 0 | 0 | 0 | 0 | 0 | 783 |
<!-- census:end summary -->

<!-- census:begin rows -->
### CommonMappings (18)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `CommonFeatureReferenceExpression_Mapping` | `TypedElement` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `CommonMembership_Mapping` | `TypedElement` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `CommonParameterReferenceUsageInFeatureTyping_Mapping` | `Element` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `CommonParameterReferenceUsageInMembership_Mapping` | `Element` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `CommonParameterReferenceUsageInUntyped_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonParameterReferenceUsageIn_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReferenceUsageInFeatureMembership_Mapping` | `TypedElement` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReferenceUsageInFeatureTyping_Mapping` | `TypedElement` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReferenceUsageInUntyped_Mapping` | `TypedElement` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReferenceUsageIn_Mapping` | `TypedElement` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterFeatureMembership_Mapping` | `Element` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterFeatureTyping_Mapping` | `Element` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterFeatureUntyped_Mapping` | `Element` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterFeature_Mapping` | `Element` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterReferenceUsageFeatureTyping_Mapping` | `Element` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterReferenceUsageMembership_Mapping` | `Element` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterReferenceUsageUntyped_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonReturnParameterReferenceUsage_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::Activities (17)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ProbabilityMetadataUsageFeatureMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ProbabilityMetadataUsageFeatureTyping_Mapping` | `Element` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ProbabilityMetadataUsageReferenceUsageFeatureValue_Mapping` | `Element` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ProbabilityMetadataUsageReferenceUsageRedefinition_Mapping` | `Element` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ProbabilityMetadataUsageReferenceUsage_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ProbabilityMetadataUsage_Mapping` | `Element` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ProbabilityOwningMembership_Mapping` | `Element` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageContinuousFeatureMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageContinuousReferenceUsageRedefinition_Mapping` | `Element` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageContinuousReferenceUsage_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageDiscreteFeatureMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageDiscreteReferenceUsageRedefinition_Mapping` | `Element` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageDiscreteReferenceUsage_Mapping` | `Element` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageFeatureTyping_Mapping` | `Element` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsageFeatureValue_Mapping` | `Element` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RateMetadataUsage_Mapping` | `Element` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RateOwningMembership_Mapping` | `Element` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::Allocations (21)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AllocationFeatureTyping_Mapping` | `NamedElement` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationSourceFeatureMembership_Mapping` | `NamedElement` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationSourceReferenceUsageRedefinition_Mapping` | `NamedElement` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationSourceReferenceUsage_Mapping` | `NamedElement` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationTargetFeatureMembership_Mapping` | `NamedElement` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationTargetReferenceUsageRedefinition_Mapping` | `NamedElement` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationTargetReferenceUsage_Mapping` | `NamedElement` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageFeatureChainingChainedFeature_Mapping` | `NamedElement` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageFeatureMembership_Mapping` | `Abstraction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageSourceEndFeatureMembership_Mapping` | `NamedElement` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageSourceFeatureChaining_Mapping` | `NamedElement` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageSourceFeatureSubsettingFeature_Mapping` | `NamedElement` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageSourceFeatureSubsetting_Mapping` | `NamedElement` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageSourceFeature_Mapping` | `NamedElement` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageTargetEndFeatureMembership_Mapping` | `NamedElement` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageTargetFeatureChaining_Mapping` | `NamedElement` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageTargetFeatureSubsettingFeature_Mapping` | `NamedElement` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageTargetFeatureSubsetting_Mapping` | `NamedElement` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsageTargetFeature_Mapping` | `NamedElement` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `AllocationUsage_Mapping` | `Abstraction` | `AllocationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Allocation_Mapping` | `Abstraction` | `AllocationDefinition` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::Blocks (14)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AssociationBlock_Mapping` | `AssociationClass` | `ConnectionDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `BindingConnector_Mapping` | `Connector` | `BindingConnectorAsUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Block_Mapping` | `Class` | `PartDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadataFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadataFeatureTyping_Mapping` | `Class` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadataFeatureValue_Mapping` | `Class` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadataMembership_Mapping` | `Class` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadataRedefinition_Mapping` | `Class` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadataReferenceUsage_Mapping` | `Class` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlockMetadata_Mapping` | `Class` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `EncapsulatedBlock_Mapping` | `Class` | `PartDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `FlowPropertyPart_Mapping` | `Property` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PartProperty_Mapping` | `Property` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ValueType_Mapping` | `DataType` | `AttributeDefinition` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::ConstraintBlocks (2)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ConstraintBlock_Mapping` | `Class` | `ConstraintDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ConstraintParameter_Mapping` | `NamedElement` | `AttributeUsage` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::ModelElements (63)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ConcernDocumentation_Mapping` | `Comment` | `Documentation` | — | — | ❔ unknown | not yet adjudicated |
| `ConcernOwningMembership_Mapping` | `Comment` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConcernStakeholderMembership_Mapping` | `Classifier` | `StakeholderMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConcernStakeholderPartUsageFeatureTyping_Mapping` | `Classifier` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ConcernStakeholderPartUsageFeature_Mapping` | `Classifier` | `Multiplicity` | — | — | ❔ unknown | not yet adjudicated |
| `ConcernStakeholderPartUsageOwningMembership_Mapping` | `Classifier` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConcernStakeholderPartUsage_Mapping` | `Classifier` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Concern_Mapping` | `Comment` | `ConcernUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadaMembership_Mapping` | `Comment` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadataFeatureMembership_Mapping` | `Comment` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadataFeatureTyping_Mapping` | `Comment` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadataFeatureValue_Mapping` | `Comment` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadataRedefinition_Mapping` | `Comment` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadataReferenceUsage_Mapping` | `Comment` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroupMetadataUsage_Mapping` | `Comment` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ElementGroup_Mapping` | `Comment` | `Package` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataFeatureMembership_Mapping` | `Comment` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataFeatureTyping_Mapping` | `Comment` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataFeatureValue_Mapping` | `Comment` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataMembership_Mapping` | `Comment` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataRedefinition_Mapping` | `Comment` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataReferenceUsage_Mapping` | `Comment` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationaleMetadataUsage_Mapping` | `Comment` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ProblemRationale_Mapping` | `Comment` | `Comment` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataFeatureMembership_Mapping` | `Classifier` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataFeatureTyping_Mapping` | `Classifier` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataOwningMembership_Mapping` | `Classifier` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataReferenceUsageFeatureValue_Mapping` | `Classifier` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataReferenceUsageRedefinition_Mapping` | `Classifier` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataReferenceUsage_Mapping` | `Classifier` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `StakeholderMetadataUsage_Mapping` | `Classifier` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Stakeholder_Mapping` | `Class` | `ItemDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointConcernReferenceSubsetting_Mapping` | `Comment` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointConcernUsage_Mapping` | `Comment` | `ConcernUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointConstraintUsageDocumentation_Mapping` | `Class` | `Documentation` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointConstraintUsageOwningMembership_Mapping` | `Class` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointConstraintUsage_Mapping` | `Class` | `ConstraintUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointFramedConcernMembership_Mapping` | `Comment` | `FramedConcernMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointLanguagesMetadataFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointLanguagesMetadataFeatureValue_Mapping` | `Class` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointLanguagesMetadataOperatorExpression_Mapping` | `Class` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointLanguagesMetadataRedefinition_Mapping` | `Class` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointLanguagesMetadataReferenceUsage_Mapping` | `Class` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointMetadataFeatureTyping_Mapping` | `Class` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointMetadataOwningMembership_Mapping` | `Class` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointMetadataUsage_Mapping` | `Class` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointPresentationsMetadataFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointPresentationsMetadataFeatureValue_Mapping` | `Class` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointPresentationsMetadataOperatorExpression_Mapping` | `Class` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointPresentationsMetadataRedefinition_Mapping` | `Class` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointPresentationsMetadataReferenceUsage_Mapping` | `Class` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointRenderingFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointRenderingUsageActionUsageFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointRenderingUsageActionUsageFeatureTyping_Mapping` | `Class` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointRenderingUsageActionUsage_Mapping` | `Class` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointRenderingUsage_Mapping` | `Class` | `RenderingUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointRequirementConstraintMembership_Mapping` | `Class` | `RequirementConstraintMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointSatisfyFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointSatisfyRequirementUsageReferenceSubsetting_Mapping` | `Class` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointSatisfyRequirementUsage_Mapping` | `Class` | `SatisfyRequirementUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointViewpointUsageFeatureMembership_Mapping` | `Class` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ViewpointViewpointUsage_Mapping` | `Class` | `ViewpointUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Viewpoint_Mapping` | `Class` | `ViewDefinition` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::Ports&Flows (20)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AcceptChangeStructuralFeatureEventAction_Mapping` | `AcceptEventAction` | `AcceptActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonFullPort_Mapping` (abstract) | `Port` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ConjugatedPortDefinition_Mapping` | `Class` | `ConjugatedPortDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `FlowPropertyAttribute_Mapping` | `Property` | `AttributeUsage` | — | — | ❔ unknown | not yet adjudicated |
| `FlowPropertyUntyped_Mapping` | `NamedElement` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `FlowProperty_Mapping` | `Property` | `OccurrenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadataFeatureMembership_Mapping` | `Port` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadataFeatureTyping_Mapping` | `Port` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadataOwningMembership_Mapping` | `Port` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadataReferenceUsageFeatureValue_Mapping` | `Port` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadataReferenceUsageRedefinition_Mapping` | `Port` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadataReferenceUsage_Mapping` | `Port` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortMetadata_Mapping` | `Port` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `FullPortUntyped_Mapping` | `Port` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `FullPort_Mapping` | `Port` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `InterfaceBlockConjugated_Mapping` | `Class` | `PortDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `InterfaceBlockOwningMembership_Mapping` | `Class` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InterfaceBlock_Mapping` | `Class` | `PortDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `OperationDirectedFeature_Mapping` | `Operation` | `PerformActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PortConjugation_Mapping` | `Class` | `PortConjugation` | — | — | ❔ unknown | not yet adjudicated |

### SysMLv1::Requirements (49)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `DeriveReqtFeatureTyping_Mapping` | `Dependency` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqtSourceEndFeatureMembership_Mapping` | `Dependency` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqtSourceFeatureReferenceSubsetting_Mapping` | `Dependency` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqtSourceFeature_Mapping` | `Dependency` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqtTargetEndFeatureMembership_Mapping` | `Dependency` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqtTargetFeatureReferenceSubsetting_Mapping` | `Dependency` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqtTargetFeature_Mapping` | `Dependency` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `DeriveReqt_Mapping` | `Abstraction` | `ConnectionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RefineAnnotation_Mapping` | `Abstraction` | `Annotation` | — | — | ❔ unknown | not yet adjudicated |
| `RefineMetadataFeatureMembership_Mapping` | `Abstraction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RefineMetadataReferenceUsageFeatureValue_Mapping` | `Abstraction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RefineMetadataReferenceUsageRedefinition_Mapping` | `Abstraction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `RefineMetadataReferenceUsage_Mapping` | `Abstraction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RefineMetadataUsageFeatureTyping_Mapping` | `Abstraction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `RefineMetadataUsage_Mapping` | `Abstraction` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Refine_Mapping` | `Abstraction` | `Dependency` | — | — | ❔ unknown | not yet adjudicated |
| `RequirementDocumentationMembership_Mapping` | `Class` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RequirementDocumentation_Mapping` | `Class` | `Documentation` | — | — | ❔ unknown | not yet adjudicated |
| `RequirementSubjectMembership_Mapping` | `Class` | `SubjectMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RequirementSubject_Mapping` | `Class` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Requirement_Mapping` | `Class` | `RequirementUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfyFeatureTyping_Mapping` | `Abstraction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfyReferenceUsageFeatureMembership_Mapping` | `Abstraction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfyReferenceUsageFeatureTyping_Mapping` | `Abstraction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfyReferenceUsage_Mapping` | `Abstraction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsageFeatureChaining_Mapping` | `Abstraction` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsageFeatureValue_Mapping` | `Abstraction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsageValueFeatureChainingProperty_Mapping` | `Abstraction` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsageValueFeature_Mapping` | `Abstraction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsageValueOwningMembership_Mapping` | `Abstraction` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsageValue_Mapping` | `Abstraction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectReferenceUsage_Mapping` | `Abstraction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SatisfySubjectSubjectMembership_Mapping` | `Abstraction` | `SubjectMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Satisfy_Mapping` | `Abstraction` | `SatisfyRequirementUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TestCaseActivityReturnParameterMembership_Mapping` | `Parameter` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TestCaseActivity_Mapping` | `Activity` | `VerificationCaseDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `TestCaseVerifyObjectiveMembership_Mapping` | `Abstraction` | `ObjectiveMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TestCaseVerifyObjectiveRequirementUsage_Mapping` | `Abstraction` | `RequirementUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TestCaseVerifyRequirementUsageReferenceSubsetting_Mapping` | `Abstraction` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `TestCaseVerifyRequirementUsage_Mapping` | `Abstraction` | `RequirementUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TraceAnnotation_Mapping` | `Abstraction` | `Annotation` | — | — | ❔ unknown | not yet adjudicated |
| `TraceMetadataFeatureMembership_Mapping` | `Abstraction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TraceMetadataReferenceUsageFeatureValue_Mapping` | `Abstraction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `TraceMetadataReferenceUsageRedefinition_Mapping` | `Abstraction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `TraceMetadataReferenceUsage_Mapping` | `Abstraction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TraceMetadataUsageFeatureTyping_Mapping` | `Abstraction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `TraceMetadataUsage_Mapping` | `Abstraction` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Trace_Mapping` | `Abstraction` | `Dependency` | — | — | ❔ unknown | not yet adjudicated |
| `Verify_Mapping` | `Abstraction` | `RequirementVerificationMembership` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::AcceptEventActions (25)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AEAChangeExpressionMembership_Mapping` | `AcceptEventAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterExpressionFeatureValue_Mapping` | `AcceptEventAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterFeatureChainExpression_Mapping` | `AcceptEventAction` | `FeatureChainExpression` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterFeatureMembership_Mapping` | `AcceptEventAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterFeatureReferenceExpression_Mapping` | `AcceptEventAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterFeatureValue_Mapping` | `AcceptEventAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterFeature_Mapping` | `AcceptEventAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterMembership_Mapping` | `AcceptEventAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterParameterMembership_Mapping` | `AcceptEventAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterResultExpressionMembership_Mapping` | `AcceptEventAction` | `ResultExpressionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterTriggerExpression_Mapping` | `AcceptEventAction` | `Expression` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameterTrigger_Mapping` | `AcceptEventAction` | `TriggerInvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `AEAChangeParameter_Mapping` | `AcceptEventAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AEAParameterMembership_Mapping` | `AcceptEventAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAReceiverFeatureReferenceExpressionMembership_Mapping` | `AcceptEventAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAReceiverFeatureReferenceExpression_Mapping` | `AcceptEventAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `AEAReceiverFeatureValue_Mapping` | `AcceptEventAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `AEAReceiverParameterMembership_Mapping` | `AcceptEventAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AEAReceiverParameter_Mapping` | `AcceptEventAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AEASignalParameterFeatureTyping_Mapping` | `AcceptEventAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `AEASignalParameter_Mapping` | `AcceptEventAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AcceptCallAction_Mapping` | `AcceptCallAction` | `AcceptActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AcceptEventAction_Mapping` | `AcceptEventAction` | `AcceptActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReplyAction_Mapping` | `ReplyAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `UnmarshallAction_Mapping` | `UnmarshallAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::Actions (8)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `CommonAction_Mapping` (abstract) | `Action` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `OABodyMembership_Mapping` | `OpaqueAction` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OABody_Mapping` | `OpaqueAction` | `TextualRepresentation` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueAction_Mapping` | `OpaqueAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Pin_Mapping` | `Pin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ValuePinFeatureValue_Mapping` | `ValuePin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ValuePinUntyped_Mapping` | `ValuePin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ValuePin_Mapping` | `ValuePin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::InvocationActions (40)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `BroadcastSignalAction_Mapping` | `BroadcastSignalAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CBAFeatureTyping_Mapping` | `CallBehaviorAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureChainExpressionMembership_Mapping` | `OutputPin` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureChainExpression_Mapping` | `OutputPin` | `FeatureChainExpression` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureFeatureMembership_Mapping` | `OutputPin` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureFeatureValue_Mapping` | `OutputPin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureFeature_Mapping` | `OutputPin` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureMembership_Mapping` | `OutputPin` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureReferenceExpressionMembership_Mapping` | `OutputPin` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeatureReferenceExpression_Mapping` | `OutputPin` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinFeature_Mapping` | `OutputPin` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinParameterMembership_Mapping` | `OutputPin` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinReferenceUsageFeatureValue_Mapping` | `OutputPin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `COAOutputPinReferenceUsage_Mapping` | `OutputPin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `COAPerformActionFeatureChainingOperation_Mapping` | `CallOperationAction` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `COAPerformActionFeatureChainingTarget_Mapping` | `CallOperationAction` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `COAPerformActionFeatureMembership_Mapping` | `CallOperationAction` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `COAPerformActionFeature_Mapping` | `CallOperationAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `COAPerformActionReferenceSubsetting_Mapping` | `CallOperationAction` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `COAPerformAction_Mapping` | `CallOperationAction` | `PerformActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CallBehaviorAction_Mapping` | `CallBehaviorAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CallOperationAction_Mapping` | `CallOperationAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SSAFeatureMembership_Mapping` | `InvocationAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SSAItemParameterMembership_Mapping` | `InvocationAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SSAItemReferenceUsageFeatureTyping_Mapping` | `InvocationAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `SSAItemReferenceUsageFeatureValue_Mapping` | `InvocationAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `SSAItemReferenceUsageInvocationExpression_Mapping` | `InvocationAction` | `InvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `SSAItemReferenceUsage_Mapping` | `InvocationAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SSAParameterMembership_Mapping` | `InvocationAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SSAReferenceUsage_Mapping` | `InvocationAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SSASendActionUsage_Mapping` | `InvocationAction` | `SendActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SSATargetParameterMembership_Mapping` | `InvocationAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SSATargetReferenceUsageFeatureValueExpression_Mapping` | `InvocationAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `SSATargetReferenceUsageFeatureValueMembership_Mapping` | `InvocationAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `SSATargetReferenceUsageFeatureValue_Mapping` | `InvocationAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `SSATargetReferenceUsage_Mapping` | `InvocationAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SendObjectAction_Mapping` | `SendObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SendSignalAction_Mapping` | `SendSignalAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `StartClassifierBehaviorAction_Mapping` | `StartClassifierBehaviorAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `StartObjectBehaviorAction_Mapping` | `StartObjectBehaviorAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::LinkActions (7)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ClearAssociationAction_Mapping` | `ClearAssociationAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CreateLinkAction_Mapping` | `CreateLinkAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CreateLinkObjectAction_Mapping` | `CreateLinkObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DestroyLinkAction_Mapping` | `DestroyLinkAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadLinkAction_Mapping` | `ReadLinkAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadLinkObjectEndAction_Mapping` | `ReadLinkObjectEndAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadLinkObjectEndQualifierAction_Mapping` | `ReadLinkObjectEndQualifierAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::ObjectActions (42)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `COAInvocationExpessionFeatureTyping_Mapping` | `CreateObjectAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `COAInvocationExpression_Mapping` | `CreateObjectAction` | `InvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `COAPinFeatureValue_Mapping` | `OutputPin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `COAPin_Mapping` | `OutputPin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CreateObjectAction_Mapping` | `CreateObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsageFeatureMembership_Mapping` | `DestroyObjectAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsageFeatureReferenceExpression_Mapping` | `DestroyObjectAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsageFeatureTyping_Mapping` | `DestroyObjectAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsageFeatureValue_Mapping` | `DestroyObjectAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsageMembership_Mapping` | `DestroyObjectAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsageReferenceUsage_Mapping` | `DestroyObjectAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyActionUsage_Mapping` | `DestroyObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DOADestroyFeatureMembership_Mapping` | `DestroyObjectAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DestroyObjectAction_Mapping` | `DestroyObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `REAFeatureValueOperatorExpressionFeatureTyping_Mapping` | `OutputPin` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `REAFeatureValueOperatorExpressionFeature_Mapping` | `OutputPin` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `REAFeatureValueOperatorExpressionMembership_Mapping` | `OutputPin` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `REAFeatureValueOperatorExpression_Mapping` | `OutputPin` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `REAFeatureValue_Mapping` | `OutputPin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `REAOutputPin_Mapping` | `OutputPin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValueOperatorExpressionFeatureValue_Mapping` | `ReadIsClassifiedObjectAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValueOperatorExpressionFeature_Mapping` | `ReadIsClassifiedObjectAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValueOperatorExpression_Mapping` | `ReadIsClassifiedObjectAction` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValueOperatorFeatureReferenceExpression_Mapping` | `ReadIsClassifiedObjectAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValueOperatorMembership_Mapping` | `ReadIsClassifiedObjectAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValueOperatorParameterMembership_Mapping` | `ReadIsClassifiedObjectAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAFeatureValue_Mapping` | `ReadIsClassifiedObjectAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RICOAOutputPin_Mapping` | `OutputPin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RSAFeatureValueFeatureReferenceExpression_Mapping` | `OutputPin` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RSAFeatureValueMembership_Mapping` | `OutputPin` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `RSAFeatureValue_Mapping` | `OutputPin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RSAOutputPin_Mapping` | `OutputPin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadExtentAction_Mapping` | `ReadExtentAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadIsClassifiedObjectAction_Mapping` | `ReadIsClassifiedObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadSelfAction_Mapping` | `ReadSelfAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReclassifyObjectAction_Mapping` | `ReclassifyObjectAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TIAOperatorExpression_Mapping` | `TestIdentityAction` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `TIAResultExpressionMembership_Mapping` | `TestIdentityAction` | `ResultExpressionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TestIdentityAction_Mapping` | `TestIdentityAction` | `CalculationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `VSAOutputPinFeatureValue_Mapping` | `OutputPin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `VSAOutputPin_Mapping` | `OutputPin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ValueSpecificationAction_Mapping` | `ValueSpecificationAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::OtherActions (2)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `RaiseExceptionAction_Mapping` | `RaiseExceptionAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReduceAction_Mapping` | `ReduceAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::StructuralFeatureActions (34)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ASFVAFeatureTyping_Mapping` | `AddStructuralFeatureValueAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVAObjectFeatureMembership_Mapping` | `AddStructuralFeatureValueAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVAObjectReferenceUsageFeatureTyping_Mapping` | `AddStructuralFeatureValueAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVAObjectReferenceUsageRedefinition_Mapping` | `AddStructuralFeatureValueAction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVAObjectReferenceUsage_Mapping` | `AddStructuralFeatureValueAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetFeatureChainExpression_Mapping` | `AddStructuralFeatureValueAction` | `FeatureChainExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetFeatureMembership_Mapping` | `AddStructuralFeatureValueAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetFeatureValue_Mapping` | `AddStructuralFeatureValueAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterExpressionFeatureMembership_Mapping` | `AddStructuralFeatureValueAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterExpressionFeature_Mapping` | `AddStructuralFeatureValueAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterExpressionMembership_Mapping` | `AddStructuralFeatureValueAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterFeatureExpressionMembership_Mapping` | `AddStructuralFeatureValueAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterFeatureReferenceExpression_Mapping` | `AddStructuralFeatureValueAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterFeatureValue_Mapping` | `AddStructuralFeatureValueAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterFeature_Mapping` | `AddStructuralFeatureValueAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetParameterMembership_Mapping` | `AddStructuralFeatureValueAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetReferenceUsageRedefinition_Mapping` | `AddStructuralFeatureValueAction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ASFVATargetReferenceUsage_Mapping` | `AddStructuralFeatureValueAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AddStructuralFeatureValueAction_Mapping` | `AddStructuralFeatureValueAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ClearStructuralFeatureAction_Mapping` | `ClearStructuralFeatureAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageExpressionFeatureMembership_Mapping` | `ReadStructuralFeatureAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageExpressionFeatureReferenceExpression_Mapping` | `ReadStructuralFeatureAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageExpressionFeatureValue_Mapping` | `ReadStructuralFeatureAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageExpressionFeature_Mapping` | `ReadStructuralFeatureAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageFeatureChainExpressionFeature_Mapping` | `ReadStructuralFeatureAction` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageFeatureChainExpressionMembership_Mapping` | `ReadStructuralFeatureAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageFeatureChainExpression_Mapping` | `ReadStructuralFeatureAction` | `FeatureChainExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageFeatureMembership_Mapping` | `ReadStructuralFeatureAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageFeatureValue_Mapping` | `ReadStructuralFeatureAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageMembership_Mapping` | `ReadStructuralFeatureAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsageParameterMembership_Mapping` | `ReadStructuralFeatureAction` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RSFAReferenceUsage_Mapping` | `ReadStructuralFeatureAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadStructuralFeatureAction_Mapping` | `ReadStructuralFeatureAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RemoveStructuralFeatureValueAction_Mapping` | `RemoveStructuralFeatureValueAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::StructuredActions (3)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `LoopNode_Mapping` | `LoopNode` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `SequenceNode_Mapping` | `SequenceNode` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `StructuredActivityNode_Mapping` | `StructuredActivityNode` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Actions::VariableActions (31)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AVVAFeatureTyping_Mapping` | `AddVariableValueAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAFeatureValue_Mapping` | `AddVariableValueAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAIsReplaceAllFeatureMembership_Mapping` | `AddVariableValueAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAIsReplaceAllRedefinition_Mapping` | `AddVariableValueAction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAIsReplaceAllValue_Mapping` | `AddVariableValueAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAIsReplaceAll_Mapping` | `AddVariableValueAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAValueExpressionMembership_Mapping` | `AddVariableValueAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAValueFeatureReferenceExpression_Mapping` | `AddVariableValueAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAVariableFeatureMembership_Mapping` | `AddVariableValueAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAVariableRedefinition_Mapping` | `AddVariableValueAction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AVVAVariable_Mapping` | `AddVariableValueAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `AddVariableValueAction_Mapping` | `AddVariableValueAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CVAFeatureMembership_Mapping` | `ClearVariableAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `CVAReferenceUsageFeatureValue_Mapping` | `ClearVariableAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `CVAReferenceUsage_Mapping` | `ClearVariableAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ClearVariableAction_Mapping` | `ClearVariableAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RVAFeatureMembership_Mapping` | `ReadVariableAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RVAReferenceUsageExpressionMembership_Mapping` | `Pin` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `RVAReferenceUsageFeatureReferenceExpression_Mapping` | `Pin` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RVAReferenceUsageFeatureTyping_Mapping` | `Pin` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `RVAReferenceUsageFeatureValue_Mapping` | `Pin` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RVAReferenceUsage_Mapping` | `Pin` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAFeatureTyping_Mapping` | `RemoveVariableValueAction` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAVariableExpressionMembership_Mapping` | `RemoveVariableValueAction` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAVariableFeatureMembership_Mapping` | `RemoveVariableValueAction` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAVariableFeatureReferenceExpression_Mapping` | `RemoveVariableValueAction` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAVariableFeatureValue_Mapping` | `RemoveVariableValueAction` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAVariableRedefinition_Mapping` | `RemoveVariableValueAction` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `RVVAVariable_Mapping` | `RemoveVariableValueAction` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ReadVariableAction_Mapping` | `ReadVariableAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `RemoveVariableValueAction_Mapping` | `RemoveVariableValueAction` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Activities (68)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ActivityAsDefinition_Mapping` | `Activity` | `ActionDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeInitialNodeFeatureMembership_Mapping` | `InitialNode` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadataFeatureMembership_Mapping` | `ActivityEdge` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadataFeatureTyping_Mapping` | `ActivityEdge` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadataFeatureValue_Mapping` | `ActivityEdge` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadataOwningMembership_Mapping` | `ActivityEdge` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadataRedefinition_Mapping` | `ActivityEdge` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadataReferenceUsage_Mapping` | `ActivityEdge` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeMetadata_Mapping` | `ActivityEdge` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeSourceEndFeatureMembership_Mapping` | `Element` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeSourceEndFeature_Mapping` | `Element` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeSourceEndSubsetting_Mapping` | `Element` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeSourceInitialNodeSubsetting_Mapping` | `InitialNode` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeSourceInitialNode_Mapping` | `InitialNode` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityEdgeTransitionUsageSourceMembership_Mapping` | `ActivityNode` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ActivityFinalNode_Mapping` | `ActivityFinalNode` | `TerminateActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CentralBufferNode_Mapping` | `CentralBufferNode` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonActivityEdgeSuccessionAsUsage_Mapping` | `ActivityEdge` | `SuccessionAsUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonVariable_Mapping` (abstract) | `Variable` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowFinalNodeFeatureMembership_Mapping` | `ActivityNode` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowSuccessionAsUsage_Mapping` | `ControlFlow` | `SuccessionAsUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTargetEndFeature_Mapping` | `ActivityNode` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTargetEndSubsetting_Mapping` | `ActivityNode` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTargetFeatureMembership_Mapping` | `ActivityNode` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTargetFinalNodeSubsetting_Mapping` | `FinalNode` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTargetFinalNode_Mapping` | `FinalNode` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTransitionUsageFeatureMembership_Mapping` | `ControlFlow` | `TransitionFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ControlFlowTransitionUsage_Mapping` | `ControlFlow` | `TransitionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ControlNodeObjectFlowFeatureMembership_Mapping` | `ObjectFlow` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ControlNodeObjectFlowFeatureValue_Mapping` | `ObjectFlow` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ControlNodeObjectFlowReferenceUsage_Mapping` | `ObjectFlow` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DataStoreNode_Mapping` | `DataStoreNode` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DecisionNode_Mapping` | `DecisionNode` | `DecisionNode` | — | — | ❔ unknown | not yet adjudicated |
| `FlowFinalNodeMembership_Mapping` | `FlowFinalNode` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ForkNodeObjectFlowFeatureReferenceExpression_Mapping` | `ObjectFlow` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ForkNodeObjectFlowMembership_Mapping` | `ObjectFlow` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ForkNode_Mapping` | `ForkNode` | `ForkNode` | — | — | ❔ unknown | not yet adjudicated |
| `InitialNodeMembership_Mapping` | `InitialNode` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `JoinMergeNodeObjectFlowFeatureReferenceExpression_Mapping` | `ObjectFlow` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `JoinMergeNodeObjectFlowFeatureValue_Mapping` | `ObjectFlow` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `JoinMergeNodeObjectFlowFeature_Mapping` | `ObjectFlow` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `JoinMergeNodeObjectFlowMembership_Mapping` | `ObjectFlow` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `JoinMergeNodeObjectFlowOperatorExpression_Mapping` | `ObjectFlow` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `JoinMergeNodeObjectFlowParameterMembership_Mapping` | `ObjectFlow` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `JoinNode_Mapping` | `JoinNode` | `JoinNode` | — | — | ❔ unknown | not yet adjudicated |
| `MergeNode_Mapping` | `MergeNode` | `MergeNode` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowEndFeatureMembership_Mapping` | `ActivityNode` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowFeatureMembership_Mapping` | `ObjectFlow` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowGuardFeatureMembership_Mapping` | `ObjectFlow` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowGuardSuccessionTargetEndFeatureMembership_Mapping` | `ObjectFlow` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowGuardSuccessionTargetEndFeature_Mapping` | `ObjectFlow` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowGuardSuccessionTargetEndSubsetting_Mapping` | `ObjectFlow` | `Subsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowGuard_Mapping` | `ObjectFlow` | `TransitionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFeatureMembership_Mapping` | `ObjectFlow` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFeatureTyping_Mapping` | `ObjectNode` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFeatureUntyped_Mapping` | `ObjectNode` | `PayloadFeature` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFeature_Mapping` | `ObjectNode` | `PayloadFeature` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFlowEndFeatureMembership_Mapping` | `ActivityNode` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFlowEndRedefinition_Mapping` | `ActivityNode` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFlowEndReferenceUsage_Mapping` | `ActivityNode` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFlowEndSubsetting_Mapping` | `ActivityNode` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowItemFlowEnd_Mapping` | `ActivityNode` | `FlowEnd` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlowTransitionUsageFeatureMembership_Mapping` | `ObjectFlow` | `TransitionFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ObjectFlow_Mapping` | `ObjectFlow` | `SuccessionFlowUsage` | — | — | ❔ unknown | not yet adjudicated |
| `VariableAttribute_Mapping` | `NamedElement` | `AttributeUsage` | — | — | ❔ unknown | not yet adjudicated |
| `VariableFeatureTyping_Mapping` | `Variable` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `VariableItem_Mapping` | `NamedElement` | `ItemUsage` | — | — | ❔ unknown | not yet adjudicated |
| `VariableMembership_Mapping` | `Variable` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Classification (48)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `BehavioralFeature_Mapping` (abstract) | `BehavioralFeature` | `Usage` | — | — | ❔ unknown | not yet adjudicated |
| `Classifier_Mapping` (abstract) | `Classifier` | `Classifier` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultLowerBound_Mapping` | `Element` | `LiteralInteger` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultMultiplicityBoundFeatureMembership_Mapping` (abstract) | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultMultiplicityElement_Mapping` | `Element` | `MultiplicityRange` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultMultiplicityLowerBoundFeatureMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultMultiplicityMembership_Mapping` | `Element` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultMultiplicityUpperBoundFeatureMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultUpperBound_Mapping` | `Element` | `LiteralInteger` | — | — | ❔ unknown | not yet adjudicated |
| `DefaultValue_Mapping` | `Property` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ElementFeatureMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Generalization_Mapping` | `Generalization` | `Subclassification` | — | — | ❔ unknown | not yet adjudicated |
| `InstanceSpecificationFeatureTyping_Mapping` | `InstanceSpecification` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `InstanceSpecificationLink_Mapping` | `InstanceSpecification` | `ConnectionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `InstanceSpecification_Mapping` | `InstanceSpecification` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `InstanceValueMembership_Mapping` | `InstanceSpecification` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `InstanceValue_Mapping` | `InstanceValue` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `LowerBoundValueFeatureMembership_Mapping` | `MultiplicityElement` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `MultiplicityElement_Mapping` | `MultiplicityElement` | `MultiplicityRange` | — | — | ❔ unknown | not yet adjudicated |
| `MultiplicityLowerBoundOwningMembership_Mapping` | `MultiplicityElement` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `MultiplicityMembership_Mapping` | `MultiplicityElement` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `MultiplicityUpperBoundOwningMembership_Mapping` | `MultiplicityElement` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Operation_Mapping` | `Operation` | `PerformActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterDefaultValue_Mapping` | `Parameter` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterMembership_Mapping` | `Parameter` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSetMembership_Mapping` | `ParameterSet` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSetParameterFeatureMembership_Mapping` | `ParameterSet` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSetParameterReferenceUsageFeatureValueExpression_Mapping` | `Parameter` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSetParameterReferenceUsageFeatureValue_Mapping` | `Parameter` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSetParameterReferenceUsageMembership_Mapping` | `Parameter` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSetParameterReferenceUsage_Mapping` | `Parameter` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterSet_Mapping` | `ParameterSet` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ParameterToFeatureTyping_Mapping` | `Parameter` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `Parameter_Mapping` | `Parameter` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PropertyCommon_Mapping` (abstract) | `Property` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `PropertySubsetting_Mapping` | `Property` | `Subsetting` | — | — | ❔ unknown | not yet adjudicated |
| `PropertyTypedByClassInterface_Mapping` | `Property` | `OccurrenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PropertyUntyped_Mapping` | `NamedElement` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Realization_Mapping` | `Realization` | `Dependency` | — | — | ❔ unknown | not yet adjudicated |
| `SlotFeatureTyping_Mapping` | `Slot` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `SlotMembership_Mapping` | `Slot` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `SlotValue_Mapping` | `ValueSpecification` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `Slot_Mapping` | `Slot` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `StructuralFeatureMembership_Mapping` (abstract) | `StructuralFeature` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StructuralFeatureToFeatureTyping_Mapping` | `StructuralFeature` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `StructuralFeature_Mapping` (abstract) | `StructuralFeature` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `TypedElementFeatureTyping_Mapping` | `TypedElement` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `UpperBoundValueFeatureMembership_Mapping` | `MultiplicityElement` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::CommonBehavior (57)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `Behavior_Mapping` (abstract) | `Behavior` | `Behavior` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeEventReturnParameterMembership_Mapping` | `ChangeEvent` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeEventReturnParameter_Mapping` | `ChangeEvent` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeEvent_Mapping` | `ChangeEvent` | `CalculationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerBindingConnector_Mapping` | `Trigger` | `BindingConnectorAsUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerConstraintUsage_Mapping` | `Trigger` | `ConstraintUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerEndFeatureMembership_Mapping` | `Trigger` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerEventChainingFeature_Mapping` | `Trigger` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerEventReturnParameterChainingFeature_Mapping` | `Trigger` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionFeatureMembership_Mapping` | `Trigger` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionFeatureReferenceExpression_Mapping` | `Trigger` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionFeatureTyping_Mapping` | `Trigger` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionFeatureValue_Mapping` | `Trigger` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionFeature_Mapping` | `Trigger` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionInvocationExpression_Mapping` | `Trigger` | `InvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerExpressionParameterMembership_Mapping` | `Trigger` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerFeatureMembership_Mapping` | `Trigger` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerFeatureValue_Mapping` | `Trigger` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerFeature_Mapping` | `Trigger` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerInvocationExpression_Mapping` | `Trigger` | `TriggerInvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReferenceSubsetting_Mapping` | `Trigger` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReferenceUsage_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReturnEndFeatureMembership_Mapping` | `Trigger` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReturnParameterMembership_Mapping` | `Trigger` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReturnParameter_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReturnReferenceSubsetting_Mapping` | `Trigger` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ChangeTriggerReturnReferenceUsage_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueBehaviorMembership_Mapping` | `OpaqueBehavior` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueBehaviorSpecification_Mapping` | `OpaqueBehavior` | `TextualRepresentation` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueBehavior_Mapping` | `OpaqueBehavior` | `ActionDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `SignalTriggerReferenceUsageFeatureTyping_Mapping` | `Trigger` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `SignalTriggerReferenceUsage_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeEvent_Mapping` | `TimeEvent` | `CalculationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerBindingConnector_Mapping` | `Trigger` | `BindingConnectorAsUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerCalculationUsage_Mapping` | `Trigger` | `CalculationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerEndFeatureMembership_Mapping` | `Trigger` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerEventChainingFeature_Mapping` | `Trigger` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerEventReturnParameterChainingFeature_Mapping` | `Trigger` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerExpressionFeatureTyping_Mapping` | `Trigger` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerExpressionFeatureValue_Mapping` | `Trigger` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerExpressionFeature_Mapping` | `Trigger` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerExpressionInvocationExpression_Mapping` | `Trigger` | `InvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerExpressionParameterMembership_Mapping` | `Trigger` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerFeatureMembership_Mapping` | `Trigger` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerFeatureTyping_Mapping` | `Trigger` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerFeatureValue_Mapping` | `Trigger` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerFeature_Mapping` | `Trigger` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerInvocationExpression_Mapping` | `Trigger` | `TriggerInvocationExpression` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReferenceSubsetting_Mapping` | `Trigger` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReferenceUsage_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReturnEndFeatureMembership_Mapping` | `Trigger` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReturnParameterMembership_Mapping` | `Trigger` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReturnParameter_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReturnReferenceSubsetting_Mapping` | `Trigger` | `ReferenceSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReturnReferenceUsage_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TriggerParameterMembership_Mapping` | `Trigger` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Trigger_Mapping` | `Trigger` | `AcceptActionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::CommonStructure (18)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `Abstraction_Mapping` | `Abstraction` | `Dependency` | — | — | ❔ unknown | not yet adjudicated |
| `CommentAnnotation_Mapping` | `Comment` | `Annotation` | — | — | ❔ unknown | not yet adjudicated |
| `CommentOwnership_Mapping` | `Comment` | `Annotation` | — | — | ❔ unknown | not yet adjudicated |
| `Comment_Mapping` | `Comment` | `Comment` | — | — | ❔ unknown | not yet adjudicated |
| `ConstrainedElementFeatureMembership_Mapping` | `Constraint` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConstraintUsageFeatureTyping_Mapping` | `Constraint` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ConstraintUsage_Mapping` | `Constraint` | `AssertConstraintUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Constraint_Mapping` | `Constraint` | `ConstraintDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `Dependency_Mapping` | `Dependency` | `Dependency` | — | — | ❔ unknown | not yet adjudicated |
| `DirectedRelationship_Mapping` (abstract) | `DirectedRelationship` | `Relationship` | — | — | ❔ unknown | not yet adjudicated |
| `ElementMain_Mapping` (abstract) | `Element` | `Element` | — | — | ❔ unknown | not yet adjudicated |
| `ElementMembership_Mapping` | `Element` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `ElementOwnership_Mapping` (abstract) | `Element` | `Relationship` | — | — | ❔ unknown | not yet adjudicated |
| `ElementOwningMembership_Mapping` | `Element` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `NamedElementMain_Mapping` (abstract) | `NamedElement` | `Element` | — | — | ❔ unknown | not yet adjudicated |
| `Namespace_Mapping` (abstract) | `Namespace` | `Namespace` | — | — | ❔ unknown | not yet adjudicated |
| `Relationship_Mapping` (abstract) | `Relationship` | `Relationship` | — | — | ❔ unknown | not yet adjudicated |
| `Usage_Mapping` | `Usage` | `Dependency` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::InformationFlows (9)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `InformationFlowConveyedFeatureMembership_Mapping` | `Classifier` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InformationFlowEndFeatureMembership_Mapping` | `InformationFlow` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InformationFlowEnd_Mapping` | `InformationFlow` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `InformationFlowFeatureTyping_Mapping` | `InformationFlow` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `InformationFlowSubclassification_Mapping` | `InformationFlow` | `Subclassification` | — | — | ❔ unknown | not yet adjudicated |
| `InformationFlow_Mapping` | `InformationFlow` | `FlowDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `InformationItemFlowConveyedItemUsageFeatureTyping_Mapping` | `Classifier` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `InformationItemFlowConveyedItemUsage_Mapping` | `Classifier` | `ItemUsage` | — | — | ❔ unknown | not yet adjudicated |
| `InformationItem_Mapping` | `InformationItem` | `ItemDefinition` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Interactions (19)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ActionExecutionSpecification_Mapping` | `ActionExecutionSpecification` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `BehaviorExecutionSpecification_Mapping` | `BehaviorExecutionSpecification` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CombinedFragmentMembership_Mapping` | `CombinedFragment` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `CombinedFragment_Mapping` | `CombinedFragment` | `Interaction` | — | — | ❔ unknown | not yet adjudicated |
| `ExecutionSpecificationMembership_Mapping` | `ExecutionSpecification` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InteractionOperandMembership_Mapping` | `InteractionOperand` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InteractionOperand_Mapping` | `InteractionOperand` | `Interaction` | — | — | ❔ unknown | not yet adjudicated |
| `InteractionUseFeatureTyping_Mapping` | `InteractionUse` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `InteractionUseMembership_Mapping` | `InteractionUse` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InteractionUse_Mapping` | `InteractionUse` | `Step` | — | — | ❔ unknown | not yet adjudicated |
| `Interaction_Mapping` | `Interaction` | `Interaction` | — | — | ❔ unknown | not yet adjudicated |
| `LifelineFeatureTyping_Mapping` | `Lifeline` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `LifelineMembership_Mapping` | `Lifeline` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `LifelinePartUsage_Mapping` | `Lifeline` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `MessageMembership_Mapping` | `Message` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Message_Mapping` | `Message` | `Flow` | — | — | ❔ unknown | not yet adjudicated |
| `StateInvariantFeatureTyping_Mapping` | `StateInvariant` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `StateInvariantMembership_Mapping` | `StateInvariant` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StateInvariant_Mapping` | `StateInvariant` | `Invariant` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Packages (34)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ElementImport_Mapping` | `ElementImport` | `MembershipImport` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataFeatureMembership_Mapping` | `Model` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataFeatureTyping_Mapping` | `Model` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataFeatureValue_Mapping` | `Model` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataMembership_Mapping` | `Model` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataRedefinition_Mapping` | `Model` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataReferenceUsage_Mapping` | `Model` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointMetadataUsage_Mapping` | `Model` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `ModelViewpointValue_Mapping` | `Model` | `LiteralString` | — | — | ❔ unknown | not yet adjudicated |
| `Model_Mapping` | `Model` | `Package` | — | — | ❔ unknown | not yet adjudicated |
| `PackageImport_Mapping` | `PackageImport` | `NamespaceImport` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIFeatureMembership_Mapping` | `Package` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIFeatureTyping_Mapping` | `Package` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIMetadataFeatureValue_Mapping` | `Package` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIMetadataMembership_Mapping` | `Package` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIMetadataReferenceUsage_Mapping` | `Package` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIMetadataUsage_Mapping` | `Package` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIRedefinition_Mapping` | `Package` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `PackageURIValue_Mapping` | `Package` | `LiteralString` | — | — | ❔ unknown | not yet adjudicated |
| `Package_Mapping` | `Package` | `Package` | — | — | ❔ unknown | not yet adjudicated |
| `ProfileMetadataMembership_Mapping` | `Profile` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ProfileMetadataUsage_Mapping` | `Profile` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Profile_Mapping` | `Profile` | `Package` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeMetadataDefinitionMembership_Mapping` | `Stereotype` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeMetadataDefinition_Mapping` | `Stereotype` | `MetadataDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageFeatureTyping_Mapping` | `Stereotype` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageInfinityReturnParameterMembership_Mapping` | `Stereotype` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageInfinityReturnParameter_Mapping` | `Stereotype` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageMembership_Mapping` | `Stereotype` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageMultiplicityMembership_Mapping` | `Stereotype` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageMultiplicityRangeInfinity_Mapping` | `Stereotype` | `LiteralInfinity` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageMultiplicityRangeMembership_Mapping` | `Stereotype` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsageMultiplicityRange_Mapping` | `Stereotype` | `MultiplicityRange` | — | — | ❔ unknown | not yet adjudicated |
| `StereotypeOccurenceUsage_Mapping` | `Stereotype` | `OccurrenceUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::SimpleClassifiers (22)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AttributeRedefinedFeatureTyping_Mapping` | `StructuralFeature` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `AttributeRedefinedMembership_Mapping` | `Element` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AttributeRedefinedRedefinition_Mapping` | `Property` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AttributeRedefined_Mapping` | `Property` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Attribute_Mapping` | `Property` | `AttributeUsage` | — | — | ❔ unknown | not yet adjudicated |
| `BehavioredClassifierActionUsage_Mapping` | `BehavioredClassifier` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `BehavioredClassifierFeatureMembership_Mapping` | `BehavioredClassifier` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `BehavioredClassifierFeatureTyping_Mapping` | `BehavioredClassifier` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `BehavioredClassifier_Mapping` (abstract) | `BehavioredClassifier` | `Classifier` | — | — | ❔ unknown | not yet adjudicated |
| `DataType_Mapping` | `DataType` | `AttributeDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `EnumerationLiteral_Mapping` | `EnumerationLiteral` | `EnumerationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `EnumerationVariantMembership_Mapping` | `EnumerationLiteral` | `VariantMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Enumeration_Mapping` | `Enumeration` | `EnumerationDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `InterfaceConjugatedPortDefinitionMembership_Mapping` | `Interface` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InterfaceConjugatedPortDefinition_Mapping` | `Interface` | `ConjugatedPortDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `InterfacePortConjugation_Mapping` | `Interface` | `PortConjugation` | — | — | ❔ unknown | not yet adjudicated |
| `InterfaceRealization_Mapping` | `InterfaceRealization` | `Subclassification` | — | — | ❔ unknown | not yet adjudicated |
| `Interface_Mapping` | `Interface` | `PortDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `PrimitiveType_Mapping` | `PrimitiveType` | `AttributeDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ReceptionFeatureTyping_Mapping` | `Reception` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `Reception_Mapping` | `Reception` | `ItemUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Signal_Mapping` | `Signal` | `ItemDefinition` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::StateMachines (26)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `ChangeTriggerReferenceUsage2_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CommonPseudostate_Mapping` (abstract) | `Pseudostate` | `Namespace` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectionPointReference_Mapping` | `ConnectionPointReference` | `StateUsage` | — | — | ❔ unknown | not yet adjudicated |
| `DoBehaviorStateSubactionMembership_Mapping` | `Behavior` | `StateSubactionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `EntryBehaviorStateSubactionMembership_Mapping` | `Behavior` | `StateSubactionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ExitBehaviorStateSubactionMembership_Mapping` | `Behavior` | `StateSubactionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `FinalState_Mapping` | `FinalState` | `StateUsage` | — | — | ❔ unknown | not yet adjudicated |
| `InitialStateSubactionMembership_Mapping` | `Pseudostate` | `StateSubactionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `InitialState_Mapping` | `Pseudostate` | `ActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PseudoState_Mapping` | `Pseudostate` | `StateUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Region_Mapping` | `Region` | `StateUsage` | — | — | ❔ unknown | not yet adjudicated |
| `StateBehaviorPerformActionUsageFeatureTyping_Mapping` | `Behavior` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `StateBehaviorPerformActionUsage_Mapping` | `Behavior` | `PerformActionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `StateBehaviorStateSubactionMembership_Mapping` (abstract) | `Behavior` | `StateSubactionMembership` | — | — | ❔ unknown | not yet adjudicated |
| `StateDefinition_Mapping` | `StateMachine` | `StateDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `State_Mapping` | `State` | `StateUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeTriggerReferenceUsage2_Mapping` | `Trigger` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionSourceToSubsetting_Mapping` | `Transition` | `Subsetting` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionSuccessionSourceMembership_Mapping` | `Transition` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionSuccessionSource_Mapping` | `Transition` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionSuccessionTargetMembership_Mapping` | `Transition` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionSuccessionTarget_Mapping` | `Transition` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionSuccession_Mapping` | `Transition` | `Succession` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionTargetToSubsetting_Mapping` | `Transition` | `Subsetting` | — | — | ❔ unknown | not yet adjudicated |
| `TransitionTriggerFeatureMembership_Mapping` | `Trigger` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `Transition_Mapping` | `Transition` | `TransitionUsage` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::StructuredClassifiers (39)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `AssociationClass_Mapping` | `AssociationClass` | `ConnectionDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationCommon_Mapping` (abstract) | `Association` | `Association` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsageFeatureMembership_Mapping` | `Association` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsageFeatureTyping_Mapping` | `Association` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsageFeatureValue_Mapping` | `Association` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsageFeature_Mapping` | `Association` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsageMembership_Mapping` | `Association` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsageRedefinition_Mapping` | `Association` | `Redefinition` | — | — | ❔ unknown | not yet adjudicated |
| `AssociationMetadataUsage_Mapping` | `Association` | `MetadataUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Class_Mapping` | `Class` | `OccurrenceDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectionDefEndMembership_Mapping` | `Property` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectionDefEnd_Mapping` | `Element` | `Element` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectionEndToSubsetting_Mapping` | `ConnectorEnd` | `Subsetting` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorEndToFeatureCommon_Mapping` (abstract) | `ConnectorEnd` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorEndToMembership_Mapping` | `ConnectorEnd` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorEndToOwnedFeature_Mapping` | `ConnectorEnd` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorEndToSubsettedFeatureMembership_Mapping` | `ConnectorEnd` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorEndToSubsettedFeature_Mapping` | `ConnectorEnd` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorTypeDerived_Mapping` | `Association` | `ConnectionDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `ConnectorType_Mapping` | `Association` | `ConnectionDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `Connector_Mapping` | `Connector` | `ConnectionUsage` | — | — | ❔ unknown | not yet adjudicated |
| `CrossSubsetting_Mapping` | `Property` | `CrossSubsetting` | — | — | ❔ unknown | not yet adjudicated |
| `EndMembership_Mapping` (abstract) | `Property` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `EndToSubsettedFeatureChaining_Mapping` | `Property` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `EndToSubsettedFeature_Mapping` | `Property` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `End_Mapping` (abstract) | `Property` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `MultiplicityReferenceUsage_Mapping` | `Property` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `NonOwnedEndFeatureTyping_Mapping` | `Property` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `NonOwnedEndMembership_Mapping` | `Property` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `NonOwnedEndSubsettingMembership_Mapping` | `Property` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `NonOwnedEndSubsetting_Mapping` | `Property` | `Subsetting` | — | — | ❔ unknown | not yet adjudicated |
| `NonOwnedEndToSubsettedFeatureMembership_Mapping` | `Property` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `NonOwnedEnd_Mapping` | `Property` | `Element` | — | — | ❔ unknown | not yet adjudicated |
| `OwnedEndMembership_Mapping` | `Property` | `EndFeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OwnedEnd_Mapping` | `Property` | `Element` | — | — | ❔ unknown | not yet adjudicated |
| `PortUntyped_Mapping` | `Port` | `PortUsage` | — | — | ❔ unknown | not yet adjudicated |
| `Port_Mapping` | `Port` | `PortUsage` | — | — | ❔ unknown | not yet adjudicated |
| `PropertyToFeatureChaining_Mapping` | `Property` | `FeatureChaining` | — | — | ❔ unknown | not yet adjudicated |
| `QualifierMembership_Mapping` | `StructuralFeature` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::UseCases (14)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `Actor_Mapping` | `Actor` | `PartDefinition` | — | — | ❔ unknown | not yet adjudicated |
| `IncludeFeatureTyping_Mapping` | `Include` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `Include_Mapping` | `Include` | `IncludeUseCaseUsage` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseActorFeatureTyping_Mapping` | `Property` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseActorMembership_Mapping` | `Property` | `ActorMembership` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseActor_Mapping` | `Property` | `PartUsage` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseEmptySubjectReferenceUsage_Mapping` | `UseCase` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseObjectiveMembership_Mapping` | `UseCase` | `ObjectiveMembership` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseObjectiveRequirementUsage_Mapping` | `UseCase` | `RequirementUsage` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseObjectiveSubjectMembership_Mapping` | `UseCase` | `SubjectMembership` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseSubjectFeatureTyping_Mapping` | `UseCase` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseSubjectMembership_Mapping` | `UseCase` | `SubjectMembership` | — | — | ❔ unknown | not yet adjudicated |
| `UseCaseSubjectReferenceUsage_Mapping` | `UseCase` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `UseCase_Mapping` | `UseCase` | `UseCaseDefinition` | — | — | ❔ unknown | not yet adjudicated |

### UML4SysML::Values (33)

| OMG mapping | v1 source | v2 target | Implementation | Test | Status | Reason |
|---|---|---|---|---|---|---|
| `EqualOperatorExpressionFeatureValue_Mapping` | `TypedElement` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `EqualOperatorExpressionFeature_Mapping` | `TypedElement` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `EqualOperatorExpressionOperandParameterMembership_Mapping` | `TypedElement` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ExpressionElseMembership_Mapping` | `Expression` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `ExpressionElseSpecification_Mapping` | `Expression` | `TextualRepresentation` | — | — | ❔ unknown | not yet adjudicated |
| `ExpressionElse_Mapping` | `Expression` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `Expression_Mapping` | `Expression` | `OperatorExpression` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralBoolean_Mapping` | `LiteralBoolean` | `LiteralBoolean` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralInteger_Mapping` | `LiteralInteger` | `LiteralInteger` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralNull_Mapping` | `LiteralNull` | `NullExpression` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralReal_Mapping` | `LiteralReal` | `LiteralRational` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralSpecificationCommon_Mapping` (abstract) | `LiteralSpecification` | `LiteralExpression` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralSpecificationFeatureTyping_Mapping` | `LiteralSpecification` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralString_Mapping` | `LiteralString` | `LiteralString` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralUnlimitedInteger_Mapping` | `LiteralUnlimitedNatural` | `LiteralInteger` | — | — | ❔ unknown | not yet adjudicated |
| `LiteralUnlimitedUnbounded_Mapping` | `LiteralUnlimitedNatural` | `LiteralInfinity` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionAsValue_Mapping` | `OpaqueExpression` | `FeatureChainExpression` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionFeatureFeatureMembership_Mapping` | `OpaqueExpression` | `FeatureMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionFeatureFeature_Mapping` | `OpaqueExpression` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionFeatureValueExpressionMembership_Mapping` | `OpaqueExpression` | `Membership` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionFeatureValueExpression_Mapping` | `OpaqueExpression` | `FeatureReferenceExpression` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionFeatureValue_Mapping` | `OpaqueExpression` | `FeatureValue` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionFeature_Mapping` | `OpaqueExpression` | `Feature` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionMembership_Mapping` | `OpaqueExpression` | `OwningMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionParameterMembership_Mapping` | `OpaqueExpression` | `ParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionReferenceUsageFeatureTyping_Mapping` | `OpaqueExpression` | `FeatureTyping` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionReferenceUsageReturnParameterMembership_Mapping` | `OpaqueExpression` | `ReturnParameterMembership` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionReferenceUsageUntyped_Mapping` | `OpaqueExpression` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionReferenceUsage_Mapping` | `OpaqueExpression` | `ReferenceUsage` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpressionSpecification_Mapping` | `OpaqueExpression` | `TextualRepresentation` | — | — | ❔ unknown | not yet adjudicated |
| `OpaqueExpression_Mapping` | `OpaqueExpression` | `CalculationUsage` | — | — | ❔ unknown | not yet adjudicated |
| `TimeExpression_Mapping` | `TimeExpression` | `Expression` | — | — | ❔ unknown | not yet adjudicated |
| `ValueSpecification_Mapping` (abstract) | `ValueSpecification` | `Expression` | — | — | ❔ unknown | not yet adjudicated |
<!-- census:end rows -->

<!-- census:begin gaps -->
| Rank | OMG mapping | Status | Scope | PSSM suite | Fixtures | Total |
|---|---|---|---|---|---|---|
<!-- census:end gaps -->
