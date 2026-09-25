// A default that is an expression over a sibling feature follows the sibling
// on each occurrence: a unit restating its bus mass derives its own dry mass,
// and the constraint asserted about it fails for that unit alone, while the
// units keeping the definition's default all read the definition's value.
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
        part heavy :> sats {
            part :>> bus { attribute :>> mass = 160.0; }
        }
    }

    part fleet : Fleet;
}
