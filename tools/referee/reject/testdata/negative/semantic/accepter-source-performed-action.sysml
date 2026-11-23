// Invalid: A transition with an accepter must have a state as its source (SysML v2.1 8.3.18.9 validateTransitionUsageTriggerActions; the source is a performed action).
package AccepterSourcePerformedAction {
    item def Sig;
    action def Act;
    state def S {
        entry; then idle;
        state idle;
        perform action work : Act;
        transition first work accept Sig then idle;
    }
}
