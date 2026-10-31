// `allTrue(assumptions()) implies allTrue(constraints())` reads nothing of the
// order a body declares its members in: an assumption written after the
// required constraint is judged first, so `off` satisfies the requirement
// though the required constraint it declares first reads false.
package Repro {
    private import ScalarValues::*;
    part def Thing { attribute enabled : Boolean; attribute level : Integer; }
    requirement def LevelOk {
        subject t : Thing;
        require constraint { t.level == 1 }
        assume constraint { t.enabled }
    }
    part off : Thing { attribute :>> enabled = false; attribute :>> level = 5; }
    part on : Thing { attribute :>> enabled = true; attribute :>> level = 5; }
    requirement levelOk : LevelOk;
    satisfy levelOk by off;
    satisfy levelOk by on;
    assert not satisfy levelOk by off;
    assert not satisfy levelOk by on;
}
