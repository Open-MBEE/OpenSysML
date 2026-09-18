// Census: validateAssociationBinarySpecialization
// Expect: error: has 3 ends but specializes a binary link
package P {
  class A; class B; class C;
  assoc Bin specializes Links::BinaryLink { end a : A; end b : B; end c : C; }
}
