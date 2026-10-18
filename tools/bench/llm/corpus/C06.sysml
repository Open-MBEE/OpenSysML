package Tank {
    private import ISQ::*;
    private import SI::*;

    calc def CylinderVolume {
        in radius : LengthValue;
        in height : LengthValue;
        return volume : VolumeValue = 3.14159 * radius * radius * height;
    }

    constraint def VolumeAtLeast {
        in volume : VolumeValue;
        in minimum : VolumeValue;
        volume >= minimum
    }

    part def WaterTank {
        attribute radius : LengthValue = 0.5 [m];
        attribute height : LengthValue = 2 [m];
        attribute volume : VolumeValue = CylinderVolume(radius, height);
        attribute requiredVolume : VolumeValue = 1.5 [m ** 3];
        assert constraint enough : VolumeAtLeast {
            in volume = volume;
            in minimum = requiredVolume;
        }
    }
}
