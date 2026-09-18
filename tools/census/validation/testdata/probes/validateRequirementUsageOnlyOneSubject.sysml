// Census: validateRequirementUsageOnlyOneSubject
// Expect: error: Only one subject is allowed.
package P { part def D; requirement def R; requirement r : R { subject a : D; subject b : D; } }
