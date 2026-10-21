// Census: validateElementFilterMembershipConditionIsBoolean
// Case: violating
// Ours: e[filter-not-boolean] Must have a Boolean result
// Pilot: e: Must have a Boolean result
package P {
  package Q { filter 1; }
}
