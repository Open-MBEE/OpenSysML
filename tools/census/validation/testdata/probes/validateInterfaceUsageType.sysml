// Census: validateInterfaceUsageType
// Expect: error: An interface must be typed by interface definitions.
package P { port def Q; part def D; part def E { port a : Q; port b : Q; interface i : D connect a to b; } }
