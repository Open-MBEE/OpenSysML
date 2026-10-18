// A coordinate frame the library declares with its axes in its own body
// (`ISQSpaceTime::universalCartesianSpatial3dCoordinateFrame`, whose `mRefs`
// default to metres) is a frame value as a model's `spatialCF` is: it is read by
// name, serves as the frame of a vector literal, and is the default the Domain
// library's `SpatialItem::coordinateFrame` names.
package test {
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::MeasurementUnit;
	private import SpatialItems::SpatialItem;

	item def Body :> SpatialItem;

	part def Rig {
		attribute frame : CartesianSpatial3dCoordinateFrame = universalCartesianSpatial3dCoordinateFrame;
		attribute axes : MeasurementUnit[3] ordered nonunique = universalCartesianSpatial3dCoordinateFrame.mRefs;
		attribute position : Position3dVector = (1.0, 2.0, 3.0) [universalCartesianSpatial3dCoordinateFrame];
		item body : Body;
	}
}
