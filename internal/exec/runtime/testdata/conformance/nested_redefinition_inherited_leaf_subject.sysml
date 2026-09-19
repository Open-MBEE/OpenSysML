// A redefinition at the intermediate level with the leaf inherited: the object
// types `mid` with a specialization, and a condition of the leaf definition is
// about the leaf object that specialization gives its value.
package test {
    private import ScalarValues::Real;

    part def Leaf {
        attribute value : Real = 1.0;
        constraint small { value < 10.0 }
    }

    part def Mid {
        part leaf : Leaf;
    }

    part def MidFast :> Mid {
        part :>> leaf {
            attribute :>> value = 50.0;
        }
    }

    part def Top {
        part mid : Mid;
    }

    part top : Top {
        part :>> mid : MidFast;
    }
}
