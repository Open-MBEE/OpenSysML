// Census: validateCaseDefinitionOnlyOneSubject
// Expect: error: Only one subject is allowed.
package P { part def D; case def C { subject a : D; subject b : D; } }
