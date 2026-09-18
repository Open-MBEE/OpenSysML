// Census: validateClassifierDefaultSupertype
// Expect: error: Must directly or indirectly specialize Objects::Object
package P {
  class D;
  struct S ~ D;
}
