// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  private import ScalarValues::*;
  attribute def R :> Real;
  attribute a : R = 5;
  part def PD { attribute x : R; }
  part p : PD { attribute :>> x = 5; }
}
