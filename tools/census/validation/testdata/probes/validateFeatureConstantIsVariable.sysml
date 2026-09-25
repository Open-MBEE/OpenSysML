// Census: validateFeatureConstantIsVariable
// Expect: error: Only a variable feature can be constant
package P {
  private import ScalarValues::*;
  attribute def A { constant attribute x : Integer; }
}
