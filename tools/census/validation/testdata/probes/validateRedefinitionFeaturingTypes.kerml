// Census: validateRedefinitionFeaturingTypes
// Expect: error: A package-level feature cannot be redefined
package P {
  feature x;
  feature y redefines x;
}
