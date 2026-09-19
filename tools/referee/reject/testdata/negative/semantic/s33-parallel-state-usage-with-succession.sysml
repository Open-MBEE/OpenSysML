// Invalid: A parallel state usage cannot have successions or transitions (SysML 7.17 States; pilot validateStateUsageParallelSubactions).
package S33ParallelStateUsageWithSuccession {
    state s parallel {
        state s1;
        state s2;
        first s1 then s2;
    }
}
