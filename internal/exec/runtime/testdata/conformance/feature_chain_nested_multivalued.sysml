// A chain whose intermediate features are both multi-valued visits every object
// of every object, in declaration order, and flattens one level per step.
package test {
    private import ScalarValues::Real;
    private import RealFunctions::*;

    part def Leaf {
        attribute v : Real;
    }

    part def Mid {
        part leaves : Leaf[*];
    }

    part def Top {
        part mids : Mid[*];
        attribute values : Real[*] = mids.leaves.v;
        attribute total : Real = sum(mids.leaves.v);
    }

    part top : Top {
        part m1 : Mid :> mids {
            part l1 : Leaf :> leaves {
                attribute :>> v = 1.0;
            }
            part l2 : Leaf :> leaves {
                attribute :>> v = 2.0;
            }
        }
        part m2 : Mid :> mids {
            part l3 : Leaf :> leaves {
                attribute :>> v = 4.0;
            }
        }
    }
}
