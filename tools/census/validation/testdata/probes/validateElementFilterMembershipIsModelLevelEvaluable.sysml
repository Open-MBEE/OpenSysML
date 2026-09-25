// Census: validateElementFilterMembershipIsModelLevelEvaluable
// Expect: error: Must be model-level evaluable
package P {
  calc def Sum { in a : ScalarValues::Integer; return : ScalarValues::Integer = a; }
  package Q { filter Sum(1) == 1; }
}
