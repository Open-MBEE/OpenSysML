package test {
    private import ScalarValues::Real;
    part def Leaf { attribute value : Real default = 1.0; }
    part def Mid { part leaf : Leaf; attribute scale : Real default = 1.0; }
    part def Top { part mid : Mid; attribute factor : Real = 7.0; }
    part def Wheel { attribute radius : Real default = 1.0; }
    part def Car { part wheels : Wheel[2]; }
    part def World {
        part shorthand : Top {
            attribute :>> mid.leaf.value = 99.0;
            attribute :>> mid.scale = 4.0;
        }
        part nestedBody : Top {
            part :>> mid {
                attribute :>> scale = 4.0;
                part :>> leaf { attribute :>> value = 99.0; }
            }
        }
        part outerRead : Top {
            attribute :>> mid.leaf.value = factor * 2.0;
            attribute :>> mid.scale = factor;
        }
        part c : Car {
            attribute :>> wheels.radius = 0.4;
        }
        constraint wheelsRedefined { c.wheels.radius == (0.4, 0.4) }
    }
}
