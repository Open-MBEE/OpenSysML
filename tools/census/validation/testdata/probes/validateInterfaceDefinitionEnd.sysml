// Census: validateInterfaceDefinitionEnd
// Expect: error: An interface definition end must be a port.
package P { part def D; interface def I { end part a : D; end part b : D; } }
