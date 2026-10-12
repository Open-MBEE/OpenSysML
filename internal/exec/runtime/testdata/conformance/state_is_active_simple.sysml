package test {
    private import ScalarValues::*;
    private import StateActivity::*;

    // `x.isActive` reads whether state x is in the machine's active
    // configuration at that moment: true during a state's own entry and exit
    // behaviors, false for both the source and the target while a transition's
    // effect runs, true again once the target has been entered.
    item def Go;

    state def Timing {
        attribute aEntry : Boolean = false;
        attribute aExit : Boolean = false;
        attribute bSeenFromA : Boolean = true;
        attribute sourceInEffect : Boolean = true;
        attribute targetInEffect : Boolean = true;
        attribute bEntry : Boolean = false;
        attribute aSeenFromB : Boolean = true;

        entry; then a;
        state a {
            entry action {
                assign aEntry := a.isActive;
                assign bSeenFromA := b.isActive;
            }
            exit assign aExit := a.isActive;
        }
        state b {
            entry action {
                assign bEntry := b.isActive;
                assign aSeenFromB := a.isActive;
            }
        }
        transition a_to_b
            first a
            accept Go
            do action {
                assign sourceInEffect := a.isActive;
                assign targetInEffect := b.isActive;
            }
            then b;
    }
}
