// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  part def PD; item def ID;
  item i : ID;
  requirement def R { subject s : PD; }
  requirement r : R;
  satisfy r by i;
}
