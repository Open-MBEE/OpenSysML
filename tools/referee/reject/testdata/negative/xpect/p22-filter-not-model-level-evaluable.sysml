// Invalid: A filter condition must be model-level evaluable (SysML 7.24 checkPackageFilterCondition; pilot validateElementFilterMembershipIsModelLevelEvaluable, MetadataUsage_Invalid.sysml.xt:68).
package P22FilterNotModelLevelEvaluable {
    metadata def A {
        attribute z : ScalarValues::Boolean;
    }
    calc def f { in x : ScalarValues::Boolean; return : ScalarValues::Boolean; }
    filter f((as A).z);
}
