part def Sorter {
    attribute bias : ScalarValues::Real default = 0.6;
    action def Route {
        first start then 'decide';
        decide 'decide';
        first 'decide' then left { @Stochastic::Probability { p = bias; } }
        first 'decide' then right { @Stochastic::Probability { p = 0.3; } }
        first 'decide' then reject { @Stochastic::Probability { p = 1.0 - (bias + 0.3); } }
        action left {
            /* body not migrated (the body is empty) */
        }
        first left then 'merge';
        action right {
            /* body not migrated (the body is empty) */
        }
        first right then 'merge';
        action reject {
            /* body not migrated (the body is empty) */
        }
        first reject then 'merge';
        merge 'merge';
        first 'merge' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about 'decide', 'merge', final;
    }
    perform action route : Route;
}
