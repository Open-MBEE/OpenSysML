// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  class C { feature x : ScalarValues::String; feature y : ScalarValues::Integer; binding x = y; }
}
