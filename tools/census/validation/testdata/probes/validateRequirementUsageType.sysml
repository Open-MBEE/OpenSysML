// Census: validateRequirementUsageType
// Expect: error: A requirement must be typed by one requirement definition.
package P { requirement def R1; requirement def R2; requirement r : R1, R2; }
