package 'Vehicle Profile' {
    metadata def PartProperty;
}
package Fleet {
    part def Vehicle {
        attribute mass : ScalarValues::Real;
        attribute mode : Mode;
        part engine : Engine;
        ref part spare : Wheel;
        ref part lead : Wheel;
        constraint limit : MaxSpeed;
        part odd : Wheel {
            /* applied stereotype «ValueProperty» */
        }
        part axle : Wheel {
            /* applied stereotype «PartProperty»: ordering = rear */
        }
        part cabin : Engine {
            @'Vehicle Profile'::PartProperty;
        }
        attribute serial : ScalarValues::String {
            /* applied stereotype «ValueProperty» */
        }
    }
    part def Engine;
    part def Wheel;
    enum def Mode {
        eco;
        sport;
    }
    constraint def MaxSpeed {
        in attribute v : ScalarValues::Real;
        in ref part wheel : Wheel;
        v <= 120.0
    }
}
