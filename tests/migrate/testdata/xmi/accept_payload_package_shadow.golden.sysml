package Model {
    package Alarm {
        item def Alarm;
        action def Receive {
            first start then receive;
            action receive accept Alarm : Model::Alarm::Alarm;
            first receive then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
    }
}
