// Invalid: a fork node may have at most one incoming succession (SysML v2 8.3.17.8; pilot validateForkNodeIncomingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN01ForkTwoIncoming {
    action def A {
        action a;
        action b;
        fork f;
        action c;
        action d;
        first a then f;
        first b then f;
        first f then c;
        first f then d;
    }
}
