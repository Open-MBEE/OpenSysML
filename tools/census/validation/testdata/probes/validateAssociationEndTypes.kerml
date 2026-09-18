// Census: validateAssociationEndTypes
// Expect: error: An association end must have exactly one type
package P {
  class A; class B;
  assoc L { end a : A, B; end b : B; }
}
