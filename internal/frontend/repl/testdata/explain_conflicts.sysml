// Elements whose conditions conflict in different ways, for %explain: a plain
// contradiction, a domain bound, a divisor guard, an inherited condition, a
// denied element, and one that is satisfiable.
package Conflicts {
    private import ScalarValues::Integer;
    private import ScalarValues::Natural;

    constraint def Contradictory {
        in i : Integer;
        assert constraint { i > 8 }
        assert constraint { i < 3 }
    }

    constraint def Satisfiable {
        in i : Integer;
        assert constraint { i > 3 and i < 8 }
    }

    constraint def NatBound {
        in n : Natural;
        assert constraint { n < 0 }
    }

    constraint def ZeroDivisor {
        in a : Integer;
        in b : Integer;
        assert constraint { b == 0 }
        assert constraint { a / b == 1 }
    }

    requirement def Base {
        subject s;
        attribute x : Integer;
        require constraint { x > 10 }
    }

    requirement def Derived :> Base {
        require constraint { x < 1 }
    }

    constraint def Always {
        in i : Integer;
        assert constraint { i == i }
    }

    part rig {
        assert not constraint always : Always;
    }
}
