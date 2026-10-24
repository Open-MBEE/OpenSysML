package Model {
    item def Ping;
    part def Ctrl {
        attribute got : ScalarValues::Real default = 0.0;
    }
    part def Rig {
        part ctrl : Ctrl;
        part other : Ctrl;
        occurrence def Pair {
            ref part a : Ctrl {
                event occurrence pingSent;
            }
            ref part b : Ctrl {
                event occurrence pingReceived;
            }
            message ping of Ping[1] from a.pingSent to b.pingReceived;
        }
        occurrence pair : Pair {
            ref part :>> a = ctrl;
            ref part :>> b = other;
        }
        metadata MigrationMetadata::SynthesizedName about pair;
    }
    part def Plant {
        part rig : Rig;
        state def Duty {
            entry; then on;
            state on {
                /* do action Rig::Pair is written as an occurrence def, which no state runs */
            }
            transition first on then done;
        }
    }
}
