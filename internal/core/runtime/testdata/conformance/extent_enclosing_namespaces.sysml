// `all D` evaluated in a nested package reaches the objects every enclosing namespace's
// usages denote — the names the context resolves — whether or not a run read them before,
// in declaration order. A sibling package's usages are not this context's.
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
                    size(all Car) == 3,
                    (all Car)#(1) === sedan,
                    (all Car)#(2) === coupe,
                    (all Car)#(3) === hatchback
                );
            }
        }
    }

    package lot {
        part truck : Car;
    }
}
