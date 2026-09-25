// Census: validateConnectorTypeFeaturing
// Expect: error: Must be an accessible feature (use dot notation for nesting)
package P {
  class A { feature x; }
  class B { feature y; connector k from A::x to y; }
}
