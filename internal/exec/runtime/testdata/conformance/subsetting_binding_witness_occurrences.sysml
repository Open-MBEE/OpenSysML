// A binding on a nonunique subsetter draws a value as many times as its other end and the
// subsetted feature both hold it (KerML 1.1 §7.3.4.4, §7.4.6): `target` holds `source`'s two
// 1s, not a 1 and the pool's 2, and `mixed` holds `picks`'s 2, 1 and 2.
package test {
    private import ScalarValues::*;
    part def Values {
        attribute pool : Integer [3] nonunique = (2, 1, 1);
        attribute target : Integer [2..3] nonunique :> pool;
        attribute source : Integer [2] nonunique = (1, 1);
        binding [2] bind [0..1] target = [0..1] source;
        attribute codes : Integer [5] nonunique = (3, 2, 1, 2, 1);
        attribute mixed : Integer [3..5] nonunique :> codes;
        attribute picks : Integer [3] nonunique = (2, 1, 2);
        binding [3] bind [0..1] mixed = [0..1] picks;
    }
    part vals : Values;
}
