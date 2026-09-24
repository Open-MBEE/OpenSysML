package Q {
    action def A;
    state def S;
    use case def U;
    requirement def R;
    constraint def C;
    part def Ctx {
        action a1 : A;
        action a2 : A;
        state s1 : S;
        state s2 : S;
        use case u1 : U;
        requirement r1 : R;
        constraint c1 : C;
    }
    part def Sub {
        action sa : A;
        state ss : S;
    }
    part ctx : Ctx {
        part sub : Sub;
        perform action pa : A;
        perform a1;
        perform sub.sa :>> a2;
        exhibit state es : S;
        exhibit s1;
        exhibit sub.ss :>> s2;
        include use case iu : U;
        include u1;
        satisfy requirement sr : R;
        satisfy r1;
        assert constraint ac : C;
        assert c1;
    }
}
