// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  class Wheel;
  feature w : Wheel;
  feature x = w + 1;
}
