// Invalid: every succession out of a decision node must have target multiplicity 0..1 (SysML v2 8.3.17.7; pilot validateDecisionNodeOutgoingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN09DecideOutgoingTargetMultiplicity {
    action def A {
        action a;
        decide d;
        action b;
        action c;
        first a then d;
        succession first d then [1] b;
        succession first d then [0..1] c;
    }
}
