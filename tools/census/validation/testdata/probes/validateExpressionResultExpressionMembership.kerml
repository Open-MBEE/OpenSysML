// Census: validateExpressionResultExpressionMembership
// Expect: error: Only one (owned or inherited) result expression is allowed
package P {
  function F1 { 1 }
  function F2 { 2 }
  expr e : F1, F2;
}
