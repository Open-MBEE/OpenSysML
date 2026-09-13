// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  part def Wheel;
  part w : Wheel;
  attribute x = w + 1;
}
