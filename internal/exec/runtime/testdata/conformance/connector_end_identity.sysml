// A connector end is the feature it attaches to (KerML 1.0 §7.4.6, SysML v2
// §7.13.2): the object a typed connection's end holds is the connected port
// itself, so a value written on the port is read through the end, and two
// connections attached to different ports are told apart.
package test {
    private import ScalarValues::Real;

    port def P {
        attribute rate : Real = 3.0;
    }

    part def A {
        port p : P;
        port spare : P { :>> rate = 7.0; }
    }

    part def B {
        port q : P;
    }

    connection def Link {
        end source : P;
        end target : P;
    }

    part def Sys {
        part a : A;
        part b : B;
        connection link : Link connect a.p to b.q;
        connection spareLink : Link connect a.spare to b.q;
        // An untyped interface usage takes its type from the library
        // (Interfaces::Interface) and attaches the same way.
        interface iface connect a.p to b.q;
        // The anonymous form declares no name and behaves as the named one.
        connect a.p to b.q;
    }

    part sys : Sys;
}
