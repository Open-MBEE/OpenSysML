package AnonymousPortions {
    attribute def Fuel;
    part def Vehicle {
        attribute fuel : Fuel;
    }
    individual def Car specializes Vehicle;
    individual car : Car {
        snapshot redefines start {
            attribute redefines fuel = 100;
        }
        timeslice redefines portionOfLife {
            attribute redefines fuel = 50;
        }
        snapshot redefines done subsets fuel {
            attribute redefines fuel = 0;
        }
        snapshot;
        timeslice : Car;
    }
}
