// Invalid: A declared perform action usage must reference an action usage (SysML 7.16 Actions; pilot validatePerformActionUsageReference).
package S107PerformActionReferencesNonAction {
    part def PD;
    part def Owner {
        part p : PD;
        perform action f references p;
    }
}
