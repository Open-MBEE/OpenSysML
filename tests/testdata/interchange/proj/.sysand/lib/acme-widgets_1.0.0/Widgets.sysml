package Widgets {
    private import ScalarValues::Real;
    private import Gears::*;
    part def Widget :> Thing;
    attribute widgetMass : Real = gearMass;
}
