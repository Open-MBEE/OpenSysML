// Census: validateFunctionResultExpressionMembership
// Expect: error: Only one (owned or inherited) result expression is allowed
package P {
  function F1 { 1 }
  function F2 { 2 }
  function F3 specializes F1, F2;
}
