// A quantity attribute nothing values reads as unset and reports nothing: the derived
// features of the quantity value it does not hold are not evaluated.
package test {
    private import ISQ::*;

    part def Engine {
        attribute mass :> ISQ::mass;
        attribute frequency :> ISQ::frequency;
    }

    part def Rocket {
        part engine : Engine;
        attribute mass :> ISQ::mass;
    }
}
