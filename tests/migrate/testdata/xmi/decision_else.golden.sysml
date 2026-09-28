package Recovery {
    part def Retrier {
        attribute attempts : ScalarValues::Integer default = 0;
        action def Recover {
            in ref context : Retrier;
            first start then 'decide';
            decide 'decide';
            else 'merge';
            comment /* Retries until the budget is spent. */
            first 'decide' if context.attempts < 3 then retry;
            action retry {
                /* body not migrated (the body is empty) */
            }
            first retry then 'merge';
            merge 'merge';
            first 'merge' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        perform action recover : Recover { in ref :>> context = this; }
    }
}
