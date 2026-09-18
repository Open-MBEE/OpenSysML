// Census: validateFunctionResultExpressionMembership
// Expect: error: Only one (owned or inherited) result expression is allowed
package P {
  class T;
  feature a : T;
  feature b : T;
  function F { a }
  function G specializes F { b }
}
