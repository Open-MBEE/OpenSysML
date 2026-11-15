package test {
    part def M;
    state def Run {
        entry; then s0;
        state s0;
        transition first s0 then viaA;
        transition first s0 then viaB;
        state viaA;
        transition first viaA then fin;
        state viaB;
        transition first viaB then fin;
        state fin;
    }
}
