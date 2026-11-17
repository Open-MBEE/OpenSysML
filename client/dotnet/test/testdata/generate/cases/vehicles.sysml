// The shapes typed generation has to get right: every library scalar and multiplicity, quantities,
// enumerations, nested definitions, redefinition under the same and another name, several bases,
// diamond ancestry, C# keywords, and usages typed by generated definitions.
package Cases {
    private import ScalarValues::*;
    private import CaseParts::*;

    part def Vehicle {
        attribute mass : Real default 1500.0;
        attribute label : String default "vehicle";
        attribute serial : Natural default 7;
        attribute ratio : Rational default 1 / 4;
        attribute electric : Boolean default false;
        attribute impedance : Complex[0..1];
        attribute fuel : Fuel default Fuel::petrol;
        attribute wheelbase : ISQ::LengthValue default 2.7 [SI::m];
        attribute size : Number[0..1];
        attribute tags : String[0..*];
        attribute nickname : String[0..1];
        part engine : Engine[0..1];

        part def Wheel {
            attribute radius : Real default 0.3;
        }

        part wheels : Wheel[4];
    }

    part def Car :> Vehicle {
        part :>> engine : BigEngine;
    }

    part def Truck :> Vehicle {
        part motor redefines engine : BigEngine[1];
        attribute payload : Real default 10000.0;
    }

    part def Registered {
        attribute plate : String default "UNREGISTERED";
    }

    part def RegisteredCar :> Car, Registered;

    part def Asset {
        attribute assetId : Integer default 1;
    }

    part def Leased :> Asset;

    part def Owned :> Asset {
        attribute :>> assetId default 2;
    }

    part def Pooled :> Leased, Owned {
        attribute shared : Boolean default true;
    }

    package 'event' {
        part def 'class' {
            attribute 'string' : String default "keyword";
        }
    }

    part car : Car;
    part truck : Truck;
    part registered : RegisteredCar;
    part pool : Pooled;
}
