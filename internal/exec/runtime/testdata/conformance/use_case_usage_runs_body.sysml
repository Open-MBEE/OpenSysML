// A use case usage is an action usage (SysML v2 §7.19): its body runs through
// the action executor, start to done, as any action's does.
package test {
    part def Drone;

    use case chargeDrone {
        subject drone : Drone;
        attribute charge = 0;

        first start;
        then action plugDrone { assign charge := charge + 1; }
        then action chargeDroneBatteries { assign charge := charge + 10; }
        then done;
    }
}
