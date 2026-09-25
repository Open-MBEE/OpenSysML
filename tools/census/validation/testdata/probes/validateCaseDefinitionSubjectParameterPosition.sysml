// Census: validateCaseDefinitionSubjectParameterPosition
// Expect: error: Subject must be first parameter.
package P { part def D; case def C { in x : D; subject a : D; } }
