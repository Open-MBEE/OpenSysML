package Plant {
    private import ScalarValues::*;
    part def Tank {
        attribute level : Integer = 0;
        attribute capacity : Integer = 5;
        constraint low { level < 2 }
        constraint capped { level <= capacity }
        action fill {
            first start;
            fork split;
            action a { assign level := 1; }
            action b { assign level := 2; }
            join sync;
            done;
            succession first start then split;
            succession first split then a;
            succession first split then b;
            succession first a then sync;
            succession first b then sync;
            succession first sync then done;
        }
    }
    part tank : Tank;
}
