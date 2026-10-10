part def Sorter {
    attribute bias : ScalarValues::Real default = 0.6;
    perform action route {
        first start then 'decide';
        decide 'decide';
        first 'decide' then left { @Stochastic::Probability { p = bias; } }
        first 'decide' then right { @Stochastic::Probability { p = 0.3; } }
        first 'decide' then reject { @Stochastic::Probability { p = 1.0 - (bias + 0.3); } }
        action left;
        first left then 'merge';
        action right;
        first right then 'merge';
        action reject;
        first reject then 'merge';
        merge 'merge';
        first 'merge' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about 'decide', 'merge', final;
        metadata MigrationMetadata::StandIn about 'merge';
    }
}
