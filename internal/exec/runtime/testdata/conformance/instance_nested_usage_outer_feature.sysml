// A nested usage naming a feature of its enclosing type, qualified or through a
// sibling chain, reads the enclosing object's value of it; a qualified name of a
// declaration outside the object still reads that declaration.
package test {
    private import ScalarValues::*;

    package Consts {
        attribute unit : Real = 7.0;
    }

    item def Frame {
        attribute span : Real;
        attribute gap : Real;
        attribute scale : Real = Consts::unit;
        item bar { attribute len : Real = Frame::span; }
        item strut { attribute len : Real = bar.len + Frame::gap; }
    }

    part def Rig {
        item frame : Frame {
            :>> span = 4.0;
            :>> gap = 1.0;
        }
    }
}
