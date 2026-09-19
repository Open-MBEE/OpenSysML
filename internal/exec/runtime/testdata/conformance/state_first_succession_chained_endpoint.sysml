// A succession end written as a feature chain (`c.c1`) names the nested vertex
// the qualified spelling (`c::c1`) does, in both the `succession` and the
// keyword-less `first ... then ...;` form, at either end of the edge. Its first
// segment reaches a vertex nested anywhere in the machine (`c1.deep`), as a
// qualified end's does.
package P {
    private import ScalarValues::*;

    state def M {
        attribute log : String = "";

        entry; then a;

        state a { entry assign log := log + "a "; }
        state b {
            state b1 { entry assign log := log + "b1 "; }
            state b2 { entry assign log := log + "b2 "; }
        }
        state c {
            state c1 {
                state deep { entry assign log := log + "deep "; }
            }
        }

        succession first a then b.b1;
        first b.b1 then b::b2;
        first b.b2 then c1.deep;
        succession first c.c1.deep then done;
    }

    part def PD {
        exhibit state m : M;
    }

    part pd : PD;
}
