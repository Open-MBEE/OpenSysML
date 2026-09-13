// `all D` evaluated outside any object — in a calc a package declares — reaches the objects
// every namespace-level usage of the model denotes, materializing those whose type may hold a
// D, nested ones included, in document then declaration order — and no other: `all Garage`
// leaves the garage's tool unmaterialized. The extent is of the type (KerML 1.0 §7.4.9.2), so
// `elsewhere::truck`, declared in a package `test` never imports, counts as a Car too.
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
            size(all Wheel) == 7,
            size(all Car) == 3,
            size(all Garage) == 1,
            (all Car)#(1) === sedan,
            (all Car)#(2) === coupe,
            (all Car)#(3) === elsewhere::truck,
            (all Wheel)#(3) === spare
        );
    }
}

package elsewhere {
    part truck : test::Car;
}
