// Invalid: a decision node may have at most one incoming succession (SysML v2 8.3.17.7; pilot validateDecisionNodeIncomingSuccessions, unimplemented at the pin, so this case lands in ours-only-rejects).
package CN04DecideTwoIncoming {
    private import ScalarValues::Boolean;
    action def A {
        in attribute go : Boolean;
        action a;
        action b;
        decide d;
        action c;
        action e;
        first a then d;
        first b then d;
        first d if go then c;
        first d if not go then e;
    }
}
