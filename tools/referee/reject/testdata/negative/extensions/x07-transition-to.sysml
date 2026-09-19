// Invalid: TransitionUsage connects with 'first ... then', never 'to' (SysML.xtext TransitionUsage).
package X07TransitionTo {
    state def S {
        state idle;
        state busy;
        transition idle to busy;
    }
}
