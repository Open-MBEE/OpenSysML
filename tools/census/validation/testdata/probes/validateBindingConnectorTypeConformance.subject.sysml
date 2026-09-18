// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  part def PD; item def ID;
  requirement def R { subject s : PD; requirement nested { subject n : ID; } }
}
