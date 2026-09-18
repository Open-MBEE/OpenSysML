// Invalid: A transition with an accepter must have a state as its source (SysML 7.16 States; pilot validateTransitionUsageTriggerActions, TransitionUsage_invalid.sysml.xt:54).
package P34AccepterSourceNotState {
    item def A;
    state def S {
        entry action init;
        transition init accept A then s1;
        state s1;
    }
}
