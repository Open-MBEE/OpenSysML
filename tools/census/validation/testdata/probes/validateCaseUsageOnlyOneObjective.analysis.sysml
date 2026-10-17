// Census: validateCaseUsageOnlyOneObjective
// Expect: error: Only one objective is allowed.
package P { analysis def A; analysis a : A { objective o1; objective o2; } }
