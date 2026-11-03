package Ctx {
    port def P;
    interface def I {
        end a : P;
        end b : ~P;
    }
    part def S { port p1 : P; port p2 : P; port p3 : P; }
    part def A { port p : ~P; }
    part def Site {
        part s : S;
        part a1 : A;
        part a2 : A;
        part a3 : A;
        interface i1 : I connect s.p1 to a1.p;
        interface i2 : I connect s.p2 to a2.p;
        interface i3 : I connect s.p3 to a3.p;
    }
    view v : StandardViewDefinitions::InterconnectionView {
        expose Ctx::Site;
        render Views::asInterconnectionDiagram;
    }
}
