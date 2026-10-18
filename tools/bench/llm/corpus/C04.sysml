package Journey {
    private import ISQ::*;
    private import SI::*;

    calc def AverageSpeed {
        in distance : LengthValue;
        in duration : DurationValue;
        return speed : SpeedValue = distance / duration;
    }

    part def Trip {
        attribute distance : LengthValue = 150000 [m];
        attribute duration : DurationValue = 5400 [s];
        attribute averageSpeed : SpeedValue = AverageSpeed(distance, duration);
    }
}
