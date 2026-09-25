// Census: validateFlowUsageType
// Expect: error: A flow connection must be typed by flow connection definitions.
package P { part def D; part def E { part a : D; part b : D; flow f : D from a to b; } }
