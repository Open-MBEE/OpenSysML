part def Chooser {
    attribute pDefault : ScalarValues::Real default = 0.25;
    attribute pMissing : ScalarValues::Real;
    perform action route {
        first start then 'with default';
        decide 'with default';
        first 'with default' then defaulted { @Stochastic::Probability { p = pDefault; } }
        first 'with default' then remainder { @Stochastic::Probability { p = 1.0 - pDefault; } }
        action defaulted;
        first defaulted then 'merge';
        action remainder;
        first remainder then 'without default';
        decide 'without default';
        first 'without default' then 'missing default' { @Stochastic::Probability { p = pMissing; } }
        first 'without default' then 'missing remainder' { @Stochastic::Probability { p = 1.0 - pMissing; } }
        action 'missing default';
        first 'missing default' then 'merge';
        action 'missing remainder';
        merge 'merge';
        first 'merge' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about 'merge', final;
        metadata MigrationMetadata::StandIn about 'merge';
    }
}
