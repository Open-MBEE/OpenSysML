// Census: validateTriggerInvocationActionAfterArgument
// Expect: error: an 'after' trigger's delay must be a DurationValue, found Natural: write it with a duration unit (`after 5 [s]`) or name a feature typed by DurationValue
package P { state def S { state a; state b; transition first a accept after 5 then b; } }
