// Member binding is one transaction: when the objective's actor binding is
// refused after its subject held the Ship as a Tanker, the Ship is left as it
// was, so the body's assertion finds it is no Tanker.
package test {
    private import ScalarValues::Real;

    part def Ship { attribute hullMass : Real; }
    part def Tanker :> Ship { attribute cargo : Real = 100.0; }
    part def Pilot;
    part ship : Ship { attribute :>> hullMass = 1000.0; }
    part pilot : Pilot;

    requirement def PilotedLimit {
        subject t : Tanker;
        actor pilots : Pilot[2];
        require constraint { t.hullMass + t.cargo < 2000.0 }
    }

    analysis def Check {
        subject ship : Ship;
        objective : PilotedLimit { subject = ship; actor :>> pilots = pilot; }
        assert constraint stillAShip { not (ship @ Tanker) }
        return picked : Ship = ship;
    }
    analysis check : Check {
        subject ship = test::ship;
    }
}
