// Census: validateRequirementUsageSubjectParameterPosition
// Expect: error: Subject must be first parameter.
package P { part def D; requirement def R; requirement r : R { in x : D; subject a : D; } }
