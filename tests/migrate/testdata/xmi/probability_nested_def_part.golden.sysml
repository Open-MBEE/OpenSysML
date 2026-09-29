part def Mission {
    attribute pr : ScalarValues::Real default = 0.25;
    part def Sub {
        part mission : Mission;
        action def Run {
            in ref context : Sub[1];
            action def Inner {
                in ref context : Sub[1];
                first start then 'decide';
                decide 'decide';
                first 'decide' then a { @Stochastic::Probability { p = context.mission.pr; } }
                first 'decide' then b { @Stochastic::Probability { p = 1.0 - context.mission.pr; } }
                action a {
                    /* body not migrated (the body is empty) */
                }
                first a then 'merge';
                action b {
                    /* body not migrated (the body is empty) */
                }
                first b then 'merge';
                merge 'merge';
                first 'merge' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about 'decide', 'merge', final;
                metadata MigrationMetadata::StandIn about 'merge';
            }
            first start then call;
            action call : Inner { in ref :>> context = Run::context; }
            first call then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action run : Run { in ref :>> context = this; }
    }
}
