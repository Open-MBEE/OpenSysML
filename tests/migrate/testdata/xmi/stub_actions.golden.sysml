part def Probe;
part def Logger;
part def Rig {
    part probe : Probe;
    part logger : Logger;
    attribute last : ScalarValues::Real;
    action record {
        in value : ScalarValues::Real[0..1];
        first start then 'set last';
        action 'set last' {
            in value[0..1];
            if value->SequenceFunctions::notEmpty() {
                assign last := value;
            }
        }
        first 'set last' then final;
        action final terminate;
        bind 'set last'.value = value;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action sampling {
        out sample : ScalarValues::Real[0..1];
        first start then gain;
        action gain {
            out result[1] = 2.5;
        }
        first gain then stamp;
        action stamp {
            out result[1] = 7;
        }
        first stamp then 'fork';
        fork 'fork';
        first 'fork' then measure;
        first 'fork' then 'join';
        action measure {
            in gain : ScalarValues::Real[1];
            out reading : ScalarValues::Real[0..1];
        }
        first measure then 'join';
        join 'join';
        first 'join' then record;
        perform action record ::> Rig::record;
        first record then final;
        action final terminate;
        succession flow of ScalarValues::Real from gain.result to measure.gain;
        flow measure.reading to record.value;
        bind sample = measure.reading;
        metadata MigrationMetadata::SynthesizedName about 'fork', 'join', final;
        metadata MigrationMetadata::StandIn about 'fork', 'join';
    }
}
dependency Rig::sampling::measure to Rig::probe;
dependency Rig::sampling::record to Rig::logger;
