// Census: validateNamespaceDistinguishibility
// Case: violating
// Ours: w[name-conflict] Duplicate of other owned member name | w[name-conflict] Duplicate of other owned member name
// Pilot: w: Duplicate of other owned member name | w: Duplicate of other owned member name
package P {
  class A;
  class A;
}
