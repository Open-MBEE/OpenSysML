// Invalid: every succession into a merge node must have source multiplicity 0..1 (SysML v2 8.3.17.13; pilot validateMergeNodeIncomingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN08MergeIncomingSourceMultiplicity {
    action def A {
        action a;
        action b;
        merge m;
        action c;
        succession first [1] a then m;
        first b then m;
        first m then c;
    }
}
