package Demo {
    part def Base { attribute mass; }
    part def Vehicle :> Base {
        attribute mass2;
        part wheel[4];
        port p;
        ref item cargo;
        action a;
        constraint c { mass2 > 0 }
        state s;
        calc def K { in x; return y = x + 1; }
    }
    part v : Vehicle;
}
