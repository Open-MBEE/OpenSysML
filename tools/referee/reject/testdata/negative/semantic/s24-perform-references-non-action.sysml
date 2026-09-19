// Invalid: A perform action usage must reference an action usage (SysML 7.16 Actions; pilot validatePerformActionUsageReference).
package S24PerformReferencesNonAction {
    action def AD;
    ref r : AD;
    part p {
        perform r;
    }
}
