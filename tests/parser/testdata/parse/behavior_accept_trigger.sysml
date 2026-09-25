action def Monitor {
    action senseTemperature { out temp : Real; }
    action atMaintenance accept at maintenanceTime;
    action afterDelay accept after 10 [s];
    action whenHot accept when senseTemperature.temp > maxTemp;
    accept when senseTemperature.temp > maxTemp;
}
