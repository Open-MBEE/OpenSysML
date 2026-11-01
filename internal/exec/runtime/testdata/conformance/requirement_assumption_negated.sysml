// A negated requirement asserts its result false; with the assumption failing
// the result is true (`false implies …`), so the denial is violated, naming the
// assumption. A subject the assumption admits and the required constraint
// rejects is what the denial asserts, so `nrOn` holds.
package Repro {
    private import ScalarValues::*;
    part def Thing { attribute enabled : Boolean; attribute level : Integer; }
    requirement def LevelOk {
        subject t : Thing;
        assume constraint { t.enabled }
        require constraint { t.level == 1 }
    }
    part off : Thing { attribute :>> enabled = false; attribute :>> level = 5; }
    part on : Thing { attribute :>> enabled = true; attribute :>> level = 5; }
    assert not requirement nrOff : LevelOk { subject t = off; }
    assert not requirement nrOn : LevelOk { subject t = on; }
    attribute offDenied = nrOff.result;
    attribute onDenied = nrOn.result;
    calc probe { out one : Integer = 1; }
}
