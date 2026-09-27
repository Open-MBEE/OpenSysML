part def Tank {
    attribute level : ScalarValues::Real default = 1.5;
    calc def Rate {
        in ref context : Tank;
        out result : ScalarValues::Real;
        context.level + 1
    }
}
part def Plant {
    part tank : Tank;
    action def Run {
        out result : ScalarValues::Real;
        in ref context : Plant;
        first start then rate;
        action rate {
            out result;
            out result = Tank::Rate(Run::context.tank);
        }
        first rate then final;
        action final terminate;
        bind result = rate.result;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
