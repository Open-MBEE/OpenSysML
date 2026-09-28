part def Mission {
    attribute pr : ScalarValues::Real default = 0.25;
    part def Sub {
        part mission : Mission;
        perform action run {
            first start then 'decide';
            decide 'decide';
            first 'decide' then a { @Stochastic::Probability { p = Sub::mission.pr; } }
            first 'decide' then b { @Stochastic::Probability { p = 1.0 - Sub::mission.pr; } }
            action a {
                /* body not migrated (the body is empty) */
            }
            first a then mission;
            action b {
                /* body not migrated (the body is empty) */
            }
            first b then 'merge';
            action mission {
                /* body not migrated (the body is empty) */
            }
            first mission then 'merge';
            merge 'merge';
            first 'merge' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about 'decide', 'merge', final;
            metadata MigrationMetadata::StandIn about 'merge';
        }
    }
}
