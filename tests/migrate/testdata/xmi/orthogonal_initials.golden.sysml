item def Next;
part def Plant {
    state def Life {
        entry; then Donate;
        state Donate {
            entry; then regions;
            state regions parallel {
                state da {
                    entry; then da1;
                    state da1;
                }
                state db {
                    entry; then db1;
                    state db1;
                }
            }
            transition first regions then done;
            metadata MigrationMetadata::SynthesizedName about regions;
        }
        state Coincide {
            entry; then regions;
            state regions parallel {
                state ca {
                    entry; then ca1;
                    state ca1;
                }
                state cb {
                    entry; then cb1;
                    state cb1;
                }
            }
            transition first regions then done;
            metadata MigrationMetadata::SynthesizedName about regions;
        }
        state Conflict {
            entry; then regions;
            state regions parallel {
                state xa {
                    /* not migrated: Pseudostate (_xaStray) — the initial transition's target has no v2 form here */
                    /* not migrated: Transition (_xaStrayT) — its target 'xb2' lies in xb, an orthogonal region whose own initial pseudostate enters 'xb1'; that entry is kept */
                    entry; then xa1;
                    state xa1;
                }
                state xb {
                    entry; then xb1;
                    state xb1;
                    state xb2;
                }
            }
            transition first regions then done;
            metadata MigrationMetadata::SynthesizedName about regions;
        }
        state Duplicate {
            /* not migrated: Pseudostate (_uaInit2) — a region has one initial pseudostate; (_uaInit1) is written as it */
            entry; then ua1;
            state ua1;
            state ua2;
        }
        state Nested {
            entry; then regions;
            state regions parallel {
                state na {
                    entry; then na1;
                    state na1;
                }
                state nb {
                    entry; then Inner;
                    state Inner {
                        entry; then inner1;
                        state inner1;
                    }
                }
            }
            transition first regions then done;
            metadata MigrationMetadata::SynthesizedName about regions;
        }
        transition first Donate accept Next then Coincide;
        transition first Coincide accept Next then Conflict;
        transition first Conflict accept Next then Duplicate;
        transition first Duplicate accept Next then Nested;
    }
}
