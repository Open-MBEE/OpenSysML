item def Stop;
item def Ping;
part def Machine {
    state def Run {
        entry; then regions;
        state regions parallel {
            state R1 {
                entry; then A;
                state A;
                action terminated terminate;
                transition first A accept Stop then terminated;
                metadata MigrationMetadata::SynthesizedName about terminated;
            }
            state R2 {
                entry; then B;
                state B;
                state C;
                transition first B accept Ping then C;
            }
        }
        transition first regions then done;
        metadata MigrationMetadata::SynthesizedName about regions;
    }
    exhibit state run : Run;
}
