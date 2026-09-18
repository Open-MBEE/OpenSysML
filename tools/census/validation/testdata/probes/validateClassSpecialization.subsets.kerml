// Census: validateClassSpecialization
// Expect: error: Cannot specialize data type or association
package P {
  class X;
  assoc A { end feature a : X; end feature b : X; }
  class C :> A;
}
