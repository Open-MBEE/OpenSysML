// Census: validateExpressionResultParameterMembership
// Expect: error: Only one return parameter is allowed
package P {
  expr e { return r1 : ScalarValues::Integer; return r2 : ScalarValues::Integer; }
}
