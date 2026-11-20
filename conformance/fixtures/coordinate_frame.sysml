package Frames {
    private import ISQ::*;
    private import SI::*;
    private import MeasurementReferences::*;
    private import SpatialItems::*;

    attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
    attribute lbcf : CartesianSpatial3dCoordinateFrame {
        :>> mRefs = (mm, mm, mm);
        :>> transformation : TranslationRotationSequence {
            :>> source = datum;
            :>> elements = (new Translation((10.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 180 ['°']));
        }
    }
}
