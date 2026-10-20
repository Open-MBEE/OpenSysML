// Census: validateFeatureEndMultiplicity
// Case: violating
// Ours: w[end-feature-multiplicity] End feature must have multiplicity 1: an end relates exactly one thing p
// Pilot: w: End feature must have multiplicity 1
package P {
  class A; class B;
  assoc L { end feature a : A [0..*]; end feature b : B [1]; }
}
