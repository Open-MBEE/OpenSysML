package Views {
    view def SystemView;
    view vehicleView : SystemView {
        expose VehicleModel;
        expose VehicleModel::*;
        expose VehicleModel::**;
        expose VehicleModel::*::**;
        expose VehicleModel::**[@SysML::PartUsage];
    }
    private import VehicleModel::*;
}
