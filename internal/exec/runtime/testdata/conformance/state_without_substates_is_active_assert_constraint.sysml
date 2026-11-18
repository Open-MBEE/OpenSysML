// A machine declaring no substate is itself active once started, so `isActive`
// of its usage holds though no substate is (SysML v2 §7.18.1).
package test {
    private import StateActivity::*;
    private import States::StateAction;

    part def Controller {
        exhibit state idle;
        exhibit state phases : StateAction;
        assert constraint idleIsActive { idle.isActive }
        assert constraint phasesIsActive { phases.isActive }
        assert constraint idleIsNotActiveWrong { not idle.isActive }
    }
}
