// Census: validatePerformActionUsageReference
// Expect: error: Must reference an action.
package P { part def D; part def E { part d : D; perform d; } }
