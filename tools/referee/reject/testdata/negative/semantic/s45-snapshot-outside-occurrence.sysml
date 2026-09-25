// Invalid: A portion (snapshot/timeslice) usage must be owned by an occurrence definition or usage (SysML 7.10 Occurrences; pilot validateOccurrenceUsageIsPortion).
package S45SnapshotOutsideOccurrence {
    part def PD;
    snapshot part s : PD;
}
