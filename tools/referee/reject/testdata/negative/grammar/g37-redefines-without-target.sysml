// Invalid: Redefines requires a redefined feature reference after the redefines keyword (SysML.xtext:429 Redefines).
package G37RedefinesWithoutTarget {
    part def D {
        attribute a;
    }
    part def E :> D {
        attribute redefines ;
    }
}
