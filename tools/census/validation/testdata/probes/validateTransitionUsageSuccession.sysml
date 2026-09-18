// Census: validateTransitionUsageSuccession
// Expect: error: is not a state or pseudostate
package S44TransitionTargetNotAction {
    part def PD;
    state def S {
        state s1;
        part p : PD;
        transition first s1 then p;
    }
}
