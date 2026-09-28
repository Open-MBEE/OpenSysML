part def Sequencer {
    attribute k : ScalarValues::Integer default = 0;
    state def 'AO Sequencer' {
        entry; then Idle;
        state Idle;
    }
    exhibit state 'aO Sequencer' : 'AO Sequencer';
}
part def Host {
    part 'aO Sequencer' : Sequencer;
    action align {
        action def Write {
            in ref context : Host;
            action def Set {
                in ref context : Host;
                first start then 'read sequencer';
                action 'read sequencer' {
                    out result = context.'aO Sequencer';
                }
                first 'read sequencer' then 'fork';
                fork 'fork';
                first 'fork' then one;
                first 'fork' then 'join';
                action one {
                    out result = 1;
                }
                first one then 'join';
                join 'join';
                first 'join' then 'set k';
                action 'set k' {
                    in object;
                    in value;
                    assign context.'aO Sequencer'.k := value;
                }
                first 'set k' then final;
                action final terminate;
                flow 'read sequencer'.result to 'set k'.object;
                flow one.result to 'set k'.value;
                metadata MigrationMetadata::SynthesizedName about 'fork', 'join', final;
                metadata MigrationMetadata::StandIn about 'fork', 'join';
            }
            first start then set;
            action set : Set { in ref :>> context = Write::context; }
            first set then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        first start then 'observe sequencer';
        action 'observe sequencer' {
            out result = 'aO Sequencer';
        }
        first 'observe sequencer' then write;
        action write : Write;
        bind write.context = this;
        first write then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Direct {
        in ref context : Host;
        first start then 'write state data';
        action 'write state data' {
            assign context.'aO Sequencer'.k := 2;
        }
        first 'write state data' then 'decide';
        decide 'decide';
        first 'decide' if context.'aO Sequencer'.k >= 2 then final;
        action final terminate;
        /* partition (_directLane) represents Host::aO Sequencer: write state data */
        metadata MigrationMetadata::SynthesizedName about 'decide', final;
    }
    perform action run {
        first start then align;
        perform action align ::> Host::align;
        first align then direct;
        action direct : Direct;
        bind direct.context = this;
        first direct then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
part def Rig {
    part h : Host;
}
