package SatisfyCases {
    part def Vehicle {
        part engine : Engine;
    }
    part def Engine;
    part vehicle : Vehicle;
    requirement def MissionRequirement;
    requirement mission : MissionRequirement {
        satisfy mission by vehicle.engine;
    }
}
