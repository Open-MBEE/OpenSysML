// Invalid: the guard of a guarded succession in an action body must be a Boolean expression (SysML v2 8.3.18.8; pilot validateTransitionFeatureMembershipGuardExpression, vacuous at the pin once the library types the guard, so this case lands in ours-only-rejects).
package S81GuardedSuccessionGuardNotBoolean {
    action def A {
        action a;
        action b;
        first a if "go" then b;
    }
}
