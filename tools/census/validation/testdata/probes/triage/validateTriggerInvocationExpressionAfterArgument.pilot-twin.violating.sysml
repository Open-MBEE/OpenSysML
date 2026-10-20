// Census: validateTriggerInvocationExpressionAfterArgument
// Case: violating
// Ours: e[trigger-after-duration] an 'after' trigger's delay must be a DurationValue, found Natural: write
// Pilot: e: An after expression must be a DurationValue.
package P { state def S { state a; state b; transition first a accept after 5 then b; } }
