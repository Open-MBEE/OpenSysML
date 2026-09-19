// The object a satisfaction supplies with `by` answers to the subject's
// declaration: one the subject's type cannot hold is refused as a type
// mismatch before the conditions are evaluated.
package test {
    private import ScalarValues::Real;

    part def Ship { attribute hullMass : Real; }
    part def Buoy { attribute hullMass : Real = 1.0; }
    part buoy : Buoy;

    requirement def MassLimit {
        subject s : Ship;
        require constraint { s.hullMass < 2000.0 }
    }
    requirement massLimit : MassLimit;

    satisfy massLimit by buoy;
}
