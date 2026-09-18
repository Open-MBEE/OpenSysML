// Census: validateFunctionResultParameterMembership
// Expect: error: Only one return parameter is allowed
package P {
  function F { return r1 : ScalarValues::Integer; return r2 : ScalarValues::Integer; }
}
