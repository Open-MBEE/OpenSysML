package NegatedSatisfactions {
    requirement def R1;

    part vehicle;
    part prototype;

    requirement r1 : R1;

    not satisfy r1 by prototype;
    assert not satisfy r1 by vehicle;

    part context {
        not satisfy r1 by prototype;
        not satisfy requirement r2 : R1 by prototype;
    }
}
