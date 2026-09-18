// A succession may be stated without its keyword, including as a package
// member (SuccessionAsUsage, SysML v2 8.2.2.13.3).
package Successions {
    part part1 {
        action action1;
    }
    requirement requirement1;

    first part1::action1 then requirement1;
}
