// A package-level action or state usage denotes one representative performance, as a
// package-level part denotes one representative object: it is a root of `all T`, a bare
// read is that object, `[0..*]` denotes none and `[2]` two. Reading it materializes the
// performance and starts what its type exhibits or performs, as reading a nested one does;
// it does not run the action's body, so `hits` stays at its initial value either way.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    action def Act {
        attribute hits : Integer := 0;
        first start;
        then assign hits := hits + 1;
        then done;
    }
    state def Sta { attribute n : Integer := 5; entry; then s0; state s0; }

    action a : Act;
    state s : Sta;
    action none : Act[0..*];
    action pair : Act[2];

    part sys {
        part x;
        action na : Act;
        state ns : Sta;
    }

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Act) == 4,
            size(all Sta) == 2,
            (all Act)#(1) === a,
            (all Act)#(4) === sys.na,
            (all Sta)#(1) === s,
            size(a) == 1,
            a == a,
            a != sys.na,
            a.hits == 0,
            sys.na.hits == 0,
            s.n == 5,
            size(none) == 0,
            size(pair) == 2,
            pair#(1) != pair#(2)
        );
    }
}
