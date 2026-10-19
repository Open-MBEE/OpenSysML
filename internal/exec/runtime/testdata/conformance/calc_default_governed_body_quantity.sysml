// An object a body made of a quantity-valued attribute — `num` from the body, `mRef` from
// the `default` the body governs over — reads as the scalar quantity it states, the body's
// num under the default's unit, and its members read from that object.
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
    part centered : Centered;
    part nudged : Nudged;

    calc c {
        out offsets : LengthValue[2] = (centered.offset, nudged.offset);
        out centeredUnit = centered.offset.mRef;
        out nudgedNum = nudged.offset.num;
        out nudgedDimensions [0..*] = nudged.offset.dimensions;
    }
}
