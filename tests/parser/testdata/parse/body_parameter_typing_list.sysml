package P {
    part def A;
    part def B;
    part g : A;
    calc def F {
        in xs : A[*];
        xs->collect { in a : A, B :> g; a }
    }
}
