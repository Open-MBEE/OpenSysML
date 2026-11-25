// An end's multiplicity counts its values linked per value of the other end (KerML 1.1 §7.4.5.2, §8.4.4.6.1):
// `[1]` ends hold the same values, and a `[1]` end opposite a `[0..1]` one holds the optional end's values.
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
