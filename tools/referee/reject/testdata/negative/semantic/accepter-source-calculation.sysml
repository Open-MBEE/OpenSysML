// Invalid: A transition with an accepter must have a state as its source (SysML v2.1 8.3.18.9 validateTransitionUsageTriggerActions; the source is a calculation usage).
package AccepterSourceCalculation {
    item def Sig;
    state def S {
        entry; then idle;
        state idle;
        calc level { 1 }
        transition first level accept Sig then idle;
    }
}
