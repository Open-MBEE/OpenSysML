// A binding connector owned by a package makes its ends denote the same objects (KerML 1.0
// §7.4.6.3, §8.4.4.6.2): usages several bindings join — one equivalence class — denote the
// class's one object whichever member is read, `all Car` reaches it once however it is
// named, a valueless usage bound to a feature chain denotes the chain's object, and a
// binding written in another package joins the usages it names there.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Car;

    part a : Car;
    part b : Car;
    bind a = b;
    part e : Car;
    bind b = e;

    part c : Car;
    part d : Car { part w2 : Car; }
    bind c = d.w2;

    part x : Car;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Car) == 4,
            a === b,
            e === a,
            x === other::y,
            c === d.w2,
            (all Car)#(1) === a,
            (all Car)#(1) === e,
            (all Car)#(2) === c,
            (all Car)#(3) === d,
            (all Car)#(4) === x
        );
    }
}

package other {
    part y : test::Car;
    bind test::x = y;
}
