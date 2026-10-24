// Census: validateSpecificationSpecificNotConjugated
// Case: violating
// Ours: e[specialization-specific-conjugated] Conjugated type cannot be a specialized type
// Pilot: e: Conjugated type cannot be a specialized type
package P {
  class A; class B ~ A;
  specialization subtype B specializes A;
}
