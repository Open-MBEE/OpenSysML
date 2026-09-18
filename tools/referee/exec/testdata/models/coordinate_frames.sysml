package Frames {
    public import ISQ::*;
    public import SI::*;
    public import ISQSpaceTime::*;
    public import MeasurementReferences::*;
    public import QuantityCalculations::*;
    public import VectorCalculations::*;
    private import ScalarValues::*;

    // The Annex A vehicle model's frames: a frame with its axis units, and a
    // frame divided by a unit.
    attribute spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }
    attribute velocityCF : CartesianVelocity3dCoordinateFrame[1] = spatialCF / s;
    attribute accelerationCF : CartesianAcceleration3dCoordinateFrame[1] = velocityCF / s;

    // The geometry example's vector literal over a frame, and its reference.
    attribute p : Position3dVector = (1.0, 2.0, 3.0) [spatialCF];
    attribute pFrame = p.mRef;

    // A frame placed in another, and a vector re-expressed in it.
    attribute shifted : CartesianSpatial3dCoordinateFrame[1] {
        :>> mRefs = (m, m, m);
        :>> transformation : CoordinateFramePlacement {
            :>> source = spatialCF;
            :>> origin = (10.0, 20.0, 30.0) [spatialCF];
        }
    }
    attribute placed : Position3dVector = transform(shifted.transformation, (11.0, 22.0, 33.0) [spatialCF]);

    // The library's Celsius scale, placed on kelvin.
    attribute warm : ThermodynamicTemperatureValue = ConvertQuantity(300.0 [K], SI::'°C_abs');
    attribute warmBack : ThermodynamicTemperatureValue = ConvertQuantity(26.85 [SI::'°C_abs'], K);
    attribute utc : Time::TimeScale = Time::UTC;
}
