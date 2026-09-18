// Invalid: a join node may have at most one outgoing succession (SysML v2 8.3.17.11; pilot validateJoinNodeOutgoingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN02JoinTwoOutgoing {
    action def A {
        action a;
        action b;
        join j;
        action c;
        action d;
        first a then j;
        first b then j;
        first j then c;
        first j then d;
    }
}
