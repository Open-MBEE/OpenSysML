// Census: validateRedefinitionEndConformance
// Expect: error: Redefining feature must be an end feature
package P {
  class A; class B;
  assoc L { end a : A; end b : B; }
  assoc M specializes L { feature c redefines a; }
}
