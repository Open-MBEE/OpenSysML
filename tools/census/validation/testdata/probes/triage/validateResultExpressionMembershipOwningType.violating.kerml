// Census: validateResultExpressionMembershipOwningType
// Case: violating
// Ours: e[syntax] expected a body member
// Pilot: e: no viable alternative at input '1' | e: extraneous input '}' expecting EOF
package P { class C { 1 + 1 } }
