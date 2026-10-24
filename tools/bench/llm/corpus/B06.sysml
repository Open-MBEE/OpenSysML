package Thermostat {
    private import ScalarValues::Real;

    part def Thermostat {
        attribute temperature : Real;
        attribute setpoint : Real;

        exhibit state behavior {
            entry; then idle;
            state idle;
            state heating;
            state cooling;
            transition idle_to_heating first idle if temperature < setpoint then heating;
            transition idle_to_cooling first idle if temperature > setpoint then cooling;
            transition heating_to_idle first heating if temperature >= setpoint then idle;
            transition cooling_to_idle first cooling if temperature <= setpoint then idle;
        }
    }
}
