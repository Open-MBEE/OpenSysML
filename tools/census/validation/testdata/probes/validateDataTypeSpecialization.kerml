// Census: validateDataTypeSpecialization
// Expect: error: Cannot specialize class or association
package P {
  class C;
  datatype D specializes C;
}
