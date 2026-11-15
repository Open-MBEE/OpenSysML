// The required end is the one whose values must cover the other's (KerML 1.1 §7.4.5.2,
// §8.4.4.5.1): `bind [1] few = [0..1] many` links each value of `many` to one of `few`, itself,
// and `many`'s 3 is no value of `few`. A connector declaring one link over an end reaching no
// object has none to declare, so it is violated rather than silent.
package test {
    private import ScalarValues::*;
    part def Hub { attribute spokes : Integer [2] = (1, 2); }
    part def Values {
        attribute few : Integer [2] = (1, 2);
        attribute many : Integer [3] = (1, 2, 3);
        bind [1] few = [0..1] many;
        part hubs : Hub [0..*];
        attribute mine : Integer [0..*] = (1, 2);
        binding [1] bind [0..1] hubs.spokes = [0..1] mine;
    }
    part vals : Values;
}
