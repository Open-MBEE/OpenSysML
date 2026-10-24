// A constraint def nested in a part def reads the part's features through its
// context parameter, which the asserted usage typed by it binds to the part:
// the form a migrated SysML v1 owned rule takes. Validating an instance
// evaluates the def's result for that part, holding for one and failing for
// another.
package test {
    private import ScalarValues::Real;

    part def Tank {
        attribute level : Real;
        attribute capacity : Real;
        constraint def 'Below capacity' {
            in ref context : Tank[1];
            context.level <= context.capacity
        }
        assert constraint 'below capacity' : 'Below capacity' { in ref :>> context = this; }
    }

    part full : Tank {
        attribute :>> level = 80.0;
        attribute :>> capacity = 100.0;
    }

    part overfull : Tank {
        attribute :>> level = 120.0;
        attribute :>> capacity = 100.0;
    }
}
