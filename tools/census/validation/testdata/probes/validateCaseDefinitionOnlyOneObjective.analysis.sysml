// Census: validateCaseDefinitionOnlyOneObjective
// Expect: error: Only one objective is allowed.
package P { analysis def A { objective o1; objective o2; } }
