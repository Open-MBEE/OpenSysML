package FlowsAndViews {
    port def FuelPort {
        out item fuelSupply;
    }
    part def Tank;
    part def Engine;
    interface def FuelInterface {
        end port supplierPort : FuelPort;
        end port consumerPort : FuelPort;
        flow supplierPort.fuelSupply to consumerPort.fuelSupply;
    }
    part tanks : Tank[*] nonunique;
    part vehicle {
        part tank : Tank;
        part spares : Tank[0..2] nonunique subsets tanks;
        part firstSpare : spares.self;
        part engine : Engine;
        allocate tank to engine;
    }
    action def Measure {
        in part measured subsets Tank;
        out mass : Real;
    }
    view def StructureView {
        filter @SysML::PartUsage;
    }
    view vehicleView : StructureView {
        expose vehicle::**;
    }
}
