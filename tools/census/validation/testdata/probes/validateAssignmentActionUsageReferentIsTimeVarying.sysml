// Census: validateAssignmentActionUsageReferentIsTimeVarying
// Expect: error: Referent must be time varying.
package P { attribute def K { attribute x : ScalarValues::Integer; } action def A { in k : K; assign k.x := 1; } }
