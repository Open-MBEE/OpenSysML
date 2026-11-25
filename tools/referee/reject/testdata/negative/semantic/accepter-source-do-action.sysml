// Invalid: A transition with an accepter must have a state as its source (SysML v2.1 8.3.18.9 validateTransitionUsageTriggerActions; the source is a do action).
package AccepterSourceDoAction {
    item def Sig;
    state def S {
        entry; then idle;
        state idle;
        do action monitor;
        transition first monitor accept Sig then idle;
    }
}
