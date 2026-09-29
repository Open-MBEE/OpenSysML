part def Plant {
    attribute level : ScalarValues::Real default = 0.0;
    action def Adjust {
        in ref context : Plant[1];
        in x : ScalarValues::Real[0..1];
        out result : ScalarValues::Real[1];
        first start then 'set level';
        action 'set level' {
            in value[0..1];
            if value->SequenceFunctions::notEmpty() {
                assign context.level := value;
            }
        }
        bind 'set level'.value = x;
    }
    action adjust : Adjust;
    action def Drive {
        in ref context : Plant[1];
        first start then adjust;
        action adjust : Adjust { in ref :>> context = Drive::context; in x[1]; out result[1]; }
        first adjust then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
action def Reach {
    first start then adjust;
    action adjust : Plant::Adjust;
    first adjust then final;
    action final terminate;
    metadata MigrationMetadata::SynthesizedName about final;
}
