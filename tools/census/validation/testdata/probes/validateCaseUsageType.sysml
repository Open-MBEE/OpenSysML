// Census: validateCaseUsageType
// Expect: error: A case must be typed by one case definition.
package P { case def C1; case def C2; case c : C1, C2; }
