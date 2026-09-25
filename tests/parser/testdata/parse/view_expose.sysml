package Views {
    view def SystemView {
        expose VehicleModel;
    }
    view vehicleView : SystemView {
        expose VehicleModel::*;
        expose VehicleModel::**;
        expose VehicleModel::*::**;
        expose VehicleModel::**[@SysML::PartUsage];
    }
    private import VehicleModel::*;
}
