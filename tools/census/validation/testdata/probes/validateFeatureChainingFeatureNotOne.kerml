// Census: validateFeatureChainingFeatureNotOne
// Expect: error: Cannot have only one chaining feature
package P {
  class A { feature b; }
  feature a : A;
  feature c chains a;
}
