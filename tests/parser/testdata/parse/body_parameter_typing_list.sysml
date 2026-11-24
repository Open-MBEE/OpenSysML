package P {
    part def A;
    part def B;
    port def C;
    part g : A;
    calc def F {
        in xs : A[*];
        xs->collect { in a : A, B :> g; a }
        xs->collect { in b : A, B[1] :> g; b }
        xs->collect { in c : ~C, ~C[1]; c }
    }
}
