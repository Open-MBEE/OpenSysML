// Census: validateBindingConnectorArgumentTypeConformance
// Case: violating
// Ours: w[bound-feature-types] Bound features should have conforming types
// Pilot: w: Bound features should have conforming types
package P {
  class Wheel;
  feature w : Wheel;
  feature x = w + 1;
}
