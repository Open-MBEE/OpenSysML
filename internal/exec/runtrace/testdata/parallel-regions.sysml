package Par {
    private import SI::*;

    part def Box {
        exhibit state modes {
            entry; then open;

            state open parallel {
                state left {
                    entry; then waiting;
                    state waiting;
                    state done;

                    transition first waiting accept after 1 [s] then done;
                }

                state right {
                    entry; then waiting;
                    state waiting;
                }
            }
        }
    }

    part box : Box;
}
