// A requirement usage whose subject fails the assumption holds: `false implies
// allTrue(constraints())` is true, so the required constraint reading false on
// `off` is not a violation (SysML v2 §7.21.2, Requirements::RequirementConstraintCheck).
package Repro {
    private import ScalarValues::*;
    part def Thing { attribute enabled : Boolean; attribute level : Integer; }
    requirement def LevelOk {
        subject t : Thing;
        assume constraint { t.enabled }
        require constraint { t.level == 1 }
    }
    part off : Thing { attribute :>> enabled = false; attribute :>> level = 5; }
    requirement offReq : LevelOk { subject t = off; }
}
