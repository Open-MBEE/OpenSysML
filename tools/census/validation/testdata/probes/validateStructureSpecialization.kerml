// Census: validateStructureSpecialization
// Expect: error: Cannot specialize behavior
package P {
  behavior B;
  struct S specializes B;
}
