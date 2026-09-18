// Invalid: A satisfaction subject requires a feature reference after by (SysML.xtext:2135 SatisfactionFeatureValue).
package G58SatisfyWithoutReference {
    requirement def R;
    part p {
        satisfy requirement : R by ;
    }
}
