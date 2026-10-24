// A derived attribute of a nested part reads an attribute of the enclosing part through a
// chain of redefinitions (`Curve::size`, redefined by `Round` and valued by `Wheel`). Where
// the chain ends in a value the derivation reads it; where nothing values it, the derived
// attribute holds no value — the typed no-value error on a read — and instantiation reports
// nothing, as it reports nothing for the unvalued attribute itself.
package test {
    private import ISQ::*;
    private import SI::*;

    attribute size : LengthValue [0..*];

    part def Curve {
        attribute :>> size [1];
        part edge {
            attribute length : LengthValue = Curve::size * 2;
        }
    }
    part def Round :> Curve {
        attribute :>> size;
    }
    part def Wheel :> Round {
        attribute :>> size = 60 [cm];
    }
    part def Rig {
        part wheel : Wheel;
        part bare : Round;
    }
}
