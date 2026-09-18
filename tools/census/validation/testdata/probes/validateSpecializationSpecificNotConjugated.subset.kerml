// Census: validateSpecializationSpecificNotConjugated
// Expect: error: Conjugated type cannot be a specialized type
package P {
  class A;
  feature f : A; feature g ~ f; feature h : A;
  specialization subset g subsets h;
}
