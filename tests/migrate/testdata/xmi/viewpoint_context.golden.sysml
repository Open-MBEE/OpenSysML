package Reviews {
    viewpoint 'Review Viewpoint' {
        port tx;
    }
    item def Ping;
    action def Inspect {
        in ref context :> 'Review Viewpoint'[1];
        first start then 'send ping';
        action 'send ping' {
            send new Ping() via context.tx;
        }
        first 'send ping' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Review {
        in ref context :> 'Review Viewpoint'[1];
        first start then inspect;
        action inspect : Inspect { in ref :>> context = Review::context; }
        first inspect then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
