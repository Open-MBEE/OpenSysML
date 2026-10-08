package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // Two parts exhibit the same state usage, each running its own machine:
    // a read written through a part, `b.modes.ready.isActive`, is answered by
    // that part's machine, not by the machine of the object the read is made
    // on or of the part declared first.
    part def Controller {
        attribute proceed : Boolean default true;
        exhibit state modes {
            entry; then armed;
            state armed;
            state ready;
            transition first armed if proceed then ready;
        }
    }

    part def System {
        part a : Controller;
        part b : Controller {
            attribute :>> proceed = false;
        }
        assert constraint aReady { a.modes.ready.isActive }
        assert constraint aNotArmed { not a.modes.armed.isActive }
        assert constraint bArmed { b.modes.armed.isActive }
        assert constraint bNotReady { not b.modes.ready.isActive }
        assert constraint bothMachinesActive { a.modes.isActive and b.modes.isActive }
        assert constraint bReadyWrong { b.modes.ready.isActive == true }
    }
}
