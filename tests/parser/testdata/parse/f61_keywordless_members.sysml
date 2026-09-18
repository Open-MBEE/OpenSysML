package F61KeywordlessMembers {
    locale "en_US" /* an anonymous comment carrying a locale */

    attribute def DerivedUnit;
    attribute scalarQuantities;
    attribute distance;
    attribute volume;
    attribute km;
    attribute L;

    T1 = 10.0;
    distancePerVolume :> scalarQuantities = distance / volume;
    kpl : DerivedUnit = km / L;

    attribute def Collection {
        attribute m : DerivedUnit;
        value :>> distance : DerivedUnit;
    }

    attribute def Size :> ScalarValues::Real;
    enum def SizeChoice :> Size {
        = 60.0;
        = 70.0;
    }

    calc def Mass {
        in v : Collection;
        v.m
    }

    analysis def FuelEconomy {
        subject v : Collection;
        v.m
    }
}
