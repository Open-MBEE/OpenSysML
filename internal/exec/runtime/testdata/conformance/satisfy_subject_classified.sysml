// The object a satisfaction supplies with `by` is held as a value of the
// requirement's subject: a Ship bound to a Tanker subject is classified as
// one, so the subject's own feature (cargo) answers the condition.
package test {
    private import ScalarValues::Real;

    part def Ship { attribute hullMass : Real; }
    part def Tanker :> Ship { attribute cargo : Real = 100.0; }
    part ship : Ship { attribute :>> hullMass = 1000.0; }
    part heavy : Ship { attribute :>> hullMass = 5000.0; }

    requirement def LadenLimit {
        subject t : Tanker;
        require constraint { t.hullMass + t.cargo < 2000.0 }
    }
    requirement laden : LadenLimit;

    part context {
        assert satisfy laden by ship;
        assert satisfy laden by heavy;
    }
}
