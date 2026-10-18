package Catalog {
    package Components {
        part def Motor;
        part def Wheel;
    }
    package Systems {
        part def Rover {
            part motor : Components::Motor;
            part Wheel : Components::Wheel;
        }
    }
}
