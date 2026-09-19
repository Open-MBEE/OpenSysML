// An actor bound without `:>>` redefines the general's actor at its position,
// so `actor pilots = ...` is held to the definition's `Pilot[2]`: two pilots
// satisfy it, whether the usage keeps the actor's name or not.
package test {
    private import ScalarValues::Real;

    part def Ship { attribute hullMass : Real; }
    part def Pilot;
    part ship : Ship { attribute :>> hullMass = 1000.0; }
    part pilot : Pilot;
    part copilot : Pilot;

    requirement def PilotedLimit {
        subject t : Ship;
        actor pilots : Pilot[2];
        require constraint { t.hullMass < 2000.0 }
    }

    analysis def Check {
        subject ship : Ship;
        objective : PilotedLimit { subject = ship; actor pilots = (pilot, copilot); }
        objective renamed : PilotedLimit { subject = ship; actor crew = (pilot, copilot); }
        return picked : Ship = ship;
    }
    analysis check : Check {
        subject ship = test::ship;
    }
}
