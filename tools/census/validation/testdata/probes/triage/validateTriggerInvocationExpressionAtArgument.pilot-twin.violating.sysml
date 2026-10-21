// Census: validateTriggerInvocationExpressionAtArgument
// Case: violating
// Ours: e[trigger-at-time-instant] an 'at' trigger's time must be a TimeInstantValue, found DurationValue:
// Pilot: e: An at expression must be a TimeInstantValue.
package P { private import ISQ::*; state def S { attribute delay : DurationValue; state a; state b; transition first a accept at delay then b; } }
