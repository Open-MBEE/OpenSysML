package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // An assert constraint on the part exhibiting the machine reads the
    // activity of the machine's states through the exhibited state usage:
    // once the machine has reached `ready` and awaits events, `ready` is
    // active and `armed` is not; the machine usage itself is active while it
    // holds an active configuration, awaiting events included.
    state def Modes {
        entry; then armed;
        state armed;
        state ready;
        transition first armed then ready;
    }

    part def Controller {
        exhibit state modes : Modes;
        assert constraint readyIsActive { modes.ready.isActive }
        assert constraint armedIsNotActive { not modes.armed.isActive }
        assert constraint readyIsActiveWrong { modes.armed.isActive == true }
        assert constraint modesIsActive { modes.isActive }
    }
}
