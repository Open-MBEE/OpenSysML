// Invalid: a merge node may have at most one outgoing succession (SysML v2 8.3.17.13; pilot validateMergeNodeOutgoingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN03MergeTwoOutgoing {
    action def A {
        action a;
        action b;
        merge m;
        action c;
        action d;
        first a then m;
        first b then m;
        first m then c;
        first m then d;
    }
}
