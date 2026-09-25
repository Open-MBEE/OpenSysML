// A condition of a nested definition, checked with the object present: the
// object redefines the nested feature two levels down, so the check is about
// that object's value rather than the declared default.
package test {
    private import ScalarValues::Real;

    part def Leaf {
        attribute value : Real = 1.0;
        attribute other : Real = 2.0;
        constraint small { value < 10.0 }
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
