// Census: validateCaseUsageSubjectParameterPosition
// Expect: error: Subject must be first parameter.
package P { part def D; case def C; case c : C { in x : D; subject a : D; } }
