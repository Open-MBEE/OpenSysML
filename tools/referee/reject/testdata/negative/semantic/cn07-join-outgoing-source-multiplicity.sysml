// Invalid: every succession out of a control node must have source multiplicity 1..1 (SysML v2 8.3.17.6; pilot validateControlNodeOutgoingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN07JoinOutgoingSourceMultiplicity {
    action def A {
        action a;
        action b;
        join j;
        action c;
        first a then j;
        first b then j;
        succession first [0..1] j then c;
    }
}
