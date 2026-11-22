// Invalid: A transition with an accepter must have a state as its source (SysML v2.1 8.3.18.9 validateTransitionUsageTriggerActions; the source is an inherited action reached through a feature chain).
package AccepterSourceInheritedAction {
    item def Sig;
    state def Base {
        action prepare;
    }
    state def S {
        entry; then idle;
        state idle : Base;
        transition first idle.prepare accept Sig then idle;
    }
}
