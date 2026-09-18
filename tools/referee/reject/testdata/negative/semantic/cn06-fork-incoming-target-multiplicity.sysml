// Invalid: every succession into a control node must have target multiplicity 1..1 (SysML v2 8.3.17.6; pilot validateControlNodeIncomingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN06ForkIncomingTargetMultiplicity {
    action def A {
        action a;
        fork f;
        action b;
        action c;
        succession first a then [0..1] f;
        first f then b;
        first f then c;
    }
}
