// In a state body, `first a then b;` is a succession between two states — a
// completion transition, as `succession first a then b;` is — not an entry into b.
package P {
    private import ScalarValues::*;

    state def M {
        attribute log : String = "";
        entry; then a;
        state a { entry assign log := log + "a "; }
        state b { entry assign log := log + "b "; }
        state c { entry assign log := log + "c "; }
        first a then b;
        first b then c;
        first c then done;
    }

    part def PD {
        exhibit state m : M;
    }
}
