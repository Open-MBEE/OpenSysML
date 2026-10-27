// Census: validateFlowUsageType
// Expect: error: A flow usage must be typed by flow definitions.
package P { part def D; part def E { part a : D; part b : D; flow f : D from a to b; } }
