package AccProbe {
    item def s3;
    item def start;
    action def Wait {
        first start then wait;
        action wait accept s3 : AccProbe::s3;
        first wait then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    part def Host {
        state def Machine {
            entry; then A;
            state A;
            state B;
            transition first A accept AccProbe::start then B;
        }
        exhibit state machine : Machine;
    }
}
