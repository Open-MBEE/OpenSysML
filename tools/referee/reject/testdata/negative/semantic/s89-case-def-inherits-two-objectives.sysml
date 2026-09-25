// Invalid: A case definition inheriting an objective from each of two generals has two objectives (SysML 7.21 Cases; pilot validateCaseDefinitionOnlyOneObjective).
package S89CaseDefInheritsTwoObjectives {
    requirement def R;
    case def A { objective a : R; }
    case def B { objective b : R; }
    case def C :> A, B;
}
