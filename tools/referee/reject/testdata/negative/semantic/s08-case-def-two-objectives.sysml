// Invalid: A case definition may have at most one objective (SysML 7.21 Cases; pilot validateCaseDefinitionOnlyOneObjective).
package S08CaseDefTwoObjectives {
    requirement def R;
    case def CD {
        objective o1 : R;
        objective o2 : R;
    }
}
