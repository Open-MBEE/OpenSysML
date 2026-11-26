package Q { part def T; }
package P {
    action def A {
        attribute n : ScalarValues::Integer;
        while (n < 3) { private import Q::*; private alias W for Q::T; alias X for Q::T; part v : T; private part w : T; }
        loop { public import Q::*; alias L for Q::T; } until n > 3;
        for q in (1, 2) { private import Q::*; alias F for Q::T; }
        if n < 3 { private import Q::*; alias I for Q::T; } else { private import Q::*; alias E for Q::T; }
    }
}
