// Census: validateRedefinitionMultiplicityConformance
// Expect: warning: Redefining feature should not have smaller multiplicity lower bound
package P {
  class A { feature x [1..2]; }
  class B specializes A { feature y [0..2] redefines x; }
}
