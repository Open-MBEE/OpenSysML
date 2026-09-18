// Invalid: The argument of an `after` trigger must be a DurationValue (SysML 7.16 TriggerInvocationExpression; pilot validateTriggerInvocationActionAfterArgument).
package S47AfterTriggerNotDuration {
    state def S {
        state a;
        state b;
        transition first a accept after 5 then b;
    }
}
