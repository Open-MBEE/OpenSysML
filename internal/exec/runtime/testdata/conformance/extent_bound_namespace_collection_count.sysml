// A namespace-level usage of two occurrences whose value yields three is bound to no value: the
// count violates its multiplicity, so reading it, and an extent reaching it, is refused with that
// violation rather than answered with three objects or two of them.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel;

    part wheels : Wheel[2] = (new Wheel(), new Wheel(), new Wheel());

    calc count { return : Natural = size(all Wheel); }
}
