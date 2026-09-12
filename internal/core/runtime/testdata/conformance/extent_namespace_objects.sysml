// `all D` evaluated outside any object — in a calc a package declares — reaches the objects
// that package's usages denote, materializing those whose type may hold a D, nested ones
// included, in declaration order — and no other: `all Garage` leaves the garage's tool
// unmaterialized. Usages of another package are not this context's.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel;
    part def Car {
        part wheels : Wheel[2];
    }
    part def Garage {
        part tool;
    }

    part garage : Garage;
    part sedan : Car;
    part spare : Wheel;
    part coupe : Car;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Wheel) == 5,
            size(all Car) == 2,
            size(all Garage) == 1,
            (all Car)#(1) === sedan,
            (all Car)#(2) === coupe,
            (all Wheel)#(3) === spare
        );
    }
}

package elsewhere {
    part truck : test::Car;
}
