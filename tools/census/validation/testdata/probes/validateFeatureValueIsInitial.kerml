// Census: validateFeatureValueIsInitial
// Expect: error: Initialized feature must be variable
package P {
  datatype D;
  class C { feature f : D := null; }
}
