// In a state body `first X then Y;` is a SuccessionAsUsage between two vertices,
// the keyword-less spelling of `succession first X then Y;`, and never an
// initial-node member: only an action body has an InitialNodeMember.
package P {
    state def M {
        entry; then a;
        state a;
        state b;
        state c {
            state c1;
            state c2;
            first c1 then c2;
        }
        first a then b;
        first b then c.c1;
        succession first c then done;
    }
    state def N {
        first start then idle;
        state idle;
        state busy;
        first idle if true then busy;
    }
    part def PD {
        exhibit state m : M {
            first a then b;
        }
    }
    action def A {
        first go;
        action go;
        first go then walk;
        action walk;
    }
}
