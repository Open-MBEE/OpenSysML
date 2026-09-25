// Invalid: A transition must own a succession whose target is an action or state (SysML 7.17 States; pilot validateTransitionUsageSuccession).
package S44TransitionTargetNotAction {
    part def PD;
    state def S {
        state s1;
        part p : PD;
        transition first s1 then p;
    }
}
