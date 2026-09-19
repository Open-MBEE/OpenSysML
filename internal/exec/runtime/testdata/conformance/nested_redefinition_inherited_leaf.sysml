// A redefinition at the intermediate level, whose leaf is inherited: the object
// redefines `mid` by typing it with a specialization, and a condition naming the
// leaf answers with the value that specialization gives it.
package test {
    private import ScalarValues::Real;

    part def Leaf {
        attribute value : Real = 1.0;
        attribute other : Real = 2.0;
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
        attribute leafValue : Real = mid.leaf.value;
        assert constraint inheritedLeafRedefinition { mid.leaf.value == 50.0 }
        assert constraint siblingKeepsDeclaration { mid.leaf.other == 2.0 }
    }

    part top : Top {
        part :>> mid : MidFast;
    }
}
