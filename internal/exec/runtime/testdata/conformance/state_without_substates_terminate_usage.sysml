// A terminate action usage (`action halt terminate worker;`) is no substate: the
// machine declaring it and no state is active in itself (SysML v2 §7.18.1).
package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    part def Mission {
        attribute started : Integer = 0;
        part worker;

        exhibit state idle {
            entry action note {
                assign started := started + 1;
            }
            action halt terminate worker;
        }
        assert constraint idleIsActive { idle.isActive }
    }
}
