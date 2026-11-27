package M {
    private import ScalarValues::*;
    part def Vehicle {
        attribute mass : Real = 1200.0;
        attribute note : String = "plain";
    }
    part car : Vehicle;
    calc def Twice { in x : Real; return : Real = x * 2; }
    constraint def Light { in v : Vehicle; v.mass < 2000.0 }
    requirement def MassReq { subject v : Vehicle; require constraint { v.mass < 2000.0 } }
    action drive {
        first start;
        then action accelerate;
        then action brake;
        then done;
    }
    state def Lamp {
        entry; then off;
        state off;
        state on;
        transition off_on first off then on;
    }
    view def Tree;
    view v : Tree { expose M::*; }
    attribute hint : String = "see; usage: %save <file>";
    doc /*
usage: %demo is how this package is used
*/
}
