// `all D` over a part definition is the sequence of this run's objects of D: those the
// object being evaluated holds through its parts, nested ones included, in the order
// their usages are declared, whatever their multiplicity; a subtype's objects are of D too.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel {
        attribute diameter : Real;
    }
    part def SpareWheel :> Wheel;
    part def Axle {
        part wheels : Wheel[2] { attribute :>> diameter = 16.0; }
    }

    part vehicle {
        part frontAxle : Axle;
        part rearAxle : Axle;
        part spare : SpareWheel { attribute :>> diameter = 15.0; }

        attribute wheelCount : Natural = size(all Wheel);
        attribute axleCount : Natural = size(all Axle);
        attribute spareCount : Natural = size(all SpareWheel);
        attribute diameters : Real[*] nonunique = (all Wheel).diameter;
        attribute lastDiameter : Real = ((all Wheel)#(5)).diameter;

        assert constraint spareIsLast { (all Wheel)#(5) === spare }
        assert constraint firstAxleFirst { (all Axle)#(1) === frontAxle }
    }
}
