// Census: validateAllocationUsageType
// Expect: error: An allocation must be typed by allocation definitions.
package P { part def D; part def E { part a : D; part b : D; allocation al : D allocate a to b; } }
