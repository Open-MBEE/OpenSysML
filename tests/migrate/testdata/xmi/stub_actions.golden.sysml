part def Probe;
part def Logger;
part def Rig {
    part probe : Probe;
    part logger : Logger;
    attribute last : ScalarValues::Real;
    action def Record {
        in value : ScalarValues::Real[0..1];
        first start then 'set last';
        action 'set last' {
            in value[0..1];
            if value->SequenceFunctions::notEmpty() {
                assign this.last := value;
            }
        }
        first 'set last' then final;
        action final terminate;
        bind 'set last'.value = value;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Sampling {
        out sample : ScalarValues::Real[0..1];
        first start then gain;
        action gain {
            out result = 2.5;
        }
        first gain then 'fork';
        fork 'fork';
        first 'fork' then stamp;
        first 'fork' then 'join';
        action stamp {
            out result = 7;
        }
        first stamp then fork2;
        fork fork2;
        first fork2 then 'join';
        first fork2 then join2;
        join 'join';
        first 'join' then measure;
        action measure {
            in gain : ScalarValues::Real;
            out reading : ScalarValues::Real[0..1];
        }
        first measure then join2;
        join join2;
        first join2 then record;
        action record : Record;
        first record then final;
        action final terminate;
        flow gain.result to measure.gain;
        flow measure.reading to record.value;
        bind sample = measure.reading;
        metadata MigrationMetadata::SynthesizedName about 'fork', fork2, 'join', join2, final;
    }
}
allocate Rig::Sampling::measure to Rig::probe;
allocate Rig::Sampling::record to Rig::logger;
