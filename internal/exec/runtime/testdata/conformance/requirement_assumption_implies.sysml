// A requirement check is `allTrue(assumptions()) implies allTrue(constraints())`
// (Requirements::RequirementConstraintCheck::result), so a subject failing the
// assumption satisfies the requirement whatever the required constraint reads,
// and only a subject the assumption admits is held to the required constraint.
// A negated satisfaction asserts the result false, so it fails on that subject.
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
    part offOk : Thing { attribute :>> enabled = false; attribute :>> level = 1; }
    part onOk : Thing { attribute :>> enabled = true; attribute :>> level = 1; }
    requirement levelOk : LevelOk;
    satisfy levelOk by off;
    satisfy levelOk by on;
    satisfy levelOk by offOk;
    satisfy levelOk by onOk;
    assert not satisfy levelOk by off;
    assert not satisfy levelOk by on;
}
