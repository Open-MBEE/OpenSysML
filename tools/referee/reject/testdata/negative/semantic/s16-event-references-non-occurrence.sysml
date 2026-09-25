// Invalid: An event occurrence usage must reference an occurrence usage (SysML 7.9 Occurrences; pilot validateEventOccurrenceUsageReferent).
package S16EventReferencesNonOccurrence {
    part def P;
    ref r : P;
    part p {
        event r;
    }
}
