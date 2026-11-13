package Ends {
    port def P;
    part def Unit {
        port p : P;
    }
    part def Pair {
        part u1 : Unit;
        part u2 : Unit;
    }
    connection def Link {
        end part supplier : Unit;
        end part consumer : Unit;
    }
    interface def Iface {
        end port a : P;
        end port b : P;
    }
    part def Vehicle {
        port q : P;
        port r : P;
        part x : Unit;
        part y : Unit;
        part m : Pair;
        part n : Pair;
        connection plain : Link connect q to r;
        connection chained connect x.p to y.p;
        connection deep connect m.u1.p to n.u2.p;
        connection named : Link connect supplier ::> x to consumer ::> y;
        connection namedChain connect (a ::> x.p, b ::> y.p);
        connection body : Link {
            end part supplier ::> x;
            end part consumer ::> y;
        }
        interface wired : Iface connect q to r;
        binding bound bind x.p = y.p;
        bind q = r;
        connect x to y;
        part inner {
            port q : P;
            part z : Unit;
            connection nested connect q to z.p;
        }
    }
    part def Special :> Vehicle {
        connection refined :>> plain;
    }
}
