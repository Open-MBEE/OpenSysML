// Census: validateUseCaseUsageType
// Expect: error: A use case must be typed by one use case definition.
package P { use case def U1; use case def U2; use case u : U1, U2; }
