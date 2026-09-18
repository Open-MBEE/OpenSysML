// Invalid: A case usage may have at most one objective (SysML 7.21 Cases; pilot validateCaseUsageOnlyOneObjective).
package S11CaseTwoObjectives {
    requirement def R;
    case c {
        objective o1 : R;
        objective o2 : R;
    }
}
