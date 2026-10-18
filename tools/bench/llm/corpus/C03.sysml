package Lift {
    private import ISQ::*;
    private import SI::*;

    constraint def MassLimit {
        in actual : MassValue;
        in limit : MassValue;
        actual <= limit
    }

    part def Elevator {
        attribute payload : MassValue = 600 [kg];
        attribute ratedLoad : MassValue = 800 [kg];
        assert constraint withinLimit : MassLimit {
            in actual = payload;
            in limit = ratedLoad;
        }
    }
}
