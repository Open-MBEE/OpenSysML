// Invalid: An exhibit state usage must reference a state usage (SysML 7.17 States; pilot validateExhibitStateUsageReference).
package S17ExhibitReferencesNonState {
    state def SD;
    ref r : SD;
    part p {
        exhibit r;
    }
}
