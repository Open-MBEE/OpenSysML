part def Spacecraft {
    exhibit state spacecraftModes {
        entry; then off;
        state off;
        state waitingGSPing;
        state transmitting;
        transition first waitingGSPing
            accept 'Ground Station Ping'
            then transmitting;
        transition first transmitting
            accept when done
            do send Telemetry() to ground
            then off;
    }
    exhibit state declaredOnly : SpacecraftModes;
}
