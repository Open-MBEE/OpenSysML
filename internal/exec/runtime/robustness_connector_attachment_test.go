package runtime

import (
	"testing"
)

// TestRuntimeRobustnessConnectorAttachment covers connectors that cannot be attached: unattachable
// ends, multiplicities on connectors and mutually attached connectors.
func TestRuntimeRobustnessConnectorAttachment(t *testing.T) {
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
}
