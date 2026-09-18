// Census: validateReturnParameterMembershipOwningType
// Expect: error: Return parameter membership not allowed
package P {
  datatype Count;
  classifier Counter { return total : Count; }
}
