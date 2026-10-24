package Elevator {
    attribute def CallButton;
    attribute def ArrivedAtFloor;
    attribute def DoorTimeout;

    action def OpenDoors;
    action def CloseDoors;
    action def MoveToFloor;

    state def ElevatorStates {
        entry; then idle;

        state idle {
            entry action open : OpenDoors;
        }
        state moving {
            entry action close : CloseDoors;
            do action move : MoveToFloor;
        }
        state doorsOpen;

        transition idle_to_moving first idle accept CallButton then moving;
        transition moving_to_doorsOpen first moving accept ArrivedAtFloor then doorsOpen;
        transition doorsOpen_to_idle first doorsOpen accept DoorTimeout then idle;
    }
}
