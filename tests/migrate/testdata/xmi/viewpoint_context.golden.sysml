package Reviews {
    viewpoint 'Review Viewpoint' {
        port tx;
    }
    item def Ping;
    action def Inspect {
        first start then 'send ping';
        action 'send ping' {
            send new Ping();
        }
        first 'send ping' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Review {
        first start then inspect;
        action inspect : Inspect;
        first inspect then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
