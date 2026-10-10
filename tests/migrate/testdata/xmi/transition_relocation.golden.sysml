item def Stop;
part def Plant {
    state def Life {
        entry; then Work;
        state Work parallel {
            state work {
                entry; then Run;
                state Run;
            }
        }
        state Parked parallel {
            state parked {
                entry; then Ready;
                state Ready;
            }
        }
        state Parallel {
            entry; then regions;
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
        transition first Work.work.Run accept Stop then Parked.parked.Ready;
    }
}
