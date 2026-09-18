// Census: validateClassSpecialization
// Expect: error: Cannot specialize attribute definition
package P {
  part def X; attribute def A;
  connection def C :> A { end x1 : X; end x2 : X; }
}
