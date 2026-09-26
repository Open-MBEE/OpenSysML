// Every occurrence of a definition holds the definition's defaults: a fleet of
// spacecraft declared with one multiplicity reads the same derived mass from
// each, and the constraint asserted about it is decided for all of them.
package test {
    private import ScalarValues::Real;

    part def Bus { attribute mass : Real default = 120.0; }
    part def Payload { attribute mass : Real default = 45.0; }

    part def Spacecraft {
        part bus : Bus;
        part payload : Payload;
        attribute margin : Real default = 1.1;
        attribute dryMass : Real = (bus.mass + payload.mass) * margin;
        assert constraint fits { dryMass < 200.0 }
    }

    part def Fleet {
        part sats : Spacecraft[4];
    }

    part fleet : Fleet;
}
