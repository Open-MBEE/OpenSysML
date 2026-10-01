package test {
    private import ScalarValues::*;
    attribute def Reset;

    state Machine {
        attribute fired : Boolean = false;
        entry; then outer;
        state outer {
            entry; then active;
            state active;
            deep history resume;
            junction dead;
            transition first resume then dead;
            transition first dead if false then active;
        }

        transition first outer accept Reset do assign fired := true then resume;
    }
}
