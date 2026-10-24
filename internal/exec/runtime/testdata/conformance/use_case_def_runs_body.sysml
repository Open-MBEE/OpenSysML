// A use case definition is an action definition (SysML v2 §7.19): it runs as an
// action def does, and its in/out parameters read as an action's.
package test {
    part def Drone;

    use case def ChargeDrone {
        subject drone : Drone;
        in level default 5;
        out charged;

        first start;
        then action plugDrone;
        then action chargeDroneBatteries { assign charged := level + 10; }
        then done;
    }
}
