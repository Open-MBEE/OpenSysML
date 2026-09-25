// Census: validatePortUsageNestedUsagesNotComposite
// Expect: error: Nested usages in a port usage (other than ports) must be referential.
package P { part def D; port def Q; part def E { port p : Q { part x : D; } } }
