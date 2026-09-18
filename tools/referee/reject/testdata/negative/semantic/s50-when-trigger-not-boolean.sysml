// Invalid: The argument of a `when` trigger must be Boolean (SysML 7.16 TriggerInvocationExpression; pilot validateTriggerInvocationActionWhenArgument).
package S49WhenTriggerNotBoolean {
    private import ScalarValues::*;
    state def S {
        attribute level : Integer;
        state a;
        state b;
        transition first a accept when level then b;
    }
}
