// Denying a requirement whose assumption fails is a violation: the requirement's
// result is `false implies …`, true, which is what `assert not` denies.
package Repro {
    private import ScalarValues::*;
    part def Thing { attribute enabled : Boolean; attribute level : Integer; }
    requirement def LevelOk {
        subject t : Thing;
        assume constraint { t.enabled }
        require constraint { t.level == 1 }
    }
    part off : Thing { attribute :>> enabled = false; attribute :>> level = 5; }
    assert not requirement nrOff : LevelOk { subject t = off; }
}
