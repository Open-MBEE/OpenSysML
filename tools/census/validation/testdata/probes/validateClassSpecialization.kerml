// Census: validateClassSpecialization
// Expect: error: Cannot specialize data type or association
package P {
  datatype D;
  class C specializes D;
}
