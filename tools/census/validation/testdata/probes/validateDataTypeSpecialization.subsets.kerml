// Census: validateDataTypeSpecialization
// Expect: error: Cannot specialize class or association
package P {
  class X;
  assoc A { end feature a : X; end feature b : X; }
  datatype D :> A;
}
