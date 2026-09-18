// Census: validateTriggerInvocationActionWhenArgument
// Expect: error: a 'when' trigger's condition must be Boolean, found Integer: compare the value (`when x > 3`) or name a feature typed by Boolean
package P { private import ScalarValues::*; state def S { attribute level : Integer; state a; state b; transition first a accept when level then b; } }
