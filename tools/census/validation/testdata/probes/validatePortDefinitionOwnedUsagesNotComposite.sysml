// Census: validatePortDefinitionOwnedUsagesNotComposite
// Expect: error: Owned usages of a port definition (other than ports) must be referential.
package P { part def D; port def Q { part p : D; } }
