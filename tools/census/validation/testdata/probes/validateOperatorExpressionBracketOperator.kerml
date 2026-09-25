// Census: validateOperatorExpressionBracketOperator
// Expect: warning: is not an index in KerML
package P {
  feature xs : ScalarValues::Integer[0..*] ordered;
  feature firstElem = xs[1];
}
