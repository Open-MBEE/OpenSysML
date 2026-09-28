// Census: validateRequirementConstraintMembershipOwningType
// Expect: error: only allowed in a requirement or case body
package P { constraint def K; part def D { require constraint k : K; } }
