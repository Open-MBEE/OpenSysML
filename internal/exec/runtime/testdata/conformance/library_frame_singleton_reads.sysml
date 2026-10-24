// A coordinate frame the library declares with its axes in its own body
// (`ISQSpaceTime::universalCartesianSpatial3dCoordinateFrame`, whose `mRefs`
// default to metres) is a frame value as a model's `spatialCF` is: it is read by
// name, serves as the frame of a vector literal, is the default the Domain
// library's `SpatialItem::coordinateFrame` names, and gives a usage subsetting it
// (`attribute local :> ...`) the axes its body states (KerML 1.0 §7.3.4.5).
package test {
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::MeasurementUnit;
	private import SpatialItems::SpatialItem;

	item def Body :> SpatialItem;
	part def Node { attribute id : ScalarValues::Integer; }

	part def Rig {
		attribute frame : CartesianSpatial3dCoordinateFrame = universalCartesianSpatial3dCoordinateFrame;
		attribute axes : MeasurementUnit[3] ordered nonunique = universalCartesianSpatial3dCoordinateFrame.mRefs;
		attribute position : Position3dVector = (1.0, 2.0, 3.0) [universalCartesianSpatial3dCoordinateFrame];
		item body : Body;
		attribute local :> universalCartesianSpatial3dCoordinateFrame;
		part anchor : Node { :>> id = 4; }
		part alias :> anchor;
	}
}
