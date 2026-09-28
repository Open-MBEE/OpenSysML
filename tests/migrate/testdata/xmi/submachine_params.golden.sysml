item def Go;
part def Plant {
    attribute count : ScalarValues::Integer default = 0;
    state def Line {
        in ref context : Plant;
        entry; then Idle;
        state Idle;
        state Cell : CellMachine {
            in x;
            in ref :>> context = Line::context;
            entry action tally {
                assign context.count := context.count + 1;
            }
        }
        transition first Idle accept Go then Cell;
    }
    state def CellMachine {
        in x : ScalarValues::Integer;
        in ref context : Plant;
        entry; then Cold;
        state Cold;
        transition first Cold then done;
    }
    exhibit state line : Line;
}
