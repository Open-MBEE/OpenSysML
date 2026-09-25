// Census: validateCaseUsageOnlyOneSubject
// Expect: error: Only one subject is allowed.
package P { part def D; case def C; case c : C { subject a : D; subject b : D; } }
