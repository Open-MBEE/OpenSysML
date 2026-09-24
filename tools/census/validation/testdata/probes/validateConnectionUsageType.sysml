// Census: validateConnectionUsageType
// Expect: error: A connection must be typed by connection definitions.
package P { part def D; part def E { part a : D; part b : D; connection c : D connect a to b; } }
