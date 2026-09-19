// An object no assertion is about is not shown valid: there is nothing to hold,
// so the validation decides nothing rather than answering valid. A named
// constraint declared without `assert` states no invariant and is not swept.
package test {
    private import ScalarValues::Real;

    part def Crate {
        attribute mass : Real = 4.0;
        constraint light { mass < 1.0 }
    }

    part crate : Crate;
}
