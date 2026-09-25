// Census: validateBehaviorSpecialization
// Expect: error: Cannot specialize structure
package P {
  struct S;
  behavior B specializes S;
}
