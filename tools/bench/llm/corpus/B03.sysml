package Dishwasher {
    item def Dish;
    item def CleanDish;

    action def Wash {
        in item dirty : Dish;
        out item clean : CleanDish;
    }

    action def Dry {
        in item wet : CleanDish;
        out item dry : CleanDish;
    }

    action def RunCycle {
        in item dishes : Dish;
        out item result : CleanDish;
        action wash : Wash {
            in item dirty = dishes;
        }
        then action dry : Dry;
        flow wash.clean to dry.wet;
        bind dry.dry = result;
    }
}
