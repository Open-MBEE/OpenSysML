// Census: validateAssignmentActionUsageReferent
// Expect: error: An assignment must have a referent.
package P { part def D; action def A { assign D := 1; } }
