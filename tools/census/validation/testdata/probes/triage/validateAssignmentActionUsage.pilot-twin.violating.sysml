// Census: validateAssignmentActionUsage
// Case: violating
// Ours: e[assignment-referent] An assignment must have a referent. D is declared `part def`, not a feat
// Pilot: e: An assignment must have a referent.
package P { part def D; action def A { assign D := 1; } }
