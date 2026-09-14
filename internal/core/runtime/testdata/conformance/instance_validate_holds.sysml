// An object every assertion about which holds is valid: the asserted constraint
// its type declares holds on the object's own value, and the walk is complete.
package test {
    private import ScalarValues::Real;

    part def Wheel {
        attribute pressure : Real default = 32.0;
        assert constraint pressureOk { pressure >= 30.0 }
    }

    part spare : Wheel;
}
