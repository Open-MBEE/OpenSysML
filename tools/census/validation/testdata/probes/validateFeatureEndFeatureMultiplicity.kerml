// Census: validateFeatureEndFeatureMultiplicity
// Expect: warning: End feature must have multiplicity 1
package P {
  class A; class B;
  assoc L { end feature a : A [0..*]; end feature b : B [1]; }
}
