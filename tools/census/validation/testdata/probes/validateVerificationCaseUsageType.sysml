// Census: validateVerificationCaseUsageType
// Expect: error: A verification case must be typed by one verification case definition.
package P { verification def V1; verification def V2; verification v : V1, V2; }
