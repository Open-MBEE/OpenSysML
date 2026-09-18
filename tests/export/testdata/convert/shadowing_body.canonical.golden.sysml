package ShadowBody {
    private import ControlFunctions::*;
    attribute def Speed;
    part def Sensor {
        attribute value : Speed;
        attribute readings : Speed[*];
        attribute highs = readings->select({ in value; value > 5 });
    }
}
