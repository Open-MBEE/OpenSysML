// Census: validateRequirementConstraintMembershipOwningType
// Expect: warning: outside a requirement body is an OpenSysML extension
package P { constraint def K; part def D { require constraint k : K; } }
