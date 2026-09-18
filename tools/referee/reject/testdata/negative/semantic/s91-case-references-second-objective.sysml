// Invalid: A case usage inherits an objective from its definition and another from the case it reference-subsets, so it has two (SysML 7.21 Cases; pilot validateCaseUsageOnlyOneObjective).
package S91CaseReferencesSecondObjective {
    requirement def R;
    case def A { objective a : R; }
    case b { objective b : R; }
    case c : A ::> b;
}
