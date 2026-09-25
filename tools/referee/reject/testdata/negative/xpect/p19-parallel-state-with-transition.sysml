// Invalid: A parallel state cannot have successions or transitions (SysML 7.16 validateStateDefinitionParallelSubactions; pilot StateUsage_invalid.sysml.xt).
package P19ParallelStateWithTransition {
    state def S parallel {
        state s1;
        state s2;
        first s1 then s2;
    }
}
