// A condition reading both a nested feature the object redefines and a sibling
// it does not: the redefinition wins, the sibling keeps its declared value, so
// the condition holds only about the object.
package test {
    private import ScalarValues::Real;

    part def Leaf {
        attribute value : Real = 1.0;
        attribute other : Real = 2.0;
        constraint overSibling { value > other }
    }

    part def Mid {
        part leaf : Leaf;
    }

    part def Top {
        part mid : Mid;
    }

    part top : Top {
        part :>> mid {
            part :>> leaf {
                attribute :>> value = 99.0;
            }
        }
    }
}
