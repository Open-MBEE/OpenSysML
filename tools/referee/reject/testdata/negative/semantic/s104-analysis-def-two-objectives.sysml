// Invalid: An analysis case definition may have at most one objective (SysML 7.21 Cases; pilot validateCaseDefinitionOnlyOneObjective).
package S104AnalysisDefTwoObjectives {
    requirement def R;
    analysis def AD {
        objective o1 : R;
        objective o2 : R;
    }
}
