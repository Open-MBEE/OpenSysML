// A modifier followed by a kind keyword and no name declares an anonymous usage of
// that kind: the keyword is never read as the name (SysML.xtext IndividualUsage).
package AnonymousModifiedUsages {
    part def Vehicle;
    occurrence def Flight;
    individual part : Vehicle;
    individual item : Integer;
    ref item : Integer;
    snapshot part : Vehicle;
    timeslice item : Integer;
    individual named : Vehicle;
    individual part vehicle : Vehicle;
}
