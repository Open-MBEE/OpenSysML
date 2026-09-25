// Census: validateRequirementDefinitionSubjectParameterPosition
// Expect: error: Subject must be first parameter.
package P { part def D; requirement def R { in x : D; subject a : D; } }
