// Invalid: A transition with an accepter must have a state as its source (SysML v2.1 8.3.18.9 validateTransitionUsageTriggerActions; a transition written without a source leaves the action declared before it).
package AccepterSourcePreviousAction {
    item def Sig;
    state def S {
        entry; then idle;
        state idle;
        action work;
        accept Sig then idle;
    }
}
