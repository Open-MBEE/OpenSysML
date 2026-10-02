// A binding connector owned by a package makes its ends denote the same objects (KerML 1.0
// §7.4.6.3, §8.4.4.6.2): two valueless usages it joins share one object, `all Car` reaching
// it once however either end is named, and a valueless usage bound to a feature chain
// denotes the chain's object.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Car;

    part a : Car;
    part b : Car;
    bind a = b;

    part c : Car;
    part d : Car { part w2 : Car; }
    bind c = d.w2;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Car) == 3,
            a === b,
            c === d.w2,
            (all Car)#(1) === a,
            (all Car)#(1) === b,
            (all Car)#(2) === c,
            (all Car)#(3) === d
        );
    }
}
