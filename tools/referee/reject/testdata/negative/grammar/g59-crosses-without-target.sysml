// Invalid: A crossing feature requires a crossed feature after crosses (SysML.xtext:458 OwnedCrossSubsetting).
package G59CrossesWithoutTarget {
    part def P {
        part a;
        part b crosses ;
    }
}
