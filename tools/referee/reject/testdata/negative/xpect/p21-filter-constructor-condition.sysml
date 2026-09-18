// Invalid: A filter condition must have a Boolean result (SysML 7.24 checkPackageFilterCondition; pilot validateElementFilterMembershipIsBoolean, MetadataUsage_Invalid.sysml.xt:74).
package P21FilterWithoutBooleanResult {
    metadata def A {
        attribute x : ScalarValues::Integer;
    }
    filter new A(null, 1);
}
