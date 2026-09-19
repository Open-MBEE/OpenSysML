// An actor bound without `:>>` is held to the definition's `Pilot[2]` as an
// explicit redefinition is: one pilot violates the lower bound and the
// objective is undecided.
package test {
    private import ScalarValues::Real;

    part def Ship { attribute hullMass : Real; }
    part def Pilot;
    part ship : Ship { attribute :>> hullMass = 1000.0; }
    part pilot : Pilot;

    requirement def PilotedLimit {
        subject t : Ship;
        actor pilots : Pilot[2];
        require constraint { t.hullMass < 2000.0 }
    }

    analysis def Check {
        subject ship : Ship;
        objective : PilotedLimit { subject = ship; actor pilots = pilot; }
        return picked : Ship = ship;
    }
    analysis check : Check {
        subject ship = test::ship;
    }
}
