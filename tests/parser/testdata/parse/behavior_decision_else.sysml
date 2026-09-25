action def Watchdog {
    action monitor;
    action sendBatteryLow;
    action idle;
    first monitor;
    then decide;
        if battery < 20 then sendBatteryLow;
        else idle;
}
