package Vehicles {
    import ScalarValues::*;
    part def Wheel {
        attribute diameter : Real = 16.0;
        attribute width : Real;
    }
    part def Car {
        part wheels : Wheel[4];
        attribute mass : Real;
    }
    part car : Car {
        part redefines wheels[4] {
            attribute redefines diameter = 17.0;
        }
    }
    part def Empty {}
}
package Behaviour {
    private attribute position : Natural[1] = (1..size(seq))->selectOne{in i; seq#(i) == value};
    constraint c { element.feature->forAll { in f : KerML::Feature; not f.isComposite } }
    assert constraint { x > 0 }
    part x : X; // trailing note
    state def S {
        state A {
            entry action a;
            do {
                action b;
                action c;
            } then B;
        }
        state B;
        transition first A accept s do {
            action d;
        } then B;
    }
    action def A {
        if x {
            action a1;
        } else {
            action a2;
        }
        loop {
            action l;
        } until done;
        for i in 1..3 {
            action f;
        }
    }
    doc /* about Behaviour */
    part def Y;
    attribute v = {in x; x + 1};
    item def I {
        attribute a = if cond ? 1 else 2;
    }
}
package Elided {
    action 'generate torque' { /* ... */ }
    assert not massLimitation {
        :>> mass = v3.mass;
        :>> massLimit = v4.mass;
    }
}
action def Chain {
    action image : Capture;
    then action detect : Find;
    then action transmit : Send;
}
state def Status {
    entry; then Off;
    state Off;
    state On {
        entry action e;
        then Off;
    }
}
