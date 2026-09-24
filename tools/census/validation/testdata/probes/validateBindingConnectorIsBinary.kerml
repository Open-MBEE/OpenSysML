// Census: validateBindingConnectorIsBinary
// Expect: error: Binding connector must be binary
package P {
  class A;
  class C {
    feature x : A; feature y : A; feature z : A;
    binding b of x = y { end feature e references z; }
  }
}
