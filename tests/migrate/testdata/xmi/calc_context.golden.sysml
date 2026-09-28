part def Tank {
    attribute level : ScalarValues::Real default = 1.5;
    calc def Rate {
        in ref context : Tank;
        in factor : ScalarValues::Real;
        out result : ScalarValues::Real;
        context.level * factor
    }
}
part def Plant {
    part tank : Tank;
    action def Run {
        out result : ScalarValues::Real;
        in ref context : Plant;
        first start then 'fork';
        fork 'fork';
        first 'fork' then 'join';
        first 'fork' then two;
        action two {
            out result = 2.0;
        }
        first two then 'join';
        join 'join';
        first 'join' then rate;
        action rate {
            in factor;
            out result;
            out result = Tank::Rate(Run::context.tank, factor);
        }
        first rate then final;
        action final terminate;
        bind result = rate.result;
        flow two.result to rate.factor;
        metadata MigrationMetadata::SynthesizedName about 'fork', 'join', final;
        metadata MigrationMetadata::StandIn about 'fork', 'join';
    }
}
