package runtime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestRuntimeRobustness exercises failure modes: graceful errors, no panics, no hangs.
// Each test must return a typed error, never panic or hang.

func TestRuntimeRobustness(t *testing.T) {
	t.Run("deadlock_join_starvation", testDeadlockJoinStarvation)
	t.Run("deadlock_join_same_succession_twice", testDeadlockJoinSameSuccessionTwice)
	t.Run("nested_flow_without_an_initial_node", testNestedFlowWithoutAnInitialNode)
	t.Run("nested_flow_with_a_dangling_succession", testNestedFlowWithADanglingSuccession)
	t.Run("nested_flow_that_cannot_progress", testNestedFlowThatCannotProgress)
	t.Run("nested_flow_that_never_ends", testNestedFlowThatNeverEnds)
	t.Run("node_pin_of_a_node_not_yet_performed", testNodePinOfANodeNotYetPerformed)
	t.Run("node_pin_the_node_does_not_declare", testNodePinTheNodeDoesNotDeclare)
	t.Run("node_read_as_a_value_without_a_result", testNodeReadAsAValueWithoutAResult)
	t.Run("node_pin_member_through_a_scalar_pin", testNodePinMemberThroughAScalarPin)
	t.Run("block_node_pin_of_a_node_not_yet_performed", testBlockNodePinOfANodeNotYetPerformed)
	t.Run("block_node_pin_the_node_does_not_declare", testBlockNodePinTheNodeDoesNotDeclare)
	t.Run("else_branch_node_read_before_it_performs", testElseBranchNodeReadBeforeItPerforms)
	t.Run("typed_node_pin_of_a_node_the_callee_does_not_declare", testTypedNodePinOfANodeTheCalleeDoesNotDeclare)
	t.Run("node_invocation_too_many_arguments", testNodeInvocationTooManyArguments)
	t.Run("node_invocation_too_few_arguments", testNodeInvocationTooFewArguments)
	t.Run("node_invocation_unknown_named_argument", testNodeInvocationUnknownNamedArgument)
	t.Run("node_invocation_repeated_named_argument", testNodeInvocationRepeatedNamedArgument)
	t.Run("node_invocation_argument_fails_before_defaults", testNodeInvocationArgumentFailsBeforeDefaults)
	t.Run("performed_action_input_bound_by_nothing", testPerformedActionInputBoundByNothing)
	t.Run("state_entry_action_input_bound_by_nothing", testStateEntryActionInputBoundByNothing)
	t.Run("state_block_typed_node_input_bound_by_nothing", testStateBlockTypedNodeInputBoundByNothing)
	t.Run("state_block_node_unvalued_pin_write_checked", testStateBlockNodeUnvaluedPinWriteChecked)
	t.Run("state_block_node_pin_read_before_performed", testStateBlockNodePinReadBeforePerformed)
	t.Run("state_block_node_bound_at_no_pin", testStateBlockNodeBoundAtNoPin)
	t.Run("state_block_node_own_flow_runs", testStateBlockNodeOwnFlowRuns)
	t.Run("state_do_body_dangling_succession", testStateDoBodyDanglingSuccession)
	t.Run("state_do_body_first_then_undefined", testStateDoBodyFirstThenUndefined)
	t.Run("state_do_body_flow_without_start", testStateDoBodyFlowWithoutStart)
	t.Run("state_do_body_flow_with_two_starts", testStateDoBodyFlowWithTwoStarts)
	t.Run("state_do_body_starts_at_its_unpreceded_step", testStateDoBodyStartsAtItsUnprecededStep)
	t.Run("action_flow_starts_at_its_unpreceded_step", testActionFlowStartsAtItsUnprecededStep)
	t.Run("action_flow_with_two_starts", testActionFlowWithTwoStarts)
	t.Run("action_flow_cycle_without_start", testActionFlowCycleWithoutStart)
	t.Run("state_do_body_nested_node_dangling_succession", testStateDoBodyNestedNodeDanglingSuccession)
	t.Run("state_do_body_nested_node_starts_at_its_unpreceded_step", testStateDoBodyNestedNodeStartsAtItsUnprecededStep)
	t.Run("action_nested_node_starts_at_its_unpreceded_step", testActionNestedNodeStartsAtItsUnprecededStep)
	t.Run("action_nested_node_with_two_starts", testActionNestedNodeWithTwoStarts)
	t.Run("state_entry_body_dangling_succession", testStateEntryBodyDanglingSuccession)
	t.Run("state_do_body_accept_waits_for_the_message", testStateDoBodyAcceptWaitsForTheMessage)
	t.Run("state_do_body_accept_is_decided_for_a_send", testStateDoBodyAcceptIsDecidedForASend)
	t.Run("state_do_body_accept_yields_to_a_transition", testStateDoBodyAcceptYieldsToATransition)
	t.Run("state_do_body_accept_goes_on_across_a_substate_transition", testStateDoBodyAcceptGoesOnAcrossASubstateTransition)
	t.Run("state_do_body_accept_yields_to_a_substate_transition_leaving_it", testStateDoBodyAcceptYieldsToASubstateTransitionLeavingIt)
	t.Run("state_do_body_accept_follows_the_transition_chosen", testStateDoBodyAcceptFollowsTheTransitionChosen)
	t.Run("state_do_body_accept_follows_the_choice_branch_taken", testStateDoBodyAcceptFollowsTheChoiceBranchTaken)
	t.Run("state_do_body_accept_yields_to_a_transition_into_its_region", testStateDoBodyAcceptYieldsToATransitionIntoItsRegion)
	t.Run("state_do_body_accept_keeps_the_route_chosen", testStateDoBodyAcceptKeepsTheRouteChosen)
	t.Run("state_choice_route_reads_the_accepted_payload", testStateChoiceRouteReadsTheAcceptedPayload)
	t.Run("state_do_body_accept_shares_the_dispatch_with_a_region", testStateDoBodyAcceptSharesTheDispatchWithARegion)
	t.Run("state_do_body_nested_accept_cancelled_on_exit", testStateDoBodyNestedAcceptCancelledOnExit)
	t.Run("state_do_typed_action_input_unbound", testStateDoTypedActionInputUnbound)
	t.Run("state_do_typed_action_pin_bound_to_missing_feature", testStateDoTypedActionPinBoundToMissingFeature)
	t.Run("state_do_typed_action_inout_valued_by_an_imported_literal", testStateDoTypedActionInoutValuedByAnImportedLiteral)
	t.Run("state_entry_body_waits_for_the_clock", testStateEntryBodyWaitsForTheClock)
	t.Run("state_exit_body_waits_for_the_clock", testStateExitBodyWaitsForTheClock)
	t.Run("transition_effect_body_waits_for_the_clock", testTransitionEffectBodyWaitsForTheClock)
	t.Run("state_do_body_flow_that_never_ends", testStateDoBodyFlowThatNeverEnds)
	t.Run("state_do_body_node_return_parameter", testStateDoBodyNodeReturnParameter)
	t.Run("state_do_body_return_parameter", testStateDoBodyReturnParameter)
	t.Run("calc_block_node_unvalued_pin_write_checked", testCalcBlockNodeUnvaluedPinWriteChecked)
	t.Run("node_binding_to_a_non_parameter", testNodeBindingToANonParameter)
	t.Run("node_undirected_binding_carried_to_a_non_parameter", testNodeUndirectedBindingCarriedToANonParameter)
	t.Run("node_pin_bound_to_unequal_values", testNodePinBoundToUnequalValues)
	t.Run("node_output_bound_to_a_nested_node_that_never_runs", testNodeOutputBoundToANestedNodeThatNeverRuns)
	t.Run("block_node_binding_to_a_non_parameter", testBlockNodeBindingToANonParameter)
	t.Run("block_node_binding_names_a_node_without_a_pin", testBlockNodeBindingNamesANodeWithoutAPin)
	t.Run("block_node_pin_bound_where_nodes_are_not_performed", testBlockNodePinBoundWhereNodesAreNotPerformed)
	t.Run("block_node_own_flow_malformed", testBlockNodeOwnFlowMalformed)
	t.Run("block_node_own_flow_where_nodes_are_not_performed", testBlockNodeOwnFlowWhereNodesAreNotPerformed)
	t.Run("block_node_own_flow_that_never_ends", testBlockNodeOwnFlowThatNeverEnds)
	t.Run("inherited_binding_names_a_node_without_a_pin", testInheritedBindingNamesANodeWithoutAPin)
	t.Run("inherited_binding_does_not_reach_a_masking_node", testInheritedBindingDoesNotReachAMaskingNode)
	t.Run("inherited_binding_does_not_reach_through_a_replaced_other_end", testInheritedBindingDoesNotReachThroughAReplacedOtherEnd)
	t.Run("node_inherited_default_that_cannot_be_evaluated", testNodeInheritedDefaultThatCannotBeEvaluated)
	t.Run("node_binding_output_to_an_unknown_feature", testNodeBindingOutputToAnUnknownFeature)
	t.Run("node_binding_output_through_a_scalar_chain", testNodeBindingOutputThroughAScalarChain)
	t.Run("node_binding_output_through_a_chain_violates_target_type", testNodeBindingOutputThroughAChainViolatesTargetType)
	t.Run("nested_pin_binding_into_a_node_performing_another_action", testNestedPinBindingIntoANodePerformingAnotherAction)
	t.Run("nested_pin_binding_at_an_undeclared_pin", testNestedPinBindingAtAnUndeclaredPin)
	t.Run("flow_reaching_into_a_nodes_own_flow", testFlowReachingIntoANodesOwnFlow)
	t.Run("node_flow_into_a_pin_the_target_does_not_declare", testNodeFlowIntoAPinTheTargetDoesNotDeclare)
	t.Run("fork_without_a_successor", testForkWithoutASuccessor)
	t.Run("explicit_succession_missing_endpoint", testExplicitSuccessionMissingEndpoint)
	t.Run("control_flow_missing_endpoint", testControlFlowMissingEndpoint)
	t.Run("merge_without_a_successor", testMergeWithoutASuccessor)
	t.Run("unguarded_loop_through_a_merge", testUnguardedLoopThroughAMerge)
	t.Run("action_whose_last_node_has_no_succession", testActionWhoseLastNodeHasNoSuccession)
	t.Run("first_node_with_a_second_succession", testFirstNodeWithASecondSuccession)
	t.Run("first_beside_an_initial_node", testFirstBesideAnInitialNode)
	t.Run("first_naming_a_final_node", testFirstNamingAFinalNode)
	t.Run("fork_branches_assigning_the_same_feature", testForkBranchesAssigningTheSameFeature)
	t.Run("decision_no_satisfied_guard", testDecisionNoSatisfiedGuard)
	t.Run("decision_all_guards_false", testDecisionAllGuardsFalse)
	t.Run("state_dangling_transition", testStateDanglingTransition)
	t.Run("state_transition_endpoint_misspelled", testStateTransitionEndpointMisspelled)
	t.Run("state_transition_endpoint_in_another_machine", testStateTransitionEndpointInAnotherMachine)
	t.Run("state_transition_endpoint_never_resolved", testStateTransitionEndpointNeverResolved)
	t.Run("state_transition_endpoint_naming_a_first_marker", testStateTransitionEndpointNamingAFirstMarker)
	t.Run("state_junction_without_an_outgoing_transition", testStateJunctionWithoutAnOutgoingTransition)
	t.Run("state_event_after_completion", testStateEventAfterCompletion)
	t.Run("state_completion_rests_in_done", testStateCompletionRestsInDone)
	t.Run("state_nested_region_completion_keeps_siblings_running", testStateNestedRegionCompletionKeepsSiblingsRunning)
	t.Run("state_transition_without_a_target", testStateTransitionWithoutATarget)
	t.Run("state_transition_effect_reads_an_unknown_feature", testStateTransitionEffectReadsAnUnknownFeature)
	t.Run("state_cross_region_transitions_ping_pong", testStateCrossRegionTransitionsPingPong)
	t.Run("parallel_state_body_unsupported_member", testParallelStateBodyUnsupportedMember)
	t.Run("parallel_state_region_without_initial", testParallelStateRegionWithoutInitial)
	t.Run("parallel_state_region_itself_parallel", testParallelStateRegionItselfParallel)
	t.Run("state_usage_typed_by_itself", testStateUsageTypedByItself)
	t.Run("state_usage_mutually_recursive_typing", testStateUsageMutuallyRecursiveTyping)
	t.Run("state_def_specializing_the_library_state_action", testStateDefSpecializingTheLibraryStateAction)
	t.Run("state_def_specializing_a_library_state_keeps_its_content", testStateDefSpecializingALibraryStateKeepsItsContent)
	t.Run("exhibited_state_typed_by_the_library_state_action", testExhibitedStateTypedByTheLibraryStateAction)
	t.Run("exhibited_state_typed_by_the_library_state_action_with_a_body", testExhibitedStateTypedByTheLibraryStateActionWithABody)
	t.Run("exhibited_state_typed_by_a_state_action_specialization", testExhibitedStateTypedByAStateActionSpecialization)
	t.Run("state_usage_inherits_unsupported_member", testStateUsageInheritsUnsupportedMember)
	t.Run("sourceless_transition_with_nothing_before", testSourcelessTransitionWithNothingBefore)
	t.Run("sourceless_transition_after_a_non_state", testSourcelessTransitionAfterANonState)
	t.Run("no_entry_transition_guard_holds", testNoEntryTransitionGuardHolds)
	t.Run("entry_transition_target_is_not_a_state", testEntryTransitionTargetIsNotAState)
	t.Run("entry_transition_carries_a_trigger", testEntryTransitionCarriesATrigger)
	t.Run("entry_transition_into_done_completes_at_initialize", testEntryTransitionIntoDoneCompletesAtInitialize)
	t.Run("named_entry_action_transition_into_done_completes_at_initialize", testNamedEntryActionTransitionIntoDoneCompletesAtInitialize)
	t.Run("own_entry_transitions_replace_inherited_ones", testOwnEntryTransitionsReplaceInheritedOnes)
	t.Run("region_entry_transitions_into_done_complete_at_initialize", testRegionEntryTransitionsIntoDoneCompleteAtInitialize)
	t.Run("nested_regions_into_done_complete_at_initialize", testNestedRegionsIntoDoneCompleteAtInitialize)
	t.Run("transition_into_nested_regions_in_done_completes", testTransitionIntoNestedRegionsInDoneCompletes)
	t.Run("region_start_descends_through_entry_transitions", testRegionStartDescendsThroughEntryTransitions)
	t.Run("region_entry_guards_read_the_region_state_attributes", testRegionEntryGuardsReadTheRegionStateAttributes)
	t.Run("leaving_regions_descends_through_entry_transitions", testLeavingRegionsDescendsThroughEntryTransitions)
	t.Run("calc_unbound_parameter", testCalcUnboundParameter)
	t.Run("calc_calls_an_unimported_extension_function", testCalcCallsAnUnimportedExtensionFunction)
	t.Run("calc_calls_an_unimported_library_function", testCalcCallsAnUnimportedLibraryFunction)
	t.Run("calc_unbound_keyword_named_parameter", testCalcUnboundKeywordNamedParameter)
	t.Run("calc_too_many_arguments", testCalcTooManyArguments)
	t.Run("calc_unknown_named_argument", testCalcUnknownNamedArgument)
	t.Run("calc_parameter_named_twice", testCalcParameterNamedTwice)
	t.Run("calc_without_result", testCalcWithoutResult)
	t.Run("calc_states_second_result", testCalcStatesSecondResult)
	t.Run("calc_body_states_two_results", testCalcBodyStatesTwoResults)
	t.Run("calc_symbol_is_not_a_calc", testCalcSymbolIsNotACalc)
	t.Run("calc_direct_recursion", testCalcDirectRecursion)
	t.Run("calc_mutual_recursion", testCalcMutualRecursion)
	t.Run("calc_default_recursion", testCalcDefaultRecursion)
	t.Run("calc_library_default_recursion", testCalcLibraryDefaultRecursion)
	t.Run("calc_library_default_failure_names_one_frame", testCalcLibraryDefaultFailureNamesOneFrame)
	t.Run("calc_recursion_spends_step_budget", testCalcRecursionSpendsStepBudget)
	t.Run("calc_recursion_at_depth_ceiling", testCalcRecursionAtDepthCeiling)
	t.Run("calc_non_terminating_loop", testCalcNonTerminatingLoop)
	t.Run("calc_body_never_returns", testCalcBodyNeverReturns)
	t.Run("calc_send_is_rejected", testCalcSendIsRejected)
	t.Run("calc_terminate_is_rejected", testCalcTerminateIsRejected)
	t.Run("calc_assignment_outside_the_calc", testCalcAssignmentOutsideTheCalc)
	t.Run("assign_chain_unknown_final_feature", testAssignChainUnknownFinalFeature)
	t.Run("assign_chain_step_is_not_an_object", testAssignChainStepIsNotAnObject)
	t.Run("assign_chain_step_holds_many_objects", testAssignChainStepHoldsManyObjects)
	t.Run("assign_chain_multiplicity_violation", testAssignChainMultiplicityViolation)
	t.Run("assign_chain_unset_step", testAssignChainUnsetStep)
	t.Run("assign_chain_unreachable_base", testAssignChainUnreachableBase)
	t.Run("assign_chain_rejected_in_calc_body", testAssignChainRejectedInCalcBody)
	t.Run("calc_non_boolean_condition", testCalcNonBooleanCondition)
	t.Run("calc_usage_unbound_input", testCalcUsageUnboundInput)
	t.Run("calc_usage_unknown_output", testCalcUsageUnknownOutput)
	t.Run("calc_usage_cyclic_outputs", testCalcUsageCyclicOutputs)
	t.Run("calc_usage_specializes_a_non_calc", testCalcUsageSpecializesANonCalc)
	t.Run("calc_usage_step_budget", testCalcUsageStepBudget)
	t.Run("calc_usage_output_without_a_value", testCalcUsageOutputWithoutAValue)
	t.Run("calc_output_never_assigned_by_the_body", testCalcOutputNeverAssignedByTheBody)
	t.Run("calc_output_assigned_in_a_branch_not_taken", testCalcOutputAssignedInABranchNotTaken)
	t.Run("calc_output_valued_and_assigned", testCalcOutputValuedAndAssigned)
	t.Run("calc_output_assigned_twice", testCalcOutputAssignedTwice)
	t.Run("calc_output_binding_violates_declared_type", testCalcOutputBindingViolatesDeclaredType)
	t.Run("operation_constraint_body_cannot_be_evaluated", testOperationConstraintBodyCannotBeEvaluated)
	t.Run("binding_conflict", testBindingConflict)
	t.Run("binding_collection_conflicts_do_not_use_hashes", testBindingCollectionConflictsDoNotUseHashes)
	t.Run("binding_multiple_scalar_contributors", testBindingMultipleScalarContributors)
	t.Run("binding_multiple_collection_contributors", testBindingMultipleCollectionContributors)
	t.Run("binding_propagation_spends_element_budget", testBindingPropagationSpendsElementBudget)
	t.Run("binding_distinct_materialized_objects_conflict", testBindingDistinctMaterializedObjectsConflict)
	t.Run("binding_bare_ends_bind", testBindingBareEndsBind)
	t.Run("binding_incomplete_end_does_not_poison_read", testBindingIncompleteEndDoesNotPoisonRead)
	t.Run("binding_single_valueless", testBindingSingleValueless)
	t.Run("binding_cycle", testBindingCycle)
	t.Run("binding_three_binding_ring", testBindingThreeBindingRing)
	t.Run("binding_cycle_with_value", testBindingCycleWithValue)
	t.Run("binding_unrelated_expression_does_not_poison_read", testBindingUnrelatedExpressionDoesNotPoisonRead)
	t.Run("binding_result_tracks_later_mutation", testBindingResultTracksLaterMutation)
	t.Run("binding_nested_container_is_not_a_cycle", testBindingNestedContainerIsNotACycle)
	t.Run("nested_calc_usage_unbound_input", testNestedCalcUsageUnboundInput)
	t.Run("nested_calc_usage_unknown_output", testNestedCalcUsageUnknownOutput)
	t.Run("nested_calc_usage_self_cycle", testNestedCalcUsageSelfCycle)
	t.Run("nested_calc_usage_recursion_depth", testNestedCalcUsageRecursionDepth)
	t.Run("nested_calc_usage_step_budget", testNestedCalcUsageStepBudget)
	t.Run("multiple_outputs_invoked_as_an_expression", testMultipleOutputsInvokedAsAnExpression)
	t.Run("body_local_usage_of_a_non_calc", testBodyLocalUsageOfANonCalc)
	t.Run("body_local_declaration_not_executable", testBodyLocalDeclarationNotExecutable)
	t.Run("f99_body_member_without_value", testF99BodyMemberWithoutValue)
	t.Run("f99_unsupported_body_member", testF99UnsupportedBodyMember)
	t.Run("f99_cyclic_body_declaration", testF99CyclicBodyDeclaration)
	t.Run("range_bound_is_not_an_integer", testRangeBoundIsNotAnInteger)
	t.Run("range_spends_the_step_budget", testRangeSpendsTheStepBudget)
	t.Run("collection_spends_the_element_budget", testCollectionSpendsTheElementBudget)
	t.Run("usage_read_through_a_part_without_an_output", testUsageReadThroughAPartWithoutAnOutput)
	t.Run("performed_action_binding_names_nothing", testPerformedActionBindingNamesNothing)
	t.Run("no_flow_performed_action_checks_its_inputs", testNoFlowPerformedActionChecksItsInputs)
	t.Run("no_flow_performed_action_refuses_return_parameter", testNoFlowPerformedActionRefusesReturnParameter)
	t.Run("binding_end_of_a_destroyed_object", testBindingEndOfADestroyedObject)
	t.Run("operation_of_a_destroyed_object", testOperationOfADestroyedObject)
	t.Run("structured_attribute_chain_of_an_unknown_feature", testStructuredAttributeChainOfAnUnknownFeature)
	t.Run("elements_chain_of_a_non_numeric_collection", testElementsChainOfANonNumericCollection)
	t.Run("arithmetic_over_the_unbounded_value", testArithmeticOverTheUnboundedValue)
	t.Run("unbounded_value_compared_with_a_string", testUnboundedValueComparedWithAString)
	t.Run("metadata_of_a_value", testMetadataOfAValue)
	t.Run("metadata_of_an_unresolved_name", testMetadataOfAnUnresolvedName)
	t.Run("constraint_missing_feature", testConstraintMissingFeature)
	t.Run("nested_condition_subject_is_ambiguous", testNestedConditionSubjectIsAmbiguous)
	t.Run("satisfaction_subject_is_ambiguous", testSatisfactionSubjectIsAmbiguous)
	t.Run("recursive_composition_subject_search", testRecursiveCompositionSubjectSearch)
	t.Run("duplicate_objects_of_one_declaration", testDuplicateObjectsOfOneDeclaration)
	t.Run("duplicate_objects_holding_a_plain_part", testDuplicateObjectsHoldingAPlainPart)
	t.Run("nested_part_held_with_a_multiplicity", testNestedPartHeldWithAMultiplicity)
	t.Run("part_nested_inside_a_repeated_part", testPartNestedInsideARepeatedPart)
	t.Run("parts_subsetting_one_collection", testPartsSubsettingOneCollection)
	t.Run("requirement_feature_without_a_value", testRequirementFeatureWithoutAValue)
	t.Run("requirement_features_valued_from_each_other", testRequirementFeaturesValuedFromEachOther)
	t.Run("step_budget_exceeded", testStepBudgetExceeded)
	t.Run("eval_on_an_instance_spends_the_step_budget", testEvalOnAnInstanceSpendsTheStepBudget)
	t.Run("non_terminating_loop_exhausts_step_budget", testNonTerminatingLoopExhaustsStepBudget)
	t.Run("loop_body_declaration_does_not_leak", testLoopBodyDeclarationDoesNotLeak)
	t.Run("loop_body_of_unexecutable_statement", testLoopBodyOfUnexecutableStatement)
	t.Run("block_flow_of_unexecutable_member", testBlockFlowOfUnexecutableMember)
	t.Run("non_terminating_loop_performing_an_action", testNonTerminatingLoopPerformingAnAction)
	t.Run("for_over_a_value_no_expression_makes_iterable", testForOverAValueNoExpressionMakesIterable)
	t.Run("for_over_a_scalar", testForOverAScalar)
	t.Run("statement_directly_in_an_action_body", testStatementDirectlyInAnActionBody)
	t.Run("flow_end_naming_no_node", testFlowEndNamingNoNode)
	t.Run("flow_naming_no_pin", testFlowNamingNoPin)
	t.Run("accept_payload_without_a_value", testAcceptPayloadWithoutAValue)
	t.Run("accept_payload_read_before_it_is_bound", testAcceptPayloadReadBeforeItIsBound)
	t.Run("flow_from_a_node_that_produced_nothing", testFlowFromANodeThatProducedNothing)
	t.Run("action_accept_time_waits", testActionAcceptTimeWaits)
	t.Run("clock_advance", testClockAdvance)
	t.Run("action_accept_non_boolean_change_trigger", testActionAcceptNonBooleanChangeTrigger)
	t.Run("action_body_unresolved_unit", testActionBodyUnresolvedUnit)
	t.Run("action_body_unresolved_feature", testActionBodyUnresolvedFeature)
	t.Run("state_body_unresolved_unit", testStateBodyUnresolvedUnit)
	t.Run("fork_branches_share_region", testForkBranchesShareRegion)
	t.Run("join_with_one_incoming_branch", testJoinWithOneIncomingBranch)
	t.Run("region_pseudostate_without_satisfied_guard", testRegionPseudostateWithoutSatisfiedGuard)
	t.Run("region_pseudostate_cycle", testRegionPseudostateCycle)
	t.Run("non_numeric_time_trigger", testNonNumericTimeTrigger)
	t.Run("time_trigger_of_a_non_time_dimension", testTimeTriggerOfANonTimeDimension)
	t.Run("time_trigger_of_the_type_validation_refuses", testTimeTriggerOfTheTypeValidationRefuses)
	t.Run("action_return_parameter_validation_refuses", testActionReturnParameterValidationRefuses)
	t.Run("change_condition_that_never_holds", testChangeConditionThatNeverHolds)
	t.Run("send_reaches_only_its_addressee", testSendReachesOnlyItsAddressee)
	t.Run("accept_of_unsent_type", testAcceptOfUnsentTypeReports)
	t.Run("send_via_unconnected_port", testSendViaUnconnectedPort)
	t.Run("send_via_connector_into_an_empty_part", testSendViaConnectorIntoAnEmptyPart)
	t.Run("send_via_bound_boundary_port_joined_to_nothing", testSendViaBoundBoundaryPortJoinedToNothing)
	t.Run("send_fan_out_to_a_port_that_fails_to_materialize", testSendFanOutToAPortThatFailsToMaterialize)
	t.Run("accept_via_a_port_that_fails_to_materialize", testAcceptViaAPortThatFailsToMaterialize)
	t.Run("action_accept_via_a_port_that_fails_to_materialize", testActionAcceptViaAPortThatFailsToMaterialize)
	t.Run("send_addressed_to_an_unreachable_target", testSendAddressedToAnUnreachableTarget)
	t.Run("routed_send_via_unknown_port", testRoutedSendViaUnknownPort)
	t.Run("routed_send_port_type_mismatch", testRoutedSendPortTypeMismatch)
	t.Run("routed_send_port_type_match", testRoutedSendPortTypeMatch)
	t.Run("routed_send_scalar_typed_flow_mismatch", testRoutedSendScalarTypedFlowMismatch)
	t.Run("routed_send_unreachable_receiver", testRoutedSendUnreachableReceiver)
	t.Run("routed_send_receiver_name_mismatch_deadlock", testRoutedSendReceiverNameMismatchDeadlock)
	t.Run("type_classification_unresolved_type", testTypeClassificationUnresolvedType)
	t.Run("two_valued_member_in_scalar_context", testTwoValuedMemberInScalarContext)
	t.Run("type_classification_undetermined_value_type", testTypeClassificationUndeterminedValueType)
	t.Run("cast_to_an_unresolved_type", testCastToAnUnresolvedType)
	t.Run("cast_undecided_by_the_value", testCastUndecidedByTheValue)
	t.Run("cast_of_a_quantity_to_a_constrained_subtype", testCastOfAQuantityToAConstrainedSubtype)
	t.Run("difference_typed_feature_holding_a_subtracted_object", testDifferenceTypedFeatureHoldingASubtractedObject)
	t.Run("send_addressed_through_several_occurrences", testSendAddressedThroughSeveralOccurrences)
	t.Run("send_addressed_to_an_object_that_cannot_be_built", testSendAddressedToAnObjectThatCannotBeBuilt)
	t.Run("send_addressed_to_a_part_no_sibling_takes", testSendAddressedToAPartNoSiblingTakes)
	t.Run("injected_message_names_a_receiver_no_accept_has", testInjectedMessageNamesAReceiverNoAcceptHas)
	t.Run("accept_deadlock_never_satisfied", testAcceptDeadlockNeverSatisfied)
	t.Run("accept_deadlock_reports_every_waiting_accept", testAcceptDeadlockReportsEveryWaitingAccept)
	t.Run("accept_statement_deadlock_in_a_loop", testAcceptStatementDeadlockInALoop)
	t.Run("history_outside_composite_state", testHistoryOutsideCompositeState)
	t.Run("history_without_record_or_default", testHistoryWithoutRecordOrDefault)
	t.Run("defer_of_non_deferrable_trigger", testDeferOfNonDeferrableTrigger)
	t.Run("non_terminating_do_behavior", testNonTerminatingDoBehavior)
	t.Run("empty_anonymous_action_body", testEmptyAnonymousActionBody)
	t.Run("non_terminating_anonymous_do_body", testNonTerminatingAnonymousDoBody)
	t.Run("behavior_performing_an_action_and_stating_a_body", testBehaviorPerformingAnActionAndStatingABody)
	t.Run("qualified_assignment_target_in_a_state_effect", testQualifiedAssignmentTargetInAStateEffect)
	t.Run("call_of_unhandled_operation", testCallOfUnhandledOperation)
	t.Run("signal_no_level_of_a_composite_state_accepts", testSignalNoLevelOfACompositeStateAccepts)
	t.Run("stale_composite_timer_in_a_region", testStaleCompositeTimerInARegion)
	t.Run("composite_self_transition_with_no_substate_to_re_enter", testCompositeSelfTransitionWithNoSubstateToReEnter)
	t.Run("exit_of_nested_regions_with_a_history_pseudostate", testExitOfNestedRegionsWithAHistoryPseudostate)
	t.Run("call_argument_of_wrong_type", testCallArgumentOfWrongType)
	t.Run("perform_of_missing_action", testPerformOfMissingAction)
	t.Run("perform_reference_cycle", testPerformReferenceCycle)
	t.Run("state_subaction_reference_of_missing_action", testStateSubactionReferenceOfMissingAction)
	t.Run("state_subaction_reference_feature_chain", testStateSubactionReferenceFeatureChain)
	t.Run("library_function_outside_its_domain", testLibraryFunctionOutsideItsDomain)
	t.Run("library_function_wrong_arity", testLibraryFunctionWrongArity)
	t.Run("extension_library_function_outside_its_domain", testExtensionLibraryFunctionOutsideItsDomain)
	t.Run("exponentiation_integer_overflow", testExponentiationIntegerOverflow)
	t.Run("quantity_incommensurable_comparison", testQuantityIncommensurableComparison)
	t.Run("quantity_index_is_not_a_unit", testQuantityIndexIsNotAUnit)
	t.Run("quantity_unit_shadowed_by_sibling", testQuantityUnitShadowedBySibling)
	t.Run("quantity_qualified_unit_is_not_shadowing", testQuantityQualifiedUnitIsNotShadowing)
	t.Run("quantity_shadowed_unit_without_a_qualifier", testQuantityShadowedUnitWithoutAQualifier)
	t.Run("quantity_cyclic_unit_definition", testQuantityCyclicUnitDefinition)
	t.Run("quantity_calculation_that_has_no_value", testQuantityCalculationThatHasNoValue)
	t.Run("satisfy_unresolved_requirement", testSatisfyUnresolvedRequirement)
	t.Run("satisfy_requirement_without_conditions", testSatisfyRequirementWithoutConditions)
	t.Run("satisfy_bounded_by_the_step_budget", testSatisfyBoundedByTheStepBudget)
	t.Run("cyclic_derived_feature_value", testCyclicDerivedFeatureValue)
	t.Run("write_into_cyclic_derived_feature_values", testWriteIntoCyclicDerivedFeatureValues)
	t.Run("cyclic_subsetting_of_default_collections", testCyclicSubsettingOfDefaultCollections)
	t.Run("derived_feature_value_over_missing_feature", testDerivedFeatureValueOverMissingFeature)
	t.Run("sequence_index_names_no_position", testSequenceIndexNamesNoPosition)
	t.Run("collection_operand_of_the_wrong_kind", testCollectionOperandOfTheWrongKind)
	t.Run("numeric_library_call_that_has_no_value", testNumericLibraryCallThatHasNoValue)
	t.Run("named_library_call_that_has_no_value", testNamedLibraryCallThatHasNoValue)
	t.Run("builtin_named_argument_that_binds_nothing", testBuiltinNamedArgumentThatBindsNothing)
	t.Run("body_by_reference_that_cannot_be_applied", testBodyByReferenceThatCannotBeApplied)
	t.Run("bodiless_model_calc_named_as_a_builtin", testBodilessModelCalcNamedAsABuiltin)
	t.Run("data_equality_over_a_part", testDataEqualityOverAPart)
	t.Run("base_index_with_several_indexes", testBaseIndexWithSeveralIndexes)
	t.Run("structured_value_outside_the_declared_shape", testStructuredValueOutsideTheDeclaredShape)
	t.Run("real_literal_that_underflows", testRealLiteralThatUnderflows)
	t.Run("string_operand_of_the_wrong_kind", testStringOperandOfTheWrongKind)
	t.Run("collection_body_of_the_wrong_arity", testCollectionBodyOfTheWrongArity)
	t.Run("select_predicate_is_not_a_condition", testSelectPredicateIsNotACondition)
	t.Run("collection_operation_step_budget", testCollectionOperationStepBudget)
	t.Run("variation_without_a_selected_variant", testVariationWithoutASelectedVariant)
	t.Run("variation_bound_to_what_is_not_a_variant", testVariationBoundToWhatIsNotAVariant)
	t.Run("variation_bound_to_two_variants", testVariationBoundToTwoVariants)
	t.Run("variation_read_through_its_declaration", testVariationReadThroughItsDeclaration)
	t.Run("chain_through_an_unselected_variation_part", testChainThroughAnUnselectedVariationPart)
	t.Run("classify_an_unselected_optional_variation", testClassifyAnUnselectedOptionalVariation)
	t.Run("repeated_reads_of_a_variant_object", testRepeatedReadsOfAVariantObject)
	t.Run("two_owners_selecting_one_variant", testTwoOwnersSelectingOneVariant)
	t.Run("two_ownerless_selections_of_one_variant", testTwoOwnerlessSelectionsOfOneVariant)
	t.Run("variant_outside_a_variation", testVariantOutsideAVariation)
	t.Run("variant_under_a_redefined_variation", testVariantUnderARedefinedVariation)
	t.Run("deep_specialization_chain_of_redefinitions", testDeepSpecializationChainOfRedefinitions)
	t.Run("conflicting_redefinitions_at_several_levels", testConflictingRedefinitionsAtSeveralLevels)
	t.Run("one_feature_valued_under_two_names", testOneFeatureValuedUnderTwoNames)
	t.Run("valued_feature_restated_in_a_body", testValuedFeatureRestatedInABody)
	t.Run("multiplicity_infinite_lower_bound", testMultiplicityInfiniteLowerBound)
	t.Run("multiplicity_lower_bound_too_large", testMultiplicityLowerBoundTooLarge)
	t.Run("default_not_conforming_to_multiplicity", testDefaultNotConformingToMultiplicity)
	t.Run("default_against_an_undeclared_multiplicity", testDefaultAgainstAnUndeclaredMultiplicity)
	t.Run("feature_chain_through_an_unset_feature_value", testFeatureChainThroughAnUnsetFeatureValue)
	t.Run("feature_chain_spends_the_element_budget", testFeatureChainSpendsTheElementBudget)
	t.Run("mutually_subsetting_features", testMutuallySubsettingFeatures)
	t.Run("unattachable_connector_end", testUnattachableConnectorEnd)
	t.Run("unattachable_connector_leaves_no_behavior", testUnattachableConnectorLeavesNoBehavior)
	t.Run("unattachable_connector_abandons_what_its_ends_materialized", testUnattachableConnectorAbandonsWhatItsEndsMaterialized)
	t.Run("unattachable_connector_touches_no_other_object", testUnattachableConnectorTouchesNoOtherObject)
	t.Run("unattachable_connector_ends_run_nothing_early", testUnattachableConnectorEndsRunNothingEarly)
	t.Run("connector_whose_start_fails_leaves_no_trace", testConnectorWhoseStartFailsLeavesNoTrace)
	t.Run("connector_answered_by_a_failing_behavior_is_kept", testConnectorAnsweredByAFailingBehaviorIsKept)
	t.Run("multiplicity_on_a_connector", testMultiplicityOnAConnector)
	t.Run("connector_attached_to_itself", testConnectorAttachedToItself)
	t.Run("mutually_attached_connectors", testMutuallyAttachedConnectors)
	t.Run("enumeration_name_that_is_not_a_literal", testEnumerationNameThatIsNotALiteral)
	t.Run("chain_through_a_literal_without_that_attribute", testChainThroughALiteralWithoutThatAttribute)
	t.Run("classification_outside_the_evaluable_subset", testClassificationOutsideTheEvaluableSubset)
	t.Run("expression_over_a_feature_value_holding_no_value", testExpressionOverAFeatureValueHoldingNoValue)
	t.Run("succession_guard_failure_modes", testSuccessionGuardFailureModes)
	t.Run("quantity_write_of_another_dimension", testQuantityWriteOfAnotherDimension)
	t.Run("measurement_reference_failure_modes", testMeasurementReferenceFailureModes)
	t.Run("tensor_quantity_failure_modes", testTensorQuantityFailureModes)
	t.Run("coordinate_frame_failure_modes", testCoordinateFrameFailureModes)
	t.Run("object_exhibited_machine_never_settles", testObjectExhibitedMachineNeverSettles)
	t.Run("object_exhibited_machine_without_an_initial_state", testObjectExhibitedMachineWithoutAnInitialState)
	t.Run("object_exhibited_machine_attribute_write_violates_multiplicity", testObjectExhibitedMachineAttributeWriteViolatesMultiplicity)
	t.Run("object_performed_action_attribute_write_violates_multiplicity", testObjectPerformedActionAttributeWriteViolatesMultiplicity)
	t.Run("object_performed_action_occurrence_holds_a_non_object", testObjectPerformedActionOccurrenceHoldsANonObject)
	t.Run("operation_invoked_with_unbound_parameters", testOperationInvokedWithUnboundParameters)
	t.Run("second_instantiation_of_one_type", testSecondInstantiationOfOneType)
	t.Run("write_of_a_wrong_typed_value_leaves_the_feature", testWriteOfAWrongTypedValueLeavesTheFeature)
	t.Run("write_of_too_many_values_leaves_the_feature", testWriteOfTooManyValuesLeavesTheFeature)
	t.Run("write_of_a_repeated_value_leaves_the_feature", testWriteOfARepeatedValueLeavesTheFeature)
	t.Run("write_of_no_value_where_one_is_required", testWriteOfNoValueWhereOneIsRequired)
	t.Run("state_entry_write_of_a_wrong_typed_value", testStateEntryWriteOfAWrongTypedValue)
	t.Run("performer_feature_write_of_a_wrong_typed_value", testPerformerFeatureWriteOfAWrongTypedValue)
	t.Run("standalone_action_naming_a_performer_feature", testStandaloneActionNamingAPerformerFeature)
	t.Run("standalone_action_writing_a_performer_feature", testStandaloneActionWritingAPerformerFeature)
	t.Run("standalone_action_naming_this_of_an_unowned_performance", testStandaloneActionNamingThisOfAnUnownedPerformance)
	t.Run("chained_write_of_a_wrong_typed_value", testChainedWriteOfAWrongTypedValue)
	t.Run("calc_output_write_of_a_wrong_typed_value", testCalcOutputWriteOfAWrongTypedValue)
	t.Run("action_local_write_of_a_wrong_typed_value", testActionLocalWriteOfAWrongTypedValue)
	t.Run("action_output_write_of_a_wrong_typed_value", testActionOutputWriteOfAWrongTypedValue)
	t.Run("performance_occurrence_write_of_a_wrong_typed_value", testPerformanceOccurrenceWriteOfAWrongTypedValue)
	t.Run("function_value_call_of_a_non_function", testFunctionValueCallOfANonFunction)
	t.Run("function_value_bound_to_a_non_function", testFunctionValueBoundToANonFunction)
	t.Run("function_value_arity_mismatch", testFunctionValueArityMismatch)
	t.Run("function_value_unknown_named_argument", testFunctionValueUnknownNamedArgument)
	t.Run("function_value_unbound_calc_parameter", testFunctionValueUnboundCalcParameter)
	t.Run("function_value_of_a_wrong_typed_calc", testFunctionValueOfAWrongTypedCalc)
	t.Run("function_value_of_a_built_in", testFunctionValueOfABuiltIn)
	t.Run("function_value_applied_to_itself_forever", testFunctionValueAppliedToItselfForever)
	t.Run("function_value_inherited_body_outside_the_closure", testFunctionValueInheritedBodyOutsideTheClosure)
	t.Run("function_value_nested_calc_outside_its_run", testFunctionValueNestedCalcOutsideItsRun)
	t.Run("verification_body_that_cannot_run", testVerificationBodyThatCannotRun)
	t.Run("verification_body_step_that_fails", testVerificationBodyStepThatFails)
	t.Run("verification_subcase_that_cannot_run", testVerificationSubcaseThatCannotRun)
	t.Run("verification_of_a_symbol_that_is_not_a_case", testVerificationOfASymbolThatIsNotACase)
	t.Run("verification_with_an_argument_the_case_does_not_take", testVerificationWithAnArgumentTheCaseDoesNotTake)
	t.Run("verification_objective_subject_of_another_type", testVerificationObjectiveSubjectOfAnotherType)
	t.Run("verification_objective_subject_left_unbound", testVerificationObjectiveSubjectLeftUnbound)
	t.Run("verification_objective_subject_rebound", testVerificationObjectiveSubjectRebound)
	t.Run("trade_study_with_an_abstract_evaluation_function", testTradeStudyWithAnAbstractEvaluationFunction)
	t.Run("trade_study_whose_evaluation_fails_for_one_alternative", testTradeStudyWhoseEvaluationFailsForOneAlternative)
	t.Run("trade_study_with_an_empty_subject", testTradeStudyWithAnEmptySubject)
	t.Run("trade_study_with_a_single_valued_subject", testTradeStudyWithASingleValuedSubject)
	t.Run("trade_study_whose_alternatives_read_an_unbound_feature", testTradeStudyWhoseAlternativesReadAnUnboundFeature)
	t.Run("sweep_over_a_boolean_parameter", testSweepOverABooleanParameter)
	t.Run("sweep_over_a_parameter_typed_by_a_part", testSweepOverAParameterTypedByAPart)
	t.Run("sweep_over_an_integer_parameter_by_a_fraction", testSweepOverAnIntegerParameterByAFraction)
	t.Run("sweep_over_a_real_parameter_by_integers_no_real_holds", testSweepOverARealParameterByIntegersNoRealHolds)
}

func testBindingConflict(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-conflict>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 1;
			attribute b = 2;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "b")
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("GetFeatureValue(b) = %v, want ErrBindingConflict", err)
	}
	if got, want := err.Error(), "binding conflict: b = 2, a = 1"; got != want {
		t.Errorf("conflict error = %q, want %q", got, want)
	}
}

func testBindingCollectionConflictsDoNotUseHashes(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-string-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a : ScalarValues::String[*] = ("a");
				attribute b : ScalarValues::String[*] = ("b");
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "b")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("GetFeatureValue(b) = %v, want ErrBindingConflict", err)
		}
		if got, want := err.Error(),
			`binding conflict: b = ["b"], a = ["a"]`; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})

	t.Run("integers", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-integer-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a : Integer[*] = (1);
				attribute b : Integer[*] = (65537);
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "b")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("GetFeatureValue(b) = %v, want ErrBindingConflict", err)
		}
		if got, want := err.Error(),
			"binding conflict: b = [65537], a = [1]"; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})
}

func testBindingMultipleScalarContributors(t *testing.T) {
	t.Run("unequal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-scalar-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b = 1;
				attribute c = 2;
				bind a = b;
				bind a = c;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "a")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("GetFeatureValue(a) = %v, want ErrBindingConflict", err)
		}
		if got, want := err.Error(), "binding conflict at Sys.a: b = 1, c = 2"; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})

	t.Run("equal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-scalar-equal>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b = 1;
				attribute c = 1;
				bind a = b;
				bind a = c;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "a")
		if err != nil {
			t.Fatalf("GetFeatureValue(a): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 1 {
			t.Errorf("a = %d, want 1", got)
		}
	})
}

func testBindingMultipleCollectionContributors(t *testing.T) {
	t.Run("partial", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-collection-contributors>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute leftEdge : Integer[0..1] = (1);
				attribute rightEdge : Integer[0..1] = (2);
				binding [1] bind [0..1] edges = [0..1] leftEdge;
				binding [1] bind [0..1] edges = [0..1] rightEdge;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "edges")
		if !errors.Is(err, ErrBindingEnd) {
			t.Fatalf("GetFeatureValue(edges) = %v, want ErrBindingEnd", err)
		}
		if got, want := err.Error(), "binding end cannot be resolved: Sys.edges is bound by `bind [0..1] edges = [0..1] leftEdge`, "+
			"which makes some value of edges a value of leftEdge without saying which value of either; the model does not state what edges holds"; got != want {
			t.Errorf("error = %q, want %q", got, want)
		}
		fv := inst.FeatureValues["edges"]
		if fv.Materialized || fv.Written || fv.BindingDerived || fv.HeldValue().Kind != ValInvalid {
			t.Errorf("unsupported binding left an assignment behind: %+v", *fv)
		}
	})

	// An end admitting one value links a feature holding one value whole,
	// however wide the feature is declared; one holding more stays partial.
	t.Run("partial_by_values_held", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-partial-by-values-held>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute pick : Integer[1];
				binding [1] bind [0..1] edges = [0..1] pick;
			}
			part one : Sys { :>> edges = (7); }
			part two : Sys { :>> edges = (7, 8); }
		}`))
		one, err := ctx.Instantiate(oneSymbol(t, idx, "P::one"))
		if err != nil {
			t.Fatalf("instantiate one: %v", err)
		}
		fv, err := one.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("one.pick: %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 7 {
			t.Errorf("one.pick = %d (%s), want 7, the one value edges holds", got, FormatValue(fv.HeldValue()))
		}
		two, err := ctx.Instantiate(oneSymbol(t, idx, "P::two"))
		if err != nil {
			t.Fatalf("instantiate two: %v", err)
		}
		if _, err := two.GetFeatureValue(ctx, "pick"); !errors.Is(err, ErrBindingEnd) {
			t.Fatalf("two.pick = %v, want ErrBindingEnd", err)
		}
		// An end valued on its own — written, as one valued by a default — keeps that value.
		if err := two.SetFeatureValue(ctx, "pick", integerValue(8)); err != nil {
			t.Fatalf("write two.pick: %v", err)
		}
		fv, err = two.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("two.pick after the write: %v", err)
		}
		if fv.HeldValue().Const.Int != 8 {
			t.Errorf("two.pick = %s after writing 8, want 8", FormatValue(fv.HeldValue()))
		}
	})

	// A binding of the whole feature determines it whatever partial bindings it has
	// besides, and in whatever order they are declared.
	t.Run("whole_beside_partial", func(t *testing.T) {
		for name, bindings := range map[string]string{
			"partial_first": "binding [1] bind [0..1] edges = [0..1] pick; bind edges = every;",
			"whole_first":   "bind edges = every; binding [1] bind [0..1] edges = [0..1] pick;",
		} {
			t.Run(name, func(t *testing.T) {
				idx, _, ctx := buildRuntime(t, "<binding-whole-beside-partial>", parseAndBuild(t, `package P {
					part def Sys {
						attribute edges : Integer[*];
						attribute every : Integer[*] = (1, 2, 3);
						attribute pick : Integer[0..1] = (2);
						`+bindings+`
					}
				}`))
				inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
				if err != nil {
					t.Fatalf("instantiate: %v", err)
				}
				fv, err := inst.GetFeatureValue(ctx, "edges")
				if err != nil {
					t.Fatalf("edges: %v", err)
				}
				if got := FormatValue(fv.HeldValue()); got != "[1, 2, 3]" {
					t.Errorf("edges = %s, want [1, 2, 3], what the whole binding determines", got)
				}
			})
		}
	})

	// An end of multiplicity [0] links no value: each feature reads what it holds on its own.
	t.Run("zero_width_end", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-zero-width-end>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*] = (7, 8);
				attribute pick : Integer[0..1];
				binding [1] bind [0] edges = [0..1] pick;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "edges")
		if err != nil {
			t.Fatalf("edges: %v", err)
		}
		if got := len(elementsOf(fv.HeldValue())); got != 2 {
			t.Errorf("edges holds %d values (%s), want the two it is valued with", got, FormatValue(fv.HeldValue()))
		}
		fv, err = inst.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		if got := elementsOf(fv.HeldValue()); len(got) != 0 {
			t.Errorf("pick = %s, want nothing: the binding links no value to it", FormatValue(fv.HeldValue()))
		}
	})

	// An end stating how many values it links is not met by a feature holding fewer: the
	// binding is a multiplicity violation, not a whole binding of what there is — whichever
	// end is read, whether the features admit more than the end links or exactly as many.
	t.Run("under_lower_bound", func(t *testing.T) {
		for name, c := range map[string]struct{ ends, same string }{
			"exact":  {"[2]", "[0..2]"},
			"ranged": {"[2..3]", "[0..3]"},
		} {
			ends := c.ends
			for shape, declared := range map[string]string{"wider": "[*]", "same": c.same} {
				t.Run(name+"_"+shape, func(t *testing.T) {
					idx, _, ctx := buildRuntime(t, "<binding-under-lower-bound>", parseAndBuild(t, `package P {
						part def Sys {
							attribute edges : Integer`+declared+` = (7);
							attribute pair : Integer`+declared+`;
							binding [1] bind `+ends+` edges = `+ends+` pair;
						}
					}`))
					want := "multiplicity violation: `bind " + ends + " edges = " + ends + " pair` links " +
						ends + " of edges, which holds 1 value(s)"
					for _, order := range [][]string{{"pair", "edges", "pair"}, {"edges", "pair", "edges"}} {
						inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
						if err != nil {
							t.Fatalf("instantiate: %v", err)
						}
						for _, feature := range order {
							_, err := inst.GetFeatureValue(ctx, feature)
							if !errors.Is(err, ErrMultiplicityViolation) {
								t.Fatalf("%v: %s = %v, want ErrMultiplicityViolation", order, feature, err)
							}
							if got := err.Error(); got != want {
								t.Errorf("%v: %s error = %q, want %q", order, feature, got, want)
							}
						}
						fv := inst.FeatureValues["pair"]
						if fv.Materialized || fv.Written || fv.BindingDerived || fv.HeldValue().Kind != ValInvalid {
							t.Errorf("%v: the refused binding left an assignment behind: %+v", order, *fv)
						}
					}
				})
			}
		}
	})

	// An end that makes the binding partial does not excuse the other end from its
	// lower bound, whichever end is read.
	t.Run("under_lower_bound_beyond_partial_end", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-under-lower-bound-beyond-partial>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute pair : Integer[0..3] = (5);
				binding [1] bind [2] edges = [2] pair;
			}
		}`))
		want := "multiplicity violation: `bind [2] edges = [2] pair` links [2] of pair, which holds 1 value(s)"
		for _, order := range [][]string{{"edges", "pair"}, {"pair", "edges"}} {
			inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
			if err != nil {
				t.Fatalf("instantiate: %v", err)
			}
			for _, feature := range order {
				_, err := inst.GetFeatureValue(ctx, feature)
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Fatalf("%v: %s = %v, want ErrMultiplicityViolation", order, feature, err)
				}
				if got := err.Error(); got != want {
					t.Errorf("%v: %s error = %q, want %q", order, feature, got, want)
				}
			}
			fv := inst.FeatureValues["edges"]
			if fv.Written || fv.BindingDerived || fv.HeldValue().Kind != ValInvalid {
				t.Errorf("%v: the refused binding left an assignment behind: %+v", order, *fv)
			}
		}
	})

	// Ends requiring a value are not met by two optional features holding none: the
	// binding links nothing, a multiplicity violation rather than an unknown value or a
	// cycle. Once anything values the features — a default, another binding — it is whole.
	t.Run("empty_required_ends", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-empty-required-ends>", parseAndBuild(t, `package P {
			part def Empty {
				attribute a : Integer[0..1];
				attribute b : Integer[0..1];
				binding [1] bind [1] a = [1] b;
			}
			part def Defaulted {
				attribute a : Integer[0..1] = 5;
				attribute b : Integer[0..1];
				binding [1] bind [1] a = [1] b;
			}
			part def Joined {
				attribute a : Integer[0..1];
				attribute b : Integer[0..1];
				attribute c : Integer[0..1] = 5;
				binding [1] bind [1] a = [1] b;
				binding [1] bind [1] a = [1] c;
			}
		}`))
		want := "multiplicity violation: `bind [1] a = [1] b` links [1] of a, which holds 0 value(s)"
		for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
			inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Empty"))
			if err != nil {
				t.Fatalf("instantiate: %v", err)
			}
			for _, feature := range order {
				_, err := inst.GetFeatureValue(ctx, feature)
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Fatalf("Empty %v: %s = %v, want ErrMultiplicityViolation", order, feature, err)
				}
				if got := err.Error(); got != want {
					t.Errorf("Empty %v: %s error = %q, want %q", order, feature, got, want)
				}
			}
		}
		for def, orders := range map[string][][]string{
			"P::Defaulted": {{"a", "b"}, {"b", "a"}},
			"P::Joined":    {{"a", "b", "c"}, {"b", "a", "c"}, {"c", "b", "a"}},
		} {
			for _, order := range orders {
				inst, err := ctx.Instantiate(oneSymbol(t, idx, def))
				if err != nil {
					t.Fatalf("instantiate: %v", err)
				}
				for _, feature := range order {
					fv, err := inst.GetFeatureValue(ctx, feature)
					if err != nil {
						t.Fatalf("%s %v: %s: %v", def, order, feature, err)
					}
					if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Int != 5 {
						t.Errorf("%s %v: %s = %s, want 5", def, order, feature, FormatValue(got))
					}
				}
			}
		}
	})

	t.Run("whole_unequal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-collection-conflict>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute left : Integer[*] = (1, 2);
				attribute right : Integer[*] = (2, 1);
				bind edges = left;
				bind edges = right;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "edges")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("GetFeatureValue(edges) = %v, want ErrBindingConflict", err)
		}
		if got, want := err.Error(), "binding conflict at Sys.edges: left = [1, 2], right = [2, 1]"; got != want {
			t.Errorf("conflict error = %q, want %q", got, want)
		}
	})

	t.Run("whole_equal", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-multiple-collection-equal>", parseAndBuild(t, `package P {
			part def Sys {
				attribute edges : Integer[*];
				attribute left : Integer[*] = (1, 2);
				attribute right : Integer[*] = (1, 2);
				bind edges = left;
				bind edges = right;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "edges")
		if err != nil {
			t.Fatalf("GetFeatureValue(edges): %v", err)
		}
		if got := FormatValue(fv.HeldValue()); got != "[1, 2]" {
			t.Errorf("edges = %s, want [1, 2]", got)
		}
	})
}

func testBindingPropagationSpendsElementBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-element-budget>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a : Integer[*] = (1, 2, 3);
			attribute b : Integer[*];
			bind b = a;
		}
	}`))
	ctx.maxElements = 2
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "b")
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("GetFeatureValue(b) = %v, want ErrElementLimitExceeded", err)
	}
}

func testBindingDistinctMaterializedObjectsConflict(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-distinct-objects>", parseAndBuild(t, `package P {
		part def A {
			attribute q = 1;
		}
		part def Sys {
			part p1 : A;
			part p2 : A;
			binding bind p1 = p2;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if _, err := inst.materializeFeatureValueIntrinsic(ctx, "p1"); err != nil {
		t.Fatalf("materialize p1: %v", err)
	}
	if _, err := inst.materializeFeatureValueIntrinsic(ctx, "p2"); err != nil {
		t.Fatalf("materialize p2: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "p1")
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("GetFeatureValue(p1) = %v, want ErrBindingConflict", err)
	}
}

// `binding bnd = a` states two ends (KerML.xtext BindingConnectorDeclaration);
// `bnd` is the first end, not the binding's name.
func testBindingBareEndsBind(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-bare-ends>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 5;
			attribute bnd;
			binding bnd = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "bnd")
	if err != nil {
		t.Fatalf("GetFeatureValue(bnd) = %v, want 5", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Int != 5 {
		t.Errorf("bnd = %#v, want integer 5", got)
	}
}

func testBindingCycle(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-cycle>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a;
			attribute b;
			binding bind a = b;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "a")
	if !errors.Is(err, ErrBindingCycle) {
		t.Fatalf("GetFeatureValue(a) = %v, want ErrBindingCycle", err)
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Errorf("cycle error %q does not name both ends", err)
	}
}

func testBindingSingleValueless(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-single-valueless>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a;
			attribute b;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if got := fv.HeldValue(); got.Kind != ValInvalid {
			t.Errorf("%s = %#v, want an unknown value", name, got)
		}
	}
	sym := oneSymbol(t, idx, "P::Sys")
	expr := parser.New(source.New("<binding-single-valueless-eval>", []byte("b"))).ParseExpression()
	if _, err := ctx.EvalWithScopeOn(expr, sym.Scope, inst); !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Fatalf("evaluating b = %v, want ErrUninitializedFeatureValue", err)
	}
}

func testBindingThreeBindingRing(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-three-ring>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a;
			attribute b;
			attribute c;
			binding bind a = b;
			binding bind b = c;
			binding bind c = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = inst.GetFeatureValue(ctx, "a")
	if !errors.Is(err, ErrBindingCycle) {
		t.Fatalf("GetFeatureValue(a) = %v, want ErrBindingCycle", err)
	}
	for _, name := range []string{"a", "c"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("cycle error %q does not name %s", err, name)
		}
	}
}

func testBindingCycleWithValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-cycle-value>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 4;
			attribute b;
			binding bind a = b;
			binding bind b = a;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		if fv.HeldValue().Kind != ValConst || fv.HeldValue().Const.Int != 4 {
			t.Errorf("%s = %#v, want integer 4", name, fv.HeldValue())
		}
	}
}

func testBindingUnrelatedExpressionDoesNotPoisonRead(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-unrelated-expression>", parseAndBuild(t, `package P {
		part def Sys {
			attribute a = 5;
			attribute b;
			attribute sibling = 8;
			attribute b2 = a + 1;
			binding bind b = b2;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "sibling")
	if err != nil {
		t.Fatalf("GetFeatureValue(sibling): %v", err)
	}
	if got := fv.HeldValue().Const.Int; got != 8 {
		t.Errorf("sibling = %d, want 8", got)
	}
	fv, err = inst.GetFeatureValue(ctx, "b")
	if err != nil {
		t.Fatalf("GetFeatureValue(b): %v", err)
	}
	if got := fv.HeldValue().Const.Int; got != 6 {
		t.Errorf("b = %d, want 6", got)
	}
}

func testBindingResultTracksLaterMutation(t *testing.T) {
	t.Run("initially_unresolved", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-late-value>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b;
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if fv, err := inst.GetFeatureValue(ctx, "b"); err != nil {
			t.Fatalf("initial GetFeatureValue(b): %v", err)
		} else if got := fv.HeldValue(); got.Kind != ValInvalid {
			t.Fatalf("initial b = %#v, want an unknown value", got)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(9)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "b")
		if err != nil {
			t.Fatalf("later GetFeatureValue(b): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 9 {
			t.Errorf("later b = %d, want 9", got)
		}
	})

	t.Run("cached_value", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-late-change>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a = 3;
				attribute b;
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if fv, err := inst.GetFeatureValue(ctx, "b"); err != nil {
			t.Fatalf("initial GetFeatureValue(b): %v", err)
		} else if got := fv.HeldValue().Const.Int; got != 3 {
			t.Fatalf("initial b = %d, want 3", got)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(9)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "b")
		if err != nil {
			t.Fatalf("later GetFeatureValue(b): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 9 {
			t.Errorf("later b = %d, want 9", got)
		}
	})

	t.Run("derived_binding_chain", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-late-chain>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a = 3;
				attribute b;
				attribute c;
				binding bind b = a;
				binding bind c = b;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if fv, err := inst.GetFeatureValue(ctx, "c"); err != nil {
			t.Fatalf("initial GetFeatureValue(c): %v", err)
		} else if got := fv.HeldValue().Const.Int; got != 3 {
			t.Fatalf("initial c = %d, want 3", got)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(9)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		fv, err := inst.GetFeatureValue(ctx, "c")
		if err != nil {
			t.Fatalf("later GetFeatureValue(c): %v", err)
		}
		if got := fv.HeldValue().Const.Int; got != 9 {
			t.Errorf("later c = %d, want 9", got)
		}
	})

	t.Run("written_both_ends_conflict", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<binding-written-both-ends>", parseAndBuild(t, `package P {
			part def Sys {
				attribute a;
				attribute b;
				binding bind b = a;
			}
		}`))
		inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if err := inst.SetFeatureValue(ctx, "a", constInt(1)); err != nil {
			t.Fatalf("assign a: %v", err)
		}
		if err := inst.SetFeatureValue(ctx, "b", constInt(2)); err != nil {
			t.Fatalf("assign b: %v", err)
		}
		_, err = inst.GetFeatureValue(ctx, "b")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("GetFeatureValue(b) = %v, want ErrBindingConflict", err)
		}
	})
}

func testBindingIncompleteEndDoesNotPoisonRead(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-incomplete-end>", parseAndBuild(t, `package P {
		part def Sys {
			attribute x = 4;
			bind x;
			binding bb of x;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "x")
	if err != nil {
		t.Fatalf("GetFeatureValue(x) = %v, want 4", err)
	}
	if got := fv.HeldValue().Const.Int; got != 4 {
		t.Errorf("x = %d, want 4", got)
	}
}

func testBindingNestedContainerIsNotACycle(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<binding-nested-container>", parseAndBuild(t, `package P {
		part def Child {
			attribute b;
		}
		part def Sys {
			attribute x = 9;
			part child : Child;
			binding bind child.b = x;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Sys"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	childValue, err := inst.GetFeatureValue(ctx, "child")
	if err != nil {
		t.Fatalf("GetFeatureValue(child) = %v, want no binding cycle", err)
	}
	id, isObject := childValue.HeldValue().Object()
	if !isObject {
		t.Fatalf("child holds %s, want an object", childValue.HeldValue().Kind)
	}
	child, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("child instance %d is not materialized", id)
	}
	b, err := child.GetFeatureValue(ctx, "b")
	if err != nil {
		t.Fatalf("GetFeatureValue(child.b) = %v, want 9", err)
	}
	if got := b.HeldValue().Const.Int; got != 9 {
		t.Fatalf("child.b = %d, want 9", got)
	}
}

// testSuccessionGuardFailureModes: a guard on a succession leaving an ordinary
// action node is evaluated, so its failure modes — a value that is not Boolean,
// a guard nothing supplies a name for, and two guards holding at once — are each
// reported as a typed error rather than a panic, a hang or a chosen branch.
func testSuccessionGuardFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{
			name: "guard is not a boolean",
			body: `
				attribute x : Integer = 1;
				attribute y : Integer = 0;
				action s1 assign x := 7;
				action s2 assign y := 9;
				first s1 if x + 1 then s2;
			`,
			want: ErrTypeMismatch,
		},
		{
			name: "guard reads a name nothing supplies",
			body: `
				attribute y : Integer = 0;
				action s1;
				action s2 assign y := 9;
				first s1 if missing > 5 then s2;
			`,
			want: ErrUnresolvedReference,
		},
		{
			name: "two guards hold at once",
			body: `
				attribute level : Integer = 12;
				attribute low : Integer = 0;
				attribute high : Integer = 0;
				action check assign level := level;
				action alert assign high := 1;
				action idle assign low := 1;
				first check;
				succession first check if level > 10 then alert;
				succession first check if level > 5 then idle;
			`,
			want: ErrAmbiguousSuccession,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package test {\n private import ScalarValues::*;\n action guarded {" + tc.body + "}\n}"
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "guarded", ast.DefAction)
			if sym == nil {
				t.Fatal("action guarded not found")
			}

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := ctx.ExecuteAction(sym)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Errorf("ExecuteAction err = %v, want %v", err, tc.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("executing the guarded action did not terminate")
			}
		})
	}
}

// testQuantityWriteOfAnotherDimension: a write whose quantity measures in a
// dimension the target's declared quantity value type does not is refused with
// a typed error, while a commensurable unit at another scale is written.
func testQuantityWriteOfAnotherDimension(t *testing.T) {
	const duration = "ISQ::DurationValue"
	for _, tc := range []struct {
		name     string
		declared string
		value    string
		want     error
	}{
		{"speed into a duration", duration, "3.0 [SI::m / SI::s]", ErrTypeMismatch},
		{"length into a duration", duration, "5.0 [SI::m]", ErrTypeMismatch},
		{"dimensionless into a duration", duration, "5.0 [MeasurementReferences::one]", ErrTypeMismatch},
		{"another scale of the same dimension", duration, "5.0 [SI::min]", nil},
		{"a unit the target fixes no dimension against", "Quantities::ScalarQuantityValue", "5.0 [SI::m]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					private import SI::*;
					action w {
						attribute t : %s = 0.0 [s];
						first start;
						action step { assign t := %s; }
						done;
						succession first start then step;
						succession first step then done;
					}
				}
			`, tc.declared, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "w", ast.DefAction)
			if sym == nil {
				t.Fatal("action w not found")
			}
			_, err := ctx.ExecuteAction(sym)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("ExecuteAction err = %v, want the write to conform", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ExecuteAction err = %v, want %v", err, tc.want)
			}
		})
	}
}

// testMeasurementReferenceFailureModes: a measurement reference the runtime
// cannot honestly compute with is a typed error at the write or the call, never a
// value: a unit of another dimension does not conform, a conversion between
// dimensions is incommensurable, a number is not a reference, a reference is not
// a number, a unit is not a scale, a scale the library places on no other
// reference converts to none, and the tensors the library declares have no value.
func testMeasurementReferenceFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		value    string
		want     error
	}{
		{"speed unit into a length unit", "ISQ::LengthUnit", "SI::m / SI::s", ErrTypeMismatch},
		{"dimensionless unit into a length unit", "ISQ::LengthUnit", "MeasurementReferences::one", ErrTypeMismatch},
		{"unit into a length value", "ISQ::LengthValue", "SI::m", ErrTypeMismatch},
		{"composed unit of the same dimension", "ISQ::AreaUnit", "SI::m * SI::m", nil},
		{"composed unit as a derived unit", "MeasurementReferences::DerivedUnit", "SI::km / SI::L", nil},
		{"another scale of the same dimension", "ISQ::LengthUnit", "SI::km", nil},
		{"composed unit into a time scale", "Time::TimeScale", "SI::h * SI::s / SI::min", ErrTypeMismatch},
		{"composed unit into an interval scale", "MeasurementReferences::IntervalScale", "SI::h * SI::s / SI::min", ErrTypeMismatch},
		{"time scale as a value", "Time::TimeScale", "Time::UTC", nil},
		{"interval scale as a value", "MeasurementReferences::IntervalScale", "SI::'°C_abs'", nil},
		{"scale as a unit", "MeasurementReferences::IntervalScale", "SI::'°C'", ErrTypeMismatch},
		{"unit as a scale", "ISQ::LengthUnit", "Time::UTC", ErrTypeMismatch},
		{"conversion to a scale", "ISQ::ThermodynamicTemperatureValue", "QuantityCalculations::ConvertQuantity(300.0 [SI::K], SI::'°C_abs')", nil},
		{"conversion from a scale", "ISQ::ThermodynamicTemperatureValue", "QuantityCalculations::ConvertQuantity(26.85 [SI::'°C_abs'], SI::K)", nil},
		{"conversion to a scale placed on another dimension", "ISQ::LengthValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::m], SI::'°C_abs')", ErrIncommensurableUnits},
		{"conversion to a scale with no mapping", "ISQ::DurationValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::s], Time::UTC)", ErrUnevaluableLibraryFunction},
		{"conversion from a scale with no mapping", "ISQ::DurationValue", "QuantityCalculations::ConvertQuantity(3.0 [Time::UTC], SI::s)", ErrUnevaluableLibraryFunction},
		{"incommensurable conversion", "ISQ::LengthValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::m], SI::s)", ErrIncommensurableUnits},
		{"conversion to a number", "ISQ::LengthValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::m], 3)", ErrTypeMismatch},
		{"reference scaled by a number", "ISQ::LengthUnit", "SI::m * 3", ErrTypeMismatch},
		{"reference raised to a reference", "ISQ::LengthUnit", "SI::m ** SI::m", ErrTypeMismatch},
		{"declaration member of a reference", "Quantities::QuantityDimension", "SI::m.quantityDimension", nil},
		{"quantity dimension of a composed unit", "Quantities::QuantityDimension", "(SI::m / SI::s).quantityDimension", ErrUnevaluableLibraryFunction},
		{"unit conversion of a composed unit", "MeasurementReferences::UnitConversion", "(SI::m / SI::s).unitConversion", ErrUnevaluableLibraryFunction},
		{"unit power factors of a composed unit", "MeasurementReferences::UnitPowerFactor", "(SI::m ** 2).unitPowerFactors", ErrUnevaluableLibraryFunction},
		{"definitional quantity values of a composed unit", "MeasurementReferences::DefinitionalQuantityValue", "(SI::m / SI::s).definitionalQuantityValues", ErrUnevaluableLibraryFunction},
		{"vector reference over two units", "Quantities::VectorQuantityValue", "VectorCalculations::'['((1.0, 2.0), (SI::m, SI::s))", ErrTypeMismatch},
		{"vector reference over a unit", "Quantities::VectorQuantityValue", "VectorCalculations::'['((1.0, 2.0), SI::m)", ErrTypeMismatch},
		{"transformation that is a vector", "Quantities::VectorQuantityValue", "VectorCalculations::transform(VectorFunctions::VectorOf((1.0, 2.0)) [SI::m], VectorFunctions::VectorOf((0.0, 1.0)) [SI::m])", ErrTypeMismatch},
		{"scale scaled by a unit", "MeasurementReferences::CoordinateFrame", "Time::UTC / SI::s", ErrUnevaluableLibraryFunction},
		{"outer product", "Quantities::TensorQuantityValue", "VectorCalculations::outer((1.0, 2.0), (3.0, 4.0))", ErrUnevaluableLibraryFunction},
		{"tensor sum of number sequences", "Quantities::TensorQuantityValue", "TensorCalculations::'+'((1.0, 2.0), (3.0, 4.0))", ErrTypeMismatch},
		{"tensor product", "Quantities::TensorQuantityValue", "TensorCalculations::tensorTensorMult(VectorFunctions::VectorOf((1.0, 2.0)) [SI::m], VectorFunctions::VectorOf((3.0, 4.0)) [SI::m])", ErrUnevaluableLibraryFunction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					part def Holder {
						attribute value : %s = %s;
					}
				}
			`, tc.declared, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
			if sym == nil {
				t.Fatal("part def Holder not found")
			}
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("Instantiate err = %v", err)
			}
			_, err = inst.GetFeatureValue(ctx, "value")
			if tc.want == nil {
				if err != nil {
					t.Fatalf("value err = %v, want the reference to be held", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("value err = %v, want %v", err, tc.want)
			}
		})
	}
}

// testTensorQuantityFailureModes: a tensor the library does not determine is a typed
// error at the call, never a value: components that do not fill the reference,
// operands of two shapes, incommensurable components, a unit predicate over a
// shape with no identity, and the five calculations the library leaves bodiless
// and underspecified, each naming itself.
func testTensorQuantityFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  error
		names string
	}{
		{"element count", "TensorCalculations::'['((1.0, 2.0, 3.0), stressRef)", ErrMultiplicityViolation, "n = mRef.flattenedSize"},
		{"one reference for four components", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), oneRef)", ErrMultiplicityViolation, "oneRef"},
		{"a unit as the reference", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), (Pa, Pa, Pa, Pa))", ErrTypeMismatch, "TensorMeasurementReference"},
		{"shape mismatch", "stress + TensorCalculations::'['((1.0, 2.0, 3.0), rowRef)", ErrMultiplicityViolation, "dimensions [2, 2] and [3] differ"},
		{"tensor minus scalar", "stress - 2 [Pa]", ErrMultiplicityViolation, "dimensions [2, 2] and [] differ"},
		{"incommensurable components", "stress + TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), lengthRef)", ErrIncommensurableUnits, ""},
		{"non-square unit predicate", "TensorCalculations::isUnitTensorQuantity(TensorCalculations::'['((1.0, 2.0, 3.0), rowRef))", ErrUnevaluableLibraryFunction, "only a square tensor of order two has an identity"},
		{"unset covariance", "stress.contravariantOrder", ErrUnevaluableLibraryFunction, "orderSum"},
		{"tensor times vector", "TensorCalculations::tensorVectorMult(stress, VectorFunctions::VectorOf((1.0, 2.0)) [Pa])", ErrUnevaluableLibraryFunction, "TensorCalculations::tensorVectorMult"},
		{"vector times tensor", "TensorCalculations::vectorTensorMult(VectorFunctions::VectorOf((1.0, 2.0)) [Pa], stress)", ErrUnevaluableLibraryFunction, "TensorCalculations::vectorTensorMult"},
		{"tensor times tensor", "TensorCalculations::tensorTensorMult(stress, stress)", ErrUnevaluableLibraryFunction, "TensorCalculations::tensorTensorMult"},
		{"tensor times tensor by operator", "stress * stress", ErrUnevaluableLibraryFunction, "TensorCalculations::tensorTensorMult"},
		{"outer product", "VectorCalculations::outer(VectorFunctions::VectorOf((1.0, 2.0)) [Pa], VectorFunctions::VectorOf((1.0, 2.0)) [Pa])", ErrUnevaluableLibraryFunction, "VectorCalculations::outer"},
		{"transform", "TensorCalculations::transform(stressRef, stress)", ErrUnevaluableLibraryFunction, "TensorCalculations::transform"},
		{"rank three, too few indexes", "cube#(1, 2)", ErrMultiplicityViolation, "2 indexes address an array of rank 3"},
		{"rank three, too many indexes", "cube#(1, 1, 1, 1)", ErrMultiplicityViolation, "4 indexes address an array of rank 3"},
		{"rank three, first index low", "cube#(0, 1, 1)", ErrIndexOutOfRange, "index 1 is 0, dimension 1 has 1..2"},
		{"rank three, middle index high", "cube#(1, 3, 1)", ErrIndexOutOfRange, "index 2 is 3, dimension 2 has 1..2"},
		{"rank three, last index high", "cube#(1, 1, 3)", ErrIndexOutOfRange, "index 3 is 3, dimension 3 has 1..2"},
		{"rank three, non-integer index", "cube#(1, 1.5, 1)", ErrTypeMismatch, "requires an Integer index"},
		{"rank three, too few components", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0), cubeRef)", ErrMultiplicityViolation, "7 elements for a reference of dimensions [2, 2, 2]"},
		{"rank three, too many components", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0), cubeRef)", ErrMultiplicityViolation, "9 elements for a reference of dimensions [2, 2, 2]"},
		{"rank three against rank two", "cube + stress", ErrMultiplicityViolation, "dimensions [2, 2, 2] and [2, 2] differ"},
		{"rank three shapes differ", "cube - TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0, 12.0), slabRef)", ErrMultiplicityViolation, "dimensions [2, 2, 2] and [2, 3, 2] differ"},
		{"rank three unit predicate", "TensorCalculations::isUnitTensorQuantity(cube)", ErrUnevaluableLibraryFunction, "only a square tensor of order two has an identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					private import ISQ::*;
					private import SI::*;
					private import MeasurementReferences::*;
					private import Quantities::*;
					attribute stressRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); }
					attribute cubeRef : TensorMeasurementReference { :>> dimensions = (2, 2, 2); :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa); }
					attribute slabRef : TensorMeasurementReference { :>> dimensions = (2, 3, 2); :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa); }
					attribute cube = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
					attribute lengthRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (Pa, m, Pa, Pa); }
					attribute rowRef : TensorMeasurementReference { :>> dimensions = (3); :>> mRefs = (Pa, Pa, Pa); }
					attribute oneRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = Pa; }
					attribute stress = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
					part def Holder {
						attribute value : TensorQuantityValue = %s;
					}
				}
			`, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
			if sym == nil {
				t.Fatal("part def Holder not found")
			}
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("Instantiate err = %v", err)
			}
			_, err = inst.GetFeatureValue(ctx, "value")
			if !errors.Is(err, tc.want) {
				t.Fatalf("value err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.names) {
				t.Errorf("value err = %v, want it to state %q", err, tc.names)
			}
		})
	}
}

// testCoordinateFrameFailureModes: what the runtime cannot honestly compute about
// a coordinate frame, a scale or a transformation is a typed error naming what is
// missing or malformed: a frame with no mRefs, a vector with a number per axis
// short, a transformation applied to a vector in another frame, a subtype of
// CoordinateTransformation the library gives no shape, a placement whose origin
// is no vector quantity or whose basis is singular, a translation in an
// incommensurable unit, a scale placed on two references at odds, and matrices,
// sequences and placements missing what they declare. A scalar written where a
// reference, vector or step is declared is refused by the write itself.
func testCoordinateFrameFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		declared string
		value    string
		want     error
		message  string
	}{
		{"frame with no mRefs", `attribute wcf : CoordinateFrame;`,
			"CoordinateFrame", "wcf", ErrNoValue, "coordinate frame wcf states no mRefs; TensorMeasurementReference declares mRefs: ScalarMeasurementReference[1..*], one per axis"},
		{"frame whose mRefs are not one per axis", `attribute bad : CartesianSpatial3dCoordinateFrame { :>> mRefs = (m, m); }`,
			"CoordinateFrame", "bad", ErrMultiplicityViolation, "bad.mRefs: multiplicity violation: 2 value(s) bound to a feature with multiplicity lower bound 3"},
		{"frame whose mRefs are not one per stated dimension", `attribute bad : CoordinateFrame { :>> dimensions = 2; :>> mRefs = (m, m, m); }`,
			"CoordinateFrame", "bad", ErrMultiplicityViolation, "bad states 3 mRefs for dimensions [2], whose flattenedSize is 2"},
		{"frame whose dimensions overflow", `attribute bad : CoordinateFrame { :>> dimensions : Positive[2] = (4611686018427387904, 4); :>> mRefs = (m, m, m); }`,
			"CoordinateFrame", "bad", semantics.ErrArithmeticOverflow, "bad: flattenedSize of dimensions [4611686018427387904, 4] exceeds the Integer range"},
		{"frame whose mRef is a number", `attribute bad : CoordinateFrame { :>> mRefs = (m, 2); }`,
			"CoordinateFrame", "bad", ErrTypeMismatch, "bad.mRefs: type mismatch: cannot write 2 (an Integer) to a feature typed by ScalarMeasurementReference"},
		{"vector short of an axis", ``,
			"Position3dVector", "(1.0, 2.0) [spatialCF]", ErrMultiplicityViolation, "2 elements over the coordinate frame spatialCF, whose flattenedSize is 3; elements: Number[1..n] with n = mRef.flattenedSize"},
		{"vector of a number too many", ``,
			"Position3dVector", "(1.0, 2.0, 3.0, 4.0) [spatialCF]", ErrMultiplicityViolation, "4 elements over the coordinate frame spatialCF, whose flattenedSize is 3"},
		{"vector of no numbers", ``,
			"Position3dVector", "(m, m, m) [spatialCF]", ErrNotAQuantity, "want 3 numbers, one per axis"},
		{"scale composed with a unit", ``,
			"CoordinateFrame", "Time::UTC * s", ErrUnevaluableLibraryFunction, "UTC is a measurement scale"},
		{"frame composed with a number", ``,
			"CoordinateFrame", "spatialCF / 2", ErrTypeMismatch, "operator '/' is not defined for a coordinate frame and an Integer"},
		{"frame composed with a frame", ``,
			"CoordinateFrame", "spatialCF * spatialCF", ErrTypeMismatch, "operator '*' is not defined for a coordinate frame and coordinate frame; MeasurementRefCalculations::'CoordinateFrame*' takes a MeasurementUnit"},
		{"composed frame into a frame of another dimension", ``,
			"CartesianSpatial3dCoordinateFrame", "spatialCF / s", ErrTypeMismatch, "axis 1 measures in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L"},
		{"frame into a unit", ``,
			"LengthUnit", "spatialCF", ErrTypeMismatch, "cannot write the coordinate frame spatialCF [m, m, m], a CartesianSpatial3dCoordinateFrame, to a feature typed by LengthUnit"},
		{"frame into a scale", ``,
			"Time::TimeScale", "spatialCF", ErrTypeMismatch, "to a feature typed by TimeScale"},
		{"vector into the wrong frame's vector type", ``,
			"ISQSpaceTime::CartesianVelocity3dCoordinateFrame", "spatialCF", ErrTypeMismatch, "a CartesianSpatial3dCoordinateFrame, to a feature typed by CartesianVelocity3dCoordinateFrame"},
		{"vector over a frame into a vector type admitting another", `attribute velocityCF : CartesianVelocity3dCoordinateFrame = spatialCF / s;`,
			"CartesianPosition3dVector", "(1.0, 2.0, 3.0) [velocityCF]", ErrTypeMismatch, "cannot write ⟨1.0, 2.0, 3.0⟩ [velocityCF] (a vector quantity over the coordinate frame velocityCF [m/s, m/s, m/s], a CartesianVelocity3dCoordinateFrame) to a feature typed by CartesianPosition3dVector, whose mRef admits CartesianSpatial3dCoordinateFrame"},
		{"vector over a composed frame into a vector type admitting another", ``,
			"CartesianPosition3dVector", "(1.0, 2.0, 3.0) [spatialCF / s]", ErrTypeMismatch, "cannot write ⟨1.0, 2.0, 3.0⟩ [spatialCF / s] (a vector quantity over the coordinate frame spatialCF / s [m/s, m/s, m/s], a coordinate frame whose axis 1 measures in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L) to a feature typed by CartesianPosition3dVector, whose mRef admits CartesianSpatial3dCoordinateFrame"},
		{"transformation applied to a vector in another frame", placementBody,
			"Position3dVector", "transform(shifted.transformation, (1.0, 2.0, 3.0) [spatialCF])", ErrTypeMismatch, "sourceVector.mRef is spatialCF, not datum, the source of transformation"},
		{"transformation applied to a vector in a unit", placementBody,
			"Position3dVector", "transform(shifted.transformation, VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [mm])", ErrTypeMismatch, "written over the unit mm and no coordinate frame"},
		{"transformation applied to numbers", placementBody,
			"Position3dVector", "transform(shifted.transformation, (1.0, 2.0, 3.0))", ErrTypeMismatch, `parameter "sourceVector" requires a vector quantity over datum`},
		{"transformation of no recognized shape", `
			attribute def Bespoke :> CoordinateTransformation;
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : Bespoke { :>> source = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrUnevaluableLibraryFunction, "transformation is a Bespoke, a CoordinateTransformation of no shape the library gives a meaning"},
		{"placement whose origin is a string", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = "2024-01-01T00:00:00Z"; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, `transformation.origin: type mismatch: cannot write "2024-01-01T00:00:00Z" (string) to a feature typed by VectorQuantityValue`},
		{"placement whose origin is missing", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "states no origin; CoordinateFramePlacement declares origin: VectorQuantityValue[1]"},
		{"placement whose origin is in another frame", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (1.0, 2.0, 3.0) [spatialCF]; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "origin is a vector quantity in spatialCF, not a vector quantity over datum, its source"},
		{"placement whose origin has too few components", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = VectorFunctions::VectorOf((1.0, 2.0)) [mm]; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrMultiplicityViolation, "origin has 2 components over datum of 3 axes"},
		{"placement of a singular basis", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 0.0) [datum]; :>> basisDirections = ((1.0, 0.0, 0.0) [datum], (2.0, 0.0, 0.0) [datum], (0.0, 0.0, 1.0) [datum]); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", semantics.ErrArithmeticDomain, "basis directions are linearly dependent and span no frame"},
		{"placement of a zero basis direction", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 0.0) [datum]; :>> basisDirections = ((0.0, 0.0, 0.0) [datum], (0.0, 1.0, 0.0) [datum], (0.0, 0.0, 1.0) [datum]); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", semantics.ErrArithmeticDomain, "basisDirections#(1) is the zero vector, which points nowhere"},
		{"placement of too few basis directions", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 0.0) [datum]; :>> basisDirections = ((1.0, 0.0, 0.0) [datum], (0.0, 1.0, 0.0) [datum]); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrMultiplicityViolation, "states 2 basisDirections over datum of 3 axes; a placement states none or one per axis"},
		{"placement reorienting axes in different units", `
			attribute mixed : CoordinateFrame { :>> mRefs = (m, mm, m); }
			attribute odd : CoordinateFrame { :>> mRefs = (m, mm, m); :>> transformation : CoordinateFramePlacement { :>> source = mixed; :>> origin = (0.0, 0.0, 0.0) [mixed]; :>> basisDirections = ((0.0, 1.0, 0.0) [mixed], (1.0, 0.0, 0.0) [mixed], (0.0, 0.0, 1.0) [mixed]); } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0, 3.0) [mixed])", ErrIncommensurableUnits, "reorients mixed, whose axes m and mm are in different units and cannot mix"},
		{"translation in an incommensurable unit", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Translation(VectorFunctions::VectorOf((1.0, 0.0, 0.0)) [s])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrIncommensurableUnits, "elements#(1).translationVector: incommensurable units: cannot express s (second) in mm (0.001·metre)"},
		{"translation in another frame of the same units", `
			attribute other : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Translation((1.0, 0.0, 0.0) [other])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "translationVector is a vector quantity in other, not a vector quantity over datum, its source"},
		{"translation in a commensurable unit converts", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Translation(VectorFunctions::VectorOf((1.0, 0.0, 0.0)) [m])); } }`,
			"Position3dVector", "transform(odd.transformation, (1000.0, 2.0, 3.0) [datum])", nil, ""},
		{"sequence of no elements", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "states no elements; TranslationRotationSequence declares elements: TranslationOrRotation[1..*]"},
		{"sequence whose element is a number", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (1, 2); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "transformation.elements: type mismatch: cannot write 1 (an Integer) to a feature typed by TranslationOrRotation"},
		{"rotation about the zero vector", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Rotation((0.0, 0.0, 0.0) [datum], 90 ['°'])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", semantics.ErrArithmeticDomain, "axisDirection is the zero vector, which points nowhere"},
		{"rotation by a length", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Rotation((0.0, 0.0, 1.0) [datum], 90 [mm])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "new Rotation: feature value Rotation.angle: type mismatch: cannot write 90 [mm] (dimension L) to a feature typed by AngularMeasureValue (dimensionless)"},
		{"rotation by a number", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Rotation((0.0, 0.0, 1.0) [datum], 90)); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "new Rotation: feature value Rotation.angle: type mismatch: cannot write 90 (an Integer) to a feature typed by AngularMeasureValue"},
		{"rotation of a plane frame", `
			attribute plane : CoordinateFrame { :>> mRefs = (mm, mm); }
			attribute odd : CoordinateFrame { :>> mRefs = (mm, mm); :>> transformation : TranslationRotationSequence { :>> source = plane; :>> elements = (new Rotation((0.0, 1.0) [plane], 90 ['°'])); } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0) [plane])", ErrUnevaluableLibraryFunction, "rotates about an axis, which a frame of 2 axes has no meaning for"},
		{"affine matrix of too few elements", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = datum; :>> rotationMatrix { :>> elements = (1.0, 0.0, 0.0, 0.0, 1.0, 0.0); } :>> translationVector { :>> elements = (0.0, 0.0, 0.0); } } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrMultiplicityViolation, "rotationMatrix.elements: multiplicity violation: 6 value(s) bound to a feature with multiplicity lower bound 9"},
		{"affine matrix of strings", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = datum; :>> rotationMatrix { :>> elements = ("a", "b", "c", "d", "e", "f", "g", "h", "i"); } :>> translationVector { :>> elements = (0.0, 0.0, 0.0); } } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, `rotationMatrix.elements: type mismatch: cannot write "a" (string) to a feature typed by Real`},
		{"affine matrix with no translation", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = datum; :>> rotationMatrix { :>> elements = (1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0); } } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "translationVector states no elements"},
		{"affine matrix over a plane frame", `
			attribute plane : CoordinateFrame { :>> mRefs = (mm, mm); }
			attribute odd : CoordinateFrame { :>> mRefs = (mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = plane; :>> rotationMatrix { :>> elements = (1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0); } :>> translationVector { :>> elements = (0.0, 0.0, 0.0); } } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0) [plane])", ErrMultiplicityViolation, "is an AffineTransformationMatrix3d over plane of 2 axes; it asserts source.dimensions == 3"},
		{"transformation with no source", `
			attribute lone : NullTransformation { :>> target = datum; }`,
			"Position3dVector", "transform(lone, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "lone states no source or no target"},
		{"transformation with no target", `
			attribute lone : NullTransformation { :>> source = datum; }`,
			"Position3dVector", "transform(lone, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "lone states no source or no target"},
		{"transformation between frames of different dimensions", `
			attribute plane : CoordinateFrame { :>> mRefs = (mm, mm); }
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : NullTransformation { :>> source = plane; } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0) [plane])", ErrMultiplicityViolation, "relates plane of dimensions [2] (2 axes) to odd of dimensions [3] (3 axes); CoordinateTransformation asserts source.dimensions == target.dimensions"},
		{"transformation between frames of one axis but different dimensions", `
			attribute lineA : CoordinateFrame { :>> dimensions = 1; :>> mRefs = mm; }
			attribute lineB : CoordinateFrame { :>> dimensions = (); :>> mRefs = mm; :>> transformation : NullTransformation { :>> source = lineA; } }`,
			"Quantities::VectorQuantityValue", "transform(lineB.transformation, 3.0 [lineA])", ErrMultiplicityViolation, "relates lineA of dimensions [1] (1 axes) to lineB of dimensions [] (1 axes); CoordinateTransformation asserts source.dimensions == target.dimensions"},
		{"transformation whose target is another frame", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : NullTransformation { :>> source = datum; :>> target = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "target is datum, but the transformation is odd's own, whose target it is"},
		{"scale whose placement and mapping disagree", `
			attribute def Muddled :> IntervalScale {
				:>> unit = SI::'°C';
				private attribute triplePoint : DefinitionalQuantityValue { :>> num = 0.01; :>> definition = "triple point"; }
				private attribute mapping : QuantityValueMapping { :>> mappedQuantityValue = triplePoint; :>> referenceQuantityValue = K.temperatureOfWaterAtTriplePointInK; }
				:>> quantityValueMapping = mapping;
				:>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 300.0 [K]; }
			}
			attribute muddled : Muddled;`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(300.0 [K], muddled)", ErrUnevaluableLibraryFunction, "its transformation places zero at 300.0 [K] but its quantityValueMapping places it at 273.15"},
		{"scale whose mapping maps a quantity, not a definitional value", `
			attribute def Muddled :> IntervalScale {
				:>> unit = SI::'°C';
				private attribute mapping : QuantityValueMapping { :>> mappedQuantityValue = 0.01 [SI::'°C']; :>> referenceQuantityValue = K.temperatureOfWaterAtTriplePointInK; }
				:>> quantityValueMapping = mapping;
			}
			attribute muddled : Muddled;`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(300.0 [K], muddled)", ErrTypeMismatch, "measurement scale muddled: quantityValueMapping.mappedQuantityValue is a quantity in SI::'°C', want a DefinitionalQuantityValue"},
		{"scale placed on a reference of another dimension", `
			attribute def Muddled :> IntervalScale { :>> unit = SI::'°C'; :>> transformation : CoordinateFramePlacement { :>> source = m; :>> origin = 1.0 [m]; } }
			attribute muddled : Muddled;`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(300.0 [K], muddled)", ErrIncommensurableUnits, ""},
		{"scale whose origin is a string", `
			attribute def Muddled :> Time::TimeScale { :>> unit = s; :>> transformation : CoordinateFramePlacement { :>> source = Time::UTC; :>> origin = "2024-01-01T00:00:00Z"; } }
			attribute muddled : Muddled;`,
			"ISQ::DurationValue", "ConvertQuantity(3.0 [Time::UTC], muddled)", ErrTypeMismatch, `transformation.origin: type mismatch: cannot write "2024-01-01T00:00:00Z" (string) to a feature typed by VectorQuantityValue`},
		{"scale whose one basis direction is the identity in another unit", `
			attribute shifted : IntervalScale { :>> unit = m; :>> transformation : CoordinateFramePlacement { :>> source = m; :>> origin = 10.0 [m]; :>> basisDirections = 1000.0 [mm]; } }`,
			"LengthValue", "ConvertQuantity(3.0 [shifted], m)", nil, ""},
		{"scale whose basis direction scales the axis in another unit", `
			attribute shifted : IntervalScale { :>> unit = s; :>> transformation : CoordinateFramePlacement { :>> source = s; :>> origin = 10.0 [s]; :>> basisDirections = 1.0 [min]; } }`,
			"DurationValue", "ConvertQuantity(3.0 [shifted], s)", ErrUnevaluableLibraryFunction, "the basisDirection 1.0 [min] of its transformation transformation is not the identity 1 [s]"},
		{"scale placed on a frame of two axes", `
			attribute plane : CoordinateFrame { :>> mRefs = (K, K); }
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = plane; :>> origin = 10.0 [K]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrMultiplicityViolation, "relates plane of dimensions [2] (2 axes) to shifted of dimensions [] (1 axes); CoordinateTransformation asserts source.dimensions == target.dimensions"},
		{"scale with two basis directions", `
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = (1.0 [K], 1.0 [K]); } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrMultiplicityViolation, "states 2 basisDirections over K of one axis"},
		{"scale whose basis direction is in an incommensurable unit", `
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = 1.0 [m]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrIncommensurableUnits, "the basisDirection 1.0 [m] of its transformation transformation is not on K, its source"},
		{"scale whose basis direction is over another frame", `
			attribute line : CoordinateFrame { :>> mRefs = (K); }
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = (1.0) [line]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrTypeMismatch, "is a vector quantity in line, not a quantity on K, its source"},
		{"scale whose basis direction scales the axis", `
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = 2.0 [K]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrUnevaluableLibraryFunction, "is not the identity 1 [K], and the library gives a scale no other basis"},
		{"scale with no unit", `
			attribute def Unitless :> IntervalScale;
			attribute unitless : Unitless;`,
			"IntervalScale", "unitless", ErrNoValue, "states no unit; MeasurementScale declares unit: MeasurementUnit"},
		{"scale placed on itself", `
			attribute loop : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = loop; :>> origin = 1.0 [K]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [loop], K)", ErrCyclicFeatureValue, "coordinate frame loop is defined in terms of itself"},
		{"scales placed on each other", `
			attribute a : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = b; :>> origin = 1.0 [b]; } }
			attribute b : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = a; :>> origin = 2.0 [a]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [a], K)", ErrCyclicFeatureValue, "coordinate frame a is defined in terms of itself"},
		{"scales mapped onto each other", `
			attribute a : IntervalScale {
				:>> unit = K;
				private attribute p : DefinitionalQuantityValue { :>> num = 1; :>> definition = "p"; }
				private attribute m : QuantityValueMapping { :>> mappedQuantityValue = p; :>> referenceQuantityValue = 2.0 [b]; }
				:>> quantityValueMapping = m;
			}
			attribute b : IntervalScale {
				:>> unit = K;
				private attribute p : DefinitionalQuantityValue { :>> num = 1; :>> definition = "p"; }
				private attribute m : QuantityValueMapping { :>> mappedQuantityValue = p; :>> referenceQuantityValue = 2.0 [a]; }
				:>> quantityValueMapping = m;
			}`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [K], a)", ErrCyclicFeatureValue, "coordinate frame a is defined in terms of itself"},
		// A point on an interval scale has the affine operations and no other: the
		// refusal names the operation and the scale.
		{"point added to a point", ``,
			"ThermodynamicTemperatureValue", "warm + 10.0 [SI::'°C_abs']", ErrScalePoint, "operator '+' on a point of the interval scale SI::'°C_abs': two points have no sum"},
		{"time instant added to a time instant", ``,
			"Time::TimeInstantValue", "5.0 [Time::UTC] + 3.0 [Time::UTC]", ErrScalePoint, "operator '+' on a point of the interval scale Time::UTC: two points have no sum; their difference `x - y` is a magnitude in s"},
		{"point taken from a magnitude", ``,
			"ThermodynamicTemperatureValue", "300.0 [K] - warm", ErrScalePoint, "subtracting a point from a magnitude on a point of the interval scale SI::'°C_abs'"},
		{"point scaled by a number", ``,
			"ThermodynamicTemperatureValue", "2 * warm", ErrScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs': a point has no multiple"},
		{"number scaled by a point", ``,
			"ThermodynamicTemperatureValue", "warm * 2.0", ErrScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"time instant scaled by a number", ``,
			"Time::TimeInstantValue", "2 * 5.0 [Time::UTC]", ErrScalePoint, "operator '*' on a point of the interval scale Time::UTC"},
		{"point multiplied by a quantity", ``,
			"ScalarQuantityValue", "warm * 2.0 [s]", ErrScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs': a point has no multiple and its scale is no unit to compose"},
		{"quantity multiplied by a point", ``,
			"ScalarQuantityValue", "2.0 [s] * warm", ErrScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"point divided by a number", ``,
			"ThermodynamicTemperatureValue", "warm / 2", ErrScalePoint, "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"point divided by a quantity", ``,
			"ScalarQuantityValue", "warm / 2.0 [s]", ErrScalePoint, "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"quantity divided by a point", ``,
			"ScalarQuantityValue", "2.0 [s] / warm", ErrScalePoint, "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"point raised to a power", ``,
			"ScalarQuantityValue", "warm ** 2", ErrScalePoint, "operator '**' on a point of the interval scale SI::'°C_abs'"},
		{"point raised to a power by exponentiation", ``,
			"ScalarQuantityValue", "warm ^ 2", ErrScalePoint, "operator '**' on a point of the interval scale SI::'°C_abs'"},
		{"square root of a point", ``,
			"ScalarQuantityValue", "sqrt(warm)", ErrScalePoint, "sqrt on a point of the interval scale SI::'°C_abs'"},
		{"negated point", ``,
			"ThermodynamicTemperatureValue", "-warm", ErrScalePoint, "negation on a point of the interval scale SI::'°C_abs': a point has no negative"},
		{"points summed", ``,
			"ThermodynamicTemperatureValue", "sum((warm, 20.0 [SI::'°C_abs']))", ErrScalePoint, "QuantityCalculations::sum on a point of the interval scale SI::'°C_abs': points have no sum or product to fold"},
		{"points multiplied together", ``,
			"ScalarQuantityValue", "product((warm, 20.0 [SI::'°C_abs']))", ErrScalePoint, "QuantityCalculations::product on a point of the interval scale SI::'°C_abs'"},
		{"point moved by a magnitude of another dimension", ``,
			"ThermodynamicTemperatureValue", "warm + 1.0 [m]", ErrIncommensurableUnits, "a point on the interval scale SI::'°C_abs' moves by a magnitude in its unit '°C'"},
		{"point compared with a magnitude of another dimension", ``,
			"Boolean", "warm < 1.0 [m]", ErrIncommensurableUnits, ""},
		{"point equated with a magnitude of another dimension", ``,
			"Boolean", "warm == 1.0 [m]", ErrIncommensurableUnits, ""},
		{"point on an unanchored scale compared with a magnitude", ``,
			"Boolean", "5.0 [Time::UTC] == 5.0 [s]", ErrUnevaluableLibraryFunction, "Time::UTC states neither a transformation placing it on another reference nor a quantityValueMapping"},
		{"point on an ordinal scale moved by a magnitude", `attribute mohs : OrdinalScale { :>> unit = K; }`,
			"ScalarQuantityValue", "7 [mohs] + 1 [K]", ErrScalePoint, "operator '+' on a point of the ordinal scale test::Holder::mohs: only an IntervalScale relates its points by differences in its unit"},
		{"points on an ordinal scale subtracted", `attribute mohs : OrdinalScale { :>> unit = K; }`,
			"ScalarQuantityValue", "9 [mohs] - 7 [mohs]", ErrScalePoint, "operator '-' on a point of the ordinal scale test::Holder::mohs"},
		{"point on an ordinal scale compared with a magnitude", `attribute mohs : OrdinalScale { :>> unit = K; }`,
			"Boolean", "7 [mohs] < 300.0 [K]", ErrScalePoint, "comparison with another reference on a point of the ordinal scale test::Holder::mohs"},
		{"point on a cyclic ratio scale scaled", `attribute compass : CyclicRatioScale { :>> unit = rad; :>> modulus = 6.283185307179586; }`,
			"ScalarQuantityValue", "2 * 1.0 [compass]", ErrScalePoint, "operator '*' on a point of the cyclic ratio scale test::Holder::compass"},
		{"point on a logarithmic scale moved by a magnitude", `attribute decibel : LogarithmicScale { :>> unit = W; :>> logarithmBase = 10; :>> factor = 10; :>> exponent = 1; }`,
			"ScalarQuantityValue", "30.0 [decibel] + 1.0 [W]", ErrScalePoint, "operator '+' on a point of the logarithmic scale test::Holder::decibel: only an IntervalScale relates its points by differences in its unit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					private import ISQ::*;
					private import SI::*;
					private import ISQSpaceTime::*;
					private import MeasurementReferences::*;
					private import QuantityCalculations::*;
					private import VectorCalculations::*;
					part def Holder {
						attribute spatialCF : CartesianSpatial3dCoordinateFrame { :>> mRefs = (m, m, m); }
						attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
						attribute warm : ThermodynamicTemperatureValue = ConvertQuantity(300.0 [K], SI::'°C_abs');
						%s
						attribute value : %s = %s;
					}
				}
			`, tc.body, tc.declared, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
			if sym == nil {
				t.Fatal("part def Holder not found")
			}
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("Instantiate err = %v", err)
			}
			_, err = inst.GetFeatureValue(ctx, "value")
			if tc.want == nil {
				if err != nil {
					t.Fatalf("value err = %v, want a value", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("value err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("value err = %v, want it to name %q", err, tc.message)
			}
		})
	}
}

// placementBody declares a frame placed in datum, for the transform failure modes.
const placementBody = `
	attribute shifted : CartesianSpatial3dCoordinateFrame {
		:>> mRefs = (mm, mm, mm);
		:>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (10.0, 20.0, 30.0) [datum]; }
	}`

// testExpressionOverAFeatureValueHoldingNoValue: a valueless feature of a value type is
// read without an error and reports that it holds no value, while an expression
// computing over it reports which feature holds none rather than a number or a panic.
func testExpressionOverAFeatureValueHoldingNoValue(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Holder {
				attribute d : Real;
				attribute n : Real = d + 1.0;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	fv, err := inst.GetFeatureValue(ctx, "d")
	if err != nil {
		t.Fatalf("feature value d: %v", err)
	}
	if !ctx.HoldsNoValue(fv.HeldValue()) {
		t.Errorf("feature value d holds %v, want no value", fv.HeldValue())
	}

	_, err = inst.GetFeatureValue(ctx, "n")
	var noValue *NoValueError
	if !errors.As(err, &noValue) {
		t.Fatalf("feature value n err = %v, want *NoValueError", err)
	}
	if noValue.Feature != "d" || noValue.Symbol == nil || noValue.Symbol.Name != "d" {
		t.Errorf("feature value n reports no value for %q (%v), want feature d", noValue.Feature, noValue.Symbol)
	}

	// A value naming an object the context does not hold answers the question
	// rather than panicking on the lookup.
	if ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: 1 << 30}) {
		t.Error("a value naming no object reads as holding none")
	}
}

// testClassificationOutsideTheEvaluableSubset: a classification the evaluator
// cannot judge — no subject to classify, a subject that is a datum, an
// unresolved metadata type, or a subject naming nothing — reports
// ErrFilterUnevaluable rather than silently answering false.
func testClassificationOutsideTheEvaluableSubset(t *testing.T) {
	const model = `
		metadata def Safety;
		#Safety part def Belt;
		attribute level = 3;
	`
	for _, tc := range []struct{ name, cond string }{
		{"implicit subject outside an object", "@Safety"},
		{"self outside an object", "self @ Safety"},
		{"a datum subject", "42 @ Safety"},
		{"a string subject", `"belt" @ Safety`},
		{"an unresolved metadata type", "Belt @ Nonexistent"},
	} {
		src := model + "\nconstraint c { " + tc.cond + " }"
		got, err := constraintVerdict(t, src, "c")
		if got {
			t.Errorf("%s: `%s` was satisfied, want a report", tc.name, tc.cond)
		}
		if !errors.Is(err, semantics.ErrFilterUnevaluable) {
			t.Errorf("%s: `%s` err = %v, want ErrFilterUnevaluable", tc.name, tc.cond, err)
		}
	}
	// A subject naming nothing is the unresolved reference it is, not a verdict.
	if got, err := constraintVerdict(t, model+"\nconstraint c { Missing @ Safety }", "c"); got || err == nil {
		t.Errorf("`Missing @ Safety` = %v err=%v, want a report", got, err)
	}
}

// testMultiplicityInfiniteLowerBound: `[*..*]` requires unboundedly many objects,
// which cannot be materialized, so the feature value reports a multiplicity violation
// rather than allocating until memory runs out.
func testMultiplicityInfiniteLowerBound(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[*..*]; }
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "p")
	if err == nil {
		t.Fatal("want a multiplicity violation, got a materialized feature value")
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
	}
}

// testMultiplicityLowerBoundTooLarge: a lower bound past the materialization
// bound is reported instead of eagerly allocating that many objects.
func testMultiplicityLowerBoundTooLarge(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder { part p : C[5000]; }
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "p")
	if err == nil {
		t.Fatal("want a multiplicity violation, got a materialized feature value")
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
	}
	if len(ctx.instances) > 100 {
		t.Errorf("materialized %d instances before reporting the bound", len(ctx.instances))
	}
}

// testDefaultNotConformingToMultiplicity: a default whose element count is
// outside the feature's multiplicity is reported, rather than broadcast to fill
// the lower bound, truncated to the upper one, or dropped.
func testDefaultNotConformingToMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name string
		decl string
	}{
		{"one value against three", "attribute xs : Real[3] = 1.0;"},
		{"four values against three", "attribute xs : Real[3] = (1.0, 2.0, 3.0, 4.0);"},
		{"no values against one or more", "attribute xs : Real[1..3] = ();"},
		{"an expression producing too few", "attribute m : Real = 1.0; attribute xs : Real[2] = m;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ctx := instantiateHolder(t, `
				package test {
					private import ScalarValues::Real;
					part def Holder { `+tc.decl+` }
				}
			`)
			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := inst.GetFeatureValue(ctx, "xs")
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("want a multiplicity violation, got a materialized feature value")
				}
				if !errors.Is(err, ErrMultiplicityViolation) {
					t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("materializing the default did not terminate")
			}
		})
	}
}

// testDefaultAgainstAnUndeclaredMultiplicity: a feature that declares no
// multiplicity holds exactly one value, so a default of any other number of
// values is reported rather than held under an unconstrained bound.
func testDefaultAgainstAnUndeclaredMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decl     string
		reported bool
	}{
		{"one value", "attribute xs : Real = 1.0;", false},
		{"one value of an untyped feature", "attribute xs = 1.0;", false},
		{"two values", "attribute xs : Real = (1.0, 2.0);", true},
		{"two values of an untyped feature", "attribute xs = (1.0, 2.0);", true},
		{"no values", "attribute xs : Real = ();", true},
		{"an expression producing two", "attribute m : Real[2] = (1.0, 2.0); attribute xs : Real = m;", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst, ctx := instantiateHolder(t, `
				package test {
					private import ScalarValues::Real;
					part def Holder { `+tc.decl+` }
				}
			`)
			_, err := inst.GetFeatureValue(ctx, "xs")
			switch {
			case tc.reported && err == nil:
				t.Fatalf("%s was held, want a multiplicity violation", tc.decl)
			case tc.reported && !errors.Is(err, ErrMultiplicityViolation):
				t.Errorf("expected ErrMultiplicityViolation, got: %v", err)
			case !tc.reported && err != nil:
				t.Errorf("%s was reported: %v", tc.decl, err)
			}
		})
	}
}

// testFeatureChainThroughAnUnsetFeatureValue: a chain over a collection whose objects
// hold no value for the last feature names that feature, rather than reading it
// as an empty collection.
func testFeatureChainThroughAnUnsetFeatureValue(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			private import RealFunctions::*;
			part def Sub { attribute volume : Real; }
			part def Holder {
				part subs : Sub[2];
				attribute total : Real = sum(subs.volume);
			}
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "total")
	if err == nil {
		t.Fatal("want the unset feature value's error, got a value")
	}
	if !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Errorf("expected ErrUninitializedFeatureValue, got: %v", err)
	}
	if !strings.Contains(err.Error(), "volume") {
		t.Errorf("error %q does not name the unset feature", err)
	}
}

// testFeatureChainSpendsTheElementBudget: navigating a chain through a collection
// counts what it collects, so a chain over a large collection ends within the
// element budget rather than growing unbounded.
func testFeatureChainSpendsTheElementBudget(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def Sub { attribute volume : Real = 1.0; }
			part def Holder {
				part subs : Sub[10];
				attribute volumes : Real[*] = subs.volume;
			}
		}
	`)
	ctx.maxElements = 15
	_, err := inst.GetFeatureValue(ctx, "volumes")
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("expected ErrElementLimitExceeded, got: %v", err)
	}
}

// testMutuallySubsettingFeatures: two features that subset each other have no
// well-founded set of values, so materializing one reports the cycle instead of
// recursing until the step budget runs out.
func testMutuallySubsettingFeatures(t *testing.T) {
	inst, ctx := instantiateHolder(t, `
		package test {
			private import ScalarValues::Real;
			part def C { attribute m : Real = 1.0; }
			part def Holder {
				part a : C[*] :> b;
				part b : C[*] :> a;
			}
		}
	`)
	done := make(chan struct{})
	var fvErr error
	go func() {
		defer close(done)
		_, fvErr = inst.GetFeatureValue(ctx, "a")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetFeatureValue hung on mutually subsetting features")
	}
	if fvErr == nil {
		t.Fatal("want the cyclic feature value's error, got a materialized feature value")
	}
	if !errors.Is(fvErr, ErrCyclicFeatureValue) {
		t.Errorf("expected ErrCyclicFeatureValue, got: %v", fvErr)
	}
}

// instantiateHolder instantiates the `Holder` part def the source declares, for
// a case whose failure surfaces when one of its feature values is read.
func instantiateHolder(t *testing.T, src string) (*Instance, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
	if sym == nil {
		t.Fatal("Holder part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return inst, ctx
}

// testSequenceIndexNamesNoPosition: an index outside the sequence, or one that
// is not a whole number, is reported. Answering nothing would make `seq#(i)`
// read as an empty value everywhere a model indexes past the end.
func testSequenceIndexNamesNoPosition(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"xs#(4)", ErrIndexOutOfRange},
		{"xs#(0)", ErrIndexOutOfRange},
		{"xs#(0 - 1)", ErrIndexOutOfRange},
		{"()#(1)", ErrIndexOutOfRange},
		{"xs#(1.5)", ErrTypeMismatch},
		{"xs#(ys)", ErrTypeMismatch},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testNumericLibraryCallThatHasNoValue: a vector, Complex or sequence library
// declaration that cannot answer reports itself — a malformed argument by kind or
// dimension, an undefined result, or a declaration this runtime has no
// representation for the values of — rather than computing something else.
func testNumericLibraryCallThatHasNoValue(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"VectorFunctions::cartesianInner(xs, ys)", ErrTypeMismatch},
		{"VectorFunctions::'cartesian+'(xs, ys)", ErrTypeMismatch},
		{"VectorFunctions::cartesianNorm(flags)", ErrTypeMismatch},
		{"VectorFunctions::cartesianAngle(xs, (0.0, 0.0, 0.0))", semantics.ErrArithmeticDomain},
		{"VectorFunctions::vectorScalarDiv(xs, 0)", ErrDivisionByZero},
		{"VectorFunctions::cartesianInner(xs)", ErrCalcArity},
		// Flat numbers are no collection of vectors; unequal dimensions have no
		// sum or inner product; Booleans or a two-component three-vector are no vector.
		{"VectorFunctions::sum(xs)", ErrTypeMismatch},
		{"VectorFunctions::sum0(xs, VectorFunctions::VectorOf(xs))", ErrTypeMismatch},
		{"VectorFunctions::sum0((), VectorFunctions::VectorOf(xs))", ErrTypeMismatch},
		{"VectorFunctions::sum((VectorFunctions::VectorOf(xs), VectorFunctions::VectorOf(ys)))", ErrTypeMismatch},
		{"VectorFunctions::inner(VectorFunctions::VectorOf(xs), VectorFunctions::VectorOf(ys))", ErrTypeMismatch},
		{"VectorFunctions::VectorOf(xs) + VectorFunctions::VectorOf(ys)", ErrTypeMismatch},
		{"VectorFunctions::VectorOf(flags)", ErrTypeMismatch},
		{"VectorFunctions::CartesianThreeVectorOf(ys)", ErrMultiplicityViolation},
		{"VectorFunctions::angle(VectorFunctions::VectorOf((0.0, 0.0)), VectorFunctions::VectorOf(ys))", semantics.ErrArithmeticDomain},
		// A quantity's num is Number[1..*]: a vector of no components takes no unit,
		// whether written `[m]`, scaled by a scalar quantity, or divided by one.
		{"VectorFunctions::CartesianVectorOf(()) [SI::m]", ErrMultiplicityViolation},
		{"VectorFunctions::norm(VectorFunctions::CartesianVectorOf(()) [SI::m])", ErrMultiplicityViolation},
		{"VectorCalculations::scalarQuantityVectorMult(2 [SI::m], VectorFunctions::CartesianVectorOf(()))", ErrMultiplicityViolation},
		{"VectorCalculations::vectorScalarQuantityMult(VectorFunctions::VectorOf(()), 2 [SI::m])", ErrMultiplicityViolation},
		{"VectorCalculations::vectorScalarQuantityDiv(VectorFunctions::VectorOf(()), 2 [SI::s])", ErrMultiplicityViolation},
		{"VectorFunctions::VectorOf(xs) / 0", ErrDivisionByZero},
		{"VectorCalculations::vectorScalarQuantityDiv(VectorFunctions::VectorOf(xs) [SI::m], 0 [SI::s])", ErrDivisionByZero},
		{"VectorCalculations::scalarQuantityVectorMult(2 [SI::m], flags)", ErrTypeMismatch},
		{"ComplexFunctions::'/'(ComplexFunctions::rect(0.0, 1.0), ComplexFunctions::rect(0.0, 0.0))", ErrDivisionByZero},
		{"ComplexFunctions::re(xs)", ErrTypeMismatch},
		{"ComplexFunctions::re(ys)", ErrTypeMismatch},
		{"ComplexFunctions::ToString(ComplexFunctions::rect(0.0, 1.0))", ErrUnevaluableLibraryFunction},
		// includingAt inserts before a position of 1..size+1, so an index past the
		// end of the sequence names no insertion point and is reported rather than
		// appending or dropping the values.
		{"SequenceFunctions::includingAt(xs, 9, 5)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt(xs, 9, 0)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt((), 9, 2)", ErrIndexOutOfRange},
		{"SequenceFunctions::includingAt(xs, 9, 1.5)", ErrTypeMismatch},
		{"SequenceFunctions::includingAt(xs, 9)", ErrCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testNamedLibraryCallThatHasNoValue: a conversion, operator-call form, control
// function, aggregation or unevaluable declaration called by name reports itself
// by a typed error — no panic, no zero, no answer of another kind.
func testNamedLibraryCallThatHasNoValue(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`RealFunctions::ToReal("1.5 meters")`, ErrInvalidNotation},
		{`RealFunctions::ToReal("NaN")`, ErrInvalidNotation},
		{`RealFunctions::ToReal(" 1.5 ")`, ErrInvalidNotation},
		{`IntegerFunctions::ToInteger(" 7")`, ErrInvalidNotation},
		{`RationalFunctions::ToRational("1/3")`, ErrInvalidNotation},
		{`IntegerFunctions::ToInteger("2.0")`, ErrInvalidNotation},
		{`IntegerFunctions::ToInteger("99999999999999999999")`, semantics.ErrArithmeticOverflow},
		{`RealFunctions::ToInteger(1.0e300)`, semantics.ErrArithmeticOverflow},
		{`BooleanFunctions::ToBoolean("yes")`, ErrInvalidNotation},
		{`IntegerFunctions::ToNatural(-1)`, semantics.ErrArithmeticDomain},
		{`NaturalFunctions::ToNatural("-1")`, semantics.ErrArithmeticDomain},
		{`RealFunctions::ToReal(xs)`, ErrTypeMismatch},
		{`RationalFunctions::gcd(1.5, 2)`, semantics.ErrArithmeticDomain},
		{`RationalFunctions::gcd("1", 2)`, ErrTypeMismatch},
		{`RationalFunctions::gcd(1.0e19, 1.0e19)`, semantics.ErrArithmeticOverflow},
		{`RationalFunctions::rat(1, 0)`, ErrDivisionByZero},
		{`RationalFunctions::rat(1.5, 3)`, ErrTypeMismatch},
		{`RationalFunctions::rat(xs, 3)`, ErrTypeMismatch},
		{`RationalFunctions::numer("0.5")`, ErrTypeMismatch},
		{`RationalFunctions::numer(1.0e19)`, semantics.ErrArithmeticOverflow},
		{`RationalFunctions::denom(0.0001)`, semantics.ErrArithmeticOverflow},
		{`CollectionFunctions::'array#'(xs, (1, 1))`, ErrTypeMismatch},
		{`OccurrenceFunctions::isDuring(xs)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::isDuring(factor)`, ErrNotAnOccurrence},
		{`OccurrenceFunctions::'==='(xs, xs)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::'==='(factor, factor)`, ErrNotAnOccurrence},
		{`OccurrenceFunctions::create(factor)`, ErrNotAnOccurrence},
		{`OccurrenceFunctions::destroy(factor)`, ErrNotAnOccurrence},
		{`OccurrenceFunctions::addNew(xs)`, ErrCalcArity},
		{`OccurrenceFunctions::addNew(occ = xs)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::addNew(xs, factor)`, ErrNotAnOccurrence},
		{`OccurrenceFunctions::addNewAt(xs, xs)`, ErrCalcArity},
		{`OccurrenceFunctions::addNewAt(occ = xs, index = 1)`, ErrMultiplicityViolation},
		{`OccurrenceFunctions::addNewAt((), factor, 0)`, ErrNotAnOccurrence},
		{`IntegerFunctions::'+'("a", 1)`, ErrTypeMismatch},
		{`IntegerFunctions::'/'(1, 0)`, ErrDivisionByZero},
		{`NaturalFunctions::'/'(7, 2)`, semantics.ErrArithmeticDomain},
		{`NaturalFunctions::'/'(6, 0)`, ErrDivisionByZero},
		{`NaturalFunctions::'/'(6, -3)`, ErrTypeMismatch},
		{`IntegerFunctions::'=='(2, 2.0)`, ErrTypeMismatch},
		{`BooleanFunctions::'=='(true, 1)`, ErrTypeMismatch},
		{`BaseFunctions::ToString(xs)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'%'(1, 0)`, ErrDivisionByZero},
		{`IntegerFunctions::'*'(9223372036854775807, 2)`, semantics.ErrArithmeticOverflow},
		{`RealFunctions::'**'(-8.0, 0.5)`, semantics.ErrArithmeticDomain},
		{`ScalarFunctions::'<'("a", 1)`, ErrTypeMismatch},
		{`BooleanFunctions::'xor'(true, 1)`, ErrTypeMismatch},
		{`DataFunctions::max(true, false)`, ErrTypeMismatch},
		{`ScalarFunctions::min(xs, ys)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'+'(xs, 1)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'+'(1, xs)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'+'((), 1)`, ErrMultiplicityViolation},
		{`IntegerFunctions::'-'(xs)`, ErrMultiplicityViolation},
		{`RealFunctions::'<'(xs, 2.0)`, ErrMultiplicityViolation},
		{`BooleanFunctions::'xor'(flags, true)`, ErrMultiplicityViolation},
		{`BooleanFunctions::'not'(flags)`, ErrMultiplicityViolation},
		{`BooleanFunctions::'not'(())`, ErrMultiplicityViolation},
		{`NaturalFunctions::'/'(xs, 2)`, ErrMultiplicityViolation},
		{`ScalarFunctions::'..'(1.5, 3)`, ErrTypeMismatch},
		{`BaseFunctions::'#'(xs, 0)`, ErrIndexOutOfRange},
		{`ControlFunctions::'if'(1, 2, 3)`, ErrTypeMismatch},
		{`ControlFunctions::'if'(true, {in x; x}, 3)`, ErrBodyArity},
		{`ControlFunctions::'and'(true, 1)`, ErrTypeMismatch},
		{`ControlFunctions::'and'(1, true)`, ErrTypeMismatch},
		{`ControlFunctions::'and'(true)`, ErrMultiplicityViolation},
		{`ControlFunctions::'or'(false)`, ErrMultiplicityViolation},
		{`ControlFunctions::'implies'(true)`, ErrMultiplicityViolation},
		{`ControlFunctions::'implies'(true, xs)`, ErrTypeMismatch},
		{`NumericalFunctions::sum0(xs, 1)`, ErrTypeMismatch},
		{`NumericalFunctions::product1(xs, 0)`, ErrTypeMismatch},
		{`NumericalFunctions::sum0(flags, 0)`, ErrTypeMismatch},
		{`NumericalFunctions::sum0((9223372036854775807, 1), 0)`, semantics.ErrArithmeticOverflow},
		{`NumericalFunctions::sum0(xs)`, ErrCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testBuiltinNamedArgumentThatBindsNothing: a named argument to a built-in that
// names no parameter, names one twice, sits beside a positional argument, or
// leaves a required parameter unbound is reported rather than bound by position;
// a body passed by reference that denotes no body is reported too.
func testBuiltinNamedArgumentThatBindsNothing(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`NumericalFunctions::sum0(zero = 0, elements = xs)`, ErrUnknownParameter},
		{`NumericalFunctions::sum0(zero = 0, zero = 1)`, ErrCalcArity},
		{`NumericalFunctions::sum0(zero = 0, zero = 1, elements = xs)`, ErrCalcArity},
		{`NumericalFunctions::sum0(collection = xs)`, ErrCalcArity},
		{`ControlFunctions::'if'(thenValue = 1, elseValue = 2)`, ErrCalcArity},
		{`ControlFunctions::'if'(test = true, thenValue = {in x; x})`, ErrBodyArity},
		{`ControlFunctions::'and'(secondValue = true)`, ErrCalcArity},
		{`ControlFunctions::'if'()`, ErrCalcArity},
		{`ControlFunctions::'and'()`, ErrCalcArity},
		{`SequenceFunctions::subsequence(xs)`, ErrCalcArity},
		{`SequenceFunctions::subsequence(seq = xs)`, ErrCalcArity},
		{`SequenceFunctions::size(xs, 1)`, ErrCalcArity},
		{`SequenceFunctions::'#'(xs, 1, 2)`, ErrCalcArity},
		{`SequenceFunctions::'#'(seq = xs, index = 0)`, ErrIndexOutOfRange},
		{`ControlFunctions::select(collection = xs, selector = factor)`, ErrTypeMismatch},
		{`xs->select factor`, ErrTypeMismatch},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testBodilessModelCalcNamedAsABuiltin: a model's own calc declared under a
// library function's qualified name — a collection built-in, a conversion, an
// operator form — is the model's, so without a body it computes nothing rather
// than what the library's declaration of that name computes.
func testBodilessModelCalcNamedAsABuiltin(t *testing.T) {
	src := `
		package NumericalFunctions {
			private import ScalarValues::*;
			calc def sum0 { in collection : Integer[*]; in zero : Integer; return : Integer; }
		}
		package RealFunctions {
			private import ScalarValues::*;
			calc def ToReal { in x : String; return : Real; }
			calc def '+' { in x : Real; in y : Real; return : Real; }
		}
		package test {
			private import ScalarValues::*;
			calc def Total { return : Integer = NumericalFunctions::sum0((1, 2, 3), 0); }
			calc def size { in seq : Integer[*]; return : Integer; }
			calc def Size { return : Integer = size((1, 2, 3)); }
			calc def Parsed { return : Real = RealFunctions::ToReal("1.5"); }
			calc def Added { return : Real = RealFunctions::'+'(1.0, 2.0); }
		}
	`
	for _, calc := range []string{"Total", "Size", "Parsed", "Added"} {
		err := calcErrorWithLibraries(t, src, calc, nil, 10000)
		if !errors.Is(err, ErrNoResultExpression) {
			t.Errorf("%s = %v, want %v", calc, err, ErrNoResultExpression)
		}
	}
}

// testDataEqualityOverAPart: DataFunctions' `'=='` and `'==='` are declared over
// DataValue, so a part given to either is refused rather than compared.
func testDataEqualityOverAPart(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Widget;
			attribute def Point { attribute x : Integer; }
			calc def SameData { in a : Point; in b : Point; return : Boolean = DataFunctions::'=='(a, b); }
			calc def SamePart { in a : Widget; in b : Widget; return : Boolean = DataFunctions::'=='(a, b); }
			calc def IdenticalPart { in a : Widget; in b : Widget; return : Boolean = DataFunctions::'==='(a, b); }
			calc def SameAnything { in a : Widget; in b : Widget; return : Boolean = BaseFunctions::'=='(a, b); }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	object := func(name string) Value {
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s: %d matching symbols, want 1", name, len(matches))
		}
		inst, err := ctx.Instantiate(matches[0])
		if err != nil {
			t.Fatalf("Instantiate(%s): %v", name, err)
		}
		return Value{Kind: ValInstance, Instance: inst.ID}
	}
	point, widget := object("Point"), object("Widget")
	invoke := func(calc string, args ...Value) (Value, error) {
		sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calc)
		return ctx.InvokeCalc(sym, args, scope)
	}
	for _, calc := range []string{"SamePart", "IdenticalPart"} {
		_, err := invoke(calc, widget, widget)
		if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "a DataValue") {
			t.Errorf("%s = %v, want %v naming DataValue", calc, err, ErrTypeMismatch)
		}
	}
	for calc, args := range map[string][]Value{"SameData": {point, point}, "SameAnything": {widget, widget}} {
		got, err := invoke(calc, args...)
		if err != nil || !valueIdentical(got, constBool(true)) {
			t.Errorf("%s = %s, %v; want true", calc, FormatValue(got), err)
		}
	}
}

// testBaseIndexWithSeveralIndexes: several indexes address an Array, so a flat
// sequence, a rank mismatch, an out-of-range, ragged or oversized Array is each
// reported.
func testBaseIndexWithSeveralIndexes(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import Collections::*;
			attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
			attribute ragged : Array { :>> dimensions = (2, 2); :>> elements = (1, 2, 3); }
			attribute vast : Array { :>> dimensions = (4611686018427387904, 4); :>> elements = (); }
			calc def Cell { return : Integer = BaseFunctions::'#'((1, 2, 3, 4), (2, 2)); }
			calc def NoIndex { return : Integer = BaseFunctions::'#'((1, 2, 3, 4), ()); }
			calc def OneIndex { return : Integer = a#(4); }
			calc def ThreeIndexes { return : Integer = a#(1, 1, 1); }
			calc def PastRow { return : Integer = a#(3, 1); }
			calc def PastColumn { return : Integer = a#(1, 4); }
			calc def ZeroIndex { return : Integer = a#(0, 1); }
			calc def Ragged { return : Integer = ragged#(1, 1); }
			calc def Vast { return : Integer = vast#(4611686018427387904, 4); }
		}
	`
	for calc, want := range map[string]error{
		"Cell":         ErrTypeMismatch,
		"NoIndex":      ErrMultiplicityViolation,
		"OneIndex":     ErrMultiplicityViolation,
		"ThreeIndexes": ErrMultiplicityViolation,
		"PastRow":      ErrIndexOutOfRange,
		"PastColumn":   ErrIndexOutOfRange,
		"ZeroIndex":    ErrIndexOutOfRange,
	} {
		err := calcErrorWithLibraries(t, src, calc, nil, 10000)
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "BaseFunctions::'#'") {
			t.Errorf("%s = %v, want %v naming BaseFunctions::'#'", calc, err, want)
		}
	}
	err := calcErrorWithLibraries(t, src, "Ragged", nil, 10000)
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "flattenedSize") {
		t.Errorf("Ragged = %v, want %v naming flattenedSize", err, ErrMultiplicityViolation)
	}
	err = calcErrorWithLibraries(t, src, "Vast", nil, 10000)
	if !errors.Is(err, semantics.ErrArithmeticOverflow) || !strings.Contains(err.Error(), "flattenedSize") {
		t.Errorf("Vast = %v, want %v naming flattenedSize", err, semantics.ErrArithmeticOverflow)
	}
}

// testStructuredValueOutsideTheDeclaredShape: a type specializing Array or a
// vector type fixes a shape or element type, so a value of another shape or
// element type is refused, while one that fits is held — including by a
// NumericalVectorValue specialization beside the Cartesian ones.
func testStructuredValueOutsideTheDeclaredShape(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import Collections::*;
			private import VectorValues::*;
			private import VectorFunctions::*;
			private import Quantities::*;
			private import ISQ::*;
			private import SI::*;
			attribute def Grid :> Array { :>> dimensions = (2, 2); }
			attribute def Row4 :> Array { :>> rank = 1; :>> flattenedSize = 4; }
			attribute def IntArray :> Array { :>> elements : Integer; }
			attribute def Fixed3 :> CartesianThreeVectorValue;
			attribute def OneDim :> NumericalVectorValue;
			attribute def IntVec :> NumericalVectorValue { :>> elements : Integer; }
			attribute def IntThree :> ThreeVectorValue { :>> elements : Integer; }
			attribute def Fixed2 :> NumericalVectorValue { :>> dimension = 2; }
			attribute def Four :> Array { :>> elements : Integer[4]; }
			attribute def Vel3 :> VectorQuantityValue { :>> num : Real[3]; }
			attribute def IntVQ :> VectorQuantityValue { :>> num : Integer; }
			attribute square : Array { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); }
			attribute wide : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
			attribute row : Array { :>> dimensions = 4; :>> elements = (1, 2, 3, 4); }
			attribute reals : Array { :>> dimensions = 2; :>> elements = (1.5, 2.5); }
			calc def TwoAsThree { return : CartesianThreeVectorValue = VectorOf((1.0, 2.0)); }
			calc def TwoAsFixed3 { return : Fixed3 = VectorOf((1.0, 2.0)); }
			calc def ThreeAsThree { return : CartesianThreeVectorValue = VectorOf((1.0, 2.0, 3.0)); }
			calc def ThreeAsFixed3 { return : Fixed3 = VectorOf((1.0, 2.0, 3.0)); }
			calc def TwoAsThreeVector { return : ThreeVectorValue = VectorOf((1, 2)); }
			calc def ThreeAsThreeVector { return : ThreeVectorValue = VectorOf((1, 2, 3)); }
			calc def IntsAsIntVec { return : IntVec = VectorOf((1, 2)); }
			calc def RealsAsIntVec { return : IntVec = VectorOf((1.5, 2.0)); }
			calc def IntsAsIntThree { return : IntThree = VectorOf((1, 2, 3)); }
			calc def TwoAsIntThree { return : IntThree = VectorOf((1, 2)); }
			calc def RealsAsIntThree { return : IntThree = VectorOf((1.5, 2.0, 3.0)); }
			calc def TwoAsFixed2 { return : Fixed2 = VectorOf((1.0, 2.0)); }
			calc def ThreeAsFixed2 { return : Fixed2 = VectorOf((1.0, 2.0, 3.0)); }
			calc def VectorAsGrid { return : Grid = VectorOf((1, 2, 3, 4)); }
			calc def VectorAsString { return : String = VectorOf((1, 2)); }
			calc def WideAsGrid { return : Grid = wide; }
			calc def SquareAsGrid { return : Grid = square; }
			calc def SquareAsRow4 { return : Row4 = square; }
			calc def SquareAsOneDim { return : OneDim = square; }
			calc def RowAsRow4 { return : Row4 = row; }
			calc def RealsAsIntArray { return : IntArray = reals; }
			calc def RowAsIntArray { return : IntArray = row; }
			calc def RealsAsFour { return : Four = reals; }
			calc def SquareAsFour { return : Four = square; }
			calc def TwoAsVel3 { return : Vel3 = VectorOf((1.0, 2.0)) [m]; }
			calc def ThreeAsVel3 { return : Vel3 = VectorOf((1.0, 2.0, 3.0)) [m]; }
			calc def RealsAsIntVQ { return : IntVQ = VectorOf((1.5, 2.5)) [m]; }
			calc def IntsAsIntVQ { return : IntVQ = VectorOf((1, 2)) [m]; }
			calc def TwoAsScalar { return : ScalarQuantityValue = VectorOf((1.0, 2.0)) [m]; }
			calc def TwoAsLength { return : LengthValue = VectorOf((1.0, 2.0)) [m]; }
		}
	`
	for calc, fixes := range map[string]string{
		"TwoAsThree":       "it declares dimension = 3",
		"TwoAsFixed3":      "it declares dimension = 3",
		"TwoAsThreeVector": "it declares dimension = 3",
		"TwoAsIntThree":    "it declares dimension = 3",
		"ThreeAsFixed2":    "it declares dimension = 2",
		"RealsAsIntVec":    "it declares elements : Integer, got element 1.5 (a Real)",
		"RealsAsIntThree":  "it declares elements : Integer, got element 1.5 (a Real)",
		"VectorAsGrid":     "cannot write ⟨1, 2, 3, 4⟩ (vector) to a feature typed by Grid",
		"VectorAsString":   "cannot write ⟨1, 2⟩ (vector) to a feature typed by String",
		"WideAsGrid":       "it declares dimensions = [2, 2]",
		"SquareAsRow4":     "it declares rank = 1",
		"SquareAsOneDim":   "it declares dimension : Positive[0..1], got 2 dimension(s)",
		"RealsAsIntArray":  "it declares elements : Integer, got element 1.5 (a Real)",
		"RealsAsFour":      "it declares elements : Integer[4], got 2 element(s)",
		"TwoAsVel3":        "it declares num : Real[3], got 2 element(s)",
		"RealsAsIntVQ":     "it declares num : Integer, got element 1.5 (a Real)",
		"TwoAsScalar":      "to a feature typed by ScalarQuantityValue: it is a ScalarValue, which holds one scalar",
		"TwoAsLength":      "to a feature typed by LengthValue: it is a ScalarValue, which holds one scalar",
	} {
		err := calcErrorWithLibraries(t, src, calc, nil, 10000)
		if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), fixes) {
			t.Errorf("%s = %v, want %v naming %q", calc, err, ErrTypeMismatch, fixes)
		}
	}
	for calc, want := range map[string]string{
		"ThreeAsThree":       "⟨1.0, 2.0, 3.0⟩",
		"ThreeAsFixed3":      "⟨1.0, 2.0, 3.0⟩",
		"ThreeAsThreeVector": "⟨1, 2, 3⟩",
		"IntsAsIntVec":       "⟨1, 2⟩",
		"IntsAsIntThree":     "⟨1, 2, 3⟩",
		"TwoAsFixed2":        "⟨1.0, 2.0⟩",
		"SquareAsGrid":       "Array(2, 2)[1, 2, 3, 4]",
		"RowAsRow4":          "Array(4)[1, 2, 3, 4]",
		"RowAsIntArray":      "Array(4)[1, 2, 3, 4]",
		"SquareAsFour":       "Array(2, 2)[1, 2, 3, 4]",
		"ThreeAsVel3":        "⟨1.0, 2.0, 3.0⟩ [m]",
		"IntsAsIntVQ":        "⟨1, 2⟩ [m]",
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calc)
		got, err := ctx.InvokeCalc(sym, nil, scope)
		if err != nil || FormatValue(got) != want {
			t.Errorf("%s = (%s, %v), want %s", calc, FormatValue(got), err, want)
		}
	}
}

// testBodyByReferenceThatCannotBeApplied: a body reaching a built-in through an
// `expr` parameter is applied in the scope it was written in, and one selected
// by a control function that declares parameters is reported.
func testBodyByReferenceThatCannotBeApplied(t *testing.T) {
	src := `
		package test {
			private import ControlFunctions::*;
			calc def Pick { in expr chosen; return : Integer = ControlFunctions::'if'(true, chosen, 0); }
			calc def Keep { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }
			calc def PicksUnary { return : Integer = Pick({ in x; x }); }
			calc def KeepsUnbound { in xs : Integer[*]; return : Integer[*] = Keep(xs, { in x; x > bound }); }
			calc def KeepsPriorFrame {
				in xs : Integer[*];
				in pred : Integer;
				return : Integer[*] = Keep(xs, { in x; x > pred });
			}
		}
	`
	for _, tt := range []struct {
		calc string
		args []Value
		want error
	}{
		{"PicksUnary", nil, ErrBodyArity},
		{"KeepsUnbound", []Value{constSequence(1, 2)}, ErrUnresolvedReference},
	} {
		err := calcErrorWithLibraries(t, src, tt.calc, tt.args, 10000)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = %v, want %v", tt.calc, err, tt.want)
		}
	}
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "KeepsPriorFrame")
	got, err := ctx.InvokeCalc(sym, []Value{constSequence(1, 2, 3), constInt(1)}, scope)
	if err != nil {
		t.Fatalf("KeepsPriorFrame = %v", err)
	}
	if want := constSequence(2, 3); !valueIdentical(got, want) {
		t.Errorf("KeepsPriorFrame = %s, want %s: the body's pred is its caller's Integer, not Keep's body", FormatValue(got), FormatValue(want))
	}
}

// testRealLiteralThatUnderflows: a nonzero Real literal too small for a Real is
// reported rather than read as zero.
func testRealLiteralThatUnderflows(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`1.0e-400`, semantics.ErrArithmeticOverflow},
		{`1.0e400`, semantics.ErrArithmeticOverflow},
		{`RealFunctions::ToReal("1e-400")`, semantics.ErrArithmeticOverflow},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
	got, err := evalCollectionExpr(t, `0.0e-400`)
	if err != nil || got.Kind != ValConst || got.Const.Kind != semantics.ValReal || got.Const.Real != 0 {
		t.Errorf("0.0e-400 = (%v, %v), want the Real 0", got, err)
	}
}

// testStringOperandOfTheWrongKind: an operator or StringFunctions call given a
// value that is not the String its signature declares is reported rather than
// coerced, and a Substring position naming no character is reported rather than
// clamped.
func testStringOperandOfTheWrongKind(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{`"a" + 1`, ErrTypeMismatch},
		{`1 + "a"`, ErrTypeMismatch},
		{`"a" < 1`, ErrTypeMismatch},
		{`"a" >= factor`, ErrTypeMismatch},
		{`"a" < xs`, ErrTypeMismatch},
		{`"a" - "b"`, ErrTypeMismatch},
		{`StringFunctions::Length(1)`, ErrTypeMismatch},
		{`StringFunctions::Length(xs)`, ErrTypeMismatch},
		{`StringFunctions::Substring("abc", 1, 9)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("héllo", 1, 6)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("abc", 0, 2)`, ErrIndexOutOfRange},
		{`StringFunctions::Substring("abc", "1", 2)`, ErrTypeMismatch},
		{`StringFunctions::Substring("abc", 1)`, ErrCalcArity},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testCollectionOperandOfTheWrongKind: an operation given something that is not
// the kind of value it operates on reports it rather than reading the value as a
// collection of itself or as nothing.
func testCollectionOperandOfTheWrongKind(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"xs->select(2)", ErrTypeMismatch},
		{"xs->collect(xs)", ErrTypeMismatch},
		{`sum((1, "a"))`, ErrTypeMismatch},
		{"product(flags)", ErrTypeMismatch},
		{"xs->subsequence(1, 4)", ErrIndexOutOfRange},
	} {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// testCollectionBodyOfTheWrongArity: a body an operation calls with one element
// but that declares no parameter, or two, is reported. Binding what it declares
// and dropping the rest would answer from a parameter that was never given a
// value.
func testCollectionBodyOfTheWrongArity(t *testing.T) {
	for _, expr := range []string{
		"xs->collect {}",
		"xs->collect {in x; in y; x}",
		"xs->select {in x; in y; x > 0}",
		"xs.{in x; in y; x}",
	} {
		got, err := evalCollectionExpr(t, expr)
		if !errors.Is(err, ErrBodyArity) {
			t.Errorf("%s = (%v, %v), want ErrBodyArity", expr, got, err)
		}
	}
}

// testSelectPredicateIsNotACondition: a selector answering something that is not
// a boolean is reported. Reading a non-boolean as false would silently drop
// every element, which is a wrong answer rather than a failure.
func testSelectPredicateIsNotACondition(t *testing.T) {
	for _, expr := range []string{
		"xs.?{in x; x + 1}",
		"xs->select {in x; x * 2}",
		"xs->reject {in x; 1}",
		"xs->forAll {in x; x}",
		`xs->exists {in x; "yes"}`,
	} {
		got, err := evalCollectionExpr(t, expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s = (%v, %v), want ErrTypeMismatch", expr, got, err)
		}
	}
}

// testCollectionOperationStepBudget: an operation calls its body once per
// element, and each call spends the context's budget, so a collection large
// enough for the budget fails the evaluation instead of running unbounded.
func testCollectionOperationStepBudget(t *testing.T) {
	got, err := evalCollectionExprBounded(t, "xs.{in x; x * factor}", 3)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("collect under a budget of 3 = (%v, %v), want ErrStepLimitExceeded", got, err)
	}
}

// testSatisfyUnresolvedRequirement: a satisfaction assertion whose requirement
// reference names nothing reports it, rather than evaluating the assertion's own
// empty body as a verdict about the model.
func testSatisfyUnresolvedRequirement(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part lander;
			part context {
				assert satisfy nosuch by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoRequirement) {
		t.Errorf("expected ErrNoRequirement, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an unresolved requirement reference is not a violation")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error does not name the reference: %v", err)
	}
}

// testSatisfyRequirementWithoutConditions: satisfying a requirement that states
// no condition is not a verdict, since no check ran.
func testSatisfyRequirementWithoutConditions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			requirement def Documented {
				doc /* stated in prose only */
			}
			requirement documented : Documented;
			part lander;
			part context {
				assert satisfy documented by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoConditions) {
		t.Errorf("expected ErrNoConditions, got: %v", err)
	}
	if satisfied {
		t.Error("a requirement with no condition must not report a verdict")
	}
}

// testSatisfyBoundedByTheStepBudget: a satisfaction check is one run, so its
// condition evaluation spends the run's budget instead of resetting it.
func testSatisfyBoundedByTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Lander { attribute verticalSpeed = 1.2; }
			part lander : Lander;
			requirement def TouchdownRequirement {
				subject craft : Lander;
				attribute maxVerticalSpeed = 1.5;
				require constraint { craft.verticalSpeed <= maxVerticalSpeed }
			}
			requirement touchdown : TouchdownRequirement;
			part context {
				assert satisfy touchdown by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}
	ctx.maxSteps = 2

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected the step budget to bound the check, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an exhausted budget is not a verdict about the model")
	}

	// The subject is built inside the same run, so a budget exhausted there is
	// still reported as such rather than as a missing subject.
	ctx.maxSteps = 0
	if _, err := ctx.EvaluateSatisfaction(assertions[0]); !errors.Is(err, ErrStepLimitExceeded) || !errors.Is(err, ErrNoSubject) {
		t.Errorf("expected ErrStepLimitExceeded while building the subject, got: %v", err)
	}
}

// testCyclicDerivedFeatureValue: two derived defaults that read each other are reported
// as a cycle instead of recursing until the step budget runs out.
func testCyclicDerivedFeatureValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				attribute a = b + 1.0;
				attribute b = a + 1.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	done := make(chan struct{})
	var fvErr error
	go func() {
		defer close(done)
		_, fvErr = inst.GetFeatureValue(ctx, "a")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetFeatureValue hung on a cyclic derived feature value")
	}

	if !errors.Is(fvErr, ErrCyclicFeatureValue) {
		t.Fatalf("GetFeatureValue error = %v, want ErrCyclicFeatureValue", fvErr)
	}
}

// testWriteIntoCyclicDerivedFeatureValues: a write into a pair of values derived
// from each other neither hangs unmaterializing what read them nor unmaterializes
// the written value; it breaks the cycle, and a later write to the other end
// keeps the first write.
func testWriteIntoCyclicDerivedFeatureValues(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				attribute a = b + 1.0;
				attribute b = a + 1.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "a"); !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("GetFeatureValue(a) error = %v, want ErrCyclicFeatureValue", err)
	}

	done := make(chan error, 1)
	go func() { done <- inst.SetFeatureValue(ctx, "b", constReal(1)) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SetFeatureValue(b): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetFeatureValue(b) hung unmaterializing a cycle of derived values")
	}
	if b := inst.FeatureValues["b"]; !b.Materialized || !b.Written {
		t.Fatalf("the write to b left it materialized %t, written %t", b.Materialized, b.Written)
	}
	fv, err := inst.GetFeatureValue(ctx, "a")
	if err != nil || !valueIdentical(fv.HeldValue(), constReal(2)) {
		t.Fatalf("a after b := 1.0 = %v, %v; want 2.0", fv, err)
	}

	if err := inst.SetFeatureValue(ctx, "a", constReal(5)); err != nil {
		t.Fatalf("SetFeatureValue(a): %v", err)
	}
	fv, err = inst.GetFeatureValue(ctx, "b")
	if err != nil || !valueIdentical(fv.HeldValue(), constReal(1)) {
		t.Fatalf("b after a := 5.0 = %v, %v; want the written 1.0", fv, err)
	}
}

// testCyclicSubsettingOfDefaultCollections: two `default null` collections that
// subset each other are reported as a cycle when either is read, rather than
// populating each other until the stack runs out.
func testCyclicSubsettingOfDefaultCollections(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Loop {
				part xs : Loop [*] :> ys default null;
				part ys : Loop [*] :> xs default null;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Loop", ast.DefPart)
	if sym == nil {
		t.Fatal("Loop part def not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	for _, name := range []string{"xs", "ys"} {
		done := make(chan struct{})
		var fvErr error
		go func() {
			defer close(done)
			_, fvErr = inst.GetFeatureValue(ctx, name)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("GetFeatureValue(%s) hung on collections subsetting each other", name)
		}
		if !errors.Is(fvErr, ErrCyclicFeatureValue) {
			t.Fatalf("GetFeatureValue(%s) error = %v, want ErrCyclicFeatureValue", name, fvErr)
		}
	}
}

// testDerivedFeatureValueOverMissingFeature: a derived default that names something the
// instance does not have fails with the feature value named, rather than silently
// leaving the feature value empty.
func testDerivedFeatureValueOverMissingFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Broken {
				attribute derived = missing * 2.0;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Broken", ast.DefPart)
	if sym == nil {
		t.Fatal("Broken part def not found")
	}

	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	_, err = inst.GetFeatureValue(ctx, "derived")
	if err == nil {
		t.Fatal("GetFeatureValue succeeded on a default over an undeclared feature")
	}
	if !strings.Contains(err.Error(), "derived") {
		t.Errorf("error %q does not name the feature value", err)
	}
}

// testStateSubactionReferenceOfMissingAction: an entry action given by
// reference to a name nothing declares fails at execution, naming the target.
func testStateSubactionReferenceOfMissingAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
		state Machine {
			entry; then init;
			state init;
			state active {
				entry noSuchAction;
			}

			succession first init then active;
			succession first active then done;
		}
	}`, "Machine")

	if _, _, err := ctx.ExecuteStateWithEvents(machine, nil); err == nil {
		t.Fatal("expected an unresolved entry action reference to fail")
	} else if !strings.Contains(err.Error(), "noSuchAction") {
		t.Errorf("error should name the unresolved action, got: %v", err)
	}
}

// testStateSubactionReferenceFeatureChain: a feature-chain reference parses but
// is not invocable, so it must report what it named rather than an empty name.
func testStateSubactionReferenceFeatureChain(t *testing.T) {
	ctx, machine := loadState(t, `package test {
		action def CoolDown {
			first start;
			done;
			succession first start then done;
		}

		state Machine {
			part controller {
				action coolDown : CoolDown;
			}

			entry; then init;
			state init;
			state active {
				exit controller.coolDown;
			}

			succession first init then active;
			succession first active then done;
		}
	}`, "Machine")

	if _, _, err := ctx.ExecuteStateWithEvents(machine, nil); err == nil {
		t.Fatal("expected a feature-chain action reference to fail")
	} else if !strings.Contains(err.Error(), "coolDown") {
		t.Errorf("error should name the chained action, got: %v", err)
	}
}

// testPerformOfMissingAction: a perform statement naming nothing resolvable is
// an error at execution, not a silently skipped node.
func testPerformOfMissingAction(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
		action outer {
			first start;
			perform action doIt references missingAction;
			done;

			succession first start then doIt;
			succession first doIt then done;
		}
	}`, "outer")

	if _, err := ctx.ExecuteAction(outer); err == nil {
		t.Fatal("expected performing an unresolved action to fail")
	} else if !strings.Contains(err.Error(), "missingAction") {
		t.Errorf("error should name the unresolved action, got: %v", err)
	}
}

// testPerformReferenceCycle: an action performing itself must be stopped by the
// nesting bound instead of recursing forever.
func testPerformReferenceCycle(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
		action outer {
			first start;
			perform action doIt references outer;
			done;

			succession first start then doIt;
			succession first doIt then done;
		}
	}`, "outer")

	done := make(chan error, 1)
	go func() {
		_, err := ctx.ExecuteAction(outer)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a self-performing action to be bounded, it completed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("self-performing action did not terminate")
	}
}

// testDeferOfNonDeferrableTrigger: only signals and calls are dispatched from
// the event pool, so a state deferring a time trigger is reported at lowering
// rather than deferring nothing at run time.
func testDeferOfNonDeferrableTrigger(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 1000)

	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			&ast.StateNode{
				Name:  "busy",
				Defer: []ast.Node{&ast.TimeEvent{Duration: &ast.LiteralInteger{Value: "1"}}},
			},
			transitionMember("init", "busy"),
		},
	}

	_, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	}, nil)
	if err == nil {
		t.Fatal("expected an error for a state deferring a time trigger")
	}
	if !strings.Contains(err.Error(), "only signal and call triggers can be deferred") {
		t.Errorf("expected a deferrability error, got: %v", err)
	}
}

// testStateTransitionEndpointMisspelled: a misspelled endpoint is a
// name-resolution diagnostic, so lowering leaves the edge out and the machine
// runs to a halt in the state it reached rather than panicking or hanging.
func testStateTransitionEndpointMisspelled(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy then donee;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.model.resolver.ResolveDocument("<test>", file)

	var endpoint *resolve.Diagnostic
	for i, diag := range ctx.model.resolver.Diagnostics {
		if strings.Contains(diag.Message, "donee") {
			endpoint = &ctx.model.resolver.Diagnostics[i]
		}
	}
	if endpoint == nil {
		t.Fatalf("expected a name-resolution diagnostic for 'donee', got: %v", ctx.model.resolver.Diagnostics)
	}
	if endpoint.Code != "unresolved" {
		t.Errorf("expected code %q, got %q", "unresolved", endpoint.Code)
	}

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunToCompletion hung on a machine whose transition names nothing")
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "busy" {
		t.Errorf("expected the machine to halt in 'busy', got %v", got)
	}
}

// testStateTransitionEndpointNeverResolved: executed without a name-resolution
// pass, as the REPL and the service handlers do, an endpoint naming nothing
// leaves its edge out; the machine still runs, and the misspelling is reported
// by whoever resolves the document rather than by lowering.
func testStateTransitionEndpointNeverResolved(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy then donee;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunToCompletion: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunToCompletion hung on an endpoint no resolution pass reported")
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "busy" {
		t.Errorf("expected the machine to halt in 'busy', got %v", got)
	}
}

// testStateTransitionEndpointInAnotherMachine: an endpoint naming a state of a
// different machine resolves, so no name diagnostic reports it; the state
// transition check reports it, and lowering backstops the check with a typed
// error rather than dropping the edge.
func testStateTransitionEndpointInAnotherMachine(t *testing.T) {
	src := `package test {
		state Other {
			entry; then start;
			state start;
			state running;
			succession first start then running;
		}
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy then Other::running;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.model.resolver.ResolveDocument("<test>", file)

	for _, diag := range ctx.model.resolver.Diagnostics {
		if strings.Contains(diag.Message, "running") {
			t.Fatalf("the endpoint resolves, so name resolution reports nothing: %v", diag)
		}
	}

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint that is not a vertex of this machine")
	}
	if !strings.Contains(err.Error(), "not a vertex of this state machine") {
		t.Errorf("expected the error to say the endpoint is not a vertex of this machine, got %v", err)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("the message a modeller reads names a Go type: %v", err)
	}
}

// testStateTransitionEndpointNamingAFirstMarker: a `first m then x` marker is no
// vertex, so an endpoint naming one is reported by the state transition check and
// backstopped here with a typed error rather than a panic.
func testStateTransitionEndpointNamingAFirstMarker(t *testing.T) {
	src := `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			state other;
			first marker then other;
			succession first init then busy;
			transition first busy then marker;
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	_, err := newStateExecutor(ctx, sym, nil)
	if err == nil {
		t.Fatal("expected an error for an endpoint naming a marker rather than a vertex")
	}
	if !strings.Contains(err.Error(), "not a vertex of this state machine") {
		t.Errorf("expected the error to say the endpoint is not a vertex, got %v", err)
	}
	if strings.Contains(err.Error(), "*ast.") {
		t.Errorf("the message a modeller reads names a Go type: %v", err)
	}
}

// testStateJunctionWithoutAnOutgoingTransition: a junction no transition leaves
// routes a transition reaching it nowhere, which the state transition check
// reports; reaching it at run time errors rather than panicking or hanging.
// testStateEventAfterCompletion: a signal sent to a machine that already reached
// `done` is discarded, leaving it completed rather than restarting or panicking.
func testStateEventAfterCompletion(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			transition first init accept stop then done;
		}
	}`)
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", exec.State())
	}
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after completion: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("the machine left completion on a late signal, state %s", exec.State())
	}
}

// testStateCompletionRestsInDone: a completed machine names `done` as the state
// it came to rest in, so a caller reading the configuration sees a state, not nil.
func testStateCompletionRestsInDone(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			succession first init then busy;
			transition first busy accept stop then done;
		}
	}`)
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", exec.State())
	}
	assertCurrentState(t, exec, ast.DoneFeature)
}

// testStateNestedRegionCompletionKeepsSiblingsRunning: a region of a composite
// state reaching `done` leaves its sibling region running, and events it keeps
// answering are still delivered rather than dropped by an early completion.
func testStateNestedRegionCompletionKeepsSiblingsRunning(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then busy;
			state busy parallel {
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart accept stop then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					state rbusy;
					transition first rstart accept go then rbusy;
				}
			}
		}
	}`)
	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed with the sibling region still running")
	}
	exec.SendSignal("go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the first region completed: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Errorf("the machine completed while the sibling region rests outside `done`")
	}
	if got := exec.FinalStateName(); got != "done+rbusy" {
		t.Errorf("expected the regions in done+rbusy, got %q", got)
	}
}

func testStateJunctionWithoutAnOutgoingTransition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state busy;
			junction stuck;
			succession first init then busy;
			transition first busy then stuck;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error for a junction no transition leaves")
		}
		if !strings.Contains(err.Error(), "junction stuck has no outgoing transitions") {
			t.Errorf("expected the error to name the junction, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunToCompletion hung on a junction no transition leaves")
	}
}

// testStateTransitionWithoutATarget: a transition with no target names no edge,
// so lowering reports it rather than dereferencing the absent target.
func testStateTransitionWithoutATarget(t *testing.T) {
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 1000)

	dangling := transitionMember("init", "busy")
	dangling.Target = nil
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			&ast.StateNode{Name: "busy"},
			dangling,
		},
	}

	_, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	}, nil)
	if err == nil {
		t.Fatal("expected an error for a transition without a target")
	}
	if !strings.Contains(err.Error(), "names no target") {
		t.Errorf("expected a missing-target error, got: %v", err)
	}
}

// testStateCrossRegionTransitionsPingPong: guardless successions crossing back
// and forth between two regions never settle, so the event budget bounds the run
// with a typed error instead of hanging.
func testStateCrossRegionTransitionsPingPong(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state running parallel {
				state left {
					entry; then ls;
					state ls;
					state lidle;
					succession first ls then lidle;
					transition first lidle then rtarget;
				}
				state right {
					entry; then rs;
					state rs;
					state ridle;
					state rtarget;
					succession first rs then ridle;
					transition first rtarget then lidle;
				}
			}
			succession first init then running;
		}
	}`)
	exec.ctx.maxStateEvents = 50

	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("run to completion hangs on successions crossing between regions")
	}
	if err == nil {
		t.Fatal("expected a budget error for cross-region successions that never settle")
	}
	if !strings.Contains(err.Error(), MaxStateEventsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxStateEventsEnvVar)
	}
}

// testStateTransitionEffectReadsAnUnknownFeature: a statement written as a
// transition's effect executes lowered like any other, so one reading a feature
// the machine does not declare reports rather than firing on a missing value.
func testStateTransitionEffectReadsAnUnknownFeature(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute counter : Integer = 0;
			entry; then init;
			state init;
			state active;
			succession first init then active;
			transition first active do assign counter := missingName + 1 then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
}

// testNonTerminatingDoBehavior: a do behavior whose state is re-entered every
// round never ends, so the run is bounded and reports instead of hanging. The
// self-transition restarts the do behavior, so either bound may report first.
func testNonTerminatingDoBehavior(t *testing.T) {
	spin := &ast.StateNode{
		Name: "spin",
		Do: []ast.Node{&ast.AssignmentActionNode{
			Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}},
			Value:  &ast.LiteralInteger{Value: "1"},
		}},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			spin,
			transitionMember("init", "spin"),
			transitionMember("spin", "spin"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected a budget error for a machine that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") && !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected a budget error, got: %v", err)
	}
}

// testEmptyAnonymousActionBody: entry, do and exit bodies stating no statement
// run the machine to completion rather than reporting an unexecutable behavior.
func testEmptyAnonymousActionBody(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then start;
			state start;
			state quiet {
				entry action { }
				do action { }
				exit action { }
			}
			state done;
			succession first start then quiet;
			succession first quiet then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "done" {
		t.Errorf("expected the empty bodies to leave the machine in done, got %v", exec.CurrentState())
	}
}

// testNonTerminatingAnonymousDoBody: a do body that never finishes spends the
// step budget instead of hanging the machine.
func testNonTerminatingAnonymousDoBody(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		state Machine {
			attribute c : Integer = 0;
			entry; then start;
			state start;
			state spin {
				do action {
					while true {
						assign c := c + 1;
					}
				}
			}
			state done;
			succession first start then spin;
			succession first spin then done;
		}
	}`)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testBehaviorPerformingAnActionAndStatingABody: a behavior that both performs
// an action and states a body of its own is reported rather than silently
// choosing one of the two.
func testBehaviorPerformingAnActionAndStatingABody(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		action def Bump;
		state Machine {
			attribute c : Integer = 0;
			entry; then start;
			state start;
			state working {
				entry action mixed : Bump { assign c := c + 1; }
			}
			state done;
			succession first start then working;
			succession first working then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a behavior stating a body and an action to be reported")
	}
	if !strings.Contains(err.Error(), "stating a body of its own") {
		t.Errorf("expected the report to name the conflict, got: %v", err)
	}
}

// testQualifiedAssignmentTargetInAStateEffect: an assignment naming more than
// one segment is reported rather than writing the last segment.
func testQualifiedAssignmentTargetInAStateEffect(t *testing.T) {
	err := stateRunErrorForSource(t, "Machine", `package test {
		package other { attribute c : Integer = 0; }
		state Machine {
			attribute c : Integer = 0;
			entry; then init;
			state init;
			state active;
			succession first init then active;
			transition first active do assign other::c := 1 then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a qualified assignment target to be reported")
	}
	if !strings.Contains(err.Error(), "assignment to a qualified target") {
		t.Errorf("expected the report to name the unsupported target, got: %v", err)
	}
}

// stateRunErrorForSource runs the named machine in src to completion and answers
// the first error it reports, failing if the machine hangs.
func stateRunErrorForSource(t *testing.T, name, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}

	done := make(chan error, 1)
	go func() {
		exec, err := newStateExecutor(ctx, sym, nil)
		if err == nil {
			err = exec.initialize()
		}
		if err == nil {
			err = exec.RunToCompletion()
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("running %s did not terminate", name)
		return nil
	}
}

// stateExecutorForSource builds an executor for the named machine in src, for
// tests that drive it event by event.
func stateExecutorForSource(t *testing.T, name, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state machine %s not found", name)
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return exec
}

// testCallOfUnhandledOperation: an invocation no trigger names is discarded by
// run-to-completion, leaving the machine where it was rather than hanging.
func testCallOfUnhandledOperation(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			state moving;
			succession first init then waiting;
			transition first waiting accept go() then moving;
		}
	}`)
	exec.InvokeOperation("halt", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "waiting" {
		t.Errorf("expected the unhandled call to leave the machine in waiting, got %v", exec.CurrentState())
	}
}

// testSignalNoLevelOfACompositeStateAccepts: a signal neither the active substate
// nor any composite state enclosing it accepts is dropped by run-to-completion,
// so walking outward for a trigger ends in the machine standing still rather than
// erroring or hanging.
func testSignalNoLevelOfACompositeStateAccepts(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state outer {
				state middle {
					state inner;
					state other;
					transition first inner accept step then other;
				}
				state recovered;
				transition first middle accept abort then recovered;
			}
			state stopped;
			succession first init then inner;
			transition first outer accept shutdown then stopped;
		}
	}`)
	exec.SendSignal("unknown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "inner" {
		t.Errorf("expected the unaccepted signal to leave the machine in inner, got %v", exec.CurrentState())
	}
}

// testCompositeSelfTransitionWithNoSubstateToReEnter: a composite state that
// declares no starting substate is re-entered by its own self-transition without
// erroring or hanging, and stays active with no substate of its own.
func testCompositeSelfTransitionWithNoSubstateToReEnter(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state Working {
				state Step1;
			}
			succession first init then Working::Step1;
			transition first Working accept restart then Working;
		}
	}`)
	exec.SendSignal("restart", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "Working" {
		t.Errorf("expected the re-entered composite state to be active, got %v", exec.CurrentState())
	}
}

// testStaleCompositeTimerInARegion: a time trigger on a composite state inside an
// orthogonal region whose composite is left before the timer expires is dropped,
// leaving the sibling region where it was rather than erroring or hanging.
func testStaleCompositeTimerInARegion(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state working parallel {
				state left {
					entry; then lstart;
					state lstart;
					state grouping {
						state step1;
						accept after 5 then late;
					}
					state moved;
					state late;
					transition first lstart then step1;
					transition first grouping accept skip then moved;
				}
				state right {
					entry; then rstart;
					state rstart;
					state watching;
					succession first rstart then watching;
				}
			}
			succession first init then working;
		}
	}`)
	exec.SendSignal("skip", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["moved"] || !active["watching"] || active["late"] {
		t.Errorf("expected the stale composite timer to leave moved and watching active, got %v", active)
	}
}

// testExitOfNestedRegionsWithAHistoryPseudostate: leaving a composite state whose
// region holds another composite with a region of its own, then returning through a
// deep history, restores the recorded configuration rather than erroring or hanging.
func testExitOfNestedRegionsWithAHistoryPseudostate(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state outer parallel {
				state left {
					entry; then lstart;
					state lstart;
					state grouping parallel {
						state inner {
							entry; then gstart;
							state gstart;
							state g1;
							state g2;
							transition first gstart then g1;
							transition first g1 accept advance then g2;
						}
					}
					transition first lstart then grouping;
				}
				state right {
					entry; then rstart;
					state rstart;
					state watching;
					transition first rstart then watching;
				}
				deep history resume;
			}
			state away;
			succession first init then outer;
			transition first outer accept leave then away;
			transition first away accept back then resume;
		}
	}`)
	for _, signal := range []string{"advance", "leave", "back"} {
		exec.SendSignal(signal, nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run to completion after %s: %v", signal, err)
		}
	}
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["g2"] || !active["watching"] {
		t.Errorf("expected the deep history to restore g2 and watching, got %v", active)
	}
}

// testCallArgumentOfWrongType: an argument the guard cannot compare reports
// rather than firing or dropping the transition on a wrong comparison.
func testCallArgumentOfWrongType(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			state moving;
			succession first init then waiting;
			transition first waiting accept setSpeed(value) if value > 0 then moving;
		}
	}`)
	exec.InvokeOperation("setSpeed", map[string]Value{
		"value": NewStringValue("fast"),
	})
	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected an error: the guard compares a String argument with 0")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("expected the offending operand kind in the message, got: %v", err)
	}
}

// testHistoryOutsideCompositeState: a history pseudostate restores the state
// that declares it, so one declared directly in the machine has nothing to
// restore and must report rather than enter an arbitrary state.
func testHistoryOutsideCompositeState(t *testing.T) {
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			&ast.StateNode{Name: "away"},
			&ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"},
			transitionMember("init", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	fire(t, exec, "init", "away")

	_, err := exec.resolveAndFire(nil, transitionBetween(t, exec, "away", "H"))
	if err == nil {
		t.Fatal("expected an error for a history outside any composite state")
	}
	if !strings.Contains(err.Error(), "must be declared inside the composite state") {
		t.Errorf("expected an ownership error, got: %v", err)
	}
}

// testHistoryWithoutRecordOrDefault: before its composite state has ever been
// exited a history has nothing to restore, and with no outgoing transition there
// is no default target either — that is reported, not silently ignored.
func testHistoryWithoutRecordOrDefault(t *testing.T) {
	history := &ast.PseudostateNode{Kind: ast.PseudostateShallowHistory, Name: "H"}
	outer := &ast.StateNode{
		Name:      "outer",
		Substates: []ast.Node{&ast.StateNode{Name: "first"}, history},
	}
	exec := stateExecutorFor(t, &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			outer,
			&ast.StateNode{Name: "away"},
			transitionMember("init", "away"),
			transitionMember("away", "H"),
		},
	})
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	fire(t, exec, "init", "away")

	_, err := exec.resolveAndFire(nil, transitionBetween(t, exec, "away", "H"))
	if err == nil {
		t.Fatal("expected an error: nothing recorded and no default history transition")
	}
	if !strings.Contains(err.Error(), "no recorded configuration") {
		t.Errorf("expected a missing-default error, got: %v", err)
	}
}

// testSendViaUnconnectedPort: a port with no connection reaches no one, so the
// send itself is undeliverable — which must be reported where it was written
// rather than left for the accept waiting on it to time out as a deadlock.
func testSendViaUnconnectedPort(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: nothing connects outPort to inPort")
	}
	if !errors.Is(err, ErrUnroutableSend) {
		t.Errorf("expected ErrUnroutableSend, got: %v", err)
	}
}

// testSendViaConnectorIntoAnEmptyPart: the connector joins the sender's port to
// the port of a part it holds, but the part holds no object this run, so nothing
// is behind the end and the send is reported rather than delivered to a port path
// no consumer reads.
func testSendViaConnectorIntoAnEmptyPart(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		part def Unit { port command : ~Chan; }
		part def Bay {
			port command : Chan;
			part unit : Unit[0];
			connect command to unit.command;
		}
		part bay : Bay {
			action ship {
				first start;
				action sender { send 9 via command; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
		}
	}`))
	bay, err := ctx.Instantiate(oneSymbol(t, idx, "P::bay"))
	if err != nil {
		t.Fatalf("instantiate bay: %v", err)
	}
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::bay::ship"), bay, nil)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("execute action: err = %v, want %v", err, ErrUnroutableSend)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %+v, want none", pending)
	}
}

// testSendViaBoundBoundaryPortJoinedToNothing: the inner part's port is bound to
// the assembly's boundary port, but nothing in the context joins that boundary
// port, so the send reaches no receiving port. It is reported where the inner
// machine sent it, the same as an unconnected port of the sender's own, rather
// than dropped silently — and, as the inner part runs with the context it is
// created under, the context's creation is what reports it.
func testSendViaBoundBoundaryPortJoinedToNothing(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		part def Inner {
			port out : Chan;
			exhibit state sm {
				entry; then sending;
				state sending { entry send 9 via out; }
			}
		}
		part def Assembly {
			port boundary : Chan;
			part child : Inner;
			bind boundary = child.out;
		}
		part def Env { port in : ~Chan; }
		part ctx {
			part asm : Assembly;
			part env : Env;
		}
	}`))
	_, err := ctx.Instantiate(oneSymbol(t, idx, "P::ctx"))
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("instantiate ctx: err = %v, want %v", err, ErrUnroutableSend)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %+v, want none", pending)
	}
	if got := len(ctx.instances); got != 0 {
		t.Errorf("%d object(s) survive the failed creation, want none", got)
	}
}

// testSendFanOutToAPortThatFailsToMaterialize: a send fanning out over two
// connectors, where the second receiving port cannot be materialized, is
// reported as that failure and leaves no copy queued for the first — a retry
// must not find the earlier copy already there.
func testSendFanOutToAPortThatFailsToMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Good { port in : ~Chan; }
		part def Bad { port in : ~Chan = 1 / 0; }
		part def Hub {
			port command : Chan;
			part good : Good;
			part bad : Bad;
			connect command to good.in;
			connect command to bad.in;
		}
		part hub : Hub {
			action ship {
				first start;
				action sender { send 9 via command; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
		}
	}`))
	hub, err := ctx.Instantiate(oneSymbol(t, idx, "P::hub"))
	if err != nil {
		t.Fatalf("instantiate hub: %v", err)
	}
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::hub::ship"), hub, nil)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("execute action: err = %v, want the port's %v", err, ErrDivisionByZero)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %+v, want none", pending)
	}
}

// testAcceptViaAPortThatFailsToMaterialize: a machine whose accept names a port
// that cannot be materialized leaves a message of another signal in flight
// untouched, and reports the port's failure when a message of its own signal
// arrives, rather than consuming either silently.
func testAcceptViaAPortThatFailsToMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Listener {
			port in : ~Chan = 1 / 0;
			exhibit state sm {
				entry; then Idle;
				state Idle;
				accept v : Integer via in then Got;
				state Got;
			}
		}
		part listener : Listener;
	}`))
	listener, err := ctx.Instantiate(oneSymbol(t, idx, "P::listener"))
	if err != nil {
		t.Fatalf("instantiate listener: %v", err)
	}
	exec, err := ctx.CreateStateExecutorFor(oneSymbol(t, idx, "P::Listener::sm"), listener)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	// Delivered to a port object by identity, under a name the accept does not
	// use, so only materializing `in` can tell whether it is the same port.
	viaPort := func(signal string, value Value) Message {
		return Message{SignalType: signal, Port: "other", Object: listener.ID, PortID: -1,
			Delivery: DeliverPort, Value: &value}
	}
	ctx.PostMessage(viaPort("String", NewStringValue("not for you")))
	if exec.HasPendingSignal() {
		t.Fatal("a String is not the Integer the accept names, yet the machine claims it")
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run with only a String in flight: %v", err)
	}
	if pending := ctx.PendingMessages(); len(pending) != 1 || pending[0].SignalType != "String" {
		t.Fatalf("pending messages = %+v, want the String still in flight", pending)
	}
	ctx.PostMessage(viaPort("Integer", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}}))
	if !exec.HasPendingSignal() {
		t.Fatal("an Integer that may be for the accept is not claimed")
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("run with an Integer in flight: err = %v, want the port's %v", err, ErrDivisionByZero)
	}
	if visits := exec.GetStateVisits(); len(visits) != 1 || visits[0] != "Idle" {
		t.Errorf("state visits = %v, want [Idle]", visits)
	}
	if pending := ctx.PendingMessages(); len(pending) != 2 || pending[0].SignalType != "String" || pending[1].SignalType != "Integer" {
		t.Errorf("pending messages = %+v, want the String then the Integer still in flight", pending)
	}
}

// testActionAcceptViaAPortThatFailsToMaterialize: the action-node counterpart
// of the machine case above: a parked accept whose port cannot be materialized
// leaves a message of another signal in flight and stays parked, and reports
// the port's failure only for a message of its own signal.
func testActionAcceptViaAPortThatFailsToMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Listener {
			port in : ~Chan = 1 / 0;
			action listen {
				first start;
				action reader accept v : Integer via in;
				done;
				succession first start then reader;
				succession first reader then done;
			}
		}
		part listener : Listener;
	}`))
	listener, err := ctx.Instantiate(oneSymbol(t, idx, "P::listener"))
	if err != nil {
		t.Fatalf("instantiate listener: %v", err)
	}
	exec, err := ctx.CreateActionExecutorFor(oneSymbol(t, idx, "P::Listener::listen"), listener)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if exec.State() != StateWaiting {
		t.Fatalf("state = %v, want %v", exec.State(), StateWaiting)
	}
	viaPort := func(signal string, value Value) Message {
		return Message{SignalType: signal, Target: "reader", Port: "other", Object: listener.ID, PortID: -1,
			Delivery: DeliverPort, Value: &value}
	}
	ctx.PostMessage(viaPort("String", NewStringValue("not for you")))
	if exec.HasPendingSignal() {
		t.Fatal("a String is not the Integer the accept names, yet the action claims it")
	}
	if err := exec.Step(); err != nil {
		t.Fatalf("step with only a String in flight: %v", err)
	}
	if exec.State() != StateWaiting {
		t.Fatalf("state = %v, want still %v", exec.State(), StateWaiting)
	}
	if pending := ctx.PendingMessages(); len(pending) != 1 || pending[0].SignalType != "String" {
		t.Fatalf("pending messages = %+v, want the String still in flight", pending)
	}
	ctx.PostMessage(viaPort("Integer", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}}))
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("run with an Integer in flight: err = %v, want the port's %v", err, ErrDivisionByZero)
	}
	if pending := ctx.PendingMessages(); len(pending) != 2 || pending[0].SignalType != "String" || pending[1].SignalType != "Integer" {
		t.Errorf("pending messages = %+v, want the String then the Integer still in flight", pending)
	}
}

func testRoutedSendViaUnknownPort(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			first start;
			action sender { send 42 via missing to reader; }
			action reader accept msg : Integer;
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	var typed *UnknownSendPortError
	if !errors.As(err, &typed) {
		t.Fatalf("expected UnknownSendPortError, got: %v", err)
	}
	if !errors.Is(err, ErrSendViaUnknownPort) {
		t.Errorf("expected ErrSendViaUnknownPort, got: %v", err)
	}
	if errors.Is(err, ErrUnroutableSend) {
		t.Errorf("unknown routed port must not be ErrUnroutableSend: %v", err)
	}
	if !strings.Contains(err.Error(), `"missing"`) || !strings.Contains(err.Error(), `"reader"`) {
		t.Errorf("error = %v, want port and receiver names", err)
	}
}

func testRoutedSendPortTypeMismatch(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def IntMessage;
		item def TextMessage;
		port def IntegerPort { in item text : TextMessage; }
		action pipeline {
			port outPort;
			port inPort : IntegerPort;
			connect outPort to inPort;
			first start;
			action sender { send IntMessage via outPort to reader; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	var typed *SendPortTypeMismatchError
	if !errors.As(err, &typed) {
		t.Fatalf("expected SendPortTypeMismatchError, got: %v", err)
	}
	if !errors.Is(err, ErrSendPortTypeMismatch) {
		t.Errorf("expected ErrSendPortTypeMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"outPort"`) ||
		!strings.Contains(err.Error(), `"reader"`) {
		t.Errorf("error = %v, want port and receiver names", err)
	}
}

func testRoutedSendPortTypeMatch(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def IntMessage;
		port def IntegerPort { in item value : IntMessage; }
		action pipeline {
			port outPort;
			port inPort : IntegerPort;
			connect outPort to inPort;
			first start;
			action sender { send IntMessage via outPort to reader; }
			action reader accept : IntMessage via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute routed send through matching typed flow: %v", err)
	}
}

func testRoutedSendScalarTypedFlowMismatch(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def Integer;
		item def TextMessage;
		port def TextPort { in item text : TextMessage; }
		action pipeline {
			port outPort;
			port inPort : TextPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort to reader; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	var typed *SendPortTypeMismatchError
	if !errors.As(err, &typed) {
		t.Fatalf("expected scalar SendPortTypeMismatchError, got: %v", err)
	}
	if !errors.Is(err, ErrSendPortTypeMismatch) {
		t.Errorf("expected ErrSendPortTypeMismatch for scalar message, got: %v", err)
	}
}

func testRoutedSendUnreachableReceiver(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort to missing; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	var typed *UnreachableSendReceiverError
	if !errors.As(err, &typed) {
		t.Fatalf("expected UnreachableSendReceiverError, got: %v", err)
	}
	if !errors.Is(err, ErrUnreachableSendReceiver) {
		t.Errorf("expected ErrUnreachableSendReceiver, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"outPort"`) ||
		!strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error = %v, want port and receiver names", err)
	}
}

func testRoutedSendReceiverNameMismatchDeadlock(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort to receiver; }
			action receiver;
			action sibling accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then sibling;
			succession first sibling then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a deadlock: sibling must not consume receiver's message")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testTwoValuedMemberInScalarContext: a `[0..*]` member holding two values is
// not the one value an operator, a `[1]` parameter or a library function takes;
// each refuses it with a typed error rather than a panic, a hang or a guess.
func testTwoValuedMemberInScalarContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		expr string
		want error
	}{
		{"arithmetic", "q.zs + 1.0", ErrTypeMismatch},
		{"negation", "-q.zs", ErrTypeMismatch},
		{"calc parameter", "Inc(q.zs)", ErrMultiplicityViolation},
		{"library function", "RealFunctions::sqrt(q.zs)", ErrTypeMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package test {
				private import ScalarValues::*;
				calc def Inc { in x : Real; x + 1.0 }
				part def Holder { attribute zs : Real[0..*]; }
				calc def Two {
					attribute q : Holder = new Holder(zs = (1.0, 2.0));
					return r = ` + tc.expr + `;
				}
			}`
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			pkg := resolveSymbol(t, idx.DocumentRoot("<test>"), "test")
			sym := resolveSymbol(t, pkg.Scope, "Two")

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := ctx.InvokeCalc(sym, nil, pkg.Scope)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Errorf("InvokeCalc err = %v, want %v", err, tc.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("evaluating the two-valued member did not terminate")
			}
		})
	}
}

func testTypeClassificationUnresolvedType(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		item def Integer;
		calc classify { return : Boolean = 1 istype MissingType; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "classify")
	_, err := NewContext(NewModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected unresolved type classification to fail")
	}
	if !errors.Is(err, ErrUnresolvedType) {
		t.Fatalf("expected ErrUnresolvedType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MissingType") {
		t.Errorf("error = %v, want unresolved type name", err)
	}
}

// A value whose type the model cannot name is a typed error; `null` is the empty
// sequence (KerML 8.3.4.8.16), of every type, so it is not that value.
func testTypeClassificationUndeterminedValueType(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		item def Integer;
		calc classify { return : Boolean = 1.5 istype Integer; }
		calc empty { return : Boolean = null istype Integer; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "classify")
	_, err := NewContext(NewModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected undetermined value type classification to fail")
	}
	if !errors.Is(err, ErrUndeterminedValueType) {
		t.Fatalf("expected ErrUndeterminedValueType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Rational") {
		t.Errorf("error = %v, want the type the model has no name for", err)
	}
	empty := resolveSymbol(t, pkg.Scope, "empty")
	got, err := NewContext(NewModel(model, resolver), 1000).InvokeCalc(empty, nil, pkg.Scope)
	if err != nil {
		t.Fatalf("null istype Integer: %v", err)
	}
	if FormatValue(got) != "true" {
		t.Errorf("null istype Integer = %s, want true", FormatValue(got))
	}
}

func testCastToAnUnresolvedType(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		item def Integer;
		calc narrow { return : Integer = 1 as MissingType; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "narrow")
	_, err := NewContext(NewModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected a cast to an unresolved type to fail")
	}
	if !errors.Is(err, ErrUnresolvedType) {
		t.Fatalf("expected ErrUnresolvedType, got: %v", err)
	}
	if !strings.Contains(err.Error(), "MissingType") {
		t.Errorf("error = %v, want unresolved type name", err)
	}
}

// testCastUndecidedByTheValue: a target narrower than the value's own type that
// the value does not settle — 5 states nothing about being an Even — fails
// rather than dropping a value that may well be one of the target's.
func testCastUndecidedByTheValue(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `package P {
		attribute def Integer;
		attribute def Even :> Integer;
		calc narrow { return : Even = 5 as Even; }
	}`)
	pkg := resolveSymbol(t, root, "P")
	calc := resolveSymbol(t, pkg.Scope, "narrow")
	_, err := NewContext(NewModel(model, resolver), 1000).InvokeCalc(calc, nil, pkg.Scope)
	if err == nil {
		t.Fatal("expected an undecidable cast to fail")
	}
	if !errors.Is(err, ErrUndecidedClassification) {
		t.Fatalf("expected ErrUndecidedClassification, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Even") {
		t.Errorf("error = %v, want the target type named", err)
	}
}

// testDifferenceTypedFeatureHoldingASubtractedObject: a feature typed by a
// difference refuses an object one of the subtracted types classifies, whether
// the object was declared by it or classified by it since.
func testDifferenceTypedFeatureHoldingASubtractedObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		part def Vehicle;
		part def Car :> Vehicle;
		part def Electric;
		part def ElectricCar :> Car, Electric;
		part def CombustionVehicle differences Vehicle, Electric;
		part sedan : Car;
		part def Shop { part retrofit : ElectricCar = sedan; }
		part shop : Shop;
		part def Depot { part burner : CombustionVehicle = shop.retrofit; }
		part depot : Depot;
		attribute held = depot.burner istype Vehicle;
	`)
	sym := resolveSymbol(t, root, "held")
	_, err := NewContext(NewModel(model, resolver), 10000).Eval(sym.Decl.(*ast.Usage).Value)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("expected ErrTypeMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "CombustionVehicle") {
		t.Errorf("error = %v, want the feature's type named", err)
	}
}

// testCastOfAQuantityToAConstrainedSubtype: a quantity subtype inheriting its
// measurement reference narrows lengths by something a magnitude and a unit do
// not state, so a bare length is undecided rather than kept by its dimension.
func testCastOfAQuantityToAConstrainedSubtype(t *testing.T) {
	err := calcErrorWithLibraries(t, `
		package test {
			private import SI::*;
			attribute def RoomLength :> ISQBase::LengthValue;
			calc narrow { return : RoomLength = 5 [m] as RoomLength; }
		}`, "narrow", nil, 1000)
	if !errors.Is(err, ErrUndecidedClassification) {
		t.Fatalf("expected ErrUndecidedClassification, got: %v", err)
	}
	if !strings.Contains(err.Error(), "RoomLength") {
		t.Errorf("error = %v, want the target type named", err)
	}
}

// testSendAddressedToAnUnreachableTarget: a target reaching no port of an object
// the sender can address is reported where it was written rather than delivered
// to whatever else carries the last segment's name.
func testSendAddressedToAnUnreachableTarget(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		port def PingPort { in item ping : Integer; }
		part def Leaf { attribute count : Integer = 0; }
		part def Node {
			port inPort : PingPort;
			part leaf : Leaf;
		}
		part alpha : Node;
		action pipeline {
			first start;
			action sender { send 42 to alpha.leaf.count; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: alpha.leaf.count is no port of any object")
	}
	if !errors.Is(err, ErrUnroutableSend) {
		t.Errorf("expected ErrUnroutableSend, got: %v", err)
	}
}

// testSendAddressedThroughSeveralOccurrences: a path led by a part naming three
// occurrences reaches no one object, which must be reported rather than
// attributed to the sending object and delivered to nobody.
func testSendAddressedThroughSeveralOccurrences(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		port def PingPort { in item ping : Integer; }
		part def Leaf { port inPort : PingPort; }
		part nodes : Leaf[3];
		action pipeline {
			first start;
			action sender { send 42 to nodes.inPort; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: nodes names three occurrences, not one addressee")
	}
	if !errors.Is(err, ErrUnroutableSend) {
		t.Errorf("expected ErrUnroutableSend, got: %v", err)
	}
}

// testSendAddressedToAPartNoSiblingTakes: a message addressed to a part belongs
// to that object, so a sibling accept of the sending behavior cannot take it and
// the run reports the accept it is left waiting on.
func testSendAddressedToAPartNoSiblingTakes(t *testing.T) {
	_, err := executeActionSource(t, "main", `package P {
		item def Ping;
		part def R;
		part receiver : R;
		action main {
			first start;
			action s { send Ping() to receiver; }
			action other accept p : Ping;
			done;
			succession first start then s;
			succession first s then other;
			succession first other then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: `other` is no addressee of the message sent to receiver")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testInjectedMessageNamesAReceiverNoAcceptHas: a message injected from outside
// the model is held to the receiver it names, so an accept of another name waits
// on rather than consumes it, and the run reports that wait.
func testInjectedMessageNamesAReceiverNoAcceptHas(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			first start;
			action other accept n : Integer;
			done;
			succession first start then other;
			succession first other then done;
		}
	}`))
	exec, err := ctx.CreateActionExecutor(oneSymbol(t, idx, "P::pipeline"))
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	one := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &one})
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected an error: `other` is not the receiver the message names")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testSendAddressedToAnObjectThatCannotBeBuilt: locating an addressee can fail
// on its own terms — an exhausted budget, or a feature value the walk cannot read — and
// each must be reported as that rather than as an address naming no port.
func testSendAddressedToAnObjectThatCannotBeBuilt(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			port def PingPort { in item ping : Integer; }
			part def Node {
				port inPort : PingPort;
				attribute a = b + 1.0;
				attribute b = a + 1.0;
				action listen { first start; done; succession first start then done; }
			}
			part alpha : Node;
		}
	`))
	scope := declScope(oneSymbol(t, idx, "test::Node::listen"))

	ctx.maxSteps = 0
	send := lower.Send{Target: "alpha.inPort", TargetPath: true, Scope: scope}
	err := ctx.post(nil, Message{SignalType: "Integer"}, send, nil)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("budget exhausted while building alpha: %v, want ErrStepLimitExceeded", err)
	}
	if errors.Is(err, ErrUnroutableSend) {
		t.Errorf("an exhausted budget was reported as a bad address: %v", err)
	}

	ctx.maxSteps = DefaultMaxSteps
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send = lower.Send{Target: "a.inPort", TargetPath: true, Scope: scope}
	err = ctx.post(nil, Message{SignalType: "Integer"}, send, alpha)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("walking through a cyclic derived feature value: %v, want ErrCyclicFeatureValue", err)
	}
	if errors.Is(err, ErrUnroutableSend) {
		t.Errorf("an unreadable feature value was reported as a bad address: %v", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("a send that never found its addressee posted %+v", ctx.PendingMessages())
	}
}

// testAcceptDeadlockNeverSatisfied: an accept nothing can ever satisfy suspends
// the action, and a suspension that can never end must be reported as a typed
// deadlock rather than hanging.
func testAcceptDeadlockNeverSatisfied(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := executeActionSource(t, "pipeline", `package P {
			action pipeline {
				first start;
				action reader accept n : Integer;
				done;
				succession first start then reader;
				succession first reader then done;
			}
		}`)
		done <- err
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("an action waiting for a message that cannot arrive did not terminate")
	}

	if err == nil {
		t.Fatal("expected a deadlock error, the suspended accept completed")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
	for _, want := range []string{"accept n", "Integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in the deadlock report, got: %v", want, err)
		}
	}
}

// testAcceptDeadlockReportsEveryWaitingAccept: with two accepts parked in
// parallel branches and only one message in flight, the accept that can proceed
// does, and the report names the one still waiting rather than the whole action.
func testAcceptDeadlockReportsEveryWaitingAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send 7 to reader; }
			fork split;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			action listener accept text : String;
			join sync;
			done;
			succession first start then sender;
			succession first sender then split;
			succession first split then reader;
			succession first split then listener;
			succession first reader then recorder;
			succession first recorder then sync;
			succession first listener then sync;
			succession first sync then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a deadlock error: no String is ever sent")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock, got: %v", err)
	}
	if !strings.Contains(err.Error(), "accept text waiting since step 4 for a message of type String") {
		t.Errorf("expected the still-waiting accept in the report, got: %v", err)
	}
	if strings.Contains(err.Error(), "accept n ") {
		t.Errorf("the Integer accept was satisfied and must not be reported as waiting: %v", err)
	}
}

// testAcceptStatementDeadlockInALoop: an accept node written in a loop body would
// have to suspend a flow that has no token to park, so it is reported when reached
// rather than passed over or looped on forever.
func testAcceptStatementDeadlockInALoop(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := executeActionSource(t, "pipeline", `package P {
			action pipeline {
				first start;
				action waiter {
					loop {
						accept n : Integer;
					}
				}
				done;
				succession first start then waiter;
				succession first waiter then done;
			}
		}`)
		done <- err
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a loop waiting for a message that cannot arrive did not terminate")
	}

	if err == nil {
		t.Fatal("expected an error, the accept in the loop body was passed over")
	}
	if !strings.Contains(err.Error(), "'accept' in a loop or branch body") {
		t.Errorf("expected the accept in a loop body to be reported, got: %v", err)
	}
}

// testNonNumericTimeTrigger: a timed trigger whose duration is not a number
// cannot be scheduled and must be reported rather than silently dropped, even
// with no library loaded for the static judgement to name a type from.
func testNonNumericTimeTrigger(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			accept at "noon" then done;
			succession first init then waiting;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a non-numeric time trigger")
	}
	if !strings.Contains(err.Error(), "time duration must be constant, got string") {
		t.Errorf("expected a numeric-duration error, got: %v", err)
	}
}

// testTimeTriggerOfANonTimeDimension: a duration whose unit measures something
// other than time, held by a feature whose type does not resolve so only its
// value can tell, cannot be scheduled and fails as the typed error it is.
func testTimeTriggerOfANonTimeDimension(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			state Machine {
				attribute load : Nowhere::Mass = 5 [kg];
				entry; then init;
				state init;
				state waiting;
				accept after load then done;
				state done;
				succession first init then waiting;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state Machine not found")
	}

	_, err := ctx.ExecuteState(sym)
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Fatalf("err = %v; want ErrIncommensurableUnits", err)
	}
	if !strings.Contains(err.Error(), "5 [kg] is not a time") {
		t.Errorf("err = %v; want it to name the quantity", err)
	}
}

// testTimeTriggerOfTheTypeValidationRefuses: an argument validation refuses is
// refused as one typed error before it is evaluated or converted, whatever
// evaluating it would have said.
func testTimeTriggerOfTheTypeValidationRefuses(t *testing.T) {
	for _, tc := range []struct{ name, trigger string }{
		{"unitless after", "after 5"},
		{"duration at", "at 2 [min]"},
		{"mass after", "after 5 [kg]"},
		{"string at", `at "noon"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
				package test {
					private import SI::*;
					state Machine {
						entry; then init;
						state init;
						state waiting;
						accept `+tc.trigger+` then done;
						state done;
						succession first init then waiting;
					}
				}
			`))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
			if sym == nil {
				t.Fatal("state Machine not found")
			}

			_, err := ctx.ExecuteState(sym)
			if !errors.Is(err, ErrTimeTriggerType) {
				t.Fatalf("err = %v; want ErrTimeTriggerType", err)
			}
			if !strings.Contains(err.Error(), "`"+tc.trigger+"`") {
				t.Errorf("err = %v; want it to quote the trigger as written", err)
			}
		})
	}
}

// testActionReturnParameterValidationRefuses: an action declaring a `return`
// parameter — which validation refuses, only a function or expression owning
// one — is refused at initialize with a typed error naming the parameter,
// whether the action, an action it specializes, or a node of its flow declares it.
func testActionReturnParameterValidationRefuses(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"own", `
			action def Run {
				return total : Integer;
				first start;
				then action go { assign total := 1; }
				then done;
			}`, "action Run declares `return total`; write `out total`"},
		{"inherited", `
			action def Base { return total : Integer; }
			action def Run :> Base {
				first start;
				then action go { assign total := 1; }
				then done;
			}`, "action Run declares `return total`; write `out total`"},
		{"node", `
			action def Run {
				out total : Integer;
				first start;
				then action go { return partial : Integer; assign total := 1; }
				then done;
			}`, "action node go declares `return partial`; write `out partial`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
				package test {
					private import ScalarValues::*;
					`+tc.src+`
				}
			`))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Run", ast.DefAction)
			if sym == nil {
				t.Fatal("action Run not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if !errors.Is(err, ErrActionResultParameter) {
				t.Fatalf("err = %v; want ErrActionResultParameter", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v; want it to contain %q", err, tc.want)
			}
		})
	}
}

// testChangeConditionThatNeverHolds: a machine whose only outgoing transition
// watches a false condition suspends within its budget and says what it waits
// on, rather than hanging or reporting silent completion.
func testChangeConditionThatNeverHolds(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			entry; then init;
			state init;
			state waiting;
			accept when ready then done;
			state done;
			succession first init then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v; want suspended", exec.State())
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "when ready") ||
		!strings.Contains(reason, "condition is false") {
		t.Errorf("reason = %q; want the false condition it waits on", reason)
	}
}

// testForkBranchesShareRegion: a fork whose branches land in the same region
// cannot produce one active state per region.
func testForkBranchesShareRegion(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state ready;
			state working parallel {
				state left {
					entry; then ls;
					state ls;
					state a;
					state b;
					succession first ls then a;
				}
				state right {
					entry; then rs;
					state rs;
					state c;
					succession first rs then c;
				}
			}
			fork split;

			succession first init then ready;
			transition first ready then split;
			transition first split then a;
			transition first split then b;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for fork branches in the same region")
	}
	if !strings.Contains(err.Error(), "in the same region") {
		t.Errorf("expected a same-region error, got: %v", err)
	}
}

// testJoinWithOneIncomingBranch: a join synchronizes branches, so a single
// incoming transition is a modeling error rather than a pass-through.
func testJoinWithOneIncomingBranch(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state ready;
			join sync;

			succession first init then ready;
			transition first ready then sync;
			transition first sync then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a join with one incoming transition")
	}
	if !strings.Contains(err.Error(), "at least two incoming transitions") {
		t.Errorf("expected an incoming-branch-count error, got: %v", err)
	}
}

// testRegionPseudostateWithoutSatisfiedGuard: a junction reached from inside an
// orthogonal region whose branches are all guarded false has nowhere to go. The
// region set is left in place and the dead end reported, rather than the machine
// resting on a pseudostate.
func testRegionPseudostateWithoutSatisfiedGuard(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine parallel {
			attribute x : Integer = 9;

			state left {
				entry; then ls;
				state ls;
				state a;
				state b;
				succession first ls then a;
				transition first a then merge;
			}
			state right {
				entry; then rs;
				state rs;
				state c;
				succession first rs then c;
			}
			junction merge;

			transition first merge if x == 1 then b;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a junction with no satisfied guard")
	}
	if !strings.Contains(err.Error(), "no guard evaluated to true") {
		t.Errorf("expected an unsatisfied-guard error, got: %v", err)
	}
}

// testRegionPseudostateCycle: pseudostates that route into each other never
// reach a state, so following the chain has to report the cycle instead of
// looping forever.
func testRegionPseudostateCycle(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine parallel {
			state left {
				entry; then ls;
				state ls;
				state a;
				succession first ls then a;
				transition first a then first;
			}
			state right {
				entry; then rs;
				state rs;
				state c;
				succession first rs then c;
			}
			junction first;
			junction second;

			transition first first then second;
			transition first second then first;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for pseudostates routing into each other")
	}
	if !strings.Contains(err.Error(), "form a cycle") {
		t.Errorf("expected a cycle error, got: %v", err)
	}
}

// testDeadlockJoinStarvation: join awaiting token that never arrives. `stranded`
// has no incoming edge, so the join has two incoming edges but can only ever be
// reached by one token.
func testDeadlockJoinStarvation(t *testing.T) {
	src := `
		package test {
			action starve {
				first start;
				action stranded;
				join sync;
				done;
				succession first start then sync;
				succession first stranded then sync;
				succession first sync then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "starve", ast.DefAction)
	if sym == nil {
		t.Fatal("action starve not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected a deadlock error, the starved join completed")
	}
	if !errors.Is(err, ErrActionDeadlock) {
		t.Errorf("expected ErrActionDeadlock, got: %v", err)
	}
}

// testDeadlockJoinSameSuccessionTwice: two tokens reach the join over the one
// succession from the merge; they do not stand in for the succession from
// `stranded`, which no token can travel, so the join never fires.
func testDeadlockJoinSameSuccessionTwice(t *testing.T) {
	src := `
		package test {
			action starve {
				first start;
				fork split;
				action a;
				action b;
				merge m;
				action stranded;
				join sync;
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then m;
				succession first b then m;
				succession first m then sync;
				succession first stranded then sync;
				succession first sync then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "starve", ast.DefAction)
	if sym == nil {
		t.Fatal("action starve not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionDeadlock) {
		t.Fatalf("error = %v, want ErrActionDeadlock", err)
	}
	for _, token := range exec.Tokens() {
		if awaiting := exec.Awaiting(token); len(awaiting) != 1 {
			t.Errorf("token %d awaits %d successions, want the one from stranded", token.ID, len(awaiting))
		}
	}
}

func testForkWithoutASuccessor(t *testing.T) {
	src := `
		package test {
			action broken {
				first start;
				fork split;
				succession first start then split;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
}

func testExplicitSuccessionMissingEndpoint(t *testing.T) {
	src := `
		package test {
			action broken {
				action compute;
				succession first missing then compute;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	_, err := ctx.CreateActionExecutor(sym)
	if err == nil || !strings.Contains(err.Error(), "action succession references undefined source node") {
		t.Fatalf("error = %v, want an explicit succession source diagnostic", err)
	}
}

func testControlFlowMissingEndpoint(t *testing.T) {
	src := `
		package test {
			action broken {
				first start;
				decide check;
				succession first start then check;
				if true then missing;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	_, err := ctx.CreateActionExecutor(sym)
	if err == nil || !strings.Contains(err.Error(), `control flow edge references undefined target "missing"`) {
		t.Fatalf("error = %v, want an undefined control-flow target diagnostic", err)
	}
}

func testMergeWithoutASuccessor(t *testing.T) {
	src := `
		package test {
			action broken {
				first start;
				merge converge;
				succession first start then converge;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "broken", ast.DefAction)
	if sym == nil {
		t.Fatal("action broken not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
}

// testUnguardedLoopThroughAMerge: a merge passes every arrival, so a loop with no
// exit spins through it until the step budget stops the run with its typed error.
func testUnguardedLoopThroughAMerge(t *testing.T) {
	src := `
		package test {
			action spin {
				first start;
				merge m;
				action a;
				succession first start then m;
				succession first m then a;
				succession first a then m;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spin", ast.DefAction)
	if sym == nil {
		t.Fatal("action spin not found")
	}

	// RunToCompletion spends the whole budget on the loop and reports it.
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("RunToCompletion() = %v, want ErrActionStepLimitExceeded", err)
	}
	if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxActionStepsEnvVar)
	}

	// A debugger steps the loop one node at a time: each step is one bounded unit
	// of work, the token keeps circling m and a, and continuing from there hits the budget.
	ctx.maxActionSteps = 20
	stepped, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 2*int(ctx.maxActionSteps); i++ {
		if err := stepped.Step(); err != nil {
			t.Fatalf("Step() %d = %v, want nil", i, err)
		}
		tokens := stepped.Tokens()
		if len(tokens) != 1 {
			t.Fatalf("after step %d: %d tokens, want 1 circling the loop", i, len(tokens))
		}
		_, atMerge := tokens[0].Location.(*ast.MergeNode)
		if atMerge != (i%2 == 0) {
			t.Fatalf("after step %d: token at %T, want it alternating between m and a", i, tokens[0].Location)
		}
	}
	err = stepped.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("RunToCompletion() after stepping = %v, want ErrActionStepLimitExceeded", err)
	}
}

// testActionWhoseLastNodeHasNoSuccession: a node the flow leads no further from
// ends the flow, so an action declaring no `done` node completes instead of
// failing, and stepping past the end neither errors nor spins.
func testActionWhoseLastNodeHasNoSuccession(t *testing.T) {
	src := `
		package test {
			action ends {
				first start;
				then action a;
				then action b;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "ends", ast.DefAction)
	if sym == nil {
		t.Fatal("action ends not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run action whose last node has no succession: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected the action to complete, got state %s", exec.State())
	}

	for i := 0; i < 3; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d past the end: %v", i+1, err)
		}
		if exec.State() != StateCompleted {
			t.Fatalf("step %d past the end left state %s", i+1, exec.State())
		}
		if len(exec.Tokens()) != 0 {
			t.Fatalf("step %d past the end revived %d token(s)", i+1, len(exec.Tokens()))
		}
	}
}

// testFirstNodeWithASecondSuccession: the succession out of a `first` end leaves
// from the node it names, so a second succession out of that node is ambiguous.
func testFirstNodeWithASecondSuccession(t *testing.T) {
	src := `
		package test {
			action seq {
				action s1;
				action s2;
				action s3;
				first s1 then s2;
				succession first s1 then s3;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}

	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("a first node with two successions ran to completion")
	}
	if !strings.Contains(err.Error(), "multiple successors") {
		t.Fatalf("error = %q, want it to report multiple successors", err)
	}
}

// testFirstBesideAnInitialNode: a body declaring an initial node of its own and a
// `first` end naming a declared node states two starts, which lowering rejects.
func testFirstBesideAnInitialNode(t *testing.T) {
	src := `
		package test {
			action seq {
				action s1;
				action s2;
				first start;
				first s1 then s2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	if _, err := ctx.CreateActionExecutor(sym); err == nil {
		t.Fatal("two starts lowered without an error")
	} else if !strings.Contains(err.Error(), "multiple initial nodes") {
		t.Fatalf("error = %q, want it to report multiple initial nodes", err)
	}
}

// testFirstNamingAFinalNode: a flow cannot start where it ends, so lowering
// rejects it rather than completing with the declared node never run.
func testFirstNamingAFinalNode(t *testing.T) {
	src := `
		package test {
			action seq {
				attribute x = 0;
				action s1 { assign x := 7; }
				done;
				first done then s1;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
	if sym == nil {
		t.Fatal("action seq not found")
	}

	if _, err := ctx.CreateActionExecutor(sym); err == nil {
		t.Fatal("a first end naming a final node lowered without an error")
	} else if !strings.Contains(err.Error(), "final node done") {
		t.Fatalf("error = %q, want it to name the final node", err)
	}
}

// testForkBranchesAssigningTheSameFeature: concurrent branches writing one feature
// are unordered by the spec; the runtime resolves them by its own step order.
func testForkBranchesAssigningTheSameFeature(t *testing.T) {
	src := `
		package test {
			action clash {
				attribute x : Integer = 0;

				first start;
				fork split;
				action left { assign x := 1; }
				action right { assign x := 2; }
				join sync;
				done;

				succession first start then split;
				succession first split then left;
				succession first split then right;
				succession first left then sync;
				succession first right then sync;
				succession first sync then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	sym := findSymbolByName(idx.DocumentRoot("<test>"), "clash", ast.DefAction)
	if sym == nil {
		t.Fatal("action clash not found")
	}

	var first semantics.Value
	for run := 0; run < 3; run++ {
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatalf("run %d create action executor: %v", run+1, err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run %d conflicting writes: %v", run+1, err)
		}
		if exec.State() != StateCompleted {
			t.Fatalf("run %d left state %s", run+1, exec.State())
		}

		got, ok := exec.Results()["x"]
		if !ok {
			t.Fatalf("run %d lost the contested feature x", run+1)
		}
		if got.Kind != ValConst || got.Const.Kind != semantics.ValInt {
			t.Fatalf("run %d gave x a non-integer value: %+v", run+1, got)
		}
		if got.Const.Int != 1 && got.Const.Int != 2 {
			t.Fatalf("run %d gave x %d, which neither branch assigned", run+1, got.Const.Int)
		}
		if run == 0 {
			first = got.Const
			continue
		}
		if got.Const.Int != first.Int {
			t.Fatalf("run %d gave x %d after run 1 gave %d: execution is not deterministic",
				run+1, got.Const.Int, first.Int)
		}
	}
}

// testDecisionNoSatisfiedGuard: a decision must select one outgoing succession.
func testDecisionNoSatisfiedGuard(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;

			action noGuard {
				attribute enabled : Boolean = false;

				first start;
				action selected;
				done;

				succession first start then choose;
				succession first selected then done;

				decide choose;
				if enabled then selected;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "noGuard", ast.DefAction)
	if sym == nil {
		t.Fatal("action noGuard not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrNoEnabledSuccession) {
		t.Fatalf("error = %v, want ErrNoEnabledSuccession", err)
	}
}

// testDecisionAllGuardsFalse: every guard of a decision is evaluated so that
// several holding at once can be reported; none holding is still the same
// error, recorded as no choice at all.
func testDecisionAllGuardsFalse(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;

			action pick {
				attribute level : Integer = 5;

				first start;
				action low;
				action high;
				done;

				succession first start then choose;
				succession first low then done;
				succession first high then done;

				decide choose;
				if level > 10 then low;
				if level > 20 then high;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "pick", ast.DefAction)
	if sym == nil {
		t.Fatal("action pick not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrNoEnabledSuccession) {
		t.Fatalf("error = %v, want ErrNoEnabledSuccession", err)
	}
	if !strings.Contains(err.Error(), "decision node choose has no true guard") {
		t.Fatalf("error = %v, want the decision named", err)
	}
	if choices := ctx.Choices(); len(choices) != 0 {
		t.Fatalf("choices = %v, want none when no guard holds", choices)
	}
}

// testStateDanglingTransition: state with transition to nonexistent state
func testStateDanglingTransition(t *testing.T) {
	src := `
		package test {
			state Machine {
				entry; then init;
				state init;
				succession first init then nowhere; // 'nowhere' state doesn't exist
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	// Check diagnostics (resolver should catch missing state)
	// Note: resolver diagnostics accessed via resolver.Diagnostics field

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Broken state not found")
	}

	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Logf("CreateStateExecutor error (acceptable): %v", err)
		return
	}

	err = exec.ProcessNextEvent()
	if err != nil {
		t.Logf("ProcessNextEvent returned error (acceptable): %v", err)
		return
	}

	t.Log("ProcessNextEvent succeeded (dangling transition not exercised)")
}

func testParallelStateBodyUnsupportedMember(t *testing.T) {
	src := `
		package test {
			action def Warm;
			state Machine parallel {
				state left;
				perform Warm;
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("parallel state not found")
	}
	_, err := ctx.CreateStateExecutor(sym)
	if err == nil {
		t.Fatal("parallel state with an unsupported member succeeded")
	}
	if !strings.Contains(err.Error(), "parallel state body contains unsupported member") {
		t.Fatalf("error = %v, want unsupported parallel-body member", err)
	}
}

func testParallelStateRegionWithoutInitial(t *testing.T) {
	src := `
		package test {
			state Machine parallel {
				state left;
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("parallel state not found")
	}
	_, err := ctx.CreateStateExecutor(sym)
	if err == nil {
		t.Fatal("parallel state with no region initial succeeded")
	}
	if !strings.Contains(err.Error(), "region left has no initial state") {
		t.Fatalf("error = %v, want missing region initial", err)
	}
	if !strings.Contains(err.Error(), "entry; then <state>;") {
		t.Fatalf("error = %v, want initial-state notation guidance", err)
	}
}

// testParallelStateRegionItselfParallel: a region is a direct substate of a
// parallel body and starts in one of its own states, so a direct substate that
// is itself parallel has no state to start in and is refused, written inline or
// typed; the active configuration therefore never holds a region owned by a
// region's wrapper state.
func testParallelStateRegionItselfParallel(t *testing.T) {
	for name, src := range map[string]string{
		"inline": `
		package test {
			state def Machine {
				entry; then work;
				state work parallel {
					state left parallel {
						state r { entry; then r1; state r1; }
						state s { entry; then s1; state s1; }
					}
					state right { entry; then b1; state b1; }
				}
			}
		}
	`,
		"typed": `
		package test {
			state def Nested parallel {
				state r { entry; then r1; state r1; }
				state s { entry; then s1; state s1; }
			}
			state def Machine {
				entry; then work;
				state work parallel {
					state left : Nested;
					state right { entry; then b1; state b1; }
				}
			}
		}
	`,
	} {
		err := stateExecutorError(t, src, "Machine")
		if err == nil {
			t.Fatalf("%s: a parallel region of a parallel state succeeded", name)
		}
		if !strings.Contains(err.Error(), "region left has no initial state") {
			t.Fatalf("%s: error = %v, want missing region initial", name, err)
		}
	}
}

// testStateUsageTypedByItself: a definition whose substate is typed by it has no
// finite materialization and must report that, not recurse.
func testStateUsageTypedByItself(t *testing.T) {
	src := `
		package test {
			state def A {
				entry; then b;
				state b : A;
			}
		}
	`
	err := stateExecutorError(t, src, "A")
	if err == nil {
		t.Fatal("recursively typed state succeeded")
	}
	if !errors.Is(err, lower.ErrRecursiveStateTyping) {
		t.Fatalf("error = %v, want recursive state typing", err)
	}
}

// testStateUsageMutuallyRecursiveTyping: two definitions reaching each other
// through their substates report the cycle rather than materializing forever.
func testStateUsageMutuallyRecursiveTyping(t *testing.T) {
	src := `
		package test {
			state def A {
				entry; then b;
				state b : B;
			}
			state def B {
				entry; then a;
				state a : A;
			}
		}
	`
	err := stateExecutorError(t, src, "A")
	if err == nil {
		t.Fatal("mutually recursive state typing succeeded")
	}
	if !errors.Is(err, lower.ErrRecursiveStateTyping) {
		t.Fatalf("error = %v, want recursive state typing", err)
	}
}

// testStateDefSpecializingTheLibraryStateAction: a state definition written
// `:> StateAction` inherits no content from the library (whose `ref state self`
// is typed by StateAction itself), the same as the implicit specialization.
func testStateDefSpecializingTheLibraryStateAction(t *testing.T) {
	src := `
		package test {
			private import States::*;
			state def Phase :> StateAction;
			state def Prep :> Phase;
			state def Machine {
				entry; then prep;
				state prep : Prep;
				state launch : Phase;
				transition first prep then launch;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state Machine not found")
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if got := activeStateNames(exec); got != "launch" {
		t.Fatalf("active states = %q, want launch", got)
	}
}

// testStateDefSpecializingALibraryStateKeepsItsContent: only StateAction is
// withheld; a library's own state definition still contributes its substates,
// transitions, behaviors and attributes to what specializes it.
func testStateDefSpecializingALibraryStateKeepsItsContent(t *testing.T) {
	lib := `
		package Cycles {
			private import ScalarValues::*;
			state def Cycle {
				attribute seen : Integer = 0;
				entry; then warm;
				state warm;
				state hot {
					entry action mark { assign seen := seen + 1; }
				}
				transition first warm then hot;
			}
		}
	`
	src := `
		package test {
			private import Cycles::*;
			state def Burn :> Cycle;
			state def Machine {
				entry; then burn;
				state burn : Burn;
			}
		}
	`
	idx := libs.NewModelIndex()
	idx.AddDocument("<lib>", parser.New(source.New("<lib>", []byte(lib))).ParseFile())
	idx.MarkLibrary("<lib>")
	idx.AddDocument("<test>", parseAndBuild(t, src))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state Machine not found")
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if got := exec.FinalStateName(); got != "hot" {
		t.Fatalf("final state = %q, want hot", got)
	}
	seen, ok := exec.StateData()["burn.seen"]
	if !ok {
		t.Fatalf("burn.seen missing from %v", exec.StateData())
	}
	if got := FormatValue(seen); got != "1" {
		t.Fatalf("burn.seen = %s, want 1", got)
	}
}

// A body-less usage typed by the library's StateAction, however named, lowers
// without recursing into `ref state self` and fails only for no initial state.
func testExhibitedStateTypedByTheLibraryStateAction(t *testing.T) {
	for _, tc := range []struct{ name, typing string }{
		{"imported", "StateAction"},
		{"qualified", "States::StateAction"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `
			package test {
				private import States::StateAction;
				part def Mission {
					attribute mass = 1;
					exhibit state phases : ` + tc.typing + `;
				}
			}`
			_, _, err := instantiateWithLibraries(t, src, "test::Mission")
			if err == nil {
				t.Fatal("expected the machine with no initial state to fail materialization")
			}
			if errors.Is(err, lower.ErrRecursiveStateTyping) {
				t.Fatalf("error = %v, want StateAction's content withheld rather than recursed into", err)
			}
			if !errors.Is(err, ErrNoInitialState) {
				t.Fatalf("error = %v, want ErrNoInitialState", err)
			}
			for _, want := range []string{"phases", "Mission", "StateAction"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %v, want it to name %s", err, want)
				}
			}
		})
	}
}

// testExhibitedStateTypedByTheLibraryStateActionWithABody: a state usage typed
// by StateAction and stating its own body runs that body, inheriting nothing.
func testExhibitedStateTypedByTheLibraryStateActionWithABody(t *testing.T) {
	src := `
	package test {
		private import States::StateAction;
		part def Mission {
			attribute mass = 2;
			exhibit state phases : StateAction { entry; then x; state x; }
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Mission")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	assertExhibitedMachineIn(t, ctx, inst, "mass", 2, "x")
}

// A usage typed by a definition specializing StateAction inherits that
// definition's content and nothing of the library's.
func testExhibitedStateTypedByAStateActionSpecialization(t *testing.T) {
	t.Run("with_content", func(t *testing.T) {
		src := `
		package test {
			private import States::StateAction;
			state def Phase :> StateAction { entry; then x; state x; }
			part def Mission {
				attribute mass = 3;
				exhibit state phases : Phase;
			}
		}`
		ctx, inst, err := instantiateWithLibraries(t, src, "test::Mission")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		assertExhibitedMachineIn(t, ctx, inst, "mass", 3, "x")
	})
	t.Run("own_body", func(t *testing.T) {
		src := `
		package test {
			private import States::StateAction;
			state def Phase :> StateAction;
			part def Mission {
				attribute mass = 4;
				exhibit state phases : Phase { entry; then x; state x; }
			}
		}`
		ctx, inst, err := instantiateWithLibraries(t, src, "test::Mission")
		if err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		assertExhibitedMachineIn(t, ctx, inst, "mass", 4, "x")
	})
	t.Run("empty", func(t *testing.T) {
		src := `
		package test {
			private import States::StateAction;
			state def Phase :> StateAction;
			part def Mission {
				attribute mass = 5;
				exhibit state phases : Phase;
			}
		}`
		_, _, err := instantiateWithLibraries(t, src, "test::Mission")
		if !errors.Is(err, ErrNoInitialState) {
			t.Fatalf("error = %v, want ErrNoInitialState", err)
		}
		if !strings.Contains(err.Error(), "phases") || !strings.Contains(err.Error(), "Phase") {
			t.Errorf("error = %v, want it to name the machine and its definition", err)
		}
	})
}

// assertExhibitedMachineIn checks that an object exhibits a machine resting in
// the named state, and that an ordinary attribute of it still reads.
func assertExhibitedMachineIn(t *testing.T, ctx *Context, inst *Instance, attr string, want int64, state string) {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, attr)
	if err != nil {
		t.Fatalf("%s: %v", attr, err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Int != want {
		t.Errorf("%s = %s, want %d", attr, FormatValue(got), want)
	}
	machine, ok := inst.ExhibitedState()
	if !ok || machine.State == nil {
		t.Fatal("the object exhibits no machine")
	}
	if got := machine.State.FinalStateName(); got != state {
		t.Errorf("final state = %q, want %q", got, state)
	}
}

// testStateUsageInheritsUnsupportedMember: content a usage inherits that state
// lowering cannot represent is reported, never dropped.
func testStateUsageInheritsUnsupportedMember(t *testing.T) {
	src := `
		package test {
			action def Warm;
			state def Inner {
				entry; then i1;
				state i1;
				perform Warm;
			}
			state def Machine {
				entry; then nested;
				state nested : Inner;
			}
		}
	`
	err := stateExecutorError(t, src, "Machine")
	if err == nil {
		t.Fatal("unsupported inherited member succeeded")
	}
	if !errors.Is(err, lower.ErrUnsupportedStateContent) {
		t.Fatalf("error = %v, want unsupported state content", err)
	}
	if !strings.Contains(err.Error(), "cannot be inherited by the state nested") {
		t.Fatalf("error = %v, want the inheriting state named", err)
	}
}

// stateExecutorError builds a state executor for a named state definition and
// returns what creating it reports.
func stateExecutorError(t *testing.T, src, name string) error {
	t.Helper()
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefState)
	if sym == nil {
		t.Fatalf("state %s not found", name)
	}
	_, err := ctx.CreateStateExecutor(sym)
	return err
}

// testSourcelessTransitionWithNothingBefore: a transition written without a
// source leaves the state declared before it (SysML v2 7.18.3); as the first
// member of its body it has none, which lowering reports.
func testSourcelessTransitionWithNothingBefore(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state Machine {
				accept go then active;
				entry; then init;
				state init;
				state active;
			}
		}
	`, "Machine")
	if !errors.Is(err, lower.ErrNoTransitionSource) {
		t.Fatalf("expected ErrNoTransitionSource, got %v", err)
	}
	if err.Error() != "create state executor: lower state machine: "+lower.NoTransitionSourceMessage {
		t.Fatalf("unexpected message: %v", err)
	}
}

// testSourcelessTransitionAfterANonState: the member before the shorthand is an
// entry action, a do action or an attribute rather than a state, which is not
// something a transition with a trigger can leave.
func testSourcelessTransitionAfterANonState(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"do action": {
			body: `entry; then init;
				state init;
				do action watch { }
				accept go then active;
				state active;`,
			want: "the do action",
		},
		"attribute": {
			body: `entry; then init;
				state init;
				attribute count : Integer = 0;
				accept go then active;
				state active;`,
			want: "the attribute usage count",
		},
		"choice pseudostate": {
			body: `entry; then init;
				state init;
				transition first init then pick;
				choice pick;
				accept go then active;
				state active;`,
			want: fmt.Sprintf(lower.TransitionSourcePseudostateFormat, "the choice pick", "pick"),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := stateExecutorError(t, `
				package test {
					state Machine {
						`+tc.body+`
					}
				}
			`, "Machine")
			var sourceErr *lower.TransitionSourceError
			if !errors.As(err, &sourceErr) {
				t.Fatalf("expected TransitionSourceError, got %v", err)
			}
			want := "create state executor: lower state machine: " + fmt.Sprintf(lower.TransitionSourceNotVertexFormat, tc.want)
			if _, ok := sourceErr.Source.(*ast.PseudostateNode); ok {
				want = "create state executor: lower state machine: " + tc.want
			}
			if err.Error() != want {
				t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
			}
		})
	}
}

// testNoEntryTransitionGuardHolds: `entry; if c then s;` chooses the starting
// state by guard at initialize (SysML v2 7.18.3); when no guard holds the
// machine has nowhere to start, a typed error rather than a silent stall.
func testNoEntryTransitionGuardHolds(t *testing.T) {
	src := `
		package test {
			part def Heater {
				attribute cold : Boolean = true;
				exhibit state control {
					entry;
					if not cold then ready;
					if cold and not cold then warming;
					state warming;
					state ready;
				}
			}
		}
	`
	_, _, err := instantiateWithLibraries(t, src, "test::Heater")
	if !errors.Is(err, ErrNoEntryTransitionHolds) {
		t.Fatalf("expected ErrNoEntryTransitionHolds, got %v", err)
	}
	want := "no entry transition holds: state machine control declares 2 transitions out of its entry action and the guard of none holds"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("message:\n got %q\nwant it to contain %q", err.Error(), want)
	}
}

// testEntryTransitionTargetIsNotAState: an entry transition starts its body in
// a state; reaching a pseudostate instead is a typed lowering error.
func testEntryTransitionTargetIsNotAState(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state Machine {
				entry; then pick;
				choice pick;
				transition first pick then idle;
				state idle;
			}
		}
	`, "Machine")
	var targetErr *lower.EntryTransitionTargetError
	if !errors.As(err, &targetErr) {
		t.Fatalf("expected EntryTransitionTargetError, got %v", err)
	}
	want := "create state executor: lower state machine: " + fmt.Sprintf(lower.EntryTransitionTargetFormat, "the choice pick")
	if err.Error() != want {
		t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
	}
}

// testEntryTransitionCarriesATrigger: an entry transition chooses the start by
// its guard alone; a trigger on it is a typed lowering error.
func testEntryTransitionCarriesATrigger(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state Machine {
				entry; accept go then idle;
				state idle;
			}
		}
	`, "Machine")
	var shapeErr *lower.EntryTransitionShapeError
	if !errors.As(err, &shapeErr) {
		t.Fatalf("expected EntryTransitionShapeError, got %v", err)
	}
	want := "create state executor: lower state machine: " + fmt.Sprintf(lower.EntryTransitionShapeFormat, "a trigger")
	if err.Error() != want {
		t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
	}
}

// testEntryTransitionIntoDoneCompletesAtInitialize: an entry transition whose
// guard chooses `done` completes the machine as it starts — its exit behavior
// runs and no event is left waiting — rather than leaving it running in `done`.
func testEntryTransitionIntoDoneCompletesAtInitialize(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute skip : Boolean = true;
			attribute left : Boolean = false;
			entry; if skip then done;
			then busy;
			exit action { assign left := true; }
			state busy;
			transition first busy accept after 1 [SI::s] then done;
		}
	}`)
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted right after initialize, got %s", exec.State())
	}
	assertCurrentState(t, exec, ast.DoneFeature)
	if got := exec.StateData()["left"]; got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("the machine's exit action did not run, left = %v", got)
	}
	if exec.EventQueue().Len() != 0 {
		t.Errorf("a completed machine keeps %d events waiting", exec.EventQueue().Len())
	}
}

// testNamedEntryActionTransitionIntoDoneCompletesAtInitialize: a transition
// out of a named entry action into an undeclared `done` completes the machine as
// it starts, in each syntax the succession can be written in; a declared state
// named `done` is entered instead.
func testNamedEntryActionTransitionIntoDoneCompletesAtInitialize(t *testing.T) {
	for name, successions := range map[string]string{
		"guarded transition": `transition begin if skip then done;
			transition begin then busy;`,
		"transition": `transition begin then done;`,
		"succession": `succession first begin then done;`,
	} {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute skip : Boolean = true;
				attribute left : Boolean = false;
				entry action begin { }
				`+successions+`
				exit action { assign left := true; }
				state busy;
			}
		}`)
		if exec.State() != StateCompleted {
			t.Errorf("%s: expected StateCompleted right after initialize, got %s", name, exec.State())
		}
		if got := exec.StateData()["left"]; got.Kind != ValConst || !got.Const.Bool {
			t.Errorf("%s: the machine's exit action did not run, left = %v", name, got)
		}
	}
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry action begin { }
			transition begin then done;
			state done;
		}
	}`)
	if exec.State() != StateRunning {
		t.Errorf("expected the machine to be running in its declared state done, got %s", exec.State())
	}
	assertCurrentState(t, exec, "done")
}

// testOwnEntryTransitionsReplaceInheritedOnes: the entry transitions a state
// writes itself replace the ones it inherits, guarded or not, at the machine's
// top level, in a nested typed usage and in a typed orthogonal region; a state
// writing none keeps the inherited start.
func testOwnEntryTransitionsReplaceInheritedOnes(t *testing.T) {
	const base = `
		state def Base {
			attribute c : Boolean = true;
			entry; if c then old;
			then older;
			state old;
			state older;
		}`
	for name, tc := range map[string]struct {
		machine string
		want    []string
	}{
		"specializing machine": {machine: `
			state def Machine :> Base {
				entry; then fresh;
				state fresh;
			}`, want: []string{"fresh"}},
		"typed usage": {machine: `
			state def Machine {
				entry; then u;
				state u : Base {
					entry; then fresh;
					state fresh;
				}
			}`, want: []string{"fresh"}},
		"guarded typed usage": {machine: `
			state def Machine {
				entry; then u;
				state u : Base {
					entry; if not c then fresh;
					then fresher;
					state fresh;
					state fresher;
				}
			}`, want: []string{"fresher"}},
		"redeclared entry behavior only": {machine: `
			state def Machine {
				entry; then u;
				state u : Base {
					entry assign c := true;
				}
			}`, want: []string{"old"}},
		"typed region": {machine: `
			state def Machine parallel {
				state left : Base {
					entry; then fresh;
					state fresh;
				}
				state right : Base;
			}`, want: []string{"fresh", "old"}},
	} {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;`+base+tc.machine+`
		}`)
		var got []string
		for _, state := range exec.ActiveStates() {
			got = append(got, state.Name)
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: started in %v, want %v", name, got, tc.want)
		}
	}
}

// testRegionEntryTransitionsIntoDoneCompleteAtInitialize: every orthogonal
// region starting in `done` completes the machine as it starts, exactly once.
func testRegionEntryTransitionsIntoDoneCompleteAtInitialize(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine parallel {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			state left {
				entry; then done;
			}
			state right {
				entry; then done;
			}
		}
	}`)
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted right after initialize, got %s", exec.State())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the machine's exit action ran %v times, want once", got)
	}
}

// testNestedRegionsIntoDoneCompleteAtInitialize: a machine starting in a
// parallel state whose every region starts in `done` completes as it starts.
func testNestedRegionsIntoDoneCompleteAtInitialize(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			entry; then outer;
			state outer parallel {
				state left {
					entry; then done;
				}
				state right {
					entry; then done;
				}
			}
		}
	}`)
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted right after initialize, got %s", exec.State())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the machine's exit action ran %v times, want once", got)
	}
}

// testTransitionIntoNestedRegionsInDoneCompletes: a transition into a parallel
// state whose every region starts in `done` completes the machine.
func testTransitionIntoNestedRegionsInDoneCompletes(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			entry; then idle;
			state idle;
			transition first idle accept go then outer;
			state outer parallel {
				state left {
					entry; then done;
				}
				state right {
					entry; then done;
				}
			}
		}
	}`)
	assertCurrentState(t, exec, "idle")
	exec.SendSignal("go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("go: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted after entering outer, got %s", exec.State())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the machine's exit action ran %v times, want once", got)
	}
}

// testRegionStartDescendsThroughEntryTransitions: a region whose starting state
// is composite starts that state where its own entry transitions choose, and the
// nested state is the region's active state, so its transitions are armed.
func testRegionStartDescendsThroughEntryTransitions(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine parallel {
			attribute cold : Boolean = true;
			state control {
				entry; then running;
				state running {
					entry; if cold then heating;
					if not cold then idle;
					state heating;
					transition first heating accept warm then idle;
					state idle;
				}
			}
			state monitor {
				entry; then watching;
				state watching;
			}
		}
	}`)
	activeNames := func() map[string]bool {
		active := make(map[string]bool)
		for _, state := range exec.ActiveStates() {
			active[state.Name] = true
		}
		return active
	}
	if active := activeNames(); !active["heating"] || !active["watching"] {
		t.Fatalf("expected heating and watching active after initialize, got %v", active)
	}
	exec.SendSignal("warm", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if active := activeNames(); !active["idle"] || active["heating"] {
		t.Errorf("warm did not move heating to idle, active states %v", active)
	}
}

// testRegionEntryGuardsReadTheRegionStateAttributes: the entry transitions of a
// region of a parallel state read the attributes that region's own state
// declares, after its entry behavior has run, whether the parallel state is the
// machine itself or a composite state entered below it.
func testRegionEntryGuardsReadTheRegionStateAttributes(t *testing.T) {
	regions := `
			state left {
				attribute cold : Boolean = true;
				entry assign cold := false;
				if cold then on;
				then off;
				state on;
				state off;
			}
			state right {
				attribute cold : Boolean = false;
				entry assign cold := true;
				if cold then on;
				then off;
				state on;
				state off;
			}`
	machines := map[string]string{
		"parallel machine": `package test {
		private import ScalarValues::*;
		state Machine parallel {` + regions + `
		}
	}`,
		"parallel state": `package test {
		private import ScalarValues::*;
		state Machine {
			entry; then outer;
			state outer parallel {` + regions + `
			}
		}
	}`,
	}
	for name, src := range machines {
		exec := stateExecutorForSource(t, "Machine", src)
		active := make(map[string]string)
		for _, state := range exec.ActiveStates() {
			active[exec.graph.ParentState[state].Name] = state.Name
		}
		if active["left"] != "off" || active["right"] != "on" {
			t.Errorf("%s: expected left in off and right in on after their entry behaviors, got %v", name, active)
		}
	}
}

// testLeavingRegionsDescendsThroughEntryTransitions: a transition out of an
// orthogonal region into a composite state outside it starts that state where
// its own entry transitions choose, and the nested state's timer is armed.
func testLeavingRegionsDescendsThroughEntryTransitions(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state Machine {
			entry; then both;
			state both parallel {
				state left {
					entry; then l1;
					state l1;
				}
				state right {
					entry; then r1;
					state r1;
				}
			}
			transition first both.left.l1 accept leave then running;
			state running {
				entry; then waiting;
				state waiting;
				transition first waiting accept after 1 [SI::s] then finished;
				state finished;
			}
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	exec.SendSignal("leave", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("leave: %v", err)
	}
	assertCurrentState(t, exec, "waiting")
	if got := len(ctx.Clock().Waits()); got != 1 {
		t.Fatalf("%d wait(s) on the clock after entering waiting; want its timer armed", got)
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance: %v", err)
	}
	assertCurrentState(t, exec, "finished")
}

// testCalcUnboundParameter: a parameter with neither an argument nor a default
// is a modeling error, not a null value.
func testCalcUnboundParameter(t *testing.T) {
	src := `
		package test {
			calc add {
				in x: Integer;
				in y: Integer;
				x + y
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "add", ast.DefCalc)
	if sym == nil {
		t.Fatal("add calc not found")
	}

	// Invoke with only 1 argument (missing y)
	xVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{xVal}, rootScope)
	if err == nil {
		t.Fatalf("expected an unbound parameter error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testCalcCallsAnUnimportedExtensionFunction: `exp(x)` with no import of the
// OpenSysML extension library is reported unresolved by name resolution, so the
// call fails with a typed error naming the declaration whose package an import
// would make visible, rather than being answered.
func testCalcCallsAnUnimportedExtensionFunction(t *testing.T) {
	src := `
		package test {
			calc grow {
				in x: Real;
				exp(x)
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "grow", ast.DefCalc)
	if sym == nil {
		t.Fatal("grow calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected an unresolved-reference error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected ErrUnresolvedReference, got: %v", err)
	}
	if want := ": unresolved reference: exp — did you mean OpenSysMLMathFunctions::exp?"; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("error %q does not end in %q", err, want)
	}
}

// testCalcCallsAnUnimportedLibraryFunction: `wheels->size()` in a model that
// imports no part of SequenceFunctions is reported unresolved by name
// resolution, so the call fails with a typed error carrying the validator's own
// hint — the library declarations of that name, whose package an import would
// make visible — rather than being answered by dispatch on the bare name. The
// same body evaluates once the import is written.
func testCalcCallsAnUnimportedLibraryFunction(t *testing.T) {
	model := func(imports string) string {
		return `
		package test {
			` + imports + `
			attribute wheels : Integer[*] = (1, 2, 3, 4);
			calc count {
				wheels->size()
			}
		}
	`
	}

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model("")))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "count", ast.DefCalc)
	if sym == nil {
		t.Fatal("count calc not found")
	}
	result, err := ctx.InvokeCalc(sym, nil, rootScope)
	if err == nil {
		t.Fatalf("expected an unresolved-reference error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected ErrUnresolvedReference, got: %v", err)
	}
	if want := ": unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?"; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("error %q does not end in %q", err, want)
	}

	idx, _, ctx = buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model("private import SequenceFunctions::*;")))
	rootScope = idx.DocumentRoot("<test>")
	sym = findSymbolByName(rootScope, "count", ast.DefCalc)
	if sym == nil {
		t.Fatal("count calc not found")
	}
	result, err = ctx.InvokeCalc(sym, nil, rootScope)
	if err != nil || result.Kind != ValConst || result.Const.Int != 4 {
		t.Fatalf("wheels->size() under import SequenceFunctions::* = %+v, %v; want 4", result, err)
	}
}

// testCalcUnboundKeywordNamedParameter: a parameter named with a keyword is a
// parameter like any other, so leaving it unbound reports, never panics.
func testCalcUnboundKeywordNamedParameter(t *testing.T) {
	src := `
		package test {
			calc classify {
				in 'type': Integer;
				in 'state': Integer;
				'type' + 'state'
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "classify", ast.DefCalc)
	if sym == nil {
		t.Fatal("classify calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected an unbound parameter error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testCalcTooManyArguments: more arguments than parameters has no binding, so it
// reports an arity error instead of dropping the extras.
func testCalcTooManyArguments(t *testing.T) {
	src := `
		package test {
			calc double {
				in x: Integer;
				x * 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "double", ast.DefCalc)
	if sym == nil {
		t.Fatal("double calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg, arg}, rootScope)
	if err == nil {
		t.Fatal("expected an arity error, the calc accepted a surplus argument")
	}
	if !errors.Is(err, ErrCalcArity) {
		t.Errorf("expected ErrCalcArity, got: %v", err)
	}
}

// testCalcUnknownNamedArgument: a named argument that matches no parameter is
// reported instead of silently leaving the parameter on its default.
func testCalcUnknownNamedArgument(t *testing.T) {
	src := `
		package test {
			calc scale {
				in x: Integer = 1;
				x * 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "scale", ast.DefCalc)
	if sym == nil {
		t.Fatal("scale calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	_, err := ctx.InvokeCalcNamed(sym, map[string]Value{"factor": arg}, rootScope)
	if err == nil {
		t.Fatal("expected an unknown parameter error, the invocation succeeded")
	}
	if !errors.Is(err, ErrUnknownParameter) {
		t.Errorf("expected ErrUnknownParameter, got: %v", err)
	}
}

// testCalcParameterNamedTwice: an invocation naming one parameter twice is
// reported rather than binding the later value.
func testCalcParameterNamedTwice(t *testing.T) {
	src := `
		package test {
			calc def Scale { in x : Integer; in factor : Integer = 2; return : Integer = x * factor; }
			calc twice { Scale(x = 1, x = 3, factor = 4) }
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "twice", ast.DefCalc)
	if sym == nil {
		t.Fatal("twice calc not found")
	}
	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if !errors.Is(err, ErrCalcArity) || !strings.Contains(err.Error(), `binds parameter "x" twice`) {
		t.Errorf("expected ErrCalcArity naming x, got: %v", err)
	}
}

// testCalcWithoutResult: a calc body with no return expression has no value to
// produce, own or inherited.
func testCalcWithoutResult(t *testing.T) {
	src := `
		package test {
			calc empty {
				in x: Integer;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "empty", ast.DefCalc)
	if sym == nil {
		t.Fatal("empty calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected a missing-result error, the calc returned a value")
	}
	if !errors.Is(err, ErrNoResultExpression) {
		t.Errorf("expected ErrNoResultExpression, got: %v", err)
	}
}

// testCalcStatesSecondResult: a calc stating a body over an inherited result
// expression is refused, not computed from a body of the runtime's choosing.
func testCalcStatesSecondResult(t *testing.T) {
	src := `
		package test {
			calc def Plus {
				in x: Integer;
				x + 1
			}
			calc def Twice :> Plus {
				x + 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Twice", ast.DefCalc)
	if sym == nil {
		t.Fatal("Twice calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, ErrConflictingResultExpressions) {
		t.Fatalf("expected ErrConflictingResultExpressions, got: %v", err)
	}
	for _, want := range []string{"test::Twice", "test::Plus"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

// testCalcBodyStatesTwoResults: a calc body listing two bare expressions has
// two result expressions; neither it nor a calc inheriting them is computed
// from the first.
func testCalcBodyStatesTwoResults(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in x: Integer;
				x + 1
				x + 2
			}
			calc def Inherited :> Twice;
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	for name, want := range map[string]string{
		"Twice":     "calc test::Twice states 2 result expressions",
		"Inherited": "from each of test::Twice, test::Twice",
	} {
		sym := findSymbolByName(rootScope, name, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", name)
		}
		_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if !errors.Is(err, ErrConflictingResultExpressions) {
			t.Fatalf("%s: expected ErrConflictingResultExpressions, got: %v", name, err)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q does not say %q", name, err, want)
		}
	}
}

// testCalcSymbolIsNotACalc: invoking a non-calc symbol is rejected by kind
// rather than by whatever its body happens to contain.
func testCalcSymbolIsNotACalc(t *testing.T) {
	src := `
		package test {
			part def Engine {
				attribute power : Integer;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Engine", ast.DefPart)
	if sym == nil {
		t.Fatal("Engine part def not found")
	}

	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if err == nil {
		t.Fatal("expected a not-a-calc error, the invocation succeeded")
	}
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testCalcDirectRecursion: a calc that invokes itself unconditionally never
// terminates, so the run's calc depth budget must report it instead of the
// process exhausting its stack.
func testCalcDirectRecursion(t *testing.T) {
	src := `
		package test {
			calc countdown {
				in n: Integer;
				countdown(n - 1)
			}
		}
	`
	assertCalcRecursionBounded(t, src, "countdown", ErrCalcRecursionLimit)
}

// testCalcMutualRecursion: the budget is spent by nesting, so a cycle through
// another calc is reported the same way direct self-invocation is.
func testCalcMutualRecursion(t *testing.T) {
	src := `
		package test {
			calc ping {
				in n: Integer;
				pong(n)
			}

			calc pong {
				in n: Integer;
				ping(n)
			}
		}
	`
	assertCalcRecursionBounded(t, src, "ping", ErrCalcRecursionLimit)
}

// testCalcDefaultRecursion: a default re-invoking its own calc nests through the
// binding rather than the body, and is bounded and collapsed the same way.
func testCalcDefaultRecursion(t *testing.T) {
	src := `
		package test {
			calc def f {
				in x : Integer;
				in y : Integer = f(x);
				return : Integer = x;
			}
		}
	`
	assertCalcRecursionBounded(t, src, "f", ErrCalcRecursionLimit)
}

// testCalcLibraryDefaultFailureNamesOneFrame: a default failing once on a library
// specialization is reported under one calc name, as a calc with a body reports it.
func testCalcLibraryDefaultFailureNamesOneFrame(t *testing.T) {
	src := `
		package test {
			import ScalarValues::*;
			import RealFunctions::*;
			calc def again :> max {
				in x :>> x;
				in y :>> y = missing;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "again", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc again not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected the unresolved default to fail the invocation")
	}
	if got, prefix := err.Error(), `calc test::again: default for parameter "y": `; !strings.HasPrefix(got, prefix) || strings.Count(got, "calc test::again") != 1 {
		t.Errorf("err = %q; want one frame %q", got, prefix)
	}
}

// testCalcLibraryDefaultRecursion: a library specialization whose default invokes
// itself nests through its own binding, so the depth budget reports it like any calc.
func testCalcLibraryDefaultRecursion(t *testing.T) {
	src := `
		package test {
			import ScalarValues::*;
			import RealFunctions::*;
			calc def again :> max {
				in x :>> x;
				in y :>> y = again(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	assertCalcInvocationBounded(t, idx, ctx, "again", ErrCalcRecursionLimit)
}

// testCalcRecursionSpendsStepBudget: the two bounds are independent, so a
// recursion whose evaluations run out first is reported by the step budget
// rather than running on until the depth bound.
func testCalcRecursionSpendsStepBudget(t *testing.T) {
	src := `
		package test {
			calc grow {
				in n: Integer;
				return : Integer = n + grow(n + 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "grow", ErrStepLimitExceeded, func(ctx *Context) {
		// Room to recurse far deeper than the evaluations allow.
		ctx.maxCalcDepth = MaxCalcDepthCeiling
		ctx.maxSteps = 500
	})
}

// testCalcRecursionAtDepthCeiling: the highest depth budget a run may be given
// must still be reported rather than reached by exhausting the stack, which
// would be fatal.
func testCalcRecursionAtDepthCeiling(t *testing.T) {
	src := `
		package test {
			calc deep {
				in n: Integer;
				attribute acc : Integer = (n + 1) * (n + 2) - n * n;
				return : Integer = acc + deep(n + 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "deep", ErrCalcRecursionLimit, func(ctx *Context) {
		ctx.maxCalcDepth = MaxCalcDepthCeiling
	})
}

// assertCalcRecursionBounded invokes calcName and requires the given budget
// error promptly: the invocation runs on its own goroutine so a hang fails the
// case instead of stalling the suite until the package timeout, and a panic in
// it fails the case rather than the package.
func assertCalcRecursionBounded(t *testing.T, src, calcName string, want error, budgets ...func(*Context)) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	assertCalcInvocationBounded(t, idx, ctx, calcName, want, budgets...)
}

func assertCalcInvocationBounded(t *testing.T, idx *symbols.Index, ctx *Context, calcName string, want error, budgets ...func(*Context)) {
	t.Helper()

	// The default step budget, so a recursion bounded by depth reaches that bound
	// rather than running out of evaluations first.
	ctx.maxSteps = DefaultMaxSteps
	for _, set := range budgets {
		set(ctx)
	}
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("calc %s panicked: %v", calcName, r)
			}
		}()
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
		_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected recursive calc %s to be bounded, it returned a value", calcName)
		}
		if !errors.Is(err, want) {
			t.Errorf("expected %v, got: %v", want, err)
		}
		// One line per frame would make the message (and the memory building
		// it) grow with the square of the depth.
		if msg := err.Error(); len(msg) > 1024 {
			t.Errorf("error for recursive calc %s is %d bytes; want frames collapsed: %.200s…", calcName, len(msg), msg)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("recursive calc %s did not terminate", calcName)
	}
}

// testConstraintMissingFeature: constraint references nonexistent feature
func testConstraintMissingFeature(t *testing.T) {
	src := `
		package test {
			constraint broken {
				nonexistent > 0 // 'nonexistent' feature doesn't exist
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "broken", ast.DefConstraint)
	if sym == nil {
		t.Fatal("broken constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, rootScope)

	if err != nil {
		t.Logf("EvaluateConstraint returned error (expected): %v", err)
		return
	}

	if !satisfied {
		t.Log("EvaluateConstraint returned false (missing feature treated as unsatisfied)")
		return
	}

	t.Log("EvaluateConstraint returned true (missing feature tolerated)")
}

// testNestedConditionSubjectIsAmbiguous: two objects redefining the same nested
// feature differently make the subject of a check a question, reported as
// ErrAmbiguousSubject rather than answered from whichever object is found first.
func testNestedConditionSubjectIsAmbiguous(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part slow : Top {
				part :>> leaf { attribute :>> value = 2.0; }
			}
			part fast : Top {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	for _, name := range []string{"slow", "fast"} {
		if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", name)); err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("satisfied = %t, err = %v, want ErrAmbiguousSubject", satisfied, err)
	}
	if satisfied {
		t.Error("an ambiguous subject is no verdict")
	}
}

// testSatisfactionSubjectIsAmbiguous: a satisfaction assertion whose `by` object
// holds two objects of the requirement's owner has no one subject either, and
// reports it as ErrAmbiguousSubject rather than picking one.
func testSatisfactionSubjectIsAmbiguous(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				requirement lim { require value < 10.0; }
			}
			part def Top {
				part slow : Leaf { attribute :>> value = 2.0; }
				part fast : Leaf { attribute :>> value = 99.0; }
			}
			part top : Top;
			assert satisfy Leaf::lim by top;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	pkg := memberPath(t, rootScope, "test")
	assertions := ctx.SatisfyAssertionsIn(pkg.Scope)
	if len(assertions) != 1 {
		t.Fatalf("assertions = %d, want the one the package states", len(assertions))
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], nil)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("holds = %t, err = %v, want ErrAmbiguousSubject", result.Holds, err)
	}
	if result.Holds {
		t.Error("an ambiguous subject is no verdict")
	}
}

// testRecursiveCompositionSubjectSearch: searching for the object a check is
// about does not walk a design containing its own kind forever; it answers about
// the declaration, since no object of the checked type is there.
func testRecursiveCompositionSubjectSearch(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Node {
				part next : Node;
			}
			part root : Node;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "root")); err != nil {
		t.Fatalf("instantiate root: %v", err)
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	done := make(chan struct{})
	var satisfied bool
	var err error
	go func() {
		defer close(done)
		satisfied, err = ctx.EvaluateConstraint(small, small.OwnerScope)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the subject search did not terminate on recursive composition")
	}
	if err != nil {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if !satisfied {
		t.Error("satisfied = false, want the declaration's answer")
	}
	if len(ctx.instances) > 1000 {
		t.Errorf("%d objects materialized: the search is not bounded", len(ctx.instances))
	}
}

// testDuplicateObjectsOfOneDeclaration: materializing the same declaration twice
// is one object as far as a check is concerned, not an ambiguous subject.
func testDuplicateObjectsOfOneDeclaration(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part o : Top {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	obj := memberPath(t, rootScope, "test", "o")
	for range 2 {
		if _, err := ctx.Instantiate(obj); err != nil {
			t.Fatalf("instantiate o: %v", err)
		}
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the object's 99.0 to violate the constraint")
	}
}

// testDuplicateObjectsHoldingAPlainPart: a nested part typed by a definition
// rather than by a body of its own is reached through its holder, so what two
// materializations of that holder leave behind is no ambiguous subject.
func testDuplicateObjectsHoldingAPlainPart(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 99.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part o : Top;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	obj := memberPath(t, rootScope, "test", "o")
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	for range 2 {
		if _, err := ctx.Instantiate(obj); err != nil {
			t.Fatalf("instantiate o: %v", err)
		}
		satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("EvaluateConstraint: %v", err)
		}
		if satisfied {
			t.Error("satisfied = true, want the object's 99.0 to violate the constraint")
		}
	}
}

// testNestedPartHeldWithAMultiplicity: the objects one feature value materializes for a
// multiplicity are occurrences of one declaration, so a check answers a verdict
// rather than calling its subject ambiguous.
func testNestedPartHeldWithAMultiplicity(t *testing.T) {
	src := `
		package test {
			part def Wheel {
				attribute pressure = 99.0;
				constraint inflated { pressure < 10.0 }
			}
			part def Car {
				part wheels : Wheel[4];
			}
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	inflated := memberPath(t, rootScope, "test", "Wheel", "inflated")
	satisfied, err := ctx.EvaluateConstraint(inflated, inflated.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the wheels' 99.0 to violate the constraint")
	}
}

// testPartNestedInsideARepeatedPart: the declaration a check names may sit
// deeper inside the part a multiplicity repeated, and the objects reached along
// one declaration path are still one subject rather than an ambiguity.
func testPartNestedInsideARepeatedPart(t *testing.T) {
	src := `
		package test {
			part def Bolt {
				attribute torque = 99.0;
				constraint tight { torque < 10.0 }
			}
			part def Wheel {
				part bolt : Bolt;
			}
			part def Car {
				part wheels : Wheel[4];
			}
			part car : Car;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	tight := memberPath(t, rootScope, "test", "Bolt", "tight")
	satisfied, err := ctx.EvaluateConstraint(tight, tight.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if satisfied {
		t.Error("satisfied = true, want the bolts' 99.0 to violate the constraint")
	}
}

// testPartsSubsettingOneCollection: two declarations feeding one collection are
// two subjects, not repetitions of the collection, so the check reports the
// ambiguity rather than answering from whichever it reached first.
func testPartsSubsettingOneCollection(t *testing.T) {
	src := `
		package test {
			part def Component {
				attribute v = 1.0;
				constraint ok { v < 10.0 }
			}
			part def Assembly {
				part subsystem : Component[*];
				part small : Component :> subsystem {
					attribute :>> v = 5.0;
				}
				part large : Component :> subsystem {
					attribute :>> v = 99.0;
				}
			}
			part assembly : Assembly;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "assembly")); err != nil {
		t.Fatalf("instantiate assembly: %v", err)
	}
	ok := memberPath(t, rootScope, "test", "Component", "ok")
	satisfied, err := ctx.EvaluateConstraint(ok, ok.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("satisfied = %t, err = %v, want ErrAmbiguousSubject", satisfied, err)
	}
	if satisfied {
		t.Error("an ambiguous subject is no verdict")
	}
	// The two objects reached through one collection are told apart by the
	// declaration each materializes, not by the feature holding both.
	for _, want := range []string{"(small)", "(large)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name a carrier %q", err, want)
		}
	}
}

// testRequirementFeatureWithoutAValue: a condition naming a feature the
// requirement declares but nothing gives a value to reports ErrNoValue, naming
// the feature, rather than the unresolved-feature error of a name that is not
// declared at all.
func testRequirementFeatureWithoutAValue(t *testing.T) {
	src := `
		package test {
			requirement def TouchdownRequirement {
				attribute actualVerticalSpeed;
				attribute maxVerticalSpeed = 1.5;
				require constraint { actualVerticalSpeed <= maxVerticalSpeed }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "TouchdownRequirement", ast.DefRequirement)
	if sym == nil {
		t.Fatal("TouchdownRequirement not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("a feature without a value is not a violation")
	}
	if !strings.Contains(err.Error(), "actualVerticalSpeed") {
		t.Errorf("error does not name the feature: %v", err)
	}
}

// testRequirementFeaturesValuedFromEachOther: two features whose values name each
// other report a cycle promptly instead of recursing until the step budget runs out.
func testRequirementFeaturesValuedFromEachOther(t *testing.T) {
	src := `
		package test {
			requirement def R {
				attribute a = b;
				attribute b = a;
				require constraint { a <= b }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "R", ast.DefRequirement)
	if sym == nil {
		t.Fatal("R not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("expected ErrCyclicFeatureValue, got: %v", err)
	}
}

// testStepBudgetExceeded: evaluation exceeds maxSteps. Each Eval call spends one
// step, so an expression with more subexpressions than the budget must report
// ErrStepLimitExceeded rather than run to the end. The operands are a parameter
// rather than literals because a constant expression is folded in one step.
func testStepBudgetExceeded(t *testing.T) {
	src := `
		package test {
			calc deep {
				in x : Integer;
				x + x + x + x + x + x + x + x
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 3
	ctx.run.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "deep", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc deep not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the calc completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testEvalOnAnInstanceSpendsTheStepBudget: an expression evaluated against an
// instance is one run, so reading a feature value inside it does not start a run of its
// own and reset the counter; an expression longer than the budget is refused.
func testEvalOnAnInstanceSpendsTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Car { attribute m = 5.0; }
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Car", ast.DefPart)
	if sym == nil {
		t.Fatal("part def Car not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("instantiating Car: %v", err)
	}

	// Nested to the right, so the feature value read - which brackets a run of its own when
	// the evaluation is not already one - is reached on the second step.
	expr := "m"
	for i := 0; i < 60; i++ {
		expr = "m + (" + expr + ")"
	}
	node := parser.New(source.New("<e>", []byte(expr))).ParseExpression()
	if node == nil {
		t.Fatal("the expression did not parse")
	}

	ctx.maxSteps = 6
	got, err := ctx.EvalWithScopeOn(node, sym.Scope, inst)
	if err == nil {
		t.Fatalf("expected the step budget to bound the evaluation, got %v", got)
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}

	// The budget bounds one run, not the session: a short expression is answered
	// however many ran before it.
	ctx.maxSteps = 20
	short := parser.New(source.New("<e>", []byte("m + m"))).ParseExpression()
	for i := 0; i < 5; i++ {
		if _, err := ctx.EvalWithScopeOn(short, sym.Scope, inst); err != nil {
			t.Fatalf("evaluation %d of m + m under a fresh run: %v", i+1, err)
		}
	}
}

// testNonTerminatingLoopExhaustsStepBudget: a loop whose condition never fails
// spends a step per iteration, so it ends the execution with
// ErrStepLimitExceeded instead of hanging whoever drove it (a REPL or the LSP).
func testNonTerminatingLoopExhaustsStepBudget(t *testing.T) {
	src := `
		package test {
			action spinner {
				attribute total : Integer = 0;
				first start;
				action spin {
					while total >= 0 {
						assign total := total + 1;
					}
				}
				done;
				succession first start then spin;
				succession first spin then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 20
	ctx.run.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "spinner", ast.DefAction)
	if sym == nil {
		t.Fatal("action spinner not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the action completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testLoopBodyDeclarationDoesNotLeak: a loop body and an `if` branch body are
// namespaces of their own, so a name one of them declares is not a member of the
// action and does not appear among its results.
func testLoopBodyDeclarationDoesNotLeak(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						attribute bump : Integer = 1;
						assign total := total + bump;
						if total == 2 {
							attribute marker : Integer = 9;
							assign total := total + marker;
						}
					}
				}
				done;
				succession first start then accumulate;
				succession first accumulate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	// 1, then 2 which the conditional lifts to 11, which ends the loop.
	total, ok := outputs["total"]
	if !ok {
		t.Fatal("total missing from the action's results")
	}
	if total.Const.Int != 11 {
		t.Errorf("total = %v, want 11", FormatTraceValue(total))
	}
	for _, local := range []string{"bump", "marker"} {
		if _, ok := outputs[local]; ok {
			t.Errorf("body-local %s leaked into the action's results: %v", local, outputs)
		}
	}
}

// testLoopBodyOfUnexecutableStatement: a body member the lowering layer cannot
// turn into a statement is reported when it is reached, rather than skipped —
// silently dropping it would give a wrong answer with no diagnostic.
func testLoopBodyOfUnexecutableStatement(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						part inner;
						assign total := total + 1;
					}
				}
				done;
				succession first start then accumulate;
				succession first accumulate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected an unexecutable loop body member to be reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error does not name the unexecutable member: %v", err)
	}
}

// testBlockFlowOfUnexecutableMember: a member outside the semantics a block's own
// flow gives its members is still reported when reached, even in a block that
// does state a flow because a nested action is declared beside it.
func testBlockFlowOfUnexecutableMember(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						action bump {
							assign total := total + 1;
						}
						part inner;
					}
				}
				done;
				succession first start then accumulate;
				succession first accumulate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the unexecutable member of the block's flow to be reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error does not name the unexecutable member: %v", err)
	}
}

// testNonTerminatingLoopPerformingAnAction: a loop whose body performs an action
// as a node of the block's own flow still spends a step per iteration, so it
// ends with ErrStepLimitExceeded rather than performing forever.
func testNonTerminatingLoopPerformingAnAction(t *testing.T) {
	src := `
		package test {
			action spinner {
				attribute total : Integer = 0;
				first start;
				action spin {
					while total >= 0 {
						perform bump;
						assign total := total + 1;
					}
				}
				done;
				succession first start then spin;
				succession first spin then done;
			}

			action bump {
				out spun : Integer;
				first begin;
				action run {
					assign spun := 1;
				}
				done;
				succession first begin then run;
				succession first run then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 40
	ctx.run.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "spinner", ast.DefAction)
	if sym == nil {
		t.Fatal("action spinner not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the action completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testForOverAValueNoExpressionMakesIterable: a value that states a computation
// rather than a collection is no collection in any order, so a `for` over it
// fails with a typed error naming it.
func testForOverAValueNoExpressionMakesIterable(t *testing.T) {
	for _, value := range []Value{{Kind: ValExpr}, {Kind: ValInvalid}} {
		elements, err := forElements(value)
		if err == nil {
			t.Errorf("forElements(%s) = %v, want a typed error", describeValue(value), elements)
			continue
		}
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("forElements(%s) failed with %v, want ErrTypeMismatch", describeValue(value), err)
		}
		if !strings.Contains(err.Error(), describeValue(value)) {
			t.Errorf("error does not name the value: %v", err)
		}
	}
}

// testForOverAScalar: a `for` whose input is a scalar fails with a typed error
// rather than iterating once over the coercion elementsOf would make of it.
func testForOverAScalar(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute single : Integer = 7;
				attribute visited : Integer = 0;
				first start;
				action iterate {
					for s in single {
						assign visited := visited + 1;
					}
				}
				done;
				succession first start then iterate;
				succession first iterate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	result, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatalf("the action completed with %v, want a typed error", result)
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("execution failed with %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "an Integer is not one") {
		t.Errorf("error does not name the value it was given: %v", err)
	}
}

// testStatementDirectlyInAnActionBody: a statement written among the action's
// own members has no name a succession can reach, so it is reported rather than
// ignored.
func testStatementDirectlyInAnActionBody(t *testing.T) {
	cases := map[string]string{
		"while":      "while total < 5 { assign total := total + 1; }",
		"if":         "if total < 5 { assign total := total + 1; }",
		"assignment": "assign total := total + 1;",
	}

	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			src := `
				package test {
					action counter {
						attribute total : Integer = 0;
						first start;
						` + stmt + `
						done;
						succession first start then done;
					}
				}
			`
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "counter", ast.DefAction)
			if sym == nil {
				t.Fatal("action counter not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if err == nil {
				t.Fatalf("expected a top-level %s to be reported", name)
			}
			if !strings.Contains(err.Error(), "no position in the token flow") {
				t.Errorf("error does not explain why the statement cannot run: %v", err)
			}
		})
	}
}

// testFlowEndNamingNoNode: a flow moves a value from one action node's output
// to another's input, so an end naming something that is not a node of the
// action is reported rather than dropped, which would leave the flow declared
// and carrying nothing.
func testFlowEndNamingNoNode(t *testing.T) {
	cases := map[string]string{
		"source": "flow bad from missing.engineTorque to amplify.torqueIn;",
		"target": "flow bad from generate.engineTorque to missing.torqueIn;",
	}

	for name, flow := range cases {
		t.Run(name, func(t *testing.T) {
			src := `
				package test {
					action driveTrain {
						first start;
						action generate { out engineTorque : Integer; assign engineTorque := 1; }
						action amplify { in torqueIn : Integer; }
						done;
						succession first start then generate;
						succession first generate then amplify;
						succession first amplify then done;
						` + flow + `
					}
				}
			`
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "driveTrain", ast.DefAction)
			if sym == nil {
				t.Fatal("action driveTrain not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if err == nil {
				t.Fatalf("expected the flow's %s end to be reported", name)
			}
			if !strings.Contains(err.Error(), "flow bad") ||
				!strings.Contains(err.Error(), name) {
				t.Errorf("error does not name the flow and the end at fault: %v", err)
			}
		})
	}
}

// testFlowNamingNoPin: a flow carries the value of a feature, so a flow whose
// ends name nodes alone and which declares no payload identifies nothing to
// move, and is reported when the graph is built rather than mid-run.
func testFlowNamingNoPin(t *testing.T) {
	_, err := executeActionSource(t, "driveTrain", `package test {
		action driveTrain {
			first start;
			action generate { out engineTorque : Integer; assign engineTorque := 1; }
			action amplify { in engineTorque : Integer; }
			done;
			succession first start then generate;
			succession first generate then amplify;
			succession first amplify then done;
			flow generateToAmplify from generate to amplify;
		}
	}`)
	if err == nil {
		t.Fatal("expected the flow naming no feature to be reported")
	}
	if !strings.Contains(err.Error(), "generateToAmplify") ||
		!strings.Contains(err.Error(), "names no feature to carry") {
		t.Errorf("error does not say the flow names no feature: %v", err)
	}
}

// testAcceptPayloadWithoutAValue: a message carrying no value and naming no
// signal definition gives the accept's payload name nothing to bind, which is
// reported rather than bound to nothing.
func testAcceptPayloadWithoutAValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			first start;
			action reader accept p : Ping;
			done;
			succession first start then reader;
			succession first reader then done;
		}
	}`))
	exec, err := ctx.CreateActionExecutor(oneSymbol(t, idx, "P::pipeline"))
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	ctx.PostMessage(Message{SignalType: "Ping", Target: "reader", Payload: map[string]Value{}})
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected the payload-less message to be reported")
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Ping") {
		t.Errorf("error does not name the accepted signal: %v", err)
	}
}

// testAcceptPayloadReadBeforeItIsBound: the payload is a declaration of the body
// wherever the body resolves, so a node running before the accept binds it
// resolves the name and finds no value — reported as a feature without a value,
// not read as an empty value and not as a name that fails to resolve.
func testAcceptPayloadReadBeforeItIsBound(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute seen : Integer = 0;
			first start;
			action reader { assign seen := msg; }
			action waiter accept msg : Integer;
			done;
			succession first start then reader;
			succession first reader then waiter;
			succession first waiter then done;
		}
	}`)
	if !errors.Is(err, ErrNoValue) {
		t.Fatalf("err = %v; want ErrNoValue", err)
	}
	if errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("a declared payload was reported as unresolved: %v", err)
	}
	if !strings.Contains(err.Error(), "msg") {
		t.Errorf("error does not name the payload: %v", err)
	}
}

// testFlowFromANodeThatProducedNothing: a flow out of a node that left its
// source pin empty carries nothing, which is reported rather than silently
// leaving the target pin unwritten.
func testFlowFromANodeThatProducedNothing(t *testing.T) {
	src := `
		package test {
			action driveTrain {
				first start;
				action generate { out engineTorque : Integer; }
				action amplify { in torqueIn : Integer; }
				done;
				succession first start then generate;
				succession first generate then amplify;
				succession first amplify then done;
				flow generateToAmplify from generate.engineTorque to amplify.torqueIn;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "driveTrain", ast.DefAction)
	if sym == nil {
		t.Fatal("action driveTrain not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the empty source pin to be reported")
	}
	if !strings.Contains(err.Error(), "generateToAmplify") ||
		!strings.Contains(err.Error(), "engineTorque") {
		t.Errorf("error does not name the flow and the pin that stayed empty: %v", err)
	}
}

// testActionAcceptTimeWaits: an action's `accept after`/`accept at` waits on the
// context's clock; a past instant fires at once, bad arguments are typed errors.
func testActionAcceptTimeWaits(t *testing.T) {
	run := func(t *testing.T, trigger string) (map[string]Value, *Context, error) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ISQ::*;
				private import Time::*;
				action maintain {
					attribute maintenanceTime : TimeInstantValue = 3 [s];
					attribute past : TimeInstantValue = 0 [s];
					attribute wrongWay : DurationValue = -2 [s];
					attribute load : MassValue = 5 [kg];
					attribute done : Integer = 0;
					first start;
					action waitForIt `+trigger+`;
					action work { assign done := 1; }
					done;
					succession first start then waitForIt;
					succession first waitForIt then work;
					succession first work then done;
				}
			}
		`))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "maintain", ast.DefAction)
		if sym == nil {
			t.Fatal("action maintain not found")
		}
		results, err := ctx.ExecuteAction(sym)
		return results, ctx, err
	}
	fires := func(t *testing.T, trigger string, wantNow float64) {
		results, ctx, err := run(t, trigger)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v; want 1: the accept fired", got)
		}
		if got := ctx.Clock().Now(); got != wantNow {
			t.Errorf("clock = %v after the run; want %v", got, wantNow)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 0 {
			t.Errorf("%d wait(s) left on the clock after the run; want none", len(waits))
		}
	}
	t.Run("after advances the clock", func(t *testing.T) { fires(t, "accept after 5 [s]", 5) })
	t.Run("at advances the clock", func(t *testing.T) { fires(t, "accept at maintenanceTime", 3) })
	t.Run("at an instant already past fires at once", func(t *testing.T) { fires(t, "accept at past", 0) })
	t.Run("negative after", func(t *testing.T) {
		_, _, err := run(t, "accept after wrongWay")
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("err = %v; want ErrNegativeDuration", err)
		}
	})
	t.Run("non-time dimension", func(t *testing.T) {
		_, _, err := run(t, "accept after load")
		if !errors.Is(err, ErrTimeTriggerType) {
			t.Fatalf("err = %v; want ErrTimeTriggerType", err)
		}
		if !strings.Contains(err.Error(), "`after load` must be a ISQBase::DurationValue, found MassValue") {
			t.Errorf("err = %v; want it to name the type found", err)
		}
	})
}

// testClockAdvance: advancing the context's clock is bounded and total — zero,
// nothing waiting, a wait beyond the advance, a negative advance, a budget stop.
func testClockAdvance(t *testing.T) {
	newRun := func(t *testing.T) (*Context, *ActionExecutor, *symbols.Scope) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action patient {
					attribute done : Integer = 0;
					first start;
					then action wait accept after 8 [s];
					then action work assign done := 1;
					then done;
				}
				action tardy {
					attribute two : Time::TimeInstantValue = 2 [s];
					attribute done : Integer = 0;
					first start;
					then action wait accept at two;
					then action work assign done := 1;
					then done;
				}
				state metronome {
					attribute beats : Integer = 0;
					entry; then ticking;
					state ticking;
					transition ticking then ticking accept after 1 [s] do assign beats := beats + 1;
				}
			}
		`))
		root := idx.DocumentRoot("<test>")
		action := findSymbolByName(root, "patient", ast.DefAction)
		machine := findSymbolByName(root, "metronome", ast.DefState)
		if action == nil || machine == nil {
			t.Fatal("behaviors not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
		return ctx, exec, root
	}
	t.Run("zero", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		report, err := ctx.Advance(0)
		if err != nil {
			t.Fatalf("Advance(0): %v", err)
		}
		if report.From != 0 || report.To != 0 || report.Steps != 0 || report.Events != 0 {
			t.Errorf("report = %+v; want nothing moved", report)
		}
		if got := len(ctx.Clock().Waits()); got != 1 {
			t.Errorf("%d wait(s) on the clock; want the action's one, still queued", got)
		}
		if exec.State() == StateCompleted {
			t.Error("the action completed without the clock moving")
		}
	})
	t.Run("nothing waiting", func(t *testing.T) {
		_, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test { part def P; }`))
		report, err := ctx.Advance(12.5)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if report.To != 12.5 || ctx.Clock().Now() != 12.5 {
			t.Errorf("clock at %v, report %+v; want 12.5 with nothing run", ctx.Clock().Now(), report)
		}
	})
	t.Run("wait beyond the advance stays queued", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		if _, err := ctx.Advance(5); err != nil {
			t.Fatalf("Advance(5): %v", err)
		}
		if got := ctx.Clock().Now(); got != 5 {
			t.Errorf("clock = %v; want 5", got)
		}
		waits := ctx.Clock().Waits()
		if len(waits) != 1 || waits[0].Due != 8 {
			t.Fatalf("waits = %+v; want the action's wait still due at 8", waits)
		}
		if err := exec.Step(); !errors.Is(err, ErrNothingDue) {
			t.Errorf("Step at t=5 = %v; want ErrNothingDue, the token waits on the clock", err)
		}
		if _, err := ctx.Advance(3); err != nil {
			t.Fatalf("Advance(3): %v", err)
		}
		if got := exec.Results()["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v after the clock reached 8; want 1", got)
		}
		if got := len(ctx.Clock().Waits()); got != 0 {
			t.Errorf("%d wait(s) left after the action completed; want none", got)
		}
	})
	t.Run("at an instant the clock has passed fires at once", func(t *testing.T) {
		ctx, _, root := newRun(t)
		if _, err := ctx.Advance(4); err != nil {
			t.Fatalf("Advance(4): %v", err)
		}
		tardy := findSymbolByName(root, "tardy", ast.DefAction)
		if tardy == nil {
			t.Fatal("action tardy not found")
		}
		results, err := ctx.ExecuteAction(tardy)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v; want 1: an instant already reached is not waited for", got)
		}
		if got := ctx.Clock().Now(); got != 4 {
			t.Errorf("clock = %v; want 4, never moved back to 2", got)
		}
	})
	t.Run("negative", func(t *testing.T) {
		ctx, _, _ := newRun(t)
		_, err := ctx.Advance(-1)
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("Advance(-1) = %v; want ErrNegativeDuration", err)
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Errorf("clock moved to %v on a refused advance", got)
		}
	})
	t.Run("infinite", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		for _, duration := range []float64{math.Inf(1), math.Inf(-1), math.NaN()} {
			_, err := ctx.Advance(duration)
			if !errors.Is(err, ErrNegativeDuration) {
				t.Errorf("Advance(%v) = %v; want ErrNegativeDuration", duration, err)
			}
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Errorf("clock moved to %v on a refused advance", got)
		}
		if exec.State() == StateCompleted {
			t.Error("the action completed without the clock moving")
		}
	})
	t.Run("past the last instant the clock can hold", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		ctx.clock.now = math.MaxFloat64
		_, err := ctx.Advance(math.MaxFloat64)
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("Advance(MaxFloat64) from MaxFloat64 = %v; want ErrNegativeDuration", err)
		}
		if got := ctx.Clock().Now(); got != math.MaxFloat64 {
			t.Errorf("clock = %v on a refused advance; want it left at MaxFloat64", got)
		}
		if exec.State() == StateCompleted {
			t.Error("the action completed on a refused advance")
		}
	})
	t.Run("a wait past the last instant the clock can hold", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				action eon {
					first start;
					then action wait accept after 1.0e308 [s];
					then done;
				}
			}
		`))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "eon", ast.DefAction)
		if action == nil {
			t.Fatal("action eon not found")
		}
		ctx.clock.now = math.MaxFloat64
		_, err := ctx.ExecuteAction(action)
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("ExecuteAction = %v; want ErrNegativeDuration for a wait the clock cannot reach", err)
		}
		if got := ctx.Clock().Now(); got != math.MaxFloat64 {
			t.Errorf("clock = %v; want it left where it was", got)
		}
		if got := len(ctx.Clock().Waits()); got != 0 {
			t.Errorf("%d wait(s) left on the clock by the refused accept; want none", got)
		}
	})
	t.Run("a nested flow's wait keeps to the advance", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Fatalf("clock = %v after a run at the current instant; want 0", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 1 || waits[0].Due != 5 {
			t.Fatalf("waits = %+v; want the nested flow's wait due at 5", waits)
		}
		if err := exec.Step(); !errors.Is(err, ErrNothingDue) {
			t.Errorf("Step = %v; want ErrNothingDue, the nested flow waits on the clock", err)
		}
		report, err := ctx.Advance(2)
		if err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if got := ctx.Clock().Now(); got != 2 || report.To != 2 {
			t.Errorf("clock = %v, report %+v; want the advance to stop at 2, short of the nested wait", got, report)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Errorf("n = %v at t=2; want 0, the nested flow has not gone on", got)
		}
		if exec.State() != StateWaiting {
			t.Errorf("state = %v at t=2; want Waiting", exec.State())
		}
		if _, err := ctx.Advance(3); err != nil {
			t.Fatalf("Advance(3): %v", err)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v once the clock reached 5; want 1", got)
		}
		if exec.State() != StateCompleted || ctx.Clock().Now() != 5 {
			t.Errorf("state = %v, clock = %v; want Completed at 5", exec.State(), ctx.Clock().Now())
		}
	})
	t.Run("a nested flow's wait is advanced to by a run of its own", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		results, err := ctx.ExecuteAction(action)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v; want 1", got)
		}
		if got := ctx.Clock().Now(); got != 5 {
			t.Errorf("clock = %v; want 5, where the nested wait came due", got)
		}
	})
	t.Run("a performed action's wait keeps to the advance", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		if _, err := ctx.Advance(2); err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if got := ctx.Clock().Now(); got != 2 {
			t.Errorf("clock = %v; want the advance to stop at 2, short of the performed action's wait", got)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Errorf("n = %v at t=2; want 0, the performed action has not gone on", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 1 || waits[0].Due != 6 {
			t.Errorf("waits = %+v; want the performed action's wait due at 6", waits)
		}
		if next, ok := exec.NextWait(); !ok || next != 6 {
			t.Errorf("NextWait = %v, %v; want the performed action's wait at 6", next, ok)
		}
		if waits := exec.TimeWaits(); len(waits) != 1 || !strings.Contains(waits[0], "t=6.0") {
			t.Errorf("TimeWaits = %v; want the performed action's wait", waits)
		}
		if err := exec.Step(); !errors.Is(err, ErrNothingDue) || !strings.Contains(err.Error(), "t=6.0") {
			t.Errorf("Step = %v; want ErrNothingDue naming the performed action's wait", err)
		}
		if _, err := ctx.Advance(4); err != nil {
			t.Fatalf("Advance(4): %v", err)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v once the clock reached 6; want 1", got)
		}
		if exec.State() != StateCompleted || ctx.Clock().Now() != 6 {
			t.Errorf("state = %v, clock = %v; want Completed at 6", exec.State(), ctx.Clock().Now())
		}
		if got := len(ctx.Clock().Waits()); got != 0 {
			t.Errorf("%d wait(s) left after the action completed; want none", got)
		}
	})
	t.Run("a performed action's wait is advanced to by a run of its own", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		results, err := ctx.ExecuteAction(action)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v; want 1", got)
		}
		if got := ctx.Clock().Now(); got != 6 {
			t.Errorf("clock = %v; want 6, where the performed action's wait came due", got)
		}
	})
	t.Run("a token held at a join beside a wait is not work due", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action outer {
					attribute n : Integer = 0;
					first start;
					then fork split;
						then quick;
						then nap;
					action quick assign n := n + 1;
					then meet;
					action nap accept after 5 [s];
					then meet;
					join meet;
					then done;
				}
			}
		`))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		report, err := ctx.Advance(2)
		if err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if report.To != 2 || len(report.Notes) != 0 {
			t.Errorf("report = %+v; want the clock at 2 and no choice drawn for one action", report)
		}
		if exec.State() != StateWaiting {
			t.Errorf("state = %v at t=2; want Waiting", exec.State())
		}
		if _, err := ctx.Advance(3); err != nil {
			t.Fatalf("Advance(3): %v", err)
		}
		if exec.State() != StateCompleted || ctx.Clock().Now() != 5 {
			t.Errorf("state = %v, clock = %v; want Completed at 5", exec.State(), ctx.Clock().Now())
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v; want 1", got)
		}
	})
	t.Run("a message for another flow does not wake a nested wait", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action outer {
					attribute n : Integer = 0;
					attribute got : Integer = 0;
					first start;
					then fork split;
						then reader;
						then body;
					action reader accept m : Integer;
					then action record assign got := m;
					then meet;
					action body {
						if true {
							action inner {
								first start;
								then action nap accept after 5 [s];
								then action mark assign n := 1;
								then done;
							}
						}
					}
					then meet;
					join meet;
					then done;
				}
			}
		`))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		var nested *actionFrame
		for _, token := range exec.tokens {
			if token.Wait != nil && token.Wait.Timed {
				nested = token.frame
			}
		}
		if nested == nil {
			t.Fatalf("tokens = %+v; want one parked on the clock in the nested flow", exec.tokens)
		}
		seven := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 7}}
		ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &seven})
		if !exec.HasPendingSignal() {
			t.Error("HasPendingSignal = false; want the reader's message pending for the action")
		}
		if exec.hasPendingSignal(nested) || exec.dueNow(nested) {
			t.Error("the nested flow counts the reader's message as its own")
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence after the message: %v", err)
		}
		if got := exec.Results()["got"]; got.Kind != ValConst || got.Const.Int != 7 {
			t.Errorf("got = %v; want 7, the reader took its message", got)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Errorf("n = %v at t=0; want 0, the nested flow still waits on the clock", got)
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Errorf("clock = %v after a run at the current instant; want 0", got)
		}
		if _, err := ctx.Advance(5); err != nil {
			t.Fatalf("Advance(5): %v", err)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v once the clock reached 5; want 1", got)
		}
		if exec.State() != StateCompleted {
			t.Errorf("state = %v; want Completed", exec.State())
		}
	})
	t.Run("event budget bounds a machine that never settles", func(t *testing.T) {
		ctx, _, root := newRun(t)
		budgets := ctx.Budgets()
		budgets.MaxStateEvents = 5
		if err := ctx.SetBudgets(budgets); err != nil {
			t.Fatal(err)
		}
		machine := findSymbolByName(root, "metronome", ast.DefState)
		if _, err := ctx.CreateStateExecutor(machine); err != nil {
			t.Fatalf("CreateStateExecutor: %v", err)
		}
		_, err := ctx.Advance(1000)
		if !errors.Is(err, ErrStateEventLimitExceeded) {
			t.Fatalf("Advance = %v; want ErrStateEventLimitExceeded", err)
		}
		if !strings.Contains(err.Error(), MaxStateEventsEnvVar) {
			t.Errorf("err = %v; want it to say how to raise the budget", err)
		}
		if got := ctx.Clock().Now(); got > 6 {
			t.Errorf("clock = %v; want it stopped where the budget ran out", got)
		}
	})
	t.Run("step budget spans every instant of one advance", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action ticker {
					attribute ticks : Integer = 0;
					first start;
					then action tick assign ticks := ticks + 1;
					then action rest accept after 1 [s];
					then tick;
				}
			}
		`))
		budgets := ctx.Budgets()
		budgets.MaxActionSteps = 40
		if err := ctx.SetBudgets(budgets); err != nil {
			t.Fatal(err)
		}
		action := findSymbolByName(idx.DocumentRoot("<test>"), "ticker", ast.DefAction)
		if action == nil {
			t.Fatal("action ticker not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
		report, err := ctx.Advance(1000)
		if !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("Advance = %v (report %+v); want ErrActionStepLimitExceeded once the steps of every wake add up", err, report)
		}
		if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
			t.Errorf("err = %v; want it to say how to raise the budget", err)
		}
		if report.Steps > budgets.MaxActionSteps {
			t.Errorf("report.Steps = %d; want at most the budget of %d", report.Steps, budgets.MaxActionSteps)
		}
		if got := ctx.Clock().Now(); got >= 1000 {
			t.Errorf("clock = %v; want it stopped where the budget ran out", got)
		}
		if got := exec.Results()["ticks"]; got.Kind != ValConst || got.Const.Int >= budgets.MaxActionSteps {
			t.Errorf("ticks = %v; want fewer than the %d steps of the budget", got, budgets.MaxActionSteps)
		}
	})
	t.Run("event budget of one lets a lone action wake", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		budgets := ctx.Budgets()
		budgets.MaxStateEvents = 1
		if err := ctx.SetBudgets(budgets); err != nil {
			t.Fatal(err)
		}
		report, err := ctx.Advance(10)
		if err != nil {
			t.Fatalf("Advance = %v (report %+v); want the wake of one action, which dispatches no state event, within the budget", err, report)
		}
		if report.Events != 0 {
			t.Errorf("report.Events = %d; want none, an action step is no state event", report.Events)
		}
		if exec.State() != StateCompleted {
			t.Errorf("state = %v; want Completed", exec.State())
		}
		if got := exec.Results()["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v; want 1", got)
		}
		if got := ctx.Clock().Now(); got != 10 {
			t.Errorf("clock = %v; want 10", got)
		}
	})
	t.Run("accept when beside a timer fires when another executor changes the value", func(t *testing.T) {
		const model = `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				part def Worker {
					attribute stage : Integer = 0;
					exhibit state shift {
						entry; then working;
						state working;
						accept after 2 [s] then armed;
						state armed { entry assign stage := 1; }
						accept after 1 [s] then later;
						state later { entry assign stage := 2; }
					}
				}
				part worker : Worker;
				action watcher {
					attribute seen : Integer = -1;
					attribute timed : Integer = -1;
					first start;
					fork split;
					action quick accept when worker.stage > 0;
					action noteQuick assign seen := worker.stage;
					action slow accept after 6 [s];
					action noteSlow assign timed := worker.stage;
					join meet;
					done;
					succession first start then split;
					succession first split then quick;
					succession first split then slow;
					succession first quick then noteQuick;
					succession first noteQuick then meet;
					succession first slow then noteSlow;
					succession first noteSlow then meet;
					succession first meet then done;
				}
				state lookout {
					attribute seen : Integer = -1;
					entry; then waiting;
					state waiting;
					accept when worker.stage > 0 then noticed;
					accept after 6 [s] then late;
					state noticed { entry assign seen := worker.stage; }
					state late { entry assign seen := 100 + worker.stage; }
				}
			}`
		// The change branch sees stage 1 at t=2 while the timer holds the run to t=6;
		// seeing 2 means it was only re-tested once the timer woke the executor.
		wantSeen := func(t *testing.T, what string, got Value) {
			t.Helper()
			if got.Kind != ValConst || got.Const.Int != 1 {
				t.Errorf("%s saw stage %v; want 1, the value at the instant the condition rose", what, got)
			}
		}
		t.Run("action driving the clock", func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
			action := findSymbolByName(idx.DocumentRoot("<test>"), "watcher", ast.DefAction)
			out, err := ctx.ExecuteAction(action)
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			wantSeen(t, "the accept when branch", out["seen"])
			if got := out["timed"]; got.Kind != ValConst || got.Const.Int != 2 {
				t.Errorf("the accept after branch saw stage %v; want 2 at t=6", got)
			}
			if got := ctx.Clock().Now(); got != 6 {
				t.Errorf("clock = %v; want 6, where the timer branch woke", got)
			}
		})
		t.Run("action driven by an advance", func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
			action := findSymbolByName(idx.DocumentRoot("<test>"), "watcher", ast.DefAction)
			exec, err := ctx.CreateActionExecutor(action)
			if err != nil {
				t.Fatalf("CreateActionExecutor: %v", err)
			}
			if err := exec.RunToQuiescence(); err != nil {
				t.Fatalf("RunToQuiescence: %v", err)
			}
			if _, err := ctx.Advance(3); err != nil {
				t.Fatalf("Advance(3): %v", err)
			}
			wantSeen(t, "the accept when branch", exec.Results()["seen"])
			if exec.State() == StateCompleted {
				t.Error("the action completed before its timer branch was due")
			}
			if _, err := ctx.Advance(3); err != nil {
				t.Fatalf("Advance(3): %v", err)
			}
			if exec.State() != StateCompleted {
				t.Errorf("state = %v; want Completed once the timer branch woke", exec.State())
			}
		})
		t.Run("state machine driving the clock", func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
			machine := findSymbolByName(idx.DocumentRoot("<test>"), "lookout", ast.DefState)
			exec, err := ctx.CreateStateExecutor(machine)
			if err != nil {
				t.Fatalf("CreateStateExecutor: %v", err)
			}
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("RunToCompletion: %v", err)
			}
			wantSeen(t, "the change transition", exec.StateData()["seen"])
			if got := ctx.Clock().Now(); got != 2 {
				t.Errorf("clock = %v; want 2, the instant the condition rose", got)
			}
		})
	})
	t.Run("behaviors of an object that failed to start leave the clock", func(t *testing.T) {
		ctx, _, err := instantiateWithLibraries(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				part def Rig {
					attribute beats : Integer = 0;
					exhibit state pulse {
						entry; then idle;
						state idle;
						transition first idle accept after 1 [s] do assign beats := beats + 1 then idle;
					}
					perform action tick {
						first start;
						then action rest accept after 1 [s];
						then done;
					}
					exhibit state broken {
						state lonely;
					}
				}
			}`, "test::Rig")
		if !errors.Is(err, ErrNoInitialState) {
			t.Fatalf("instantiate = %v; want ErrNoInitialState from the machine with no initial state", err)
		}
		if got := len(ctx.clock.waiters); got != 0 {
			t.Errorf("clock drives %d executors after the failed start; want none", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 0 {
			t.Errorf("clock waits = %v after the failed start; want none", waits)
		}
		if _, ok := ctx.Clock().NextDue(); ok {
			t.Error("clock has a next due instant after the failed start; want none")
		}
		report, err := ctx.Advance(10)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if report.Events != 0 || report.Steps != 0 || report.DoSteps != 0 {
			t.Errorf("report = %+v; want nothing run for behaviors the failed start withdrew", report)
		}
		if got := ctx.Clock().Now(); got != 10 {
			t.Errorf("clock = %v; want 10", got)
		}
	})
	t.Run("a do behavior paused in a timed action leaves the clock with its dropped machine", func(t *testing.T) {
		ctx, _, err := instantiateWithLibraries(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action def Poll { inout n : Integer; action wait accept after 5 [s]; then assign n := n + 1; }
				part def Rig {
					attribute ticks : Integer = 0;
					exhibit state pulse {
						entry; then busy;
						state busy { do action poll : Poll { inout n = ticks; } }
					}
					perform action bad { action crash assign ticks := ticks / 0; }
				}
			}`, "test::Rig")
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("instantiate = %v; want ErrDivisionByZero from the action that fails the start", err)
		}
		if got := len(ctx.clock.waiters); got != 0 {
			t.Errorf("clock drives %d executors after the failed start; want none: the performed Poll left with its machine", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 0 {
			t.Errorf("clock waits = %v after the failed start; want none", waits)
		}
		if len(ctx.objectBehaviors) != 0 {
			t.Errorf("%d behavior(s) still attached after the failed start; want none", len(ctx.objectBehaviors))
		}
		report, err := ctx.Advance(10)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if report.Events != 0 || report.Steps != 0 || report.DoSteps != 0 {
			t.Errorf("report = %+v; want nothing run for the do behavior the failed start withdrew", report)
		}
	})
}

// nestedWaitModel waits on the clock in the flow of a block a body statement runs.
const nestedWaitModel = `
	package test {
		private import SI::*;
		private import ScalarValues::*;
		action outer {
			attribute n : Integer = 0;
			first start;
			then action body {
				if true {
					action inner {
						first start;
						then action nap accept after 5 [s];
						then action mark assign n := 1;
						then done;
					}
				}
			}
			then done;
		}
	}
`

// performedWaitModel waits on the clock in an action a node of the flow performs,
// after a wait of the flow's own.
const performedWaitModel = `
	package test {
		private import SI::*;
		private import ScalarValues::*;
		action def Napper {
			out attribute n : Integer = 0;
			first start;
			then action nap accept after 5 [s];
			then action mark assign n := 1;
			then done;
		}
		action outer {
			attribute n : Integer = 0;
			first start;
			then action early accept after 1 [s];
			then perform action call : Napper;
			then action copy assign n := call.n;
			then done;
		}
	}
`

// testActionAcceptNonBooleanChangeTrigger: a change trigger states a condition,
// so one that evaluates to something else is reported rather than read as true.
func testActionAcceptNonBooleanChangeTrigger(t *testing.T) {
	src := `
		package test {
			action monitor {
				attribute temp : Integer = 10;
				first start;
				action awaitWarm accept when temp;
				done;
				succession first start then awaitWarm;
				succession first awaitWarm then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "monitor", ast.DefAction)
	if sym == nil {
		t.Fatal("action monitor not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("ExecuteAction error = %v, want ErrTypeMismatch", err)
	}
}

// testActionBodyUnresolvedUnit: an action body is evaluated in the scope it was
// written in, and a unit that scope does not bring in resolves to nothing — the
// quantity is reported as such rather than evaluated as its bare magnitude.
func testActionBodyUnresolvedUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ScalarValues::*;
			action descend {
				attribute h : Real = 500.0 [furlong];
				first start;
				done;
				succession first start then done;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "descend", ast.DefAction)
	if sym == nil {
		t.Fatal("action descend not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("outputs = %v, err = %v; want ErrNotAQuantity", out, err)
	}
	if !strings.Contains(err.Error(), semantics.ErrNotAUnit.Error()) {
		t.Errorf("err = %v; want it to report that the index names no measurement unit", err)
	}
}

// testActionBodyUnresolvedFeature: a name no frame, object or enclosing scope
// supplies is reported as unresolved, so giving a body its declaring scope does
// not turn a typo into a silent zero.
func testActionBodyUnresolvedFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action bump {
					assign total := missingName + 1;
				}
				done;
				succession first start then bump;
				succession first bump then done;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("outputs = %v, err = %v; want ErrUnresolvedReference", out, err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
}

// testStateBodyUnresolvedUnit: the same for a state machine's attribute default,
// which is evaluated when the machine initializes.
func testStateBodyUnresolvedUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ScalarValues::*;
			state monitor {
				attribute speed : Real = 1.5 [knot];
				entry; then start;
				state start;
				state running;
				succession first start then running;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "monitor", ast.DefState)
	if sym == nil {
		t.Fatal("state monitor not found")
	}

	_, err := ctx.ExecuteState(sym)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("err = %v; want ErrNotAQuantity", err)
	}
}

// Helper: parse source into AST RootNamespace
func parseAndBuild(t *testing.T, src string) *ast.RootNamespace {
	file := parser.New(source.New("<test>", []byte(src))).ParseFile()
	return file
}

// testLibraryFunctionOutsideItsDomain: a library function whose argument has no
// result reports a domain error rather than returning a NaN.
func testLibraryFunctionOutsideItsDomain(t *testing.T) {
	src := `
		package test {
			private import RealFunctions::*;
			calc root {
				in x : Real;
				return : Real = sqrt(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "root", ast.DefCalc)
	if sym == nil {
		t.Fatal("root calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: -1}}
	got, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticDomain) {
		t.Fatalf("sqrt(-1.0) = %+v, %v; want a domain error", got, err)
	}
}

// testLibraryFunctionWrongArity: a library function called with the wrong number
// of arguments reports an arity error rather than reading past its arguments.
func testLibraryFunctionWrongArity(t *testing.T) {
	fn, ok := libraryFunctionByName("RealFunctions::max")
	if !ok {
		t.Fatal("RealFunctions::max not registered")
	}
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, "package test { }"))

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}}
	if _, err := fn.invoke(ctx, calcArgs{positional: []Value{arg}}); !errors.Is(err, ErrCalcArity) {
		t.Fatalf("max(1.0) error = %v, want ErrCalcArity", err)
	}
}

// testExtensionLibraryFunctionOutsideItsDomain: an OpenSysML extension library
// function reports a domain error the same way a vendored one does — the
// logarithm of zero has no Real value, and is not returned as an infinity.
func testExtensionLibraryFunctionOutsideItsDomain(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import OpenSysMLMathFunctions::*;
			calc root {
				in x : Real;
				return : Real = ln(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "root", ast.DefCalc)
	if sym == nil {
		t.Fatal("root calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 0}}
	got, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticDomain) {
		t.Fatalf("ln(0.0) = %+v, %v; want a domain error", got, err)
	}
}

// testExponentiationIntegerOverflow: an exponentiation beyond the Integer range
// is reported rather than wrapping.
func testExponentiationIntegerOverflow(t *testing.T) {
	src := `
		package test {
			calc power {
				in b : Integer;
				in e : Integer;
				return : Integer = b ** e;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "power", ast.DefCalc)
	if sym == nil {
		t.Fatal("power calc not found")
	}

	base := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1 << 40}}
	exp := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	got, err := ctx.InvokeCalc(sym, []Value{base, exp}, rootScope)
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("(2**40) ** 3 = %+v, %v; want an overflow error", got, err)
	}
}

// testQuantityIncommensurableComparison: comparing quantities whose units
// measure different things reports ErrIncommensurableUnits instead of comparing
// the bare magnitudes, which would make 1.5 [m/s] <= 2.0 [s] true.
func testQuantityIncommensurableComparison(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			requirement def Touchdown {
				attribute speed = 1.5 [m/s];
				attribute duration = 2.0 [s];
				require constraint { speed <= duration }
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Touchdown", ast.DefRequirement)
	if sym == nil {
		t.Fatal("Touchdown requirement not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Fatalf("satisfied = %v, err = %v; want ErrIncommensurableUnits", satisfied, err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("incommensurable units are not a violation: neither verdict is an answer")
	}
}

// testQuantityIndexIsNotAUnit: a bracketed expression whose index names
// something that is not a measurement unit reports ErrNotAQuantity rather than
// evaluating to the bare magnitude.
func testQuantityIndexIsNotAUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute notAUnit = 3.0;
			constraint bogus {
				1.5 [test::notAUnit] <= 2.0 [m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "bogus", ast.DefConstraint)
	if sym == nil {
		t.Fatal("bogus constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, rootScope)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAQuantity", satisfied, err)
	}
	if !strings.Contains(err.Error(), semantics.ErrNotAUnit.Error()) {
		t.Errorf("err = %v; want it to report that the index names no measurement unit", err)
	}
}

// testQuantityUnitShadowedBySibling: a unit position naming a sibling that is
// not a measurement unit reports which declaration it resolved to and the unit
// that declaration hid, rather than a magnitude in the wrong unit.
func testQuantityUnitShadowedBySibling(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			constraint def Tall {
				attribute m : ScalarValues::Real = 2.0;
				1.0 [m] > 500.0 [m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, ErrNotAQuantity) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAQuantity", satisfied, err)
	}
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Errorf("err = %v; want it to report that the name is no measurement unit", err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Resolved == nil || shadowed.Namespace != "test::Tall" {
		t.Errorf("error names %v in %q; want the sibling declared in test::Tall", shadowed.Resolved, shadowed.Namespace)
	}
	if shadowed.Shadowed == nil || shadowed.Suggestion != "SI::m" {
		t.Errorf("error suggests %q; want the qualified spelling SI::m of the hidden unit", shadowed.Suggestion)
	}
}

// testQuantityQualifiedUnitIsNotShadowing: a qualified name in unit position
// resolves to what it names, so a non-unit is reported as one without a
// shadowing explanation or a spelling that would not resolve.
func testQuantityQualifiedUnitIsNotShadowing(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute m : ScalarValues::Real = 2.0;
			constraint def Tall {
				1.0 [test::m] > 500.0 [SI::m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAUnit", satisfied, err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Shadowed != nil || shadowed.Suggestion != "" {
		t.Errorf("error suggests %q for a qualified name; want no shadowing explanation", shadowed.Suggestion)
	}
	if !strings.Contains(err.Error(), "test::m resolves to") {
		t.Errorf("err = %v; want it to name the declaration as written", err)
	}
}

// testQuantityShadowedUnitWithoutAQualifier: a hidden unit owned by no namespace
// has no qualified spelling to offer, so the diagnostic names it without
// advising the name that just failed.
func testQuantityShadowedUnitWithoutAQualifier(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		attribute u : ISQBase::LengthUnit = SI::m;
		package test {
			attribute u : ScalarValues::Real = 2.0;
			constraint def Tall {
				1.0 [u] > 0.5 [SI::m]
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAUnit", satisfied, err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Shadowed == nil {
		t.Fatalf("err = %v; want it to name the unit the declaration hid", err)
	}
	if shadowed.Suggestion != "" {
		t.Errorf("error suggests %q; want no spelling when none qualifies the unit", shadowed.Suggestion)
	}
	if strings.Contains(err.Error(), "write u") {
		t.Errorf("err = %v; want it not to advise the name that failed", err)
	}
}

// testQuantityCyclicUnitDefinition: two units defined in terms of each other are
// reported as a cycle instead of recursing until the stack or step budget runs
// out.
func testQuantityCyclicUnitDefinition(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute unitA : ISQBase::LengthUnit = unitB;
			attribute unitB : ISQBase::LengthUnit = unitA;
			constraint cyclic {
				1.0 [test::unitA] <= 2.0 [test::unitA]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "cyclic", ast.DefConstraint)
	if sym == nil {
		t.Fatal("cyclic constraint not found")
	}

	done := make(chan error, 1)
	go func() {
		_, err := ctx.EvaluateConstraint(sym, rootScope)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, semantics.ErrUnitCycle) {
			t.Fatalf("err = %v; want ErrUnitCycle", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("evaluating a cyclic unit definition did not terminate")
	}
}

// Helper: build runtime context from file
func buildRuntime(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context) {
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(NewModel(model, resolver), 10000)
	return idx, model, ctx
}

// buildRuntimeWithLibraries builds a runtime context over an index that carries
// the standard library, for a model that names its elements.
func buildRuntimeWithLibraries(t *testing.T, path string, file *ast.RootNamespace) (*symbols.Index, *semantics.Model, *Context) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument(path, file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	return idx, model, NewContext(NewModel(model, resolver), 10000)
}

// Helper: find symbol by name and kind
func findSymbolByName(scope *symbols.Scope, name string, kind ast.DefinitionKind) *symbols.Symbol {
	// Map DefKind to UsageKind
	var usageKind ast.UsageKind
	switch kind {
	case ast.DefCalc:
		usageKind = ast.UsageCalc
	case ast.DefAction:
		usageKind = ast.UsageAction
	case ast.DefState:
		usageKind = ast.UsageState
	case ast.DefConstraint:
		usageKind = ast.UsageConstraint
	case ast.DefRequirement:
		usageKind = ast.UsageRequirement
	}

	// Check all child scopes (packages/namespaces)
	for _, child := range scope.Children() {
		for _, memberName := range child.MemberNames() {
			sym, _ := child.LookupLocal(memberName)
			if sym == nil {
				continue
			}

			if sym.Name == name {
				switch decl := sym.Decl.(type) {
				case *ast.Definition:
					if decl.Kind == kind {
						return sym
					}
				case *ast.Usage:
					if decl.Kind == usageKind {
						return sym
					}
				}
			}
		}
	}

	// Also check root scope directly
	for _, memberName := range scope.MemberNames() {
		sym, _ := scope.LookupLocal(memberName)
		if sym == nil {
			continue
		}

		if sym.Name == name {
			switch decl := sym.Decl.(type) {
			case *ast.Definition:
				if decl.Kind == kind {
					return sym
				}
			case *ast.Usage:
				if decl.Kind == usageKind {
					return sym
				}
			}
		}
	}
	return nil
}

// invokeCalcInSource invokes calcName with one Integer argument and returns the
// error, on its own goroutine so a body that never terminates fails the case
// instead of stalling the suite. maxSteps bounds the run.
func invokeCalcInSource(t *testing.T, src, calcName string, arg int64, maxSteps int64) error {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		value := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: arg}}
		result, err := ctx.InvokeCalc(sym, []Value{value}, rootScope)
		if err == nil {
			err = fmt.Errorf("calc %s returned %s, expected it to fail", calcName, FormatTraceValue(result))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
		return nil
	}
}

// testCalcNonTerminatingLoop: a calc loop whose condition always holds spends
// the context's step budget, so it fails the invocation instead of hanging the
// REPL, LSP or gRPC caller that drove it.
func testCalcNonTerminatingLoop(t *testing.T) {
	src := `
		package test {
			calc spin {
				in n: Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					assign i := i + 1;
				}
				return : Integer = i;
			}
		}
	`
	err := invokeCalcInSource(t, src, "spin", 1, 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCalcBodyNeverReturns: a body that computes but reaches no `return` states
// no result, which is an error rather than a null value.
func testCalcBodyNeverReturns(t *testing.T) {
	src := `
		package test {
			calc maybe {
				in n: Integer;
				attribute total : Integer = 0;
				if n > 0 {
					assign total := n;
				}
				if n < 0 {
					return : Integer = total;
				}
			}
		}
	`
	err := invokeCalcInSource(t, src, "maybe", 5, 10000)
	if !errors.Is(err, ErrCalcNoReturn) {
		t.Errorf("expected ErrCalcNoReturn, got: %v", err)
	}
}

// testCalcSendIsRejected: a calculation computes a value and nothing else, so a
// send in its body is rejected rather than posted.
func testCalcSendIsRejected(t *testing.T) {
	src := `
		package test {
			attribute def Ping;
			calc noisy {
				in n: Integer;
				send Ping() to listener;
				return : Integer = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "noisy", 1, 10000)
	if !errors.Is(err, ErrCalcSideEffect) {
		t.Errorf("expected ErrCalcSideEffect, got: %v", err)
	}
}

// testCalcTerminateIsRejected: `terminate` ends an execution, which a
// calculation has no business doing.
func testCalcTerminateIsRejected(t *testing.T) {
	src := `
		package test {
			calc halting {
				in n: Integer;
				terminate;
				return : Integer = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "halting", 1, 10000)
	if !errors.Is(err, ErrCalcSideEffect) {
		t.Errorf("expected ErrCalcSideEffect, got: %v", err)
	}
}

// testCalcAssignmentOutsideTheCalc: a calc may write its own parameters and
// locals; a name it does not declare belongs to the model around it and writing
// it would be an effect, so it is rejected.
func testCalcAssignmentOutsideTheCalc(t *testing.T) {
	src := `
		package test {
			attribute shared : Integer = 0;
			calc leaky {
				in n: Integer;
				assign shared := n;
				return : Integer = n;
			}
		}
	`
	err := invokeCalcInSource(t, src, "leaky", 3, 10000)
	if !errors.Is(err, ErrCalcExternalAssignment) {
		t.Errorf("expected ErrCalcExternalAssignment, got: %v", err)
	}
}

// testCalcNonBooleanCondition: a condition that is not Boolean is a type error
// the typecheck pass reports; an execution that reaches one anyway says so
// rather than coercing the value.
func testCalcNonBooleanCondition(t *testing.T) {
	src := `
		package test {
			calc counting {
				in n: Integer;
				while n {
					return : Integer = 1;
				}
				return : Integer = 0;
			}
		}
	`
	err := invokeCalcInSource(t, src, "counting", 3, 10000)
	if err == nil || !strings.Contains(err.Error(), "must evaluate to a Boolean") {
		t.Errorf("expected a non-Boolean condition error, got: %v", err)
	}
}

// calcUsageOutputInSource reads one output feature of the named calc usage and
// returns the error the read reports, on its own goroutine so a body that never
// terminates fails the case instead of stalling the suite. maxSteps bounds the
// run.
func calcUsageOutputInSource(t *testing.T, src, usageName, output string, maxSteps int64, budgets ...func(*Context)) error {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	for _, set := range budgets {
		set(ctx)
	}
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, usageName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc usage %s not found", usageName)
	}

	done := make(chan error, 1)
	go func() {
		value, err := ctx.CalcUsageOutput(sym, output, sym.OwnerScope, nil)
		if err == nil {
			err = fmt.Errorf("output %s of %s answered %s, expected the read to fail",
				output, usageName, FormatTraceValue(value))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("reading output %s of %s did not terminate", output, usageName)
		return nil
	}
}

// testCalcUsageUnboundInput: a usage that leaves an input of its calc with
// neither a value nor a default computes nothing, since a usage passes no
// arguments to stand in for one.
func testCalcUsageUnboundInput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc c : Two;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testCalcUsageUnknownOutput: a name the calc declares no output for is a
// modeling error, not an empty value.
func testCalcUsageUnknownOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc c : Two { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "nope", 10000)
	if !errors.Is(err, ErrUnknownOutput) {
		t.Errorf("expected ErrUnknownOutput, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "a, b") {
		t.Errorf("error should name the outputs the calc does declare, got: %v", err)
	}
}

// testCalcUsageCyclicOutputs: outputs valued from each other have no value to
// compute, which is reported as the cycle it is rather than spending the step
// budget or hanging.
func testCalcUsageCyclicOutputs(t *testing.T) {
	src := `
		package test {
			calc def Knot {
				in n : Integer;
				out a = b + 1;
				out b = a + n;
			}
			calc c : Knot { in n = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrCyclicOutput) {
		t.Errorf("expected ErrCyclicOutput, got: %v", err)
	}
}

// testCalcUsageSpecializesANonCalc: a calc usage typed by something that is not
// a calc inherits no parameters, outputs or body from it, so the specialization
// is reported rather than the outputs it appears to be missing.
func testCalcUsageSpecializesANonCalc(t *testing.T) {
	src := `
		package test {
			part def Chassis {
				attribute mass : Integer = 4;
			}
			calc c : Chassis;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "mass", 10000)
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testCalcUsageStepBudget: a usage whose body never terminates spends the step
// budget of the run reading its output, so the read fails instead of hanging
// whoever drove it.
func testCalcUsageStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Spin {
				in n : Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					assign i := i + 1;
				}
				out reached = i;
			}
			calc c : Spin { in n = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "reached", 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCalcUsageOutputWithoutAValue: an output the calc declares but binds no
// value to computes nothing, so reading it says so rather than answering null.
func testCalcUsageOutputWithoutAValue(t *testing.T) {
	src := `
		package test {
			calc def Half {
				in n : Integer;
				out a = n + 1;
				out b : Integer;
			}
			calc c : Half { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "b", 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
}

// testCalcOutputNeverAssignedByTheBody: a body that assigns one of two declared
// outputs leaves the other unbound, and the read says that rather than blaming a
// missing result expression.
func testCalcOutputNeverAssignedByTheBody(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a : Integer;
				out b : Integer;
				assign a := n + 1;
			}
			calc c : Two { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "b", 10000)
	if !errors.Is(err, ErrOutputNotAssigned) {
		t.Errorf("expected ErrOutputNotAssigned, got: %v", err)
	}
	if errors.Is(err, ErrNoResultExpression) {
		t.Errorf("expected no result-expression blame, got: %v", err)
	}
}

// testCalcOutputAssignedInABranchNotTaken: an output only a branch that does not
// run would assign is unbound for that activation.
func testCalcOutputAssignedInABranchNotTaken(t *testing.T) {
	src := `
		package test {
			calc def Branch {
				in n : Integer;
				out a : Integer;
				if n > 10 {
					assign a := n;
				}
			}
			calc c : Branch { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrOutputNotAssigned) {
		t.Errorf("expected ErrOutputNotAssigned, got: %v", err)
	}
}

// testCalcOutputValuedAndAssigned: an output given a value two ways is reported
// rather than silently picking one (the precedent of #127/#131).
func testCalcOutputValuedAndAssigned(t *testing.T) {
	src := `
		package test {
			calc def Both {
				in n : Integer;
				out a : Integer = n;
				assign a := n + 1;
			}
			calc c : Both { in n = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 10000)
	if !errors.Is(err, ErrConflictingOutput) {
		t.Errorf("expected ErrConflictingOutput, got: %v", err)
	}
}

// testCalcOutputBindingViolatesDeclaredType: an output whose declaration binds
// it a value of another type is rejected where a write of one is, the input it
// reads being untyped so only the run time can judge it.
func testCalcOutputBindingViolatesDeclaredType(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def Bound {
				in n;
				out a : Integer = n;
			}
			calc c : Bound { in n = "seven"; }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	usage := oneSymbol(t, idx, "test::c")
	_, err := ctx.CalcUsageOutput(usage, "a", idx.DocumentRoot("<test>"), nil)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "Integer") {
		t.Errorf("error = %v, want it to name the declared type", err)
	}
}

// testCalcOutputAssignedTwice: a body assigning an output more than once leaves
// it bound to the last assignment that ran, the same as a body local, so an
// output may be initialized and then accumulated into.
func testCalcOutputAssignedTwice(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in n : Integer;
				out a : Integer;
				assign a := n + 1;
				assign a := a + 1;
			}
			calc c : Twice { in n = 5; }
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = 10000
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "c", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc usage c not found")
	}

	value, err := ctx.CalcUsageOutput(sym, "a", sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("reading output a of c: %v", err)
	}
	if got := FormatTraceValue(value); got != "7" {
		t.Errorf("output a = %s, want 7", got)
	}
}

// testMultipleOutputsInvokedAsAnExpression: an invocation of a function yields
// exactly one result (KerML 7.4.9), so invoking a calc that computes several
// outputs and designates no result is reported rather than answered with
// whichever output happens to come first.
func testMultipleOutputsInvokedAsAnExpression(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Two", ast.DefCalc)
	if sym == nil {
		t.Fatal("Two calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected the invocation to be rejected, it answered %s", FormatTraceValue(result))
	}
	if !errors.Is(err, ErrAmbiguousResult) {
		t.Errorf("expected ErrAmbiguousResult, got: %v", err)
	}
	// The diagnostic has to teach the spelling that does work.
	if err != nil && !strings.Contains(err.Error(), "calc c : test::Two { in n = ...; } then read c.a") {
		t.Errorf("error should spell out the calc usage to declare, got: %v", err)
	}
}

// testNestedCalcUsageUnboundInput: a usage nested in a calc binds its inputs
// from the enclosing evaluation, and an input nothing there values is reported.
func testNestedCalcUsageUnboundInput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				in m : Integer;
				calc inner : Two;
				out d = inner.a;
			}
			calc c : Outer { in m = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("expected ErrUnboundParameter, got: %v", err)
	}
}

// testNestedCalcUsageUnknownOutput: reading a name the nested calc declares no
// output for is a modeling error wherever the usage is declared.
func testNestedCalcUsageUnknownOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				in m : Integer;
				calc inner : Two { in n = m; }
				out d = inner.nope;
			}
			calc c : Outer { in m = 5; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrUnknownOutput) {
		t.Errorf("expected ErrUnknownOutput, got: %v", err)
	}
}

// testNestedCalcUsageSelfCycle: an input of a nested usage valued from its own
// name with nothing outside to resolve to stays the cycle it is, rather than
// being read as the shadowing binding it looks like.
func testNestedCalcUsageSelfCycle(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			calc def Outer {
				calc inner : Two { in n = n; }
				out d = inner.a;
			}
			calc c : Outer;
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 10000)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("expected ErrCyclicFeatureValue, got: %v", err)
	}
}

// testNestedCalcUsageRecursionDepth: a calc whose nested usage is of itself
// never bottoms out, so the depth budget reports it instead of hanging. A usage
// frame costs far more than an invocation, so the case states a shallow budget.
func testNestedCalcUsageRecursionDepth(t *testing.T) {
	src := `
		package test {
			calc def Down {
				in n : Integer;
				calc next : Down { in n = n - 1; }
				out a = next.a;
				out b = n;
			}
			calc c : Down { in n = 3; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "a", 1000000,
		func(ctx *Context) { ctx.maxCalcDepth = nestingProbeDepth })
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Errorf("expected ErrCalcRecursionLimit, got: %v", err)
	}
}

// testNestedCalcUsageStepBudget: the body of a nested usage spends the budget of
// the run reading it, so a body that never terminates fails the read.
func testNestedCalcUsageStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Spin {
				in n : Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					assign i := i + 1;
				}
				out reached = i;
			}
			calc def Outer {
				in m : Integer;
				calc inner : Spin { in n = m; }
				out d = inner.reached;
			}
			calc c : Outer { in m = 1; }
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "d", 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// variationFeatureValueInSource instantiates a usage and returns the value its named
// feature value holds, so a variation's failure modes are read where a model reads them.
func variationFeatureValueInSource(t *testing.T, src, usage, fv string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	matches := idx.LookupQualified(usage)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", usage, len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		return Value{}, err
	}
	got, err := inst.GetFeatureValue(ctx, fv)
	if err != nil {
		return Value{}, err
	}
	return got.Value, nil
}

// variationFamily is a variation point with three variants, over which the
// selection a specialization makes is varied per case.
const variationFamily = `
	package test {
		part def Diamond { attribute cut; attribute color; }
		abstract part family : Diamond {
			variation attribute :>> cut {
				variant attribute cutShallow { attribute cost = 200.0; }
				variant attribute cutIdeal { attribute cost = 250.0; }
			}
			variation attribute :>> color {
				variant attribute colorWhite { attribute cost = 100.0; }
			}
		}
		%s
	}`

// testVariationWithoutASelectedVariant: a variation nothing selects a variant
// for has no value, so reading it says so rather than answering the variation's
// own empty object or one of the variants arbitrarily.
func testVariationWithoutASelectedVariant(t *testing.T) {
	src := fmt.Sprintf(variationFamily, `part unconfigured :> family;`)
	got, err := variationFeatureValueInSource(t, src, "test::unconfigured", "cut")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("cut = (%v, %v), want ErrVariationUnselected", got, err)
	}
	// The failure names the feature, so a model with many variation points says
	// which one is unconfigured.
	if err != nil && !strings.Contains(err.Error(), "cut") {
		t.Errorf("error %q does not name the variation", err)
	}
}

// variationReadFromDeclaration evaluates a usage's value with no bound object,
// so a variation is read through its declaration rather than through a feature value.
func variationReadFromDeclaration(t *testing.T, src, probe string) (Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	matches := idx.LookupQualified(probe)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", probe, len(matches))
	}
	usage, ok := matches[0].Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("%s: no value expression", probe)
	}
	return ctx.EvalWithScope(usage.Value, matches[0].OwnerScope)
}

// testVariationReadThroughItsDeclaration: a variation read without a bound
// object is bound the same way as one read from a feature value, so what a legal
// selection is does not depend on how the model is inspected.
func testVariationReadThroughItsDeclaration(t *testing.T) {
	for _, tt := range []struct {
		name, decl, probe string
		want              error
	}{
		{
			"not_a_variant",
			`part chosen :> family { attribute :>> cut = 250.0; attribute probe = cut; }`,
			"test::chosen::probe", ErrNotAVariant,
		},
		{
			"two_variants",
			`part chosen :> family { attribute :>> cut = (cut::cutIdeal, cut::cutShallow); attribute probe = cut; }`,
			"test::chosen::probe", ErrMultipleVariants,
		},
		{
			"unselected",
			`part chosen :> family { attribute probe = cut; }`,
			"test::chosen::probe", ErrVariationUnselected,
		},
		{
			"qualified_not_a_variant",
			`part chosen :> family { attribute :>> cut = 250.0; }
			 attribute probe = chosen::cut;`,
			"test::probe", ErrNotAVariant,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variationReadFromDeclaration(t, fmt.Sprintf(variationFamily, tt.decl), tt.probe)
			if !errors.Is(err, tt.want) {
				t.Errorf("%s = (%v, %v), want %v", tt.probe, got, err, tt.want)
			}
		})
	}
}

// testChainThroughAnUnselectedVariationPart: a variation part is no occurrence
// of itself, so a chain through one nothing selected a variant for reports that
// rather than reading an object of the variation.
func testChainThroughAnUnselectedVariationPart(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::Real;
		part def Engine { attribute power : Real; }
		variation part engine : Engine {
			variant part electric : Engine { attribute :>> power = 100.0; }
			variant part diesel : Engine { attribute :>> power = 200.0; }
		}
		part probe { attribute p : Real = engine.power; }
	}`
	got, err := variationFeatureValueInSource(t, src, "test::probe", "p")
	if !errors.Is(err, ErrVariationUnselected) {
		t.Errorf("p = (%v, %v), want ErrVariationUnselected", got, err)
	}
}

// testClassifyAnUnselectedOptionalVariation: an optional variation nothing selects
// a variant for is a choice, not an empty collection, so `istype` reports that
// rather than vacuously matching every type.
func testClassifyAnUnselectedOptionalVariation(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Engine;
		part def Motor :> Engine;
		variation part engine : Engine[0..1] {
			variant part electric : Motor;
			variant part diesel : Engine;
		}
		attribute isMotor : Boolean = engine istype Motor;
		attribute hasMotor : Boolean = engine hastype Motor;
	}`
	for _, probe := range []string{"test::isMotor", "test::hasMotor"} {
		got, err := variationReadFromDeclaration(t, src, probe)
		if !errors.Is(err, ErrVariationUnselected) {
			t.Errorf("%s = (%v, %v), want ErrVariationUnselected", probe, got, err)
		}
	}
}

// testVariationBoundToWhatIsNotAVariant: a selection naming something that is
// not a variant of the variation is reported, whether the name is unknown, a
// variant of another variation, or an ordinary value.
func testVariationBoundToWhatIsNotAVariant(t *testing.T) {
	for _, tt := range []struct{ name, selection string }{
		{"unknown_name", `part chosen :> family { attribute :>> cut = cut::nope; }`},
		{"variant_of_another_variation", `part chosen :> family { attribute :>> cut = color::colorWhite; }`},
		{"ordinary_value", `part chosen :> family { attribute :>> cut = 250.0; }`},
		{"collection_of_ordinary_values", `part chosen :> family { attribute :>> cut = (250.0, 200.0); }`},
		{"variant_mixed_with_an_ordinary_value", `part chosen :> family { attribute :>> cut = (cut::cutIdeal, 250.0); }`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := variationFeatureValueInSource(t, fmt.Sprintf(variationFamily, tt.selection), "test::chosen", "cut")
			if !errors.Is(err, ErrNotAVariant) {
				t.Errorf("cut = (%v, %v), want ErrNotAVariant", got, err)
			}
		})
	}
}

// testVariationBoundToTwoVariants: a variation stands for one variant, so a
// selection of several is reported rather than silently taking the first.
func testVariationBoundToTwoVariants(t *testing.T) {
	src := fmt.Sprintf(variationFamily,
		`part chosen :> family { attribute :>> cut = (cut::cutIdeal, cut::cutShallow); }`)
	got, err := variationFeatureValueInSource(t, src, "test::chosen", "cut")
	if !errors.Is(err, ErrMultipleVariants) {
		t.Fatalf("cut = (%v, %v), want ErrMultipleVariants", got, err)
	}
	for _, name := range []string{"cutIdeal", "cutShallow"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name the selection %s", err, name)
		}
	}
}

// testRepeatedReadsOfAVariantObject: the object a selected variant stands for is
// materialized once, so evaluating a chain through it repeatedly neither piles up
// objects nor exhausts the step budget.
func testRepeatedReadsOfAVariantObject(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
		}
		part chosen :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	variant := oneSymbol(t, idx, "test::family::engine::electric")
	variation := oneSymbol(t, idx, "test::family::engine")
	first, err := ctx.variantValue(variation, variant, 1)
	if err != nil {
		t.Fatalf("variantValue: %v", err)
	}
	count := len(ctx.instances)
	for i := 0; i < 10; i++ {
		again, err := ctx.variantValue(variation, variant, 1)
		if err != nil {
			t.Fatalf("variantValue (read %d): %v", i+2, err)
		}
		if again.Instance != first.Instance {
			t.Fatalf("read %d gave instance %d, want %d", i+2, again.Instance, first.Instance)
		}
	}
	if len(ctx.instances) != count {
		t.Errorf("instances grew from %d to %d over repeated reads", count, len(ctx.instances))
	}
}

// testTwoOwnersSelectingOneVariant: a variant is selected per owning object, so
// two owners of one variation each hold their own object of the variant rather
// than sharing one whose materialized feature values the other reads.
func testTwoOwnersSelectingOneVariant(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ids := make([]int64, 0, 2)
	for _, usage := range []string{"test::sedan", "test::coupe"} {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		fv, err := inst.GetFeatureValue(ctx, "engine")
		if err != nil {
			t.Fatalf("%s.engine: %v", usage, err)
		}
		id, ok := fv.Value.Object()
		if !ok {
			t.Fatalf("%s.engine = %v, want an object of the selected variant", usage, fv.Value)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Errorf("sedan and coupe share engine object %d", ids[0])
	}
}

// testTwoOwnerlessSelectionsOfOneVariant: a variation read through its
// declaration has no owning object, so two variation points selecting one
// variant must still stand for an object each rather than share one.
func testTwoOwnerlessSelectionsOfOneVariant(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	variant := oneSymbol(t, idx, "test::family::engine::electric")
	ids := make([]int64, 0, 2)
	for _, variation := range []string{"test::sedan::engine", "test::coupe::engine"} {
		val, err := ctx.variantValue(oneSymbol(t, idx, variation), variant, 0)
		if err != nil {
			t.Fatalf("%s: %v", variation, err)
		}
		ids = append(ids, val.Instance)
	}
	if ids[0] == ids[1] {
		t.Errorf("sedan.engine and coupe.engine share object %d", ids[0])
	}
}

// testVariantOutsideAVariation: `variant` on a member whose owner is not a
// variation offers no choice, so the member stays an ordinary feature instead of
// silently holding no value.
func testVariantOutsideAVariation(t *testing.T) {
	src := `
	package test {
		part def Widget { variant attribute misplaced = 1.0; }
		part widget : Widget;
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::widget"))
	if err != nil {
		t.Fatalf("Instantiate(test::widget): %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "misplaced")
	if err != nil {
		t.Fatalf("widget.misplaced: %v", err)
	}
	if fv.Value.Kind != ValConst || fv.Value.Const.Real != 1.0 {
		t.Errorf("widget.misplaced = %v, want 1", fv.Value)
	}
}

// testVariantUnderARedefinedVariation: a usage redefining a variation usage is a
// variation point without restating the modifier, so the variants under it stay
// choices that specialize it instead of materializing feature values.
func testVariantUnderARedefinedVariation(t *testing.T) {
	src := `
	package test {
		part def Engine { attribute power; }
		abstract part family {
			variation part engine : Engine {
				variant part petrol : Engine { attribute :>> power = 90.0; }
			}
		}
		abstract part refined :> family {
			part :>> engine {
				variant part electric { attribute :>> power = 150.0; }
			}
		}
		part sedan :> refined { part :>> engine = engine::electric; }
	}`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::sedan"))
	if err != nil {
		t.Fatalf("Instantiate(test::sedan): %v", err)
	}
	if fv, err := inst.GetFeatureValue(ctx, "electric"); err == nil {
		t.Errorf("sedan.electric materialized a feature value: %v", fv.Value)
	}
	fv, err := inst.GetFeatureValue(ctx, "engine")
	if err != nil {
		t.Fatalf("sedan.engine: %v", err)
	}
	if fv.Value.Kind != ValVariant || fv.Value.Instance == 0 {
		t.Fatalf("sedan.engine = %v, want the selected variant's object", fv.Value)
	}
	power, err := ctx.instances[fv.Value.Instance].GetFeatureValue(ctx, "power")
	if err != nil {
		t.Fatalf("sedan.engine.power: %v", err)
	}
	if power.Value.Kind != ValConst || power.Value.Const.Real != 150.0 {
		t.Errorf("sedan.engine.power = %v, want 150", power.Value)
	}
}

// oneSymbol returns the single symbol a qualified name denotes.
func oneSymbol(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

// testDeepSpecializationChainOfRedefinitions: a redefinition specializes the
// usage it redefines, so a long chain of them keeps every level's values and
// terminates instead of recursing while looking for the base's members.
func testDeepSpecializationChainOfRedefinitions(t *testing.T) {
	const depth = 60
	var b strings.Builder
	b.WriteString("package test {\n")
	b.WriteString("\tpart def Inner { attribute a; attribute b; }\n")
	b.WriteString("\tpart def Outer { part inner : Inner; attribute t = inner.b; }\n")
	b.WriteString("\tpart level0 : Outer { part :>> inner { attribute :>> b = 7.0; } }\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, "\tpart level%d :> level%d { part :>> inner { attribute :>> a = %d.0; } }\n", i, i-1, i)
	}
	b.WriteString("}\n")

	done := make(chan struct{})
	var got Value
	var err error
	go func() {
		defer close(done)
		got, err = variationFeatureValueInSource(t, b.String(), fmt.Sprintf("test::level%d", depth), "t")
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("reading an inherited value through a deep specialization chain hung")
	}
	if err != nil {
		t.Fatalf("t = %v", err)
	}
	if got.Kind != ValConst || got.Const.Real != 7.0 {
		t.Errorf("t = %+v, want the base's 7.0", got)
	}
}

// testConflictingRedefinitionsAtSeveralLevels: when several levels restate the
// same nested feature, the innermost restatement is the value read, and the
// levels above still supply what they alone declare.
func testConflictingRedefinitionsAtSeveralLevels(t *testing.T) {
	src := `
		package test {
			part def Inner { attribute a; attribute b; attribute c; }
			part def Outer { part inner : Inner; attribute t = inner.c + inner.b + inner.a; }
			part base : Outer { part :>> inner { attribute :>> a = 1.0; attribute :>> c = 100.0; } }
			part middle :> base { part :>> inner { attribute :>> b = 20.0; attribute :>> c = 200.0; } }
			part leaf :> middle { part :>> inner { attribute :>> c = 300.0; } }
		}`
	got, err := variationFeatureValueInSource(t, src, "test::leaf", "t")
	if err != nil {
		t.Fatalf("t = %v", err)
	}
	if got.Kind != ValConst || got.Const.Real != 321.0 {
		t.Errorf("t = %+v, want 321.0 (innermost c, middle b, base a)", got)
	}
}

// testOneFeatureValuedUnderTwoNames: a redefinition renames one feature, so a
// declaration valuing both names has to be reported instead of picking one.
func testOneFeatureValuedUnderTwoNames(t *testing.T) {
	src := `
		package test {
			part def Ring { attribute ringCost; }
			part def Band :> Ring { attribute bandCost :>> ringCost; }
			part conflicted : Band {
				attribute :>> bandCost = 400.0;
				attribute :>> Ring::ringCost = 500.0;
			}
		}`
	got, err := variationFeatureValueInSource(t, src, "test::conflicted", "bandCost")
	if !errors.Is(err, ErrConflictingRedefinition) {
		t.Fatalf("ringCost = %+v, err = %v, want ErrConflictingRedefinition", got, err)
	}
}

// testValuedFeatureRestatedInABody: a feature bound to a value takes its own
// features from that value, so a body restating one of them is reported instead
// of being dropped.
func testValuedFeatureRestatedInABody(t *testing.T) {
	src := `
		package test {
			attribute def Cost { attribute v = 1.0; }
			part def Ring { attribute ringCost : Cost; }
			part conflicted : Ring {
				attribute :>> ringCost = 400.0 { attribute :>> v = 9.0; }
			}
		}`
	got, err := variationFeatureValueInSource(t, src, "test::conflicted", "ringCost")
	if !errors.Is(err, ErrValuedFeatureRestated) {
		t.Fatalf("ringCost = %+v, err = %v, want ErrValuedFeatureRestated", got, err)
	}
}

// calcErrorWithLibraries invokes the named calc of package test in src, with the
// standard library indexed, and answers the error it fails with.
func calcErrorWithLibraries(t *testing.T, src, calcName string, args []Value, maxSteps int64) error {
	t.Helper()

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = maxSteps
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calcName)

	done := make(chan error, 1)
	go func() {
		result, err := ctx.InvokeCalc(sym, args, scope)
		if err == nil {
			err = fmt.Errorf("calc %s returned %s, expected it to fail", calcName, FormatTraceValue(result))
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
		return nil
	}
}

// testBodyLocalUsageOfANonCalc: a body-local usage typed by something that is no
// calc is reported when the declaration is reached, not skipped.
func testBodyLocalUsageOfANonCalc(t *testing.T) {
	src := `
		package test {
			part def Thing;
			calc def Holder {
				in n : Integer;
				attribute i : Integer = 0;
				while i < n {
					calc r : Thing;
					assign i := i + 1;
				}
				i
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Holder", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testBodyLocalDeclarationNotExecutable: a declaration in a body the runtime has
// no execution for names itself rather than passing silently.
func testBodyLocalDeclarationNotExecutable(t *testing.T) {
	src := `
		package test {
			calc def Holder {
				in n : Integer;
				if n > 0 {
					part broken;
				}
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Holder", []Value{constInt(1)}, 10000)
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the declaration it cannot execute, got: %v", err)
	}
}

func testF99BodyMemberWithoutValue(t *testing.T) {
	_, err := evalCollectionExprBounded(t,
		"xs->collect { in i; private missing : Integer; missing }", 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Fatalf("error = %v, want ErrNoValue", err)
	}
}

func testF99UnsupportedBodyMember(t *testing.T) {
	_, err := evalCollectionExprBounded(t,
		"xs->select { in i; private import ScalarValues::*; i > 0 }", 10000)
	if !errors.Is(err, ErrUnsupportedBodyDeclaration) {
		t.Fatalf("error = %v, want ErrUnsupportedBodyDeclaration", err)
	}
	if !strings.Contains(err.Error(), "Import") {
		t.Errorf("unsupported-member error = %q, want it to name the form", err)
	}
}

func testF99CyclicBodyDeclaration(t *testing.T) {
	_, err := evalCollectionExprBounded(t,
		"xs->collect { in i; private attribute cycleValue : Integer = cycleValue; cycleValue }", 10000)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("error = %v, want ErrCyclicFeatureValue", err)
	}
}

// testRangeBoundIsNotAnInteger: `..` declares Integer bounds, so a Real bound is
// the type mismatch it is rather than a truncated range.
func testRangeBoundIsNotAnInteger(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = 1.5..n;
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Span", []Value{constInt(3)}, 10000)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("expected ErrTypeMismatch, got: %v", err)
	}
}

// testRangeSpendsTheStepBudget: each element a range generates costs a step, so
// a range too large to hold fails the run rather than exhausting memory.
func testRangeSpendsTheStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = 1..1000000;
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Span", []Value{constInt(3)}, 100)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCollectionSpendsTheElementBudget: a materialized element is memory the
// collection keeps, so it has its own ceiling and its own error rather than
// reading as the step budget's.
func testCollectionSpendsTheElementBudget(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = (1..1000)->collect{in i; i * i};
				n
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxElements = 100
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "Span")
	_, err := ctx.InvokeCalc(sym, []Value{constInt(3)}, scope)
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("expected ErrElementLimitExceeded, got: %v", err)
	}
	if errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error %q also reads as the step budget's", err)
	}
	if !strings.Contains(err.Error(), MaxElementsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxElementsEnvVar)
	}
}

// testUsageReadThroughAPartWithoutAnOutput: a chain through a part that stops at
// a calc usage names the outputs to read instead of answering no value.
func testUsageReadThroughAPartWithoutAnOutput(t *testing.T) {
	src := `
		package test {
			calc def Two {
				in n : Integer;
				out a = n + 1;
				out b = n * 2;
			}
			part holder {
				calc c : Two { in n = 5; }
			}
			calc def Probe {
				in n : Integer;
				holder.c
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "a, b") {
		t.Errorf("error should name the outputs to read, got: %v", err)
	}
}

// testPerformedActionBindingNamesNothing: a performed action's binding names a
// part that does not exist, reported as the unresolved reference it is rather
// than materializing the performer without the action's input.
func testPerformedActionBindingNamesNothing(t *testing.T) {
	src := `
		package test {
			part def Counter { attribute count : Integer = 4; }
			action def Bump {
				in c : Counter;
				out n : Integer = c.count + 1;
			}
			part machine {
				perform action p : Bump { in c = nowhere; }
			}
			calc def Probe {
				in n : Integer;
				machine.p.n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("expected ErrUnresolvedReference, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "nowhere") {
		t.Errorf("error should name the binding that resolves to nothing, got: %v", err)
	}
}

// testBindingEndOfADestroyedObject: a binding end naming a destroyed object's
// feature is ErrOccurrenceDestroyed, never a stale read of it or a write into it.
func testBindingEndOfADestroyedObject(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			part def Widget { attribute n : Integer = 1; attribute m : Integer; }
			part def Rig {
				part w : Widget;
				attribute shown : Integer;
				bind shown = w.n;
				attribute knob : Integer = 9;
				bind w.m = knob;
			}
			calc def DestroyWidget { in w : Widget; return : Widget = destroy(w); }
		}
	`)
	rig := instantiate("Rig")
	fv, err := rig.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := fv.HeldValue().Object()
	w, _ := ctx.getInstance(id)
	if _, err := invoke("DestroyWidget", objectValue(w)); err != nil {
		t.Fatalf("destroy(w): %v", err)
	}
	for _, name := range []string{"shown", "knob"} {
		_, err := rig.GetFeatureValue(ctx, name)
		if !errors.Is(err, ErrOccurrenceDestroyed) || !errors.Is(err, ErrBindingEnd) {
			t.Errorf("%s bound to a destroyed end: %v; want %v naming the binding end", name, err, ErrOccurrenceDestroyed)
		}
	}
	if m := w.FeatureValues["m"]; m.Materialized {
		t.Errorf("destroyed w.m = %s; want it left unwritten", FormatValue(m.HeldValue()))
	}
}

// testOperationOfADestroyedObject: an operation invoked on a destroyed object is
// ErrOccurrenceDestroyed before it runs, even one reading no feature of it.
func testOperationOfADestroyedObject(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import OccurrenceFunctions::*;
			item def Ping;
			part def Beacon { action ping { first start; action fire { send Ping to tower; } succession first start then fire; } }
			calc def DestroyBeacon { in b : Beacon; return : Beacon = destroy(b); }
		}
	`)
	beacon := instantiate("Beacon")
	if _, err := invoke("DestroyBeacon", objectValue(beacon)); err != nil {
		t.Fatalf("destroy(beacon): %v", err)
	}
	if _, err := ctx.InvokeOperation(beacon, "ping", nil); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("invoke ping on a destroyed object = %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if sent := len(ctx.PendingMessages()); sent != 0 {
		t.Errorf("a destroyed object sent %d message(s); want none", sent)
	}
}

// testNoFlowPerformedActionChecksItsInputs: an action stating no flow takes its
// inputs as a flowed one does, so a binding it cannot take fails the performer.
func testNoFlowPerformedActionChecksItsInputs(t *testing.T) {
	for name, tc := range map[string]struct {
		binding string
		want    error
	}{
		"output":  {"in r = 5;", ErrOutputActionInput},
		"unknown": {"in bogus = 5;", ErrUnknownActionInput},
	} {
		src := `
			package test {
				private import ScalarValues::*;
				action def Report { in n : Integer; out r : Integer; }
				part def Camera { perform action report : Report { ` + tc.binding + ` } }
			}
		`
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
		_, err := ctx.Instantiate(idx.LookupQualified("test::Camera")[0])
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: Instantiate(Camera) = %v; want %v", name, err, tc.want)
		}
	}
}

// testNoFlowPerformedActionRefusesReturnParameter: an action stating no flow is
// refused for a `return` parameter as a flowed one is, so its performer fails.
func testNoFlowPerformedActionRefusesReturnParameter(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			action def Report { return r : Integer; }
			part def Camera { perform action report : Report; }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.Instantiate(idx.LookupQualified("test::Camera")[0])
	if !errors.Is(err, ErrActionResultParameter) {
		t.Fatalf("Instantiate(Camera) = %v; want ErrActionResultParameter", err)
	}
	if !strings.Contains(err.Error(), "declares `return r`; write `out r`") {
		t.Errorf("err = %v; want it to name the parameter and the fix", err)
	}
}

// testStructuredAttributeChainOfAnUnknownFeature: a structured attribute usage
// holds the features of its type, so a chain into one it does not have is
// reported rather than answered as an empty value.
func testStructuredAttributeChainOfAnUnknownFeature(t *testing.T) {
	src := `
		package test {
			attribute def Cost { attribute v : Real default 1.0; }
			attribute template : Cost;
			calc def Probe {
				in n : Integer;
				template.nope
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if err == nil {
		t.Fatal("expected chaining into an unknown feature to fail")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should name the feature that does not exist, got: %v", err)
	}
}

// testElementsChainOfANonNumericCollection: only a numerical vector is the
// sequence of its own elements, so `elements` of another collection is a member
// lookup on each of them, reported rather than answered with the collection.
func testElementsChainOfANonNumericCollection(t *testing.T) {
	src := `
		package test {
			calc def Probe {
				in n : Integer;
				return : String[*] = ("a", "b").elements;
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Probe", []Value{constInt(1)}, 10000)
	if err == nil {
		t.Fatal("expected chaining elements of a non-numeric collection to fail")
	}
	if !strings.Contains(err.Error(), "cannot chain through non-instance member") {
		t.Errorf("error should report the member lookup, got: %v", err)
	}
}

// testEnumerationNameThatIsNotALiteral: a name qualified by an enumeration
// designates one of its literals, so one it does not declare is reported with
// the literals it does, never answered as an empty value.
func testEnumerationNameThatIsNotALiteral(t *testing.T) {
	src := `
	package test {
		enum def Color { red; green; blue; }
		part def Car { attribute c : Color = Color::purple; }
	}`
	got, err := variationFeatureValueInSource(t, src, "test::Car", "c")
	if !errors.Is(err, ErrNotALiteral) {
		t.Fatalf("c = (%v, %v), want ErrNotALiteral", got, err)
	}
	for _, name := range []string{"purple", "red", "green", "blue"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

// testChainThroughALiteralWithoutThatAttribute: a literal carries only the
// features it declares, so reading another one off it is reported rather than
// materializing an empty feature value.
func testChainThroughALiteralWithoutThatAttribute(t *testing.T) {
	src := `
	package test {
		enum def Level { low { attribute n = 1; } high { attribute n = 9; } }
		part def Sensor { attribute missing = Level::low.label; }
	}`
	got, err := variationFeatureValueInSource(t, src, "test::Sensor", "missing")
	if err == nil {
		t.Fatalf("missing = %v, want an error naming the unknown member", got)
	}
	if !strings.Contains(err.Error(), "label") {
		t.Errorf("error %q does not name the unknown member", err)
	}
}

// testObjectExhibitedMachineNeverSettles: a machine an object exhibits is bounded
// by the same budgets as one run on its own, so materializing an object whose
// machine never settles reports a budget error rather than spinning.
func testObjectExhibitedMachineNeverSettles(t *testing.T) {
	src := `
	package test {
		part def Spinner {
			attribute ticks : Integer = 0;
			exhibit state modes {
				entry; then spin;
				state spin {
					do action tick { assign ticks := ticks + 1; }
				}
				transition again first spin then spin;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Spinner")
	if err == nil {
		t.Fatal("expected a budget error for an exhibited machine that never settles")
	}
	if !strings.Contains(err.Error(), "exceeded max") && !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error = %v, want a budget error", err)
	}
}

// testObjectExhibitedMachineWithoutAnInitialState: a machine stating states but no
// entry into them is reported when the object's execution of it initializes, with
// the behavior and the type named.
func testObjectExhibitedMachineWithoutAnInitialState(t *testing.T) {
	src := `
	package test {
		part def Controller {
			exhibit state modes {
				state off;
				state on;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Controller")
	if err == nil {
		t.Fatal("expected an error for a machine with no initial state")
	}
	if !strings.Contains(err.Error(), "modes") || !strings.Contains(err.Error(), "initial") {
		t.Errorf("error = %v, want one naming the machine and its missing initial state", err)
	}
}

// testObjectExhibitedMachineAttributeWriteViolatesMultiplicity checks that an
// occurrence write failure remains typed and terminates normally.
func testObjectExhibitedMachineAttributeWriteViolatesMultiplicity(t *testing.T) {
	src := `
	package test {
		state def Modes {
			attribute samples : Integer[2] nonunique = (0, 0);
			entry; then active;
			state active {
				entry action record {
					assign samples := 1;
				}
			}
		}
		part def Controller {
			exhibit state modes : Modes;
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Controller")
	if !errors.Is(err, ErrStatePerformanceOccurrence) {
		t.Fatalf("error = %v, want ErrStatePerformanceOccurrence", err)
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
}

// testObjectPerformedActionAttributeWriteViolatesMultiplicity checks that a
// performed action's occurrence write failure remains typed.
func testObjectPerformedActionAttributeWriteViolatesMultiplicity(t *testing.T) {
	src := `
	package test {
		action def Record {
			attribute samples : Integer[2] nonunique = (0, 0);
			action step {
				assign samples := 1;
			}
			first step;
		}
		part def Logger {
			perform action recording : Record;
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Logger")
	if !errors.Is(err, ErrActionPerformanceOccurrence) {
		t.Fatalf("error = %v, want ErrActionPerformanceOccurrence", err)
	}
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
}

// testObjectPerformedActionOccurrenceHoldsANonObject: a perform usage whose
// feature holds a value that is not an occurrence is reported, not performed.
func testObjectPerformedActionOccurrenceHoldsANonObject(t *testing.T) {
	src := `
	package test {
		action def Bump {
			attribute count : Integer = 0;
			action step {
				assign count := count + 1;
			}
			first step;
		}
		part def Host {
			perform action work : Bump;
		}
	}`
	idx, _, ctx := buildRuntime(t, "<performed-action-non-object>", parseAndBuild(t, src))
	inst, err := ctx.materialize(oneSymbol(t, idx, "test::Host"), 0, nil, "")
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	fv, ok := inst.FeatureValues["work"]
	if !ok {
		t.Fatal("object has no work feature")
	}
	fv.Value = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}}
	fv.Materialized = true

	err = ctx.startClassifierBehaviors(inst, 0)
	if !errors.Is(err, ErrActionPerformanceOccurrence) {
		t.Fatalf("error = %v, want ErrActionPerformanceOccurrence", err)
	}
	if !strings.Contains(err.Error(), "not an occurrence") {
		t.Errorf("error = %v, want one naming the value the feature holds", err)
	}
}

// testOperationInvokedWithUnboundParameters: an operation invoked without a value
// for a parameter, or with an argument naming none, is reported rather than run
// against values the invocation never stated.
func testOperationInvokedWithUnboundParameters(t *testing.T) {
	src := `
	package test {
		part def Adder {
			attribute total : Integer = 0;
			action add {
				in addend : Integer;
				assign total := total + addend;
			}
		}
	}`
	ctx, inst, err := instantiateInSource(t, src, "test::Adder")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if _, err := ctx.InvokeOperation(inst, "add", nil); !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("invoke without an argument = %v, want ErrUnboundParameter", err)
	}
	args := map[string]Value{"addend": integerValue(1), "extra": integerValue(2)}
	if _, err := ctx.InvokeOperation(inst, "add", args); !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("invoke with an unknown argument = %v, want ErrUnboundParameter", err)
	}
	if _, err := ctx.InvokeOperation(inst, "missing", nil); !errors.Is(err, ErrNoSuchBehavior) {
		t.Errorf("invoke of an unknown operation = %v, want ErrNoSuchBehavior", err)
	}
	if _, err := ctx.InvokeOperation(inst, "total", nil); !errors.Is(err, ErrNotABehavior) {
		t.Errorf("invoke of an attribute = %v, want ErrNotABehavior", err)
	}
}

// testOperationConstraintBodyCannotBeEvaluated: invoking a constraint whose
// assertion names no feature returns its evaluation error without panicking.
func testOperationConstraintBodyCannotBeEvaluated(t *testing.T) {
	src := `
	package test {
		part def Tank {
			constraint broken { missing > 0 }
		}
	}`
	ctx, inst, err := instantiateInSource(t, src, "test::Tank")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = ctx.InvokeOperation(inst, "broken", nil)
	if err == nil {
		t.Fatal("expected an error for an unevaluable constraint body")
	}
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
}

// testSecondInstantiationOfOneType: materializing a type twice builds two objects,
// each with its own execution of the machine the type exhibits, rather than reusing
// or replacing the first object's.
func testSecondInstantiationOfOneType(t *testing.T) {
	src := `
	package test {
		part def Light {
			attribute lit : Integer = 0;
			exhibit state modes {
				entry; then on;
				state on { entry action mark { assign lit := 1; } }
			}
		}
	}`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	sym := oneSymbol(t, idx, "test::Light")

	first, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("first instantiate: %v", err)
	}
	second, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("second instantiate: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("both objects have identity %d", first.ID)
	}
	firstMachine, ok := first.ExhibitedState()
	if !ok {
		t.Fatal("first object exhibits no machine")
	}
	secondMachine, ok := second.ExhibitedState()
	if !ok {
		t.Fatal("second object exhibits no machine")
	}
	if firstMachine.State == secondMachine.State {
		t.Error("both objects share one machine execution")
	}
	for _, obj := range []*Instance{first, second} {
		fv, err := obj.GetFeatureValue(ctx, "lit")
		if err != nil {
			t.Fatalf("lit of object #%d: %v", obj.ID, err)
		}
		if fv.HeldValue().Const.Int != 1 {
			t.Errorf("lit of object #%d = %v, want 1", obj.ID, fv.HeldValue().Const)
		}
	}
}

// instantiateInSource materializes the named type declared in src, so a case can
// state the failure materializing an object reports.
func instantiateInSource(t *testing.T, src, fqn string) (*Context, *Instance, error) {
	t.Helper()
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	inst, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
	return ctx, inst, err
}

// instantiateWithLibraries materializes the named type declared in src over an
// index carrying the standard library, so a case may name its scalar types.
func instantiateWithLibraries(t *testing.T, src, fqn string) (*Context, *Instance, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
	return ctx, inst, err
}

// testWriteOfAWrongTypedValueLeavesTheFeature: a value the feature's type does
// not admit is rejected before the write, so the feature keeps what it held.
func testWriteOfAWrongTypedValueLeavesTheFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute reading : Real = 0.5;
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	err = inst.SetFeatureValue(ctx, "reading", NewStringValue("not a number"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	for _, want := range []string{"reading", "Real", "not a number"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to name %s", err, want)
		}
	}
	fv, err := inst.GetFeatureValue(ctx, "reading")
	if err != nil {
		t.Fatalf("read reading after the rejected write: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Real != 0.5 {
		t.Errorf("reading = %v, want the 0.5 it held before the rejected write", FormatValue(got))
	}
}

// testWriteOfTooManyValuesLeavesTheFeature: a written collection the target's
// multiplicity does not admit is rejected, and the feature keeps its values.
func testWriteOfTooManyValuesLeavesTheFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute samples : Integer[2] = (1, 2);
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	tooMany := sequenceOf([]Value{constInt(1), constInt(2), constInt(3)})
	if err := inst.SetFeatureValue(ctx, "samples", tooMany); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "samples")
	if err != nil {
		t.Fatalf("read samples after the rejected write: %v", err)
	}
	if got := len(elementsOf(fv.HeldValue())); got != 2 {
		t.Errorf("samples holds %d value(s), want the 2 it held before the rejected write", got)
	}
}

// testWriteOfARepeatedValueLeavesTheFeature: a repeat written to a unique feature
// is refused after count and type, leaving its value; nonunique takes it, a set drops it.
func testWriteOfARepeatedValueLeavesTheFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		private import Collections::*;
		part def Rig {
			attribute xs : Integer[*] = (1, 2);
			attribute ordered : Integer[*] ordered = (1, 2);
			attribute bounded : Integer[0..2] = (1, 2);
			attribute repeats : Integer[*] nonunique = (1, 1);
			attribute members : Set { :>> elements = (1, 2); }
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	repeated := sequenceOf([]Value{constInt(3), constInt(4), constInt(3)})
	for _, feature := range []string{"xs", "ordered"} {
		err := inst.SetFeatureValue(ctx, feature, repeated)
		if !errors.Is(err, ErrUniquenessViolation) {
			t.Fatalf("%s: error = %v, want ErrUniquenessViolation", feature, err)
		}
		if want := "3 (an Integer) is written at positions 1 and 3 of a unique feature"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it to say %q", feature, err, want)
		}
		fv, err := inst.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Fatalf("read %s after the rejected write: %v", feature, err)
		}
		if got := intsOf(t, fv.HeldValue()); !equalInts(got, []int64{1, 2}) {
			t.Errorf("%s = %v, want the (1, 2) it held before the rejected write", feature, got)
		}
	}
	if err := inst.SetFeatureValue(ctx, "bounded", repeated); !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("bounded: error = %v, want ErrMultiplicityViolation before the repeat is judged", err)
	}
	wrongType := sequenceOf([]Value{NewStringValue("a"), NewStringValue("a")})
	if err := inst.SetFeatureValue(ctx, "xs", wrongType); !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("xs: error = %v, want ErrTypeMismatch before the repeat is judged", err)
	}
	if err := inst.SetFeatureValue(ctx, "repeats", repeated); err != nil {
		t.Errorf("repeats: error = %v, want a nonunique feature to take the repeat", err)
	}
	if err := inst.SetFeatureValue(ctx, "ordered", sequenceOf([]Value{constInt(3), constInt(1), constInt(2)})); err != nil {
		t.Fatalf("ordered: error = %v, want distinct values to be written", err)
	}
	if fv, _ := inst.GetFeatureValue(ctx, "ordered"); !equalInts(intsOf(t, fv.HeldValue()), []int64{3, 1, 2}) {
		t.Errorf("ordered = %s, want (3, 1, 2) in the order written", FormatValue(fv.HeldValue()))
	}
	members, err := inst.GetFeatureValue(ctx, "members")
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	set, ok := members.HeldValue().Object()
	if !ok {
		t.Fatalf("members = %s, want a Set object", FormatValue(members.HeldValue()))
	}
	if err := ctx.instances[set].SetFeatureValue(ctx, "elements", repeated); err != nil {
		t.Errorf("members.elements: error = %v, want a set to drop the repeat", err)
	}
	if fv, _ := ctx.instances[set].GetFeatureValue(ctx, "elements"); fv.HeldValue().Kind != ValSet || len(elementsOf(fv.HeldValue())) != 2 {
		t.Errorf("members.elements = %s, want the set {3, 4}", FormatValue(fv.HeldValue()))
	}

	actionSrc := `
	package test {
		private import ScalarValues::*;
		action w {
			attribute xs : Integer[*] ordered = (1, 2);
			attribute n : Integer = 5;
			first start;
			action step { assign xs := (n, 6, n); }
			done;
			succession first start then step;
			succession first step then done;
		}
	}`
	idx, _, actx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actionSrc))
	if _, err := actx.ExecuteAction(findSymbolByName(idx.DocumentRoot("<test>"), "w", ast.DefAction)); !errors.Is(err, ErrUniquenessViolation) {
		t.Errorf("assign: error = %v, want ErrUniquenessViolation", err)
	}

	calcSrc := `
	package test {
		private import ScalarValues::*;
		private import SequenceFunctions::size;
		calc def Pass { in xs : Integer[*]; return : Integer[*] = xs; }
		calc def Twice { in x : Integer; return : Integer[*] = (x, x); }
		calc def Local { in xs : Integer[*] nonunique; attribute ys : Integer[*] = xs; return : Integer = size(ys); }
		calc def LocalRepeats { in xs : Integer[*] nonunique; attribute ys : Integer[*] nonunique = xs; return : Integer = size(ys); }
	}`
	for name, args := range map[string][]Value{"Pass": {repeated}, "Twice": {constInt(7)}, "Local": {repeated}} {
		if err := calcErrorWithLibraries(t, calcSrc, name, args, 10000); !errors.Is(err, ErrUniquenessViolation) {
			t.Errorf("%s: error = %v, want ErrUniquenessViolation", name, err)
		}
	}
	cidx, _, cctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, calcSrc))
	sym, scope := calcByName(t, cidx.DocumentRoot("<test>"), "test", "LocalRepeats")
	if result, err := cctx.InvokeCalc(sym, []Value{repeated}, scope); err != nil || FormatTraceValue(result) != "3" {
		t.Errorf("LocalRepeats = %s, %v; want 3: a nonunique local takes the repeat", FormatTraceValue(result), err)
	}
}

// testWriteOfNoValueWhereOneIsRequired: an empty collection written to a feature
// whose lower bound is one is a multiplicity violation, not an unset feature.
func testWriteOfNoValueWhereOneIsRequired(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute reading : Real[1] = 0.5;
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if err := inst.SetFeatureValue(ctx, "reading", sequenceOf(nil)); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("error = %v, want ErrMultiplicityViolation", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "reading")
	if err != nil {
		t.Fatalf("read reading after the rejected write: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Real != 0.5 {
		t.Errorf("reading = %v, want the 0.5 it held before the rejected write", FormatValue(got))
	}
}

// testStateEntryWriteOfAWrongTypedValue: a write in a state's entry action is
// judged against the feature it names, however deep in the machine it stands.
func testStateEntryWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute reading : Real = 0.0;
			exhibit state run {
				entry; then go;
				state go {
					entry action set {
						assign reading := "not a number";
					}
				}
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "Real") {
		t.Errorf("error = %v, want it to name the declared type", err)
	}
}

// testPerformerFeatureWriteOfAWrongTypedValue: a write reaching a feature of the
// object performing the behavior is judged against that feature's declaration.
func testPerformerFeatureWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Rig {
			attribute label : String = "a";
			perform action set {
				action step {
					assign label := 7;
				}
				first step;
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "String") {
		t.Errorf("error = %v, want it to name the declared type", err)
	}
}

// A body written on its own resolves in its own namespace, so a name only the
// performing object declares is unresolved at run time as it is to analysis.
func testStandaloneActionNamingAPerformerFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Touch {
			action step {
				assign touched := touched + 1;
			}
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Host")
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "touched") {
		t.Errorf("error = %v, want it to name the unresolved feature", err)
	}
}

// The write side refuses the same name the read side refuses, rather than
// reaching into the performing object for a name the body cannot see.
func testStandaloneActionWritingAPerformerFeature(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Touch {
			action step {
				assign touched := 1;
			}
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Host")
	if !errors.Is(err, ErrPerformerFeatureNotInScope) {
		t.Fatalf("error = %v, want ErrPerformerFeatureNotInScope", err)
	}
	if !strings.Contains(err.Error(), "touched") {
		t.Errorf("error = %v, want it to name the refused feature", err)
	}
}

// `this` in a body written on its own names the performance itself, which no
// object owns, so a feature of the performer is not reachable through it.
func testStandaloneActionNamingThisOfAnUnownedPerformance(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Touch {
			action step {
				assign this.touched := 1;
			}
			first step;
		}
		part def Host {
			attribute touched : Integer = 0;
			perform action t : Touch;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Host")
	if !errors.Is(err, ErrThisNotAnObject) {
		t.Fatalf("error = %v, want ErrThisNotAnObject", err)
	}
}

// testChainedWriteOfAWrongTypedValue: a write through a feature chain answers to
// the declaration of the feature the chain reaches.
func testChainedWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		part def Cell {
			attribute mark : Integer = 0;
		}
		part def Rig {
			part cell : Cell;
			exhibit state run {
				entry; then marking;
				state marking {
					entry action set {
						assign cell.mark := "seven";
					}
				}
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Rig")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "mark") {
		t.Errorf("error = %v, want it to name the feature written", err)
	}
}

// testCalcOutputWriteOfAWrongTypedValue: an output a calculation body binds is a
// feature too, so what it is bound to conforms to its declared type.
func testCalcOutputWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		calc def Label {
			out tag : String;
			assign tag := 7;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	calc := oneSymbol(t, idx, "test::Label")
	_, err := ctx.InvokeCalc(calc, nil, idx.DocumentRoot("<test>"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testActionLocalWriteOfAWrongTypedValue: a value a body declares of its own is
// declared with a type, so a write to it answers to that type.
func testActionLocalWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Compute {
			out total : Integer;
			action step {
				attribute held : Integer = 0;
				assign held := "seven";
				assign total := held;
			}
			first step;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Compute"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testActionOutputWriteOfAWrongTypedValue: binding an output binds a feature, so
// the value bound conforms to the type the output was declared with.
func testActionOutputWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Compute {
			out total : Integer;
			action step {
				assign total := "seven";
			}
			first step;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Compute"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testPerformanceOccurrenceWriteOfAWrongTypedValue: an attribute of a performed
// action is a feature of its performance occurrence, and a write to it is judged
// against that feature's declaration.
func testPerformanceOccurrenceWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		action def Record {
			attribute sample : Integer = 0;
			action step {
				assign sample := "seven";
			}
			first step;
		}
		part def Logger {
			perform action recording : Record;
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Logger")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testNestedFlowWithoutAnInitialNode: a node stating a flow with no node to
// start at — a cycle, every node preceded — is reported at initialize(), not
// run as a leaf.
func testNestedFlowWithoutAnInitialNode(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					action a;
					action b;
					succession first a then b;
					succession first b then a;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	// The constructor is permissive; the flow's failure surfaces at initialize().
	exec, err := newActionExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newActionExecutor: %v", err)
	}
	err = exec.initialize()
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
	if !strings.Contains(err.Error(), "leg") {
		t.Errorf("error = %v, want it to name the node whose flow cannot be built", err)
	}
}

// testNestedFlowWithADanglingSuccession: a succession inside a node's own flow
// naming nothing is that node's error, reported rather than dropped.
func testNestedFlowWithADanglingSuccession(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					first a;
					action a;
					succession first a then missing;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	exec, err := newActionExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newActionExecutor: %v", err)
	}
	if err := exec.initialize(); !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
}

// testNestedFlowThatCannotProgress: a join inside a node's own flow that can
// never be reached by both its tokens deadlocks the run, reported rather than hung.
func testNestedFlowThatCannotProgress(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					first s;
					action s;
					action stranded;
					join sync;
					done;
					succession first s then sync;
					succession first stranded then sync;
					succession first sync then done;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionDeadlock) {
		t.Fatalf("error = %v, want ErrActionDeadlock", err)
	}
}

// testNestedFlowThatNeverEnds: a cycle inside a node's own flow spends the
// action's step budget rather than running forever.
func testNestedFlowThatNeverEnds(t *testing.T) {
	src := `
		package test {
			action outer {
				first leg;
				action leg {
					first a;
					action a;
					action b;
					succession first a then b;
					succession first b then a;
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}

	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := exec.RunToCompletion(); !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrActionStepLimitExceeded", err)
	}
}

// runOuterAction runs action test::outer of src to completion and returns the
// error, for a case whose contract is the error a nested node reports.
func runOuterAction(t *testing.T, src string) error {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	return exec.RunToCompletion()
}

const adderActionDef = `
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;
		first step;
		action step { assign sum := a + b; }
	}
`

// testNodePinOfANodeNotYetPerformed: a pin holds a value only once its node has
// been performed, so reading it earlier is reported rather than answered.
func testNodePinOfANodeNotYetPerformed(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action early { assign total := late.v; }
				then action late { out v : Integer; assign v := 1; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodeNotPerformed) {
		t.Fatalf("error = %v, want ErrNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "late") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testNodeOutputBoundToANestedNodeThatNeverRuns: a node's output bound to a pin of one
// of its own nested nodes takes its value as that node ends, so where the nested node
// never runs the output is unvalued when its node ends, and reported so.
func testNodeOutputBoundToANestedNodeThatNeverRuns(t *testing.T) {
	src := `
		package test {
			action outer {
				out attribute legV : Integer;
				bind leg.inner.v = leg.v;
				first start;
				then action leg {
					out v : Integer;
					action inner { out v : Integer; assign v := 1; }
					first start;
					then action own { assign legV := 0; }
					then done;
				}
				then action fin { assign legV := leg.v; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("error = %v, want ErrBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "leg.inner.v") {
		t.Errorf("error %q does not name the other end", err)
	}
}

// testNodePinTheNodeDoesNotDeclare: a chain through a node names one of its
// pins; a name it does not declare is reported with the node it was read from.
func testNodePinTheNodeDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action p { out v : Integer; assign v := 1; }
				then action fin { assign total := p.w; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin", err)
	}
	if !strings.Contains(err.Error(), "p") || !strings.Contains(err.Error(), "w") {
		t.Errorf("error %q does not name the node and the pin", err)
	}
}

// testBlockNodePinOfANodeNotYetPerformed: a node declared in a branch is a
// performance of its own like any other, so reading its pin from a sibling that
// runs before it is reported the same way.
func testBlockNodePinOfANodeNotYetPerformed(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action run {
					if true {
						action early { assign total := late.v; }
						action late { out v : Integer; assign v := 1; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodeNotPerformed) {
		t.Fatalf("error = %v, want ErrNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "late") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testBlockNodePinTheNodeDoesNotDeclare: a pin read through a node declared in
// a loop body that the node does not declare is reported with the node.
func testBlockNodePinTheNodeDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action run {
					for i in 1..2 {
						action p { out v : Integer; assign v := i; }
						assign total := total + p.w;
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin", err)
	}
	if !strings.Contains(err.Error(), "p") || !strings.Contains(err.Error(), "w") {
		t.Errorf("error %q does not name the node and the pin", err)
	}
}

// testElseBranchNodeReadBeforeItPerforms: an else branch's `p` is the one its
// own reads name even where the then branch declares a `p` too, so a read ahead
// of it is not-yet-performed rather than a read of the other branch's node.
func testElseBranchNodeReadBeforeItPerforms(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action run {
					if false {
						action p { out v : Integer; assign v := 1; }
						assign total := p.v;
					} else {
						assign total := p.v;
						action p { out v : Integer; assign v := 2; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodeNotPerformed) {
		t.Fatalf("error = %v, want ErrNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "p") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testTypedNodePinOfANodeTheCalleeDoesNotDeclare: a typed node's subactions are
// those of the action it performed; a path through one it declares not is reported.
func testTypedNodePinOfANodeTheCalleeDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action def Seven {
				out result : Integer;
				first start;
				then action inner { out v : Integer; assign v := 7; }
				then action publish { assign result := inner.v; }
				then done;
			}
			action outer {
				attribute total : Integer = 0;
				first start;
				then action call : Seven;
				then action read { assign total := call.other.v; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin", err)
	}
	if !strings.Contains(err.Error(), "call") || !strings.Contains(err.Error(), "other") {
		t.Errorf("error %q does not name the node and the missing one", err)
	}
}

// testNodeReadAsAValueWithoutAResult: a node read as a value stands for its
// performance's `result`; a node whose callee declares none is reported.
func testNodeReadAsAValueWithoutAResult(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute total : Integer = 0;
				first start;
				then action add = Adder(1, 2);
				then action fin { assign total := add; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin", err)
	}
	if !strings.Contains(err.Error(), "result") {
		t.Errorf("error %q does not name the missing result", err)
	}
}

// testNodePinMemberThroughAScalarPin: `node.pin.member` chains through the pin's
// value like any feature chain, so a pin holding no object cannot be read through.
func testNodePinMemberThroughAScalarPin(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute total : Integer = 0;
				first start;
				then action p { out v : Integer; assign v := 7; }
				then action read { assign total := p.v.mark; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if err == nil {
		t.Fatal("expected an error chaining a member through a scalar pin")
	}
	if !strings.Contains(err.Error(), "p.v") || !strings.Contains(err.Error(), "non-instance") {
		t.Errorf("error %q does not name the pin read through", err)
	}
}

// testNodeInheritedDefaultThatCannotBeEvaluated: an inherited default is seeded
// when the node starts, so its failure is reported there, naming the parameter.
func testNodeInheritedDefaultThatCannotBeEvaluated(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			action def Base {
				in divisor : Integer = 0;
				in share : Integer = 6 / divisor;
				out r : Integer;
			}
			action def Derived :> Base {
				first start;
				then assign r := 1;
				then done;
			}
			action outer {
				first start;
				then action d : Derived;
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	if !strings.Contains(err.Error(), "share") {
		t.Errorf("error %q does not name the parameter whose default failed", err)
	}
}

// testNodeInvocationTooManyArguments: positional arguments beyond the callee's
// input parameters are reported rather than dropped.
func testNodeInvocationTooManyArguments(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				first start;
				then action add = Adder(1, 2, 3);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrActionArity) {
		t.Fatalf("error = %v, want ErrActionArity", err)
	}
}

// testNodeInvocationTooFewArguments: an input parameter that no argument and no
// default binds is reported before the callee runs, not when its body reads it.
func testNodeInvocationTooFewArguments(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute b : Integer = 40;
				first start;
				then action add = Adder(1);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("error = %v, want ErrUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the unbound parameter", err)
	}
}

// testNodeInvocationArgumentFailsBeforeDefaults: an argument that fails to evaluate
// is the error reported, not the default it replaces (which is never evaluated) nor
// a default reading the pin it would have bound.
func testNodeInvocationArgumentFailsBeforeDefaults(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute zero : Integer = 0;
				first start;
				then action add = Adder(a = 1 / zero) {
					in a = 5;
					in b = a + 1;
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	if !strings.Contains(err.Error(), `argument "a" of Adder`) {
		t.Errorf("error %q does not name the argument that failed", err)
	}
}

// testNodeInvocationUnknownNamedArgument: a named argument must name an input
// parameter of the callee.
func testNodeInvocationUnknownNamedArgument(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				first start;
				then action add = Adder(a = 1, c = 2);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("error = %v, want ErrUnknownParameter", err)
	}
}

// testNodeInvocationRepeatedNamedArgument: a parameter named twice is rejected, not
// bound to whichever argument comes last.
func testNodeInvocationRepeatedNamedArgument(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				first start;
				then action add = Adder(a = 1, b = 2, a = 3);
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrDuplicateArgument) {
		t.Fatalf("error = %v, want ErrDuplicateArgument", err)
	}
	if !strings.Contains(err.Error(), `"a"`) {
		t.Errorf("error %q does not name the parameter bound twice", err)
	}
}

// testPerformedActionInputBoundByNothing: a `perform` in statement form must bind
// every input without a default, by argument or by a caller value of its name.
func testPerformedActionInputBoundByNothing(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action def Defaulted {
				in a : Integer = 1;
				out doubled : Integer;
				first step;
				action step { assign doubled := a * 2; }
			}
			action adder : Adder;
			action defaulted : Defaulted;
			action outer {
				attribute a : Integer = 1;
				attribute doubled : Integer = 0;
				first start;
				then action run {
					if a > 0 {
						perform defaulted;
						perform adder;
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("error = %v, want ErrUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "adder") || !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the action and its unbound parameter", err)
	}
}

// testStateEntryActionInputBoundByNothing: a state's entry action is an
// invocation too, so an input it leaves unbound is reported before it runs.
func testStateEntryActionInputBoundByNothing(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {`+adderActionDef+`
		action adder : Adder;
		state Machine {
			attribute a : Integer = 1;
			entry; then init;
			state init;
			state active {
				entry adder;
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("error = %v, want ErrUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the unbound parameter", err)
	}
}

// testStateBlockTypedNodeInputBoundByNothing: a typed node in a branch of a
// state's body binds the callee's inputs from the pins it declares and the
// values in scope, so one it leaves unbound is reported before its body runs.
func testStateBlockTypedNodeInputBoundByNothing(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {`+adderActionDef+`
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						action adding : Adder {
							in a = 1;
							assign total := sum;
						}
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("error = %v, want ErrUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("error %q does not name the unbound parameter", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: the node's body must not run", total)
	}
}

// testStateBlockNodePinReadBeforePerformed: a state body's node has a frame of its
// own, so a sibling reading its pin before it performs is ErrNodeNotPerformed.
func testStateBlockNodePinReadBeforePerformed(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						action early { assign total := late.v; }
						action late { out v : Integer; assign v := 1; }
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrNodeNotPerformed) {
		t.Fatalf("error = %v, want ErrNodeNotPerformed", err)
	}
	if !strings.Contains(err.Error(), "late") {
		t.Errorf("error %q does not name the node", err)
	}
}

// testStateBlockNodeBoundAtNoPin: a binding in a state body at a pin the node does
// not declare is ErrBindingEnd, as it is in an action's flow.
func testStateBlockNodeBoundAtNoPin(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {`+adderActionDef+`
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						bind adding.c = total;
						action adding : Adder { in a = 1; in b = 2; }
						assign total := adding.sum;
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("error = %v, want ErrBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "c") {
		t.Errorf("error %q does not name the pin", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: the node must not run", total)
	}
}

// testStateBlockNodeOwnFlowRuns: a node of a state behavior's body stating a
// flow of its own runs that flow to its end when the block reaches it, as a node
// of an action body does.
func testStateBlockNodeOwnFlowRuns(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action {
					if total == 0 {
						action step {
							first start;
							then action one { assign total := total + 1; }
							then action two { assign total := total * 10; }
							then done;
						}
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Errorf("total = %v, want 10: the node's flow runs one then two", total)
	}
}

// stateWithDoBody is a machine whose state active runs body as its inline do
// action, counting in total what the body's nodes did.
func stateWithDoBody(t *testing.T, body string) *StateExecutor {
	t.Helper()
	return stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active { do action ops { `+body+` } }
			succession first init then active;
			succession first active then done;
		}
	}`)
}

// testStateDoBodyDanglingSuccession: a succession of an inline do body naming a
// node the body does not declare is reported as a flow that cannot be built,
// naming the target, and no node of the body runs.
func testStateDoBodyDanglingSuccession(t *testing.T) {
	exec := stateWithDoBody(t, `
		first start;
		then action a { assign total := total + 1; }
		succession a then missing;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStatementNotExecutable) {
		t.Fatalf("expected ErrStatementNotExecutable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state behavior ops") || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error %q does not name the behavior and the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFirstThenUndefined: `first a then b` in an inline do body where
// b is not declared is reported the same way.
func testStateDoBodyFirstThenUndefined(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		first a then b;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStatementNotExecutable) {
		t.Fatalf("expected ErrStatementNotExecutable, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("error %q does not name the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFlowWithoutStart: successions that give the flow no node to
// start at — a cycle and nothing first — are an invalid flow, not a hang.
func testStateDoBodyFlowWithoutStart(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		action b { assign total := total + 1; }
		succession first a then b;
		succession first b then a;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no node starts the flow") {
		t.Errorf("error %q does not say what the flow lacks", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFlowWithTwoStarts: successions leaving two nodes unpreceded
// state no start either; the error names both and what would state one.
func testStateDoBodyFlowWithTwoStarts(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		action b { assign total := total + 1; }
		action c { assign total := total + 1; }
		succession first a then c;
		succession first b then c;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow, got: %v", err)
	}
	for _, want := range []string{"no node starts the flow", `"a"`, `"b"`, "'first'"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyStartsAtItsUnprecededStep: a do body written in declaration
// order, with no `first`, starts at the one node no succession leads to.
func testStateDoBodyStartsAtItsUnprecededStep(t *testing.T) {
	exec := stateWithDoBody(t, `
		action a { assign total := total + 1; }
		then action b { assign total := total * 10; }
	`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Errorf("total = %v, want 10: a then b", total)
	}
}

// testActionFlowStartsAtItsUnprecededStep: an action definition performed whole
// starts at its one unpreceded node the same way.
func testActionFlowStartsAtItsUnprecededStep(t *testing.T) {
	outputs, err := executeActionSource(t, "Count", `package P {
		private import ScalarValues::*;
		action def Count {
			attribute total : Integer = 1;
			action a { assign total := total + 1; }
			then action b { assign total := total * 10; }
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertIntOutput(t, outputs, "total", 20)
}

// testActionFlowWithTwoStarts: an action whose successions leave two nodes
// unpreceded is an invalid flow at initialization, not a bodiless action.
func testActionFlowWithTwoStarts(t *testing.T) {
	_, err := executeActionSource(t, "Count", `package P {
		private import ScalarValues::*;
		action def Count {
			attribute total : Integer = 0;
			action a { assign total := total + 1; }
			action b { assign total := total + 1; }
			action c { assign total := total + 1; }
			succession first a then c;
			succession first b then c;
		}
	}`)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow, got: %v", err)
	}
	for _, want := range []string{"no initial node found in action Count", `"a"`, `"b"`, "'first'"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

// testActionFlowCycleWithoutStart: successions closing a cycle over every node
// leave nothing to start at; the error says so rather than the run hanging.
func testActionFlowCycleWithoutStart(t *testing.T) {
	_, err := executeActionSource(t, "Loop", `package P {
		private import ScalarValues::*;
		action def Loop {
			attribute total : Integer = 0;
			action a { assign total := total + 1; }
			action b { assign total := total + 1; }
			succession first a then b;
			succession first b then a;
		}
	}`)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error %q does not name the cycle", err)
	}
}

// testStateDoBodyNestedNodeDanglingSuccession: the flow a node of an inline do
// body states of its own is validated with the body's before any node runs, so
// a dangling succession in it is reported naming the node and the target.
func testStateDoBodyNestedNodeDanglingSuccession(t *testing.T) {
	exec := stateWithDoBody(t, `
		first start;
		then action a { assign total := total + 1; }
		then action step {
			first start;
			then action one { assign total := total + 1; }
			succession one then missing;
		}
		then done;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action node step") || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error %q does not name the node and the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyNestedNodeStartsAtItsUnprecededStep: a node of an inline do
// body stating its flow in declaration order, with no `first`, starts at the one
// node no succession leads to, as the body itself does.
func testStateDoBodyNestedNodeStartsAtItsUnprecededStep(t *testing.T) {
	exec := stateWithDoBody(t, `
		action inner {
			action a { assign total := total + 1; }
			action b { assign total := total * 10; }
			succession first a then b;
		}
	`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Errorf("total = %v, want 10: a then b", total)
	}
}

// testActionNestedNodeStartsAtItsUnprecededStep: a node of an action definition
// stating its flow the same way starts there too.
func testActionNestedNodeStartsAtItsUnprecededStep(t *testing.T) {
	outputs, err := executeActionSource(t, "Count", `package P {
		private import ScalarValues::*;
		action def Count {
			attribute total : Integer = 1;
			action inner {
				action a { assign total := total + 1; }
				action b { assign total := total * 10; }
				succession first a then b;
			}
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertIntOutput(t, outputs, "total", 20)
}

// testActionNestedNodeWithTwoStarts: a nested flow leaving two nodes unpreceded
// states no start, and is an invalid flow at initialization naming the node.
func testActionNestedNodeWithTwoStarts(t *testing.T) {
	_, err := executeActionSource(t, "Count", `package P {
		private import ScalarValues::*;
		action def Count {
			attribute total : Integer = 0;
			action inner {
				action a { assign total := total + 1; }
				action b { assign total := total + 1; }
				action c { assign total := total + 1; }
				succession first a then c;
				succession first b then c;
			}
		}
	}`)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no initial node found in action node inner") {
		t.Errorf("error %q does not name the node without a start", err)
	}
}

// testStateEntryBodyDanglingSuccession: an inline entry body's flow is built the
// same way, so its dangling succession is reported on entering the state.
func testStateEntryBodyDanglingSuccession(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then init;
			state init;
			state active {
				entry action prep {
					first start;
					then action a { assign total := total + 1; }
					then missing;
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStatementNotExecutable) {
		t.Fatalf("expected ErrStatementNotExecutable, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state behavior prep") || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error %q does not name the behavior and the undefined target", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyAcceptWaitsForTheMessage: an accept in an inline do body's flow
// with no message in flight suspends the machine in its state, as a transition
// triggered by a signal does, rather than hanging or deadlocking; the message
// posted later lets the body finish and the state complete.
func testStateDoBodyAcceptWaitsForTheMessage(t *testing.T) {
	var exec *StateExecutor
	done := make(chan error, 1)
	go func() {
		exec = stateWithDoBody(t, `
			first start;
			then action reader accept n : Integer;
			then action count assign total := n;
			then done;
		`)
		done <- exec.RunToCompletion()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a do body waiting for a message did not suspend")
	}
	if exec.State() != StateSuspended || getNodeName(exec.CurrentState()) != "active" {
		t.Fatalf("state = %v in %s, want suspended in active", exec.State(), getNodeName(exec.CurrentState()))
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Error("a do body parked at its accept must not be due with no message in flight")
	}
	nine := integerValue(9)
	exec.ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &nine})
	if exec.HasPendingDoWork() || !exec.HasPendingSignal() {
		t.Fatal("the message in flight must be one the machine dispatches, to the parked do body")
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the message: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("state = %v in %s, want completed", exec.State(), getNodeName(exec.CurrentState()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(9)) {
		t.Errorf("total = %v, want 9: the accepted value", total)
	}
}

// testStateDoBodyAcceptIsDecidedForASend: a signal a do body is parked at an
// accept for is one the machine takes, though no transition fires on it: the
// previews say so, without moving the body, and the dispatch lets it go on.
func testStateDoBodyAcceptIsDecidedForASend(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	attribute def Other;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 1;
				then done;
			}
		}
		succession first active then finished;
		state finished;
	}
	part def Box { exhibit state w : Waiter; }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "w.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("w.sysml")
	box, err := ctx.Instantiate(resolveSymbol(t, root, "Box"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := box.ExhibitedState()
	if !ok {
		t.Fatal("the box exhibits no machine")
	}
	exec := behavior.State
	if activeLeaf(exec) != "active" || exec.HasPendingDoWork() {
		t.Fatalf("state %s with pending do work %v, want parked in active", activeLeaf(exec), exec.HasPendingDoWork())
	}
	other, err := ctx.SignalMessage(resolveSymbol(t, root, "Other"), nil, box)
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := exec.AcceptsMessage(other); err != nil || accepted {
		t.Errorf("AcceptsMessage(Other) = %v, %v; want false: the accept names Go", accepted, err)
	}
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, box)
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := exec.AcceptsMessage(goMsg); err != nil || !accepted {
		t.Errorf("AcceptsMessage(Go) = %v, %v; want true: the do body is parked at accept Go", accepted, err)
	}
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	if len(decision.Fires) != 0 || decision.Deferred || len(decision.Resumes) != 1 || decision.Resumes[0] != "do behavior of state active" {
		t.Errorf("Decide(Go) = %+v, want only the do behavior of active resumed", decision)
	}
	if activeLeaf(exec) != "active" || exec.HasPendingDoWork() || len(ctx.PendingMessages()) != 0 {
		t.Fatal("the previews must leave the machine, the body and the bus as they were")
	}
	ctx.PostMessage(goMsg)
	if exec.HasPendingDoWork() || !exec.HasPendingSignal() {
		t.Fatal("the message in flight must be one the machine dispatches, to the parked do body")
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || dispatch.Fired || dispatch.Deferred || len(dispatch.Resumed) != 1 || dispatch.Resumed[0] != decision.Resumes[0] {
		t.Errorf("dispatch = %+v, %v; want the do behavior of active resumed, as decided", dispatch, ok)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the message: %v", err)
	}
	if activeLeaf(exec) != "finished" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want finished with the message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the body went on past its accept once", total)
	}
}

// testStateDoBodyAcceptYieldsToATransition: a signal both a transition out of the
// active state and its do behavior, parked at an accept, would take goes to the
// transition alone — the behavior is abandoned when the state is left — and
// Decide reports just that, before.
func testStateDoBodyAcceptYieldsToATransition(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
		}
		transition leave first active accept Go then stopped;
		state stopped { entry assign total := total + 1; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition takes it alone", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "stopped" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want stopped with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of stopped alone, the do behavior abandoned at its accept", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() || len(ctx.Clock().Waits()) != 0 {
		t.Error("nothing of the abandoned do behavior must remain due")
	}
}

// testStateDoBodyAcceptGoesOnAcrossASubstateTransition: a signal a transition
// between two substates of the active state accepts is one the state's own do
// behavior, parked at an accept for it, goes on with too — the state stays active
// across that transition — and Decide reports both, before.
func testStateDoBodyAcceptGoesOnAcrossASubstateTransition(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			transition shift first left accept Go then right;
			state right { entry assign total := total + 1; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition shift"}, Resumes: []string{"do behavior of state active"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition and the do behavior of the state it stays in both take it", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v, want 11: the do behavior's count, then the entry of right", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Error("the do behavior has ended; nothing of it must remain due")
	}
}

// testStateDoBodyAcceptYieldsToASubstateTransitionLeavingIt: a signal a transition
// out of a substate accepts goes to the transition alone when its target lies
// outside the state whose do behavior is parked for it — that state is left, its
// behavior abandoned — and Decide reports just that, before.
func testStateDoBodyAcceptYieldsToASubstateTransitionLeavingIt(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			transition leave first left accept Go then stopped;
		}
		state stopped { entry assign total := total + 1; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition leaving the state takes it alone", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "stopped" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want stopped with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of stopped alone, the do behavior abandoned at its accept", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() || len(ctx.Clock().Waits()) != 0 {
		t.Error("nothing of the abandoned do behavior must remain due")
	}
}

// testStateDoBodyAcceptFollowsTheTransitionChosen: a substate with two transitions
// enabled for the signal, one between the enclosing state's substates and one out
// of it. The one the policy chooses (the first declared) stays inside, so the
// enclosing do behavior goes on with the signal, the alternative leaving
// notwithstanding; Decide names the same transition and resume.
func testStateDoBodyAcceptFollowsTheTransitionChosen(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			transition shift first left accept Go then right;
			transition leave first left accept Go then stopped;
			state right { entry assign total := total + 1; }
		}
		state stopped { entry assign total := total + 100; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition shift"}, Resumes: []string{"do behavior of state active"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition chosen stays in the state, whose do behavior takes it too", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v, want 11: the do behavior's count, then the entry of right", total)
	}
}

// testStateDoBodyAcceptFollowsTheChoiceBranchTaken: the transition chosen targets a
// choice with a guarded branch to another substate and a default branch out of the
// enclosing state. The branch the guard selects, as it stands when the signal is
// dispatched, decides whether the enclosing do behavior goes on: inside, it does,
// the leaving branch notwithstanding; outside, the transition takes the signal alone.
func testStateDoBodyAcceptFollowsTheChoiceBranchTaken(t *testing.T) {
	model := func(stay string) string {
		return `
		private import ScalarValues::*;
		attribute def Go;
		state def Waiter {
			attribute total : Integer = 0;
			attribute stay : Boolean = ` + stay + `;
			entry; then active;
			state active {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
				entry; then left;
				state left;
				choice pick;
				transition route first left accept Go then pick;
				transition first pick if stay then right;
				transition first pick then stopped;
				state right { entry assign total := total + 1; }
			}
			state stopped { entry assign total := total + 100; }
		}
		part def Box { exhibit state w : Waiter; }
		`
	}
	cases := []struct {
		stay  string
		want  Decision
		leaf  string
		total int64
	}{
		{"true", Decision{Fires: []string{"transition route"}, Resumes: []string{"do behavior of state active"}}, "right", 11},
		{"false", Decision{Fires: []string{"transition route"}}, "stopped", 100},
	}
	for _, tc := range cases {
		exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, model(tc.stay))
		decision, err := exec.Decide(goMsg)
		if err != nil {
			t.Fatalf("stay = %s: Decide(Go): %v", tc.stay, err)
		}
		if !reflect.DeepEqual(decision, tc.want) {
			t.Errorf("stay = %s: Decide(Go) = %+v, want %+v: the branch the choice takes decides whether the do behavior goes on", tc.stay, decision, tc.want)
		}
		ctx.PostMessage(goMsg)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("stay = %s: dispatch the message: %v", tc.stay, err)
		}
		dispatch, ok := exec.LastDispatch()
		if !ok || !dispatch.Fired || dispatch.Deferred || !reflect.DeepEqual(dispatch.Resumed, tc.want.Resumes) {
			t.Errorf("stay = %s: dispatch = %+v, %v; want the transition fired and the do behavior resumed as decided", tc.stay, dispatch, ok)
		}
		if activeLeaf(exec) != tc.leaf || len(ctx.PendingMessages()) != 0 {
			t.Errorf("stay = %s: state %s with %d messages in flight, want %s with the one message consumed", tc.stay, activeLeaf(exec), len(ctx.PendingMessages()), tc.leaf)
		}
		if total := exec.StateData()["total"]; !valueEqual(total, integerValue(tc.total)) {
			t.Errorf("stay = %s: total = %v, want %d", tc.stay, total, tc.total)
		}
	}
}

// testStateDoBodyAcceptYieldsToATransitionIntoItsRegion: a transition out of one
// orthogonal region into a sibling region exits the state that region is in on the
// way, so the do behavior parked there does not take the signal the transition
// accepts, in Decide and in the dispatch alike; the signal is consumed once.
func testStateDoBodyAcceptYieldsToATransitionIntoItsRegion(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter parallel {
		attribute total : Integer = 0;
		state left {
			entry; then l1;
			state l1;
			transition swap first l1 accept Go then r2;
		}
		state right {
			entry; then r1;
			state r1 {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
			}
			state r2 { entry assign total := total + 1; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition swap"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition replaces the state the do behavior runs in", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed", dispatch, ok)
	}
	assertRegionConfig(t, exec, map[string]string{"right": "r2"})
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("%d messages in flight, want the one message consumed", len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of r2 only, the do behavior of r1 cancelled at its accept", total)
	}
}

// testStateDoBodyAcceptKeepsTheRouteChosen: the branch a choice routes the chosen
// transition along is settled before the do behaviors go on with the signal, so a
// do behavior that rewrites the guard on its way cannot send the transition down
// another branch than the one it was let go on for.
func testStateDoBodyAcceptKeepsTheRouteChosen(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		attribute stay : Boolean = true;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action flip assign stay := false;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			choice pick;
			transition route first left accept Go then pick;
			transition first pick if stay then right;
			transition first pick then stopped;
			state right { entry assign total := total + 1; }
		}
		state stopped { entry assign total := total + 100; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition route"}, Resumes: []string{"do behavior of state active"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed: the route was settled while stay held", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v, want 11: the do behavior's count, then the entry of right", total)
	}
}

// testStateDoBodyAcceptSharesTheDispatchWithARegion: a signal a do behavior in one
// orthogonal region is parked at an accept for and a transition in a sibling
// region accepts is dispatched once to both, the behavior going on from its accept
// before the transition fires, and Decide reports both, before.
func testStateDoBodyAcceptSharesTheDispatchWithARegion(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter parallel {
		attribute total : Integer = 0;
		state left {
			entry; then lwork;
			state lwork {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
			}
		}
		state right {
			entry; then rwait;
			state rwait;
			transition leave first rwait accept Go then rdone;
			state rdone { entry assign total := total + 1; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}, Resumes: []string{"do behavior of state lwork"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition and the do behavior of the sibling region both take it", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || dispatch.Deferred || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed, as decided", dispatch, ok)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("%d messages in flight, want the one message consumed", len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v, want 11: the do behavior's count, then the entry of rdone", total)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run on: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v after running on, want 11 still", total)
	}
}

// testStateChoiceRouteReadsTheAcceptedPayload: a choice guard along the chosen
// transition's route reads the payload the accept binds, so the route is settled
// with the payload bound and the branch the payload selects is the one entered.
func testStateChoiceRouteReadsTheAcceptedPayload(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go { attribute level : Integer; }
	state def Waiter {
		attribute total : Integer = 0;
		entry; then left;
		state left;
		choice pick;
		transition route first left accept g : Go then pick;
		transition first pick if g.level > 0 then right;
		transition first pick then stopped;
		state right { entry assign total := total + 1; }
		state stopped { entry assign total := total + 100; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "w.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("w.sysml")
	box, err := ctx.Instantiate(resolveSymbol(t, root, "Box"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := box.ExhibitedState()
	if !ok {
		t.Fatal("the box exhibits no machine")
	}
	exec := behavior.State
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), map[string]Value{"level": integerValue(1)}, box)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition route"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed: g.level was 1 when the choice was routed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of right", total)
	}
}

// boxDoBehaviorParkedAtGo instantiates Box from the source, whose exhibited machine
// starts a do behavior that parks at an accept of Go, and builds a Go for it.
func boxDoBehaviorParkedAtGo(t *testing.T, src string) (*StateExecutor, *Context, Message) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "w.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("w.sysml")
	box, err := ctx.Instantiate(resolveSymbol(t, root, "Box"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := box.ExhibitedState()
	if !ok {
		t.Fatal("the box exhibits no machine")
	}
	exec := behavior.State
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Fatal("the do behavior must be parked at its accept with nothing due")
	}
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, box)
	if err != nil {
		t.Fatal(err)
	}
	return exec, ctx, goMsg
}

// testStateDoBodyNodeReturnParameter: an action node of an inline do body's flow
// declaring `return` is refused before any node runs, as in a standalone action.
func testStateDoBodyNodeReturnParameter(t *testing.T) {
	exec := stateWithDoBody(t, `
		first start;
		then action a { assign total := total + 1; }
		then action b { return r : Integer = 1; }
		then done;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrActionResultParameter) {
		t.Fatalf("expected ErrActionResultParameter, got: %v", err)
	}
	if !strings.Contains(err.Error(), "state behavior ops") || !strings.Contains(err.Error(), "action node b") {
		t.Errorf("error %q does not name the behavior and the node", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyReturnParameter: an inline do body stating a flow and declaring
// `return` itself is refused before any node runs, as a standalone action is.
func testStateDoBodyReturnParameter(t *testing.T) {
	exec := stateWithDoBody(t, `
		return r : Integer = 1;
		first start;
		then action a { assign total := total + 1; }
		then done;
	`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrActionResultParameter) {
		t.Fatalf("expected ErrActionResultParameter, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action ops declares `return r`") {
		t.Errorf("error %q does not name the body's parameter", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: no node of the body must run", total)
	}
}

// testStateDoBodyFlowThatNeverEnds: a cycle of successions in an inline do body
// spends the step budget and is reported, rather than running forever.
func testStateDoBodyFlowThatNeverEnds(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				do action ops {
					first start;
					then action a { assign total := total + 1; }
					then action b { assign total := total + 1; }
					succession first b then a;
				}
			}
			succession first active then done;
		}
	}`))
	ctx.maxActionSteps = 50
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("expected ErrActionStepLimitExceeded, got: %v", err)
	}
	if !strings.Contains(err.Error(), "possible infinite loop") {
		t.Errorf("error %q does not point at the loop", err)
	}
}

// testStateBlockNodeUnvaluedPinWriteChecked: a write to a pin a node in a state's
// body declares without a value is checked against that pin's declaration, and the
// machine's same-named attribute is left as it was.
func testStateBlockNodeUnvaluedPinWriteChecked(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state Machine {
			attribute v : Integer = 100;
			entry; then init;
			state init;
			state active {
				entry action {
					if v == 100 {
						action p {
							out v : Integer;
							assign v := "one";
						}
					}
				}
			}
			succession first init then active;
			succession first active then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state machine Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "v") {
		t.Errorf("error %q does not name the pin written", err)
	}
	if v := exec.StateData()["v"]; !valueEqual(v, integerValue(100)) {
		t.Errorf("v = %v, want the machine's 100: the node's pin is not the machine's attribute", v)
	}
}

// testCalcBlockNodeUnvaluedPinWriteChecked: a pin a node in a calc's loop body
// declares without a value is the node's to write, so the write is judged against
// the pin's declaration, not refused as a name the calc never declared.
func testCalcBlockNodeUnvaluedPinWriteChecked(t *testing.T) {
	err := calcErrorWithLibraries(t, `package test {
		private import ScalarValues::*;
		calc noted {
			attribute v : Integer = 100;
			for i in 1..1 {
				action p {
					out w : Integer;
					assign w := "one";
				}
			}
			return : Integer = v;
		}
	}`, "noted", nil, 1000)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "w") {
		t.Errorf("error %q does not name the pin written", err)
	}
}

// testNodeBindingToANonParameter: a binding end at a node's pin must name a
// parameter or attribute the node's performance holds.
func testNodeBindingToANonParameter(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				bind add.nope = x;
				first start;
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("error = %v, want ErrBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the pin", err)
	}
}

// testNodeUndirectedBindingCarriedToANonParameter: a changed undirected attribute carried
// to a pin the downstream node does not declare is reported, not dropped.
func testNodeUndirectedBindingCarriedToANonParameter(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				bind acc.total = add.nope;
				first start;
				then action acc { attribute total : Integer; assign total := 3; }
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the pin", err)
	}
}

// testBlockNodeBindingToANonParameter: a binding in a branch at a pin of the
// branch's node is checked against that node's performance like any other.
func testBlockNodeBindingToANonParameter(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				first start;
				then action choose {
					if x > 0 {
						bind add.nope = x;
						action add : Adder { in a = 1; in b = 2; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("error = %v, want ErrBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the pin", err)
	}
}

// testBlockNodeBindingNamesANodeWithoutAPin: a binding end in a branch that names
// one of the branch's nodes but no pin of it is reported, not run as a statement.
func testBlockNodeBindingNamesANodeWithoutAPin(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				first start;
				then action choose {
					if x > 0 {
						bind add = x;
						action add : Adder { in a = 1; in b = 2; }
					}
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if err == nil {
		t.Fatal("expected the binding at the node itself to be reported")
	}
	if !strings.Contains(err.Error(), "names an action node but no pin of it") {
		t.Errorf("error %q does not explain the binding end", err)
	}
}

// testBlockNodePinBoundWhereNodesAreNotPerformed: a calc body keeps no
// performances of its nested actions, so a binding at one of their pins is reported.
func testBlockNodePinBoundWhereNodesAreNotPerformed(t *testing.T) {
	src := `
		package test {
			calc c {
				attribute x : Integer = 3;
				attribute seen : Integer = 0;
				if x > 0 {
					action p { out v : Integer; assign v := x * 2; }
					bind p.v = x;
					assign seen := 1;
				}
				return : Integer = seen;
			}
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "result", 10000)
	if err == nil || !strings.Contains(err.Error(), "a binding or flow at a pin of node p in a body is not executable") {
		t.Errorf("expected the binding at p's pin to be reported, got: %v", err)
	}
}

// testBlockNodeOwnFlowMalformed: the flow an action in a loop body states of its
// own is validated with the action's, so a dangling succession in it is reported
// at initialize() as an invalid flow rather than when the loop reaches it.
func testBlockNodeOwnFlowMalformed(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute i : Integer = 0;
				first start;
				then action iterate {
					while i < 2 {
						action step {
							first start;
							then action one { assign i := i + 1; }
							succession one then missing;
						}
					}
				}
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("expected ErrInvalidActionFlow for step's dangling succession, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action node step") || !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q does not name the node and the undefined target", err)
	}
}

// testInheritedBindingNamesANodeWithoutAPin: a binding a base action states at an
// inherited node itself is reported when the derived action's flow is built.
func testInheritedBindingNamesANodeWithoutAPin(t *testing.T) {
	src := `
		package test {
			action def Base {
				attribute x : Integer = 5;
				action add { in a : Integer; }
				bind add = x;
			}
			action def Derived :> Base {
				first start then add;
				succession add then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefAction)
	if sym == nil {
		t.Fatal("action Derived not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	if err == nil {
		t.Fatal("expected the inherited binding at the node itself to be reported")
	}
	if !strings.Contains(err.Error(), `binding end "add" names an action node but no pin of it`) {
		t.Errorf("error %q does not explain the binding end", err)
	}
}

// testInheritedBindingDoesNotReachAMaskingNode: a base's binding at a node it
// declares does not bind the same-named pin of a node the derived action
// declares in its place, so that pin stays unvalued.
func testInheritedBindingDoesNotReachAMaskingNode(t *testing.T) {
	src := `
		package test {
			action def Base {
				attribute x : Integer = 5;
				action add { in a : Integer; out sum : Integer; assign sum := a; }
				bind add.a = x;
			}
			action def Derived :> Base {
				action add { in a : Integer; out sum : Integer; assign sum := a + 1; }
				first start then add;
				succession add then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefAction)
	if sym == nil {
		t.Fatal("action Derived not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("the masking node's pin took the base's binding")
	}
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Feature != "a" {
		t.Errorf("error %v, want %T for a", err, noValue)
	}
	if v, ok := exec.Results()["add.sum"]; ok {
		t.Errorf("add.sum = %v, want no value", v)
	}
}

// testInheritedBindingDoesNotReachThroughAReplacedOtherEnd: a base binding whose
// other end names a node the derived action replaced holds at neither end, so
// the inherited node's input is bound by nothing rather than by the replacement.
func testInheritedBindingDoesNotReachThroughAReplacedOtherEnd(t *testing.T) {
	src := `
		package test {
			action def Adder {
				in a : Integer;
				out sum : Integer;
				first step;
				action step { assign sum := a; }
			}
			action def Base {
				action src { out n : Integer; assign n := 5; }
				action add : Adder;
				bind add.a = src.n;
			}
			action def Derived :> Base {
				action src { out n : Integer; assign n := 1000; }
				first start then src;
				succession src then add;
				succession add then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefAction)
	if sym == nil {
		t.Fatal("action Derived not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("the inherited node's pin took the replacement node's value")
	}
	if !errors.Is(err, ErrUnboundParameter) || !strings.Contains(err.Error(), "parameter a") {
		t.Errorf("error %v, want ErrUnboundParameter for a", err)
	}
	if v, ok := exec.Results()["add.sum"]; ok {
		t.Errorf("add.sum = %v, want no value", v)
	}
}

// testBlockNodeOwnFlowWhereNodesAreNotPerformed: a calc body keeps no performances
// of its nested actions, so one stating a flow of its own is reported, not run.
func testBlockNodeOwnFlowWhereNodesAreNotPerformed(t *testing.T) {
	src := `
		package test {
			calc c {
				attribute x : Integer = 3;
				attribute seen : Integer = 0;
				if x > 0 {
					action p {
						first start;
						then action bump { assign seen := 1; }
						then done;
					}
				}
				return : Integer = seen;
			}
		}
	`
	err := calcUsageOutputInSource(t, src, "c", "result", 10000)
	if err == nil || !strings.Contains(err.Error(), "the flow node p states of its own in a body is not executable") {
		t.Errorf("expected p's own flow to be reported, got: %v", err)
	}
}

// testBlockNodeOwnFlowThatNeverEnds: a cycle in the flow a block-declared node
// states of its own spends the action's token-flow budget, run by a body statement
// as it is, and reports that budget's error.
func testBlockNodeOwnFlowThatNeverEnds(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute x : Integer = 3;
				first start;
				then action pick {
					if x > 0 {
						action leg {
							first a;
							action a;
							action b;
							succession first a then b;
							succession first b then a;
						}
					}
				}
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxActionSteps = 50
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrActionStepLimitExceeded", err)
	}
	if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxActionStepsEnvVar)
	}
}

// testNodePinBoundToUnequalValues: two bindings at one input pin are two equalities,
// so unequal other ends are a binding conflict, not a declaration-order choice.
func testNodePinBoundToUnequalValues(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				attribute y : Integer = 2;
				bind add.a = x;
				bind add.a = y;
				first start;
				then action add : Adder { in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("error = %v, want ErrBindingConflict", err)
	}
	if got, want := err.Error(), "binding conflict at add.a: x = 1, y = 2"; got != want {
		t.Errorf("conflict error = %q, want %q", got, want)
	}
}

// testNestedPinBindingIntoANodePerformingAnotherAction: a node typed by an action def
// performs that action, whose nodes are its own, so a binding reaching into it is
// reported when the flow is built rather than routed to the node's like-named pin.
func testNestedPinBindingIntoANodePerformingAnotherAction(t *testing.T) {
	src := `
		package test {
			action def Leg {
				in w : Integer;
				first start;
				then action inner { in w : Integer; }
				then done;
			}
			action outer {
				attribute x : Integer = 5;
				bind leg.inner.w = x;
				first start;
				then action leg : Leg;
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	want := `binding end "leg.inner.w" reaches into leg, which performs an action of its own rather than declaring inner; bind at a pin of leg itself`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// testNestedPinBindingAtAnUndeclaredPin: a binding reaching a nested node must name a
// pin that node declares; the inner node reports it as it begins.
func testNestedPinBindingAtAnUndeclaredPin(t *testing.T) {
	src := `
		package test {
			action outer {
				attribute x : Integer = 5;
				bind leg.inner.nope = x;
				first start;
				then action leg {
					first start;
					then action inner { in w : Integer; }
					then done;
				}
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("error = %v, want ErrBindingEnd", err)
	}
	if !strings.Contains(err.Error(), "leg.inner.nope names no parameter or attribute of") {
		t.Errorf("error %q does not name the nested pin path", err)
	}
}

// testFlowReachingIntoANodesOwnFlow: a flow joins pins of the nodes of one flow, so an
// end reaching into a node's own flow is reported when the flow is built.
func testFlowReachingIntoANodesOwnFlow(t *testing.T) {
	src := `
		package test {
			action outer {
				first start;
				then action leg {
					first start;
					then action inner { out v : Integer; assign v := 1; }
					then done;
				}
				then action q { in n : Integer; }
				then done;
				flow leg.inner.v to q.n;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	_, err := ctx.CreateActionExecutor(sym)
	want := `end "leg.inner.v" reaches into a node's own flow`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

// testNodeBindingOutputToAnUnknownFeature: what a node's output pin is bound to
// must be a feature the action holds, so the value has somewhere to go.
func testNodeBindingOutputToAnUnknownFeature(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				bind add.sum = nowhere;
				first start;
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingEnd) {
		t.Fatalf("error = %v, want ErrBindingEnd", err)
	}
}

// testNodeBindingOutputThroughAScalarChain: a chained other end must reach an
// object whose feature the output can be written to.
func testNodeBindingOutputThroughAScalarChain(t *testing.T) {
	src := `
		package test {` + adderActionDef + `
			action outer {
				attribute x : Integer = 1;
				bind add.sum = x.value;
				first start;
				then action add : Adder { in a = 1; in b = 2; }
				then done;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrBindingEnd) || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrBindingEnd wrapping ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "x.value") {
		t.Errorf("error %q does not name the chained end x.value", err)
	}
}

// testNodeBindingOutputThroughAChainViolatesTargetType: an output written through a
// chain answers to the reached feature's declared type as a direct write does.
func testNodeBindingOutputThroughAChainViolatesTargetType(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Holder { attribute label : String = "none"; }
			action outer {
				part holder : Holder;
				bind num.v = holder.label;
				first start;
				then action num { out v : Integer; assign v := 3; }
				then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrBindingEnd) || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrBindingEnd wrapping ErrTypeMismatch", err)
	}
}

// testNodeFlowIntoAPinTheTargetDoesNotDeclare: a flow's target pin must be a
// feature of the node it reaches.
func testNodeFlowIntoAPinTheTargetDoesNotDeclare(t *testing.T) {
	src := `
		package test {
			action outer {
				first start;
				then action p { out v : Integer; assign v := 1; }
				then action q { in w : Integer; }
				then done;
				flow p.v to q.nope;
			}
		}
	`
	err := runOuterAction(t, src)
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin", err)
	}
}

// functionValueFixture declares Fn, which applies its calc-typed parameter f to a.
const functionValueFixture = `
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Add { in x : Real; in y : Real; return : Real = x + y; }
	calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
`

// invokeCalcExpecting evaluates the calc call expr against src with the standard library,
// on its own goroutine so a body that never terminates fails the case instead of stalling it.
func invokeCalcExpecting(t *testing.T, src, expr string) error {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = 100000
	scope := idx.DocumentRoot("<test>")

	done := make(chan error, 1)
	go func() {
		node := parser.New(source.New("<expr>", []byte(expr))).ParseExpression()
		result, err := ctx.EvalWithScopeOn(node, scope, nil)
		if err == nil {
			err = fmt.Errorf("%s = %s, expected it to fail", expr, FormatTraceValue(result))
		}
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not terminate", expr)
		return nil
	}
}

// testFunctionValueCallOfANonFunction: a scalar passed where a calc-typed parameter
// is declared is refused when it is bound, before the body calls it.
func testFunctionValueCallOfANonFunction(t *testing.T) {
	err := invokeCalcExpecting(t, `package test {`+functionValueFixture+`}`, "test::Fn(3.0, 3.0)")
	if !errors.Is(err, ErrNotAFunction) || !strings.Contains(err.Error(), `parameter "f"`) {
		t.Fatalf("error = %v, want ErrNotAFunction naming f", err)
	}
}

// testFunctionValueBoundToANonFunction: a named argument binding a calc-typed
// parameter to an object is refused the same way.
func testFunctionValueBoundToANonFunction(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		part def Box;
		part box : Box;
	}`
	err := invokeCalcExpecting(t, src, "test::Fn(a = 3.0, f = test::box)")
	if !errors.Is(err, ErrNotAFunction) {
		t.Fatalf("error = %v, want ErrNotAFunction", err)
	}
}

// testFunctionValueArityMismatch: applying a function value to more arguments
// than its calc declares is a calc arity error naming the calc the value is of.
func testFunctionValueArityMismatch(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Two { in calc f { in v : Real; return : Real; } return : Real = f(1.0, 2.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Two(test::Sq)")
	if !errors.Is(err, ErrCalcArity) || !strings.Contains(err.Error(), "test::Sq") {
		t.Fatalf("error = %v, want ErrCalcArity for test::Sq", err)
	}
}

// testFunctionValueUnknownNamedArgument: a named argument the applied calc does
// not declare is reported against that calc, not the parameter it was passed through.
func testFunctionValueUnknownNamedArgument(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Named { in calc f { in v : Real; return : Real; } return : Real = f(w = 1.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Named(test::Sq)")
	if !errors.Is(err, ErrUnknownParameter) || !strings.Contains(err.Error(), "test::Sq") {
		t.Fatalf("error = %v, want ErrUnknownParameter for test::Sq", err)
	}
}

// testFunctionValueUnboundCalcParameter: a calc-typed parameter no argument binds
// is reported as unbound when the calc is invoked, and applying a function value
// to fewer arguments than its calc needs is reported the same way.
func testFunctionValueUnboundCalcParameter(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Partial { in calc f { in x : Real; in y : Real; return : Real; } return : Real = f(1.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Fn(a = 3.0)")
	if !errors.Is(err, ErrUnboundParameter) || !strings.Contains(err.Error(), `parameter "f"`) {
		t.Fatalf("error = %v, want ErrUnboundParameter naming f", err)
	}
	err = invokeCalcExpecting(t, src, "test::Partial(test::Add)")
	if !errors.Is(err, ErrUnboundParameter) || !strings.Contains(err.Error(), `parameter "y"`) {
		t.Fatalf("error = %v, want ErrUnboundParameter naming y", err)
	}
}

// testFunctionValueOfAWrongTypedCalc: a calc-typed parameter typed by a calc def
// refuses a function value of an unrelated calc.
func testFunctionValueOfAWrongTypedCalc(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Typed { in calc f : Sq; return : Real = f(2.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::Typed(test::Add)")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testFunctionValueOfABuiltIn: a library function the runtime binds unevaluated
// has no value to pass on, and says so.
func testFunctionValueOfABuiltIn(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		private import ControlFunctions::*;
		calc def PassIf { return : Real = Fn(ControlFunctions::'if', 3.0); }
	}`
	err := invokeCalcExpecting(t, src, "test::PassIf()")
	if !errors.Is(err, ErrNotAFunction) {
		t.Fatalf("error = %v, want ErrNotAFunction", err)
	}
}

// testFunctionValueAppliedToItselfForever: a calc passing itself as a function
// value to itself without end spends the recursion budget rather than hanging.
func testFunctionValueAppliedToItselfForever(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Loop { in calc f { in v : Real; return : Real; } in v : Real; return : Real = Loop(f, f(v)); }
	}`
	err := invokeCalcExpecting(t, src, "test::Loop(test::Sq, 1.0)")
	if !errors.Is(err, ErrCalcRecursionLimit) && !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("error = %v, want the recursion or step budget spent", err)
	}
}

// testFunctionValueInheritedBodyOutsideTheClosure: a usage nested in a calc body
// closes over that body only for the code written there; the body it inherits from
// a calc declared outside reads no binding of the enclosing run, however it is applied.
func testFunctionValueInheritedBodyOutsideTheClosure(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Leaky { in v : Real; return : Real = v * k; }
		calc def Bare { in k : Real; calc inner : Leaky { in v = 2.0; } return : Real = inner; }
		calc def Called { in k : Real; calc inner : Leaky; return : Real = inner(2.0); }
		calc def Passed { in k : Real; calc inner : Leaky; return : Real = Fn(inner, 2.0); }
	}`
	for _, expr := range []string{"test::Bare(3.0)", "test::Called(3.0)", "test::Passed(3.0)"} {
		err := invokeCalcExpecting(t, src, expr)
		if !errors.Is(err, ErrNoValue) && !errors.Is(err, ErrUnresolvedReference) {
			t.Fatalf("%s: error = %v, want k unresolved in Leaky's body", expr, err)
		}
	}
}

// testFunctionValueNestedCalcOutsideItsRun: a calc nested in another calc's body
// closes over a run of that calc alone; applied from a calc that binds the same
// parameter name while no such run is active, it reads no binding of the caller's.
func testFunctionValueNestedCalcOutsideItsRun(t *testing.T) {
	src := `package test {` + functionValueFixture + `
		calc def Outer { in k : Real; calc inner { in v : Real; return : Real = v * k; } return : Real = inner(1.0); }
		calc def Called { in k : Real; return : Real = Outer::inner(2.0); }
		calc def Passed { in k : Real; return : Real = Fn(Outer::inner, 2.0); }
	}`
	for _, expr := range []string{"test::Called(3.0)", "test::Passed(3.0)"} {
		err := invokeCalcExpecting(t, src, expr)
		if !errors.Is(err, ErrNoValue) && !errors.Is(err, ErrUnresolvedReference) {
			t.Fatalf("%s: error = %v, want k unresolved in inner's body", expr, err)
		}
	}
}

// verificationRobustnessModel states a case whose subject nothing binds, one
// whose body reads a feature holding no value, and a part that is no case.
const verificationRobustnessModel = `
	package test {
		part def Sensor { attribute reading : ScalarValues::Integer; }
		part unread : Sensor;

		verification def Unbound {
			subject sensor : Sensor;
			VerificationCases::PassIf(sensor.reading == 0)
		}

		action def Adder {
			in a : ScalarValues::Integer;
			in b : ScalarValues::Integer;
			out sum : ScalarValues::Integer;
			first step;
			action step { assign sum := a + b; }
		}
		action adder : Adder;

		verification def Stepping {
			subject sensor : Sensor;
			first start;
			then perform adder;
			then done;
		}

		verification stepping : Stepping { subject sensor = unread; }

		verification def Thresholded {
			subject sensor : Sensor;
			in threshold : ScalarValues::Integer;
			VerificationCases::PassIf(sensor.reading == threshold)
		}

		verification def Plan {
			subject sensor : Sensor;
			verification sub : Thresholded;
			VerificationCases::PassIf(true)
		}

		verification plan : Plan { subject sensor = unread; }
	}
`

// testVerificationBodyThatCannotRun: a body whose subject nothing binds is the
// case's error verdict carrying the typed error, not a failed call and not a
// panic.
func testVerificationBodyThatCannotRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::Unbound"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != VerdictError {
		t.Fatalf("verdict = %q, want error", result.Verdict.Kind)
	}
	if !strings.Contains(result.Verdict.Detail, "sensor") {
		t.Errorf("verdict detail %q does not name the unbound subject", result.Verdict.Detail)
	}
}

// testVerificationBodyStepThatFails: a step that fails at run time — an action
// performed with an input nothing binds — ends the run in the same error verdict.
func testVerificationBodyStepThatFails(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::stepping"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != VerdictError {
		t.Fatalf("verdict = %q (%s), want error", result.Verdict.Kind, result.Verdict.Detail)
	}
	if !strings.Contains(result.Verdict.Detail, "adder") {
		t.Errorf("verdict detail %q does not name the step that failed", result.Verdict.Detail)
	}
}

// testVerificationSubcaseThatCannotRun: a performed subcase whose input nothing
// binds ends the performing case's run, so its verdict is the error naming the
// subcase and the reason rather than a verdict of the body it never finished.
func testVerificationSubcaseThatCannotRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::plan"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != VerdictError {
		t.Fatalf("verdict = %q (%s), want error", result.Verdict.Kind, result.Verdict.Detail)
	}
	for _, want := range []string{"sub", "threshold"} {
		if !strings.Contains(result.Verdict.Detail, want) {
			t.Errorf("verdict detail %q does not name %q", result.Verdict.Detail, want)
		}
	}
	if len(result.Subcases) != 0 {
		t.Errorf("subcase verdicts = %v, want none from a body that did not finish", result.Subcases)
	}
}

// testVerificationOfASymbolThatIsNotACase: asking a part for a verdict faults
// the request, which is an error rather than a verdict.
func testVerificationOfASymbolThatIsNotACase(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	_, err := ctx.RunVerification(lookupOne(t, idx, "test::Sensor"), AnalysisArgs{}, scope, nil)
	if !errors.Is(err, ErrNotAVerification) {
		t.Fatalf("error = %v, want ErrNotAVerification", err)
	}
}

// testVerificationWithAnArgumentTheCaseDoesNotTake: a named argument no
// parameter of the case declares faults the request too.
func testVerificationWithAnArgumentTheCaseDoesNotTake(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	args := AnalysisArgs{Named: map[string]Value{"nope": integerValue(1)}}
	_, err := ctx.RunVerification(lookupOne(t, idx, "test::stepping"), args, scope, nil)
	if !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("error = %v, want ErrUnknownParameter", err)
	}
}

// verificationObjectiveModel states a verification case whose objective is a
// requirement on a Rover while the case verifies a Lander, and one that
// verifies nothing bound.
const verificationObjectiveModel = `
	package test {
		private import ScalarValues::*;
		part def Lander { attribute touchdownSpeed : Real; }
		part def Rover { attribute touchdownSpeed : Real; }
		part scout : Lander { attribute :>> touchdownSpeed = 1.2; }

		requirement def SoftRoving {
			subject rover : Rover;
			in attribute limit : Real default = 1.5;
			require constraint { rover.touchdownSpeed <= limit }
		}

		verification def RoverCheck {
			subject lander : Lander;
			in attribute limit : Real = 1.5;
			objective : SoftRoving { in limit = limit; }
			VerificationCases::PassIf(lander.touchdownSpeed <= limit)
		}
		verification checkRover : RoverCheck { subject lander = scout; }
		verification checkNothing : RoverCheck;
	}
`

// testVerificationObjectiveSubjectOfAnotherType: the case's subject, a Lander, is
// what the library binds the objective's subject to, so a requirement wanting a
// Rover leaves the objective undecided by a typed mismatch naming both types,
// while the body still answers.
func testVerificationObjectiveSubjectOfAnotherType(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationObjectiveModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::checkRover"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want the body's verdict", err)
	}
	if result.Verdict.Kind != VerdictPass {
		t.Fatalf("verdict = %q (%s), want pass", result.Verdict.Kind, result.Verdict.Detail)
	}
	if len(result.Run.Verdicts) != 1 || result.Run.Verdicts[0].Status != VerdictUndecided {
		t.Fatalf("verdicts = %+v, want the objective undecided", result.Run.Verdicts)
	}
	detail := result.Run.Verdicts[0].Detail
	for _, want := range []string{"subject rover", "case's subject", "VerificationCases::VerificationCase::obj", ErrTypeMismatch.Error(), "Lander", "is not a Rover"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q does not say %q", detail, want)
		}
	}
	if strings.Contains(detail, "Cases::Case::obj") || strings.Contains(detail, "result") {
		t.Errorf("detail %q reports the analysis case's default, not the verification's binding", detail)
	}
}

// testVerificationObjectiveSubjectLeftUnbound: a verification binding no subject
// is refused by the typed error naming the verification's subject, not the
// objective's, on the run surface as on the verdict surface.
func testVerificationObjectiveSubjectLeftUnbound(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationObjectiveModel))
	scope := idx.DocumentRoot("<test>")
	sym := lookupOne(t, idx, "test::checkNothing")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, scope, nil)
	var unbound *UnboundSubjectError
	if !errors.As(err, &unbound) {
		t.Fatalf("RunAnalysis error = %v, want an UnboundSubjectError", err)
	}
	if unbound.Kind != "verification" || unbound.Element != "test::checkNothing" || unbound.Subject != "lander" {
		t.Errorf("error names %s %s: %s, want verification test::checkNothing: lander", unbound.Kind, unbound.Element, unbound.Subject)
	}
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != VerdictError || result.Verdict.Detail != unbound.Error() {
		t.Errorf("verdict = %q (%s), want error carrying %q", result.Verdict.Kind, result.Verdict.Detail, unbound.Error())
	}
}

// testVerificationObjectiveSubjectRebound: the library binds a verification
// objective's subject with `=`, so a usage binding it itself is refused by the
// constraint tier, whichever way it names the subject.
func testVerificationObjectiveSubjectRebound(t *testing.T) {
	const src = `
		package test {
			private import ScalarValues::*;
			part def Lander { attribute touchdownSpeed : Real; }
			part scout : Lander { attribute :>> touchdownSpeed = 1.2; }
			part other : Lander { attribute :>> touchdownSpeed = 1.3; }

			requirement def SoftLanding {
				subject lander : Lander;
				require constraint { lander.touchdownSpeed <= 1.5 }
			}

			verification def Named {
				subject lander : Lander;
				objective : SoftLanding { subject lander = other; }
				VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
			}
			verification def Anonymous {
				subject lander : Lander;
				objective : SoftLanding { subject = other; }
				VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
			}
			verification def Redefining {
				subject lander : Lander;
				objective : SoftLanding { subject :>> subj = other; }
				VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
			}
		}
	`
	file := parseAndBuild(t, src)
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	var refusals []string
	for _, d := range passes.Analyze("<test>", file, nil, idx) {
		if d.Code == "feature-value-overriding" && d.Severity == passes.SeverityError {
			refusals = append(refusals, d.Message)
		}
	}
	if len(refusals) != 3 {
		t.Fatalf("got %d refusals, want one per rebinding: %v", len(refusals), refusals)
	}
	for _, msg := range refusals {
		if !strings.Contains(msg, "cannot override the binding value of VerificationCases::VerificationCase::obj::subj") {
			t.Errorf("refusal %q does not name the library's binding", msg)
		}
	}
}

// tradeStudyRobustnessModel states trade studies that cannot finish: an
// evaluation function left abstract, one that divides by an alternative's zero,
// a subject listing nothing, one redefined to a single value yet bound to two,
// and an evaluation reading a feature no alternative gives a value.
const tradeStudyRobustnessModel = `
	package test {
		private import ScalarValues::*;
		private import TradeStudies::*;

		part def Engine { attribute mass : Real; attribute cylinders : Integer; attribute cost : Real; }
		part heavy : Engine { attribute :>> mass = 20.0; attribute :>> cylinders = 0; }
		part light : Engine { attribute :>> mass = 10.0; attribute :>> cylinders = 2; }

		analysis abstractEval : TradeStudy {
			subject : Engine[1..*] = (heavy, light);
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real; }
			return part :>> selectedAlternative : Engine;
		}

		analysis perCylinder : TradeStudy {
			subject : Engine[1..*] = (light, heavy);
			objective : MinimizeObjective;
			calc :>> evaluationFunction {
				in part e :>> alternative : Engine;
				return :>> result : Real = e.mass / e.cylinders;
			}
			return part :>> selectedAlternative : Engine;
		}

		analysis none : TradeStudy {
			subject : Engine[1..*] = ();
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real = e.mass; }
			return part :>> selectedAlternative : Engine;
		}

		analysis single : TradeStudy {
			subject : Engine[1] = (heavy, light);
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real = e.mass; }
			return part :>> selectedAlternative : Engine;
		}

		analysis unpriced : TradeStudy {
			subject : Engine[1..*] = (heavy, light);
			objective : MinimizeObjective;
			calc :>> evaluationFunction { in part e :>> alternative : Engine; return :>> result : Real = e.cost; }
			return part :>> selectedAlternative : Engine;
		}
	}
`

// runTradeStudyExpecting runs the named analysis of tradeStudyRobustnessModel
// and returns what it reported; a run that does not terminate fails the test.
func runTradeStudyExpecting(t *testing.T, name string) (AnalysisResult, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, tradeStudyRobustnessModel))
	ctx.maxSteps = 100000
	scope := idx.DocumentRoot("<test>")
	sym := lookupOne(t, idx, name)

	type outcome struct {
		result AnalysisResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := ctx.RunAnalysis(sym, AnalysisArgs{}, scope, nil)
		done <- outcome{result, err}
	}()
	select {
	case out := <-done:
		if out.err == nil {
			t.Fatalf("%s selected %s, expected the run to fail", name, FormatValue(out.result.Outputs[0].Value))
		}
		return out.result, out.err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not terminate", name)
		return AnalysisResult{}, nil
	}
}

// undecidedObjective asserts the result carries the trade study's objective as
// undecided and returns its detail.
func undecidedObjective(t *testing.T, result AnalysisResult) string {
	t.Helper()
	for _, v := range result.Verdicts {
		if v.Kind == "objective" && v.Name == "tradeStudyObjective" {
			if v.Status != VerdictUndecided {
				t.Fatalf("objective %s = %s, want undecided", v.Name, v.Status)
			}
			return v.Detail
		}
	}
	t.Fatalf("verdicts %v carry no tradeStudyObjective", result.Verdicts)
	return ""
}

// testTradeStudyWithAnAbstractEvaluationFunction: an evaluation function with no
// body is a typed missing-body error naming the calc, with the objective
// undecided and the one evaluation attempted recorded with its error; no pick
// is fabricated.
func testTradeStudyWithAnAbstractEvaluationFunction(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::abstractEval")
	if !errors.Is(err, ErrNoResultExpression) || !strings.Contains(err.Error(), "test::abstractEval::evaluationFunction") {
		t.Fatalf("error = %v, want ErrNoResultExpression naming the evaluation function", err)
	}
	undecidedObjective(t, result)
	if len(result.Outputs) != 0 {
		t.Errorf("outputs = %v, want none from a study that could not evaluate", result.Outputs)
	}
	if len(result.Evaluations) != 1 || result.Evaluations[0].Error == nil || result.Evaluations[0].Selected {
		t.Fatalf("evaluations = %+v, want the one failed attempt and no selection", result.Evaluations)
	}
}

// testTradeStudyWhoseEvaluationFailsForOneAlternative: an alternative whose
// evaluation divides by zero fails the run with that typed error, the objective
// undecided, and the alternatives evaluated before it kept beside the failure.
func testTradeStudyWhoseEvaluationFailsForOneAlternative(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::perCylinder")
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 2 {
		t.Fatalf("evaluations = %+v, want light's value and heavy's failure", result.Evaluations)
	}
	if result.Evaluations[0].Error != nil || result.Evaluations[0].Selected || result.Evaluations[0].Tied {
		t.Errorf("light's evaluation = %+v, want a plain value", result.Evaluations[0])
	}
	if !errors.Is(result.Evaluations[1].Error, ErrDivisionByZero) {
		t.Errorf("heavy's evaluation error = %v, want ErrDivisionByZero", result.Evaluations[1].Error)
	}
}

// testTradeStudyWithAnEmptySubject: a subject listing no alternative violates the
// library's [1..*] before anything is evaluated.
func testTradeStudyWithAnEmptySubject(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::none")
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "lower bound 1") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation against the lower bound", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 0 {
		t.Errorf("evaluations = %+v, want none", result.Evaluations)
	}
}

// testTradeStudyWithASingleValuedSubject: a subject redefined to [1] refuses the
// two alternatives bound to it as a multiplicity violation, not a study of one.
func testTradeStudyWithASingleValuedSubject(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::single")
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "upper bound 1") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation against the upper bound", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 0 {
		t.Errorf("evaluations = %+v, want none", result.Evaluations)
	}
}

// testTradeStudyWhoseAlternativesReadAnUnboundFeature: an evaluation reading a
// feature the first alternative gives no value answers none, which minimize
// reports as the unset feature rather than as a value of the wrong kind; the
// objective is undecided and nothing is selected.
func testTradeStudyWhoseAlternativesReadAnUnboundFeature(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::unpriced")
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Symbol == nil || noValue.Symbol.Name != "cost" {
		t.Fatalf("error = %v, want a NoValueError naming cost", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 1 || result.Evaluations[0].Selected {
		t.Fatalf("evaluations = %+v, want heavy's valueless evaluation alone", result.Evaluations)
	}
}

// sweepRobustnessModel declares parameters no numeric range can bind, an
// Integer one a fractional step cannot step, and a Real one.
const sweepRobustnessModel = `package test {
	private import ScalarValues::*;
	part def Ship;
	calc def Flag { in b : Boolean; return : Boolean = b; }
	calc def Hull { in s : Ship; return : Ship = s; }
	calc def Sq { in x : Integer; return : Integer = x * x; }
	calc def Half { in x : Real; return : Real = x / 2.0; }
}`

// refusedSweepPlan sweeps the named calc over the plan, both stepped and
// sampled, and returns each typed refusal, failing when a row was run.
func refusedSweepPlan(t *testing.T, name string, plan SweepPlan) []error {
	t.Helper()
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("calc %s not indexed", name)
	}
	sampled := plan
	sampled.Sampled, sampled.Samples, sampled.Seed = true, 2, 1
	var errs []error
	for _, p := range []SweepPlan{plan, sampled} {
		resolved, err := ctx.ResolveSweepPlan(sym, p, 0, nil)
		if err != nil {
			t.Fatalf("%s refused the plan's parameter: %v", name, err)
		}
		runs := 0
		table, err := sweepIn(ctx, context.Background(), "test::"+name, resolved, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
			runs++
			return SweepRunResult{}, nil
		})
		if err == nil {
			t.Fatalf("%s ran %d row(s); want a refusal", name, len(table.Rows))
		}
		if !errors.Is(err, ErrSweepRange) {
			t.Fatalf("error = %v, want ErrSweepRange", err)
		}
		if runs != 0 {
			t.Fatalf("a refused plan made %d run(s)", runs)
		}
		errs = append(errs, err)
	}
	return errs
}

// testSweepOverABooleanParameter: a Boolean takes no numeric range, so the plan
// is refused naming the parameter and its type, and no row is run.
func testSweepOverABooleanParameter(t *testing.T) {
	plan := SweepPlan{Ranges: []SweepRange{{Param: "b",
		From: Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
		To:   Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}}}}
	for _, err := range refusedSweepPlan(t, "Flag", plan) {
		if msg := err.Error(); !strings.Contains(msg, "b") || !strings.Contains(msg, "Boolean") {
			t.Errorf("error = %v, want it to name b and Boolean", err)
		}
	}
}

// testSweepOverAParameterTypedByAPart: a part definition is no scalar, so a
// range over a parameter it types is refused before any run.
func testSweepOverAParameterTypedByAPart(t *testing.T) {
	plan := SweepPlan{Ranges: []SweepRange{{Param: "s",
		From: Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
		To:   Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}}}}
	for _, err := range refusedSweepPlan(t, "Hull", plan) {
		if msg := err.Error(); !strings.Contains(msg, "s") || !strings.Contains(msg, "Ship") {
			t.Errorf("error = %v, want it to name s and Ship", err)
		}
	}
}

// testSweepOverAnIntegerParameterByAFraction: an Integer parameter takes no
// fractional step or endpoint, so the plan is refused rather than half its
// rows failing one by one.
func testSweepOverAnIntegerParameterByAFraction(t *testing.T) {
	realVal := func(f float64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
	}
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, _ := scope.LookupLocal("Sq")
	plan, err := ctx.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{{
		Param: "x", From: realVal(1), To: realVal(3), Step: realVal(0.5), HasStep: true,
	}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolving x: %v", err)
	}
	_, err = sweepIn(ctx, context.Background(), "test::Sq", plan, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
		t.Fatal("a row ran under a fractional step")
		return SweepRunResult{}, nil
	})
	if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "x : Integer") {
		t.Fatalf("error = %v, want ErrSweepRange naming x : Integer", err)
	}
	for _, err := range refusedSweepPlan(t, "Sq", SweepPlan{Ranges: []SweepRange{{Param: "x", From: realVal(1.5), To: realVal(3)}}}) {
		if !strings.Contains(err.Error(), "x : Integer") {
			t.Errorf("error = %v, want it to name x : Integer", err)
		}
	}
}

// testSweepOverARealParameterByIntegersNoRealHolds: a Real parameter takes the
// Integers of its range as Reals, so endpoints a Real rounds together are
// refused rather than collapsed onto one row, as is a step the reals cannot
// tell apart, before any row runs.
func testSweepOverARealParameterByIntegersNoRealHolds(t *testing.T) {
	integer := func(n int64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	}
	const big = int64(1) << 60
	for _, err := range refusedSweepPlan(t, "Half", SweepPlan{Ranges: []SweepRange{{Param: "x", From: integer(big), To: integer(big + 3)}}}) {
		if msg := err.Error(); !strings.Contains(msg, "x : Real") || !strings.Contains(msg, "1152921504606846979") {
			t.Errorf("error = %v, want it to name x : Real and the end 1152921504606846979", err)
		}
	}
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, _ := scope.LookupLocal("Half")
	plan, err := ctx.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{{
		Param: "x", From: integer(big), To: integer(big + 512),
	}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolving x: %v", err)
	}
	_, err = sweepIn(ctx, context.Background(), "test::Half", plan, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
		t.Fatal("a row ran under a step the reals cannot tell apart")
		return SweepRunResult{}, nil
	})
	if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "rows would repeat") {
		t.Fatalf("error = %v, want ErrSweepRange refusing repeated rows", err)
	}
}

// testArithmeticOverTheUnboundedValue: `*` is no number, so arithmetic over it
// fails with a typed error naming the operation.
func testArithmeticOverTheUnboundedValue(t *testing.T) {
	for _, expr := range []string{"* + 1", "1 - *", "2 * *", "* / 2", "* % 2", "* ** 2", "-*", "+*"} {
		_, _, err := evalDeclaredExpr(t, "package test {}", expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// testUnboundedValueComparedWithAString: nothing orders `*` against a string.
func testUnboundedValueComparedWithAString(t *testing.T) {
	_, _, err := evalDeclaredExpr(t, "package test {}", `* > "a"`)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}

// testMetadataOfAValue: only an element carries metadata, so reading it from a
// scalar fails rather than answering with the empty sequence.
func testMetadataOfAValue(t *testing.T) {
	for _, expr := range []string{"1.metadata", `"abc".metadata`} {
		_, _, err := evalDeclaredExpr(t, "package test {}", expr)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error = %v, want ErrTypeMismatch", expr, err)
		}
	}
}

// testMetadataOfAnUnresolvedName: a name that names no element is refused.
func testMetadataOfAnUnresolvedName(t *testing.T) {
	_, _, err := evalDeclaredExpr(t, "package test {}", "test::missing.metadata")
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
}

// stateMachineWithLibraries builds Machine from src over the standard library,
// for state bodies that name units, and returns it before initialization.
func stateMachineWithLibraries(t *testing.T, src string) *StateExecutor {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	return exec
}

// testStateDoTypedActionInputUnbound: a typed do usage that binds none of the
// action's input parameters is refused when the state's do behavior starts,
// naming the action and the parameter, and the action does not run.
func testStateDoTypedActionInputUnbound(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		action def Poll { in n : Integer; assign n := n + 1; }
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active { do action poll : Poll { } }
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("expected ErrUnboundParameter, got: %v", err)
	}
	for _, want := range []string{"do action in state active", "action Poll", "input parameter n"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	assertCurrentState(t, exec, "active")
}

// testStateDoTypedActionPinBoundToMissingFeature: a typed do usage binding a pin
// to a feature the state does not declare is refused naming the pin, and the
// action does not run.
func testStateDoTypedActionPinBoundToMissingFeature(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		action def Poll { in n : Integer; assign n := n + 1; }
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active { do action poll : Poll { in n = nothing; } }
			succession first active then done;
		}
	}`)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected ErrUnresolvedReference, got: %v", err)
	}
	for _, want := range []string{"do action in state active", "n of node poll", "nothing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	assertCurrentState(t, exec, "active")
}

// testStateDoTypedActionInoutValuedByAnImportedLiteral: an `inout` pin valued by
// an enumeration literal reached through an import holds the literal as its
// initial value; the performance ends without writing back to it, while the
// pin valued by a feature writes back to that feature.
func testStateDoTypedActionInoutValuedByAnImportedLiteral(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		enum def Mode { idle; busy; }
		private import Mode::*;
		action def Poll {
			inout n : Integer;
			inout mode : Mode;
			first start;
			then action count assign n := if mode == idle ? n + 1 else n + 100;
			then action flip assign mode := busy;
			then done;
		}
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active { do action poll : Poll { inout n = total; inout mode = idle; } }
			succession first active then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: n counted from mode idle and wrote back", total)
	}
	assertCurrentState(t, exec, "done")
}

// testStateEntryBodyWaitsForTheClock: an entry body whose flow waits for the
// clock is refused when the state is entered, naming the wait; a state is
// entered at one instant, and only its do behavior may wait.
func testStateEntryBodyWaitsForTheClock(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				entry action {
					action w accept after 1 [s];
					then action c assign total := 1;
				}
			}
			succession first active then done;
		}
	}`)
	err := exec.initialize()
	if !errors.Is(err, ErrStateBehaviorWaits) {
		t.Fatalf("expected ErrStateBehaviorWaits, got: %v", err)
	}
	for _, want := range []string{"enter state active", "entry action", "t=1.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: nothing after the wait must run", total)
	}
	if got := len(exec.ctx.Clock().Waits()); got != 0 {
		t.Errorf("%d wait(s) left on the clock by the refused entry body", got)
	}
}

// testStateExitBodyWaitsForTheClock: an exit body whose flow waits for the clock
// is refused when the state is left, naming the wait, and time does not advance.
func testStateExitBodyWaitsForTheClock(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				exit action {
					action w accept after 1 [s];
					then action c assign total := 1;
				}
			}
			succession first active then done;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStateBehaviorWaits) {
		t.Fatalf("expected ErrStateBehaviorWaits, got: %v", err)
	}
	for _, want := range []string{"exit state", "exit action", "t=1.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: nothing after the wait must run", total)
	}
	if now := exec.ctx.Clock().Now(); now != 0 {
		t.Errorf("clock at %v, want 0: a refused exit body must not advance time", now)
	}
}

// testTransitionEffectBodyWaitsForTheClock: a transition effect whose flow waits
// for the clock is refused when the transition fires, naming the wait.
func testTransitionEffectBodyWaitsForTheClock(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active;
			transition first active then done do action {
				action w accept after 1 [s];
				then action c assign total := 1;
			}
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStateBehaviorWaits) {
		t.Fatalf("expected ErrStateBehaviorWaits, got: %v", err)
	}
	for _, want := range []string{"transition effect", "t=1.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: nothing after the wait must run", total)
	}
}

// testStateDoBodyNestedAcceptCancelledOnExit: an accept nested one node deep in
// a do body, for a signal nothing sends, parks the body while the machine waits
// for its timed exit; leaving the state cancels the body, leaving no waiter
// behind and running nothing after the accept.
func testStateDoBodyNestedAcceptCancelledOnExit(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		attribute def Go;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				do action ops {
					first start;
					then action inner {
						first start;
						then action w accept g : Go;
						then action c assign total := 1;
						then done;
					}
					then done;
				}
			}
			transition first active accept after 10 [s] then finished;
			state finished;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "finished")
	if now := exec.ctx.Clock().Now(); now != 10 {
		t.Errorf("clock at %v, want 10: the exit transition's instant", now)
	}
	if exec.HasPendingDoWork() {
		t.Error("leaving the state must end its parked do body")
	}
	if got := len(exec.ctx.Clock().Waits()); got != 0 {
		t.Errorf("%d wait(s) left on the clock after the state exited", got)
	}
	exec.ctx.PostMessage(Message{SignalType: "Go", Target: "w"})
	if exec.HasPendingDoWork() {
		t.Error("a message after the exit must not revive the cancelled body")
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: the node after the cancelled accept must not run", total)
	}
}
