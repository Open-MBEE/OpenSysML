package Recovery {
    part def Retrier {
        attribute attempts : ScalarValues::Integer default = 0;
        perform action recover {
            first start then 'decide';
            decide 'decide';
            else 'merge';
            comment /* Retries until the budget is spent. */
            first 'decide' if attempts < 3 then retry;
            action retry;
            first retry then 'merge';
            merge 'merge';
            first 'merge' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
    }
}
