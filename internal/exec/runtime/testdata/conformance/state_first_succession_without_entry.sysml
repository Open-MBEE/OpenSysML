// `first b then c;` orders b before c and enters neither: with no entry marker
// the exhibited machine has no initial state, and is reported as such.
package P {
    private import ScalarValues::*;

    state def M {
        attribute log : String = "";
        state b { entry assign log := log + "b "; }
        state c { entry assign log := log + "c "; }
        first b then c;
    }

    part def PD {
        exhibit state m : M;
    }
}
