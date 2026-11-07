// A binding draws no more occurrences of a value than the subsetter may hold: a unique `once`
// holds `source`'s 1 once, and `scarce` the one 1 its pool holds; the rest of each lower bound
// comes from the pool and the binding is violated, as no value is made up (KerML 1.1 §7.3.4.4).
package test {
    private import ScalarValues::*;
    part def Values {
        attribute source : Integer [2] nonunique = (1, 1);
        attribute pool : Integer [3] nonunique = (2, 1, 1);
        attribute once : Integer [2..3] :> pool;
        binding [2] bind [0..1] once = [0..1] source;
        attribute few : Integer [3] nonunique = (2, 1, 3);
        attribute scarce : Integer [2..3] nonunique :> few;
        binding [2] bind [0..1] scarce = [0..1] source;
    }
    part vals : Values;
}
