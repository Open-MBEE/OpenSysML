part def Plant {
    attribute level : ScalarValues::Real default = 0.0;
    action adjust {
        in x : ScalarValues::Real[0..1];
        out result : ScalarValues::Real[1];
        first start then 'set level';
        action 'set level' {
            in value[0..1];
            if value->SequenceFunctions::notEmpty() {
                assign level := value;
            }
        }
        bind 'set level'.value = x;
    }
    action drive {
        first start then adjust;
        perform action adjust ::> Plant::adjust;
        first adjust then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
action def Reach {
    first start then adjust;
    action adjust;
    first adjust then final;
    action final terminate;
    metadata MigrationMetadata::SynthesizedName about final;
}
