// Census: validateFeatureEndNotDerivedAbstractCompositeOrPortion
// Expect: error: End feature cannot be derived, abstract, composite or portion
package P {
  part def A;
  connection def K { end derived a : A; end b : A; }
}
