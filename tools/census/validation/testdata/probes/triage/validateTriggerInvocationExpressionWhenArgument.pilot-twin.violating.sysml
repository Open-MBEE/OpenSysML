// Census: validateTriggerInvocationExpressionWhenArgument
// Case: violating
// Ours: e[trigger-when-boolean] a 'when' trigger's condition must be Boolean, found Integer: compare the
// Pilot: e: A when expression must be Boolean.
package P { private import ScalarValues::*; state def S { attribute level : Integer; state a; state b; transition first a accept when level then b; } }
