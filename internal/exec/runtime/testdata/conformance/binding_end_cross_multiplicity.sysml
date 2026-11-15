// A binding end's multiplicity is a cross multiplicity: how many of that end's values each value
// of the other end is linked to (KerML 1.1 §7.4.5.2, §8.4.4.5.1, §8.4.4.6.1), a value being linked
// to itself alone (§7.4.6.1). `[1]` ends make the ends hold the same values however many there
// are, so the valueless `mirror` holds `source`'s two; a `[1]` end opposite a `[0..1]` one makes
// the optional end's values a part of the required end's, so `some` in `whole` holds.
package test {
    private import ScalarValues::*;
    part def Values {
        attribute source : Integer [2] = (1, 2);
        attribute mirror : Integer [2];
        bind [1] source = [1] mirror;
        attribute whole : Integer [3] = (1, 2, 3);
        attribute some : Integer [2] = (1, 2);
        bind [0..1] some = [1] whole;
    }
    part vals : Values;
}
