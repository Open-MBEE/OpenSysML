package States {
    action def A;
    action x;
    state def S {
        entry action e : A;
        do x;
        exit action;
        state s1;
        state s2;
        transition first s1 do action f : A then s2;
        transition first s2 do action then s1;
        do action ops {
            first start;
            then action a : A;
            then done;
        }
    }
}
