package test {
    private import ScalarValues::Real;
    part def Leaf { attribute value : Real default = 1.0; attribute other : Real default = 0.0; }
    part def Mid { part leaf : Leaf; attribute scale : Real default = 1.0; }
    part def Top {
        part mid : Mid;
        attribute factor : Real = 7.0;
        constraint readsLeaf { mid.leaf.value == 99.0 }
        constraint readsScale { mid.scale == 4.0 }
    }
    part top : Top {
        attribute :>> mid.leaf.value = 99.0;
        attribute :>> mid.scale = 4.0;
        part :>> mid.leaf { attribute :>> other = 5.0; }
    }
    part top2 : Top {
        part :>> mid {
            attribute :>> scale = 4.0;
            part :>> leaf { attribute :>> value = 99.0; attribute :>> other = 5.0; }
        }
    }
    part top3 : Top {
        attribute :>> mid.leaf.value = factor * 2.0;
        attribute :>> mid.scale = factor;
    }
    part def Wheel { attribute radius : Real default = 1.0; }
    part def Car { part wheels : Wheel[2]; }
    part car : Car {
        attribute :>> wheels.radius = 0.4;
    }
}
