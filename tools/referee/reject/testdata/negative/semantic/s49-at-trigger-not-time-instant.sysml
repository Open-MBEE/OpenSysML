// Invalid: The argument of an `at` trigger must be a TimeInstantValue (SysML 7.16 TriggerInvocationExpression; pilot validateTriggerInvocationActionAtArgument).
package S48AtTriggerNotTimeInstant {
    private import ISQ::*;
    private import SI::*;
    state def S {
        attribute delay : DurationValue;
        state a;
        state b;
        transition first a accept at delay then b;
    }
}
