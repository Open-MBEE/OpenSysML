package Annotated {
    metadata def Safety {
        attribute level : Integer;
        attribute reviewer : String;
        attribute isSet : Boolean;
        item audit {
            attribute year : Integer;
        }
    }
    part def Car {
        attribute mass : Real;
    }
    part car : Car {
        attribute range : Real = 500.0;
        @Safety { level = 2; }
        @Safety {
            :>> level = 3;
            reviewer = "ops";
            isSet = true;
            audit { year = 2026; }
        }
        @ checked : Safety;
        @Safety;
    }
    @Safety about Car, car {
        level = car.range;
        reviewer = Car::mass;
    }
    metadata tagged : Safety about Car;
    metadata Safety about car;
    metadata $::Annotated::Safety about Car {
        level = 4;
    }
}
