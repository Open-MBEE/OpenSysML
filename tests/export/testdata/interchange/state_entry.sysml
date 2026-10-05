package Behave {
    private import ScalarValues::*;
    item def Go;
    part def Rover {
        attribute mass : Real = 10.0;
        attribute halfMass : Real = mass / 2;
        action def Charge {
            action step1 { }
            then action step2 { }
        }
        perform action charge : Charge;
        state def Mission {
            entry; then idle;
            state idle;
            transition idle_to_active first idle accept Go then active;
            state active;
        }
        exhibit state mission : Mission;
    }
    part rover1 : Rover;
}
