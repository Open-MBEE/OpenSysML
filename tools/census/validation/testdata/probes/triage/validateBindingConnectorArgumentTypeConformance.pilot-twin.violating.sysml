// Census: validateBindingConnectorArgumentTypeConformance
// Case: violating
// Ours: w[bound-feature-types] Bound features should have conforming types
// Pilot: w: Bound features should have conforming types
package P {
  part def Wheel;
  part w : Wheel;
  attribute x = w + 1;
}
