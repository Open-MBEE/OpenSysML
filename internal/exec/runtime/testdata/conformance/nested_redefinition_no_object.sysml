// A model declaring an object nothing materialized: the condition of the nested
// definition is about the declaration, which the redefinition on the undeclared
// object does not reach, so it holds on the declared default.
package test {
    private import ScalarValues::Real;

    part def Leaf {
        attribute value : Real = 1.0;
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
