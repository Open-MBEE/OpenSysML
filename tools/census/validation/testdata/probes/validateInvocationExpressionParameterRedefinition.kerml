// Census: validateInvocationExpressionParameterRedefinition
// Expect: error: Must correspond to one input parameter of the invoked type
package P {
  function F { in x : ScalarValues::Integer; return : ScalarValues::Integer; x }
  feature r : ScalarValues::Integer = F(y = 1);
}
