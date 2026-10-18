// A bound member reading a chain through a package-level usage whose member's count
// the model leaves open is undetermined on the instantiated holder, as the chain read
// directly is: an unbound `[1..*]` bounds the count and picks no element (KerML 1.1
// §7.3.4.1, §7.4.12), so instantiating the holder makes up no value for it. A count a
// binding or a redefinition closes still evaluates, and so does a fixed one.
package test {
    private import ScalarValues::Real;

    part def Sub { attribute limit : Real = 5.0; }
    part def Req { part some : Sub[1..*]; attribute own : Real = some.limit; }
    part req : Req;

    part def Bound { part some : Sub[1..*] = (a, b); part a : Sub; part b : Sub; }
    part bound : Bound;
    part closed : Req { part :>> some[2]; }
    part def Fixed { part two : Sub[2]; part one : Sub; }
    part fixed : Fixed;

    part def Holder {
        attribute boundLimit : Real = req.some.limit;
        attribute viaOwn : Real = req.own;
        attribute fromBinding : Real[*] nonunique = bound.some.limit;
        attribute fromRedefinition : Real[*] nonunique = closed.some.limit;
        attribute fromFixed : Real[*] nonunique = fixed.two.limit;
        attribute fromOne : Real = fixed.one.limit;
    }
    part h : Holder;
}
