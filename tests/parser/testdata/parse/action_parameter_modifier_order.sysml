package Probe {
    attribute def T;
    action def A {
        in derived p : T;
        in abstract ref q : T;
        action n {
            out variation r : T;
            out constant ref s : T;
        }
        loop {
            inout constant t : T;
            inout derived ref u : T;
        } until true;
        for i in (1, 2) {
            in variation v : T;
            in abstract ref w : T;
        }
        if true {
            out abstract x : T;
            out variation ref y : T;
        }
        if true { } else {
            inout derived z : T;
            inout constant ref aa : T;
        }
    }
}
