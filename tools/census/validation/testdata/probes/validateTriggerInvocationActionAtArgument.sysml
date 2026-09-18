// Census: validateTriggerInvocationActionAtArgument
// Expect: error: an 'at' trigger's time must be a TimeInstantValue, found DurationValue: name a feature typed by TimeInstantValue or a clock reading such as `TimeOf(this)`
package P { private import ISQ::*; state def S { attribute delay : DurationValue; state a; state b; transition first a accept at delay then b; } }
