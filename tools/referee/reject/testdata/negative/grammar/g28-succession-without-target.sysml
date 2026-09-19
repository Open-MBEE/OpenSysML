// Invalid: SuccessionAsUsage requires a connector end after then (SysML.xtext:1033 SuccessionAsUsage).
package G28SuccessionWithoutTarget {
    action def A {
        action a1;
        action a2;
        succession s first a1 then ;
    }
}
