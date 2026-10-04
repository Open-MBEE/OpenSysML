part def Chooser {
    attribute pDefault : ScalarValues::Real default = 0.25;
    attribute pMissing : ScalarValues::Real;
    perform action route {
        first start then 'decide';
        decide 'decide';
        first 'decide' then defaulted { @Stochastic::Probability { p = pDefault; } }
        first 'decide' if pMissing > 0.0 then missing { @Stochastic::Probability { p = pMissing; } }
        action defaulted;
        first defaulted then 'merge';
        action missing;
        first missing then 'merge';
        merge 'merge';
        first 'merge' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about 'merge', final;
        metadata MigrationMetadata::StandIn about 'merge';
    }
}
