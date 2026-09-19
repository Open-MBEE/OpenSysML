// `all D` evaluated in a nested package reaches the objects every namespace-level usage of
// the model denotes — those of the enclosing namespaces and of a sibling package alike, since
// the extent is the set of things the type classifies (KerML 1.0 §7.3.2.1, §7.4.9.2) and not
// what the context resolves — whether or not a run read them before, in declaration order.
package fleet {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Car;
    part sedan : Car;

    package depot {
        part coupe : Car;

        package bay {
            part hatchback : Car;

            calc extents {
                return : Boolean[*] nonunique = (
                    size(all Car) == 4,
                    (all Car)#(1) === sedan,
                    (all Car)#(2) === coupe,
                    (all Car)#(3) === hatchback,
                    (all Car)#(4) === lot::truck
                );
            }
        }
    }

    package lot {
        part truck : Car;
    }
}
