// Census: validateRequirementVerificationMembershipOwningType
// Expect: error: A requirement verification must be in the objective of a verification case.
package P { requirement def R; requirement def Q { verify requirement r : R; } }
