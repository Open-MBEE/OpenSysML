// An object a body made of a quantity-valued attribute — `num` from the body, `mRef` from
// the `default` the body governs over — reads as the scalar quantity it states, the body's
// num under the default's unit, and its members read from that object: the library's and
// the ones a specialization adds (`tag`), through the quantity a usage or a chain answers.
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

    attribute def Tagged :> LengthValue {
        attribute tag : ScalarValues::Integer;
    }
    attribute tagged : Tagged { :>> num = 2; :>> mRef = m; :>> tag = 7; }
    part def Holder :> Shape { attribute :>> offset : Tagged { :>> num = 4; :>> tag = 9; } }
    part holder : Holder;

    calc c {
        out offsets : LengthValue[2] = (centered.offset, nudged.offset);
        out centeredUnit = centered.offset.mRef;
        out nudgedNum = nudged.offset.num;
        out nudgedDimensions [0..*] = nudged.offset.dimensions;
        out taggedValue : Tagged = tagged;
        out taggedTag = tagged.tag;
        out taggedNum = tagged.num;
        out taggedElements [1..*] = tagged.elements;
        out heldValue : Tagged = holder.offset;
        out heldTag = holder.offset.tag;
    }
}
