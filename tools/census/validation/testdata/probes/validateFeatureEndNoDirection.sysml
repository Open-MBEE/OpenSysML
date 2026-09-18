// Census: validateFeatureEndNoDirection
// Expect: error: End feature cannot have direction
package P {
  part def A;
  connection def K { end in a : A; end b : A; }
}
