// Census: validateInvocationExpressionNoDuplicateParameterRedefinition
// Expect: error: F binds parameter "x" twice
package P {
  class T;
  function F { in x : T; return r : T; }
  feature t : T;
  feature f : T = F(x = t, x = t);
}
