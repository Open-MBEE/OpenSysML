package 'Empty Behavior Names' {
    part def P;
    part a : P;
    part b : P;
    connection '' connect a to b;
    connect '' ::> a to b;
    dependency '' from a to b;
    action def Act {
        action '';
        action b;
        first '' then b;
    }
    action def Forked {
        action b;
        fork '';
        first b then '';
    }
    action def Joined {
        action b;
        join '';
    }
    action def Decided {
        action b;
        decide '';
    }
    action def Looped {
        for '' in (1, 2) {
        }
    }
    state def S {
        state '';
        state t;
        transition x first t then '';
    }
    state def Named {
        state t;
        transition '' first t then t;
    }
    state def Entered {
        entry action '';
        state t;
    }
    enum def E {
        enum '';
        '   ';
    }
    calc def C {
        in '';
        return r = '';
    }
    requirement def R {
        subject '';
    }
    part p {
        perform action '';
        exhibit state '';
    }
}
