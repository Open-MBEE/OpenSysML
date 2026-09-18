// Census: validateRedefinitionDirectionConformance
// Expect: error: Redefining feature must have a compatible direction
package P {
  class A { in feature x; }
  class B specializes A { out feature y redefines x; }
}
