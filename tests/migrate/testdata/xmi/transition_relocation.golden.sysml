item def Stop;
part def Plant {
    state def Life {
        entry; then Work;
        state Work {
            entry; then Run;
            state Run;
        }
        state Parked {
            entry; then Ready;
            state Ready;
        }
        state Parallel {
            /* no default entry: these regions have no written entry: 'region4'; only a fork or a transition naming a nested state enters them */
            state regions parallel {
                state region3 {
                    entry; then wait3;
                    state wait3;
                }
                state region4 {
                    /* every initial pseudostate of the region enters another region: nothing enters this one */
                }
            }
            transition first regions then done;
            metadata MigrationMetadata::SynthesizedName about regions;
        }
        transition first Work.Run accept Stop then Parked.Ready;
    }
}
