package test {
    port def P;
    part def A { port p : P; }
    part def B { port q : P; }

    part def Sys {
        part a : A;
        part b : B;
        binding bnd bind a.p = b.q;
        flow f from a.p to b.q;
    }

    part sys : Sys;
}
