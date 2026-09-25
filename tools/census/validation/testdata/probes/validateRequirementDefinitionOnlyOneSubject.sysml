// Census: validateRequirementDefinitionOnlyOneSubject
// Expect: error: Only one subject is allowed.
package P { part def D; requirement def R { subject a : D; subject b : D; } }
