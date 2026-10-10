part def Tank {
    attribute level : ScalarValues::Real default = 1.5;
    calc def Rate {
        in ref context : Tank[1];
        in factor : ScalarValues::Real[1];
        out result : ScalarValues::Real[1];
        rep language "English" /* level * factor*/
        context.level * factor
    }
}
part def Plant {
    part tank : Tank;
    action def Run {
        out result : ScalarValues::Real[1];
        in ref context : Plant[1];
        first start then 'fork';
        fork 'fork';
        first 'fork' then rate;
        first 'fork' then two;
        action two {
            out result[1] = 2.0;
        }
        action rate {
            in factor[1];
            out result[1];
            out result[1] = Tank::Rate(Run::context.tank, factor);
        }
        first rate then final;
        action final terminate;
        bind result = rate.result;
        succession flow two.result to rate.factor;
        metadata MigrationMetadata::SynthesizedName about 'fork', final;
        metadata MigrationMetadata::StandIn about 'fork';
    }
}
