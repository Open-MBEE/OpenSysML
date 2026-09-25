// A fleet's per-unit values are a table of subsetting members, each restating
// what its unit measured; the other occurrences keep the definition's catalog
// values. The requirement each unit carries reads the unit's own row where one
// is stated and the catalog value where none is, so its verdict differs only
// where the as-built mass does.
package test {
    private import ScalarValues::Real;
    private import ScalarValues::String;

    part def Spacecraft {
        attribute serial : String default = "catalog";
        attribute catalogMass : Real default = 180.0;
        attribute asBuiltMass : Real default = catalogMass;
        attribute growth : Real = asBuiltMass - catalogMass;
        requirement massGrowth {
            attribute g : Real = growth;
            require constraint { g <= 5.0 }
        }
    }

    part def Fleet {
        part sats : Spacecraft[6];
        part unit1 :> sats {
            attribute :>> serial = "SN-001";
            attribute :>> asBuiltMass = 183.5;
        }
        part unit2 :> sats {
            attribute :>> serial = "SN-002";
            attribute :>> asBuiltMass = 191.0;
        }
        part unit3 :> sats {
            attribute :>> serial = "SN-003";
        }
    }

    part fleet : Fleet;
}
