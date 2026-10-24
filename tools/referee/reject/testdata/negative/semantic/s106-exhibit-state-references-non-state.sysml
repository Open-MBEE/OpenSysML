// Invalid: A declared exhibit state usage must reference a state usage (SysML 7.17 States; pilot validateExhibitStateUsageReference).
package S106ExhibitStateReferencesNonState {
    part def PD;
    part def Owner {
        part p : PD;
        exhibit state e references p;
    }
}
