// Census: validateTypeAtMostOneConjugator
// Expect: error: Cannot have more than one conjugator
package P {
  classifier A; classifier B;
  classifier C ~A ~B;
}
