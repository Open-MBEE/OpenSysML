// Census: validateCaseUsageOnlyOneObjective
// Expect: error: Only one objective is allowed.
package P { case def C; case c : C { objective o1; objective o2; } }
