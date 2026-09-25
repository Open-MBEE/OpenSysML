// A requirement satisfied by several occurrences of one definition is decided
// once for the occurrences reading only the definition's defaults and once more
// for each occurrence stating a value of its own; every satisfaction still
// reports its own verdict, in the order the assertions are declared.
package test {
    private import ScalarValues::Real;

    part def Spacecraft {
        attribute catalogMass : Real default = 180.0;
        attribute asBuiltMass : Real default = catalogMass;
        attribute margin : Real default = 1.1;
        attribute launchMass : Real = asBuiltMass * margin;
    }

    requirement def LaunchMassLimit {
        subject s : Spacecraft;
        attribute limit : Real = 205.0;
        require constraint { s.launchMass <= limit }
    }
    requirement launchLimit : LaunchMassLimit;

    part def Fleet {
        part sats : Spacecraft[5];
        part unit1 :> sats;
        part unit2 :> sats { attribute :>> asBuiltMass = 190.0; }
        part unit3 :> sats;
        part unit4 :> sats { attribute :>> asBuiltMass = 184.0; }
    }

    part fleet : Fleet {
        assert satisfy launchLimit by unit1;
        assert satisfy launchLimit by unit2;
        assert satisfy launchLimit by unit3;
        assert satisfy launchLimit by unit4;
    }
}
