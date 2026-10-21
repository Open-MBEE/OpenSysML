// A body valuing one feature of a quantity-valued attribute (`:>> offset { :>> num = 0; }`)
// governs over the `default` the redefined declaration wrote only for that feature: the
// default still supplies what the body leaves unvalued (`mRef`), so `dimensions`, derived
// from `mRef.dimensions`, reads and instantiation reports nothing. The redefined
// declaration is a package-level attribute, as the Geometry library writes its offsets.
package test {
    private import ISQ::*;
    private import SI::*;

    attribute offset : LengthValue [0..*] default 0 [m];

    part def Shape {
        attribute :>> offset [1];
    }
    part def Centered :> Shape {
        attribute :>> offset { attribute :>> num = 0; }
    }
    part def Shifted :> Shape {
        attribute :>> offset default 5 [m];
    }
    part def Nudged :> Shifted {
        attribute :>> offset { attribute :>> num = 3; }
    }
    part def Rig {
        part centered : Centered;
        part nudged : Nudged;
    }
}
