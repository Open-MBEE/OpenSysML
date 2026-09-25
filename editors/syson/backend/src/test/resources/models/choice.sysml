package test {
    private import ScalarValues::*;

    action tally {
        attribute passes : Integer = 0;
        attribute lefts : Integer = 0;
        attribute rights : Integer = 0;

        first start;
        merge again;
        decide pick;
        action left { assign lefts := lefts + 1; assign passes := passes + 1; }
        action right { assign rights := rights + 1; assign passes := passes + 1; }
        done;

        succession first start then again;
        succession first again then pick;
        succession first pick if passes < 2 then left;
        succession first pick if passes < 2 then right;
        succession first pick if passes >= 2 then done;
        succession first left then again;
        succession first right then again;
    }
}
