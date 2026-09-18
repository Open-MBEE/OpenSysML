// Census: validateBindingConnectorTypeConformance
// Expect: warning: Bound features should have conforming types
package P {
  datatype C; datatype D;
  feature c : C;
  function F { return r : D; c }
}
