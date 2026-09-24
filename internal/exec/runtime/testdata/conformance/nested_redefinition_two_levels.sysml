// Two levels of nested parts, redefined on the object: a condition naming a
// nested feature is about the object, so the redefinition wins while a sibling
// the object does not redefine keeps the value its declaration gives it.
package test {
    private import ScalarValues::Real;

    part def Leaf {
        attribute value : Real = 1.0;
        attribute other : Real = 2.0;
        constraint small { value < 10.0 }
    }

    part def Mid {
        part leaf : Leaf;
        attribute scale : Real = 1.0;
    }

    part def Top {
        part mid : Mid;
        attribute leafValue : Real = mid.leaf.value;
        // A chain crossing two part boundaries, and one condition reading both a
        // redefined nested feature and an unredefined sibling.
        assert constraint leafRedefined { mid.leaf.value == 99.0 }
        assert constraint redefinedOverSibling { mid.leaf.value > mid.leaf.other }
        assert constraint intermediateRedefined { mid.scale == 4.0 }
    }

    part top : Top {
        part :>> mid {
            attribute :>> scale = 4.0;
            part :>> leaf {
                attribute :>> value = 99.0;
            }
        }
    }
}
