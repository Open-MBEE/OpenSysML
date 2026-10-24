package Orbit {
    private import ISQ::*;
    private import SI::*;

    calc def KineticEnergy {
        in mass : MassValue;
        in speed : SpeedValue;
        return energy : EnergyValue = 0.5 * mass * speed * speed;
    }

    calc def PotentialEnergy {
        in mass : MassValue;
        in height : LengthValue;
        in gravity : AccelerationValue;
        return energy : EnergyValue = mass * gravity * height;
    }

    calc def TotalEnergy {
        in kinetic : EnergyValue;
        in potential : EnergyValue;
        return total : EnergyValue = kinetic + potential;
    }

    part def Projectile {
        attribute mass : MassValue = 10 [kg];
        attribute speed : SpeedValue = 300 [m/s];
        attribute height : LengthValue = 1000 [m];
        attribute gravity : AccelerationValue = 9.81 [m/s**2];
        attribute kinetic : EnergyValue = KineticEnergy(mass, speed);
        attribute potential : EnergyValue = PotentialEnergy(mass, height, gravity);
        attribute total : EnergyValue = TotalEnergy(kinetic, potential);
    }
}
