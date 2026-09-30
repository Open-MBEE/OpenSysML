package Routing {
    part def Chooser {
        attribute speed : ScalarValues::Real;
        perform action route {
            first start then 'read speed';
            action 'read speed' {
                /* reads speed, which flows nowhere */
            }
            first 'read speed' then final;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about final;
        }
        view 'Route View' {
            expose route;
        }
    }
}
