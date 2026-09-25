// Census: validateOperatorExpressionCastConformance
// Expect: warning: cast argument is typed by String, unrelated to the target Integer
package P {
  feature s : ScalarValues::String;
  feature n = s as ScalarValues::Integer;
}
