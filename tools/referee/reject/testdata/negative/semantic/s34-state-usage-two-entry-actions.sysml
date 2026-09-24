// Invalid: A state usage may have at most one entry action (SysML 7.17 States; pilot validateStateUsageSubactionKind).
package S34StateUsageTwoEntryActions {
    state s {
        entry action a1;
        entry action a2;
    }
}
