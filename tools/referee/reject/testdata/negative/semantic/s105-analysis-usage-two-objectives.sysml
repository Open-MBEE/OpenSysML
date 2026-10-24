// Invalid: An analysis case usage may have at most one objective (SysML 7.21 Cases; pilot validateCaseUsageOnlyOneObjective).
package S105AnalysisUsageTwoObjectives {
    requirement def R;
    analysis def AD;
    analysis a : AD {
        objective o1 : R;
        objective o2 : R;
    }
}
