package HouseWiring {
    item def Current;

    port def ElectricalPort {
        inout item current : Current;
    }

    interface def WireInterface {
        end source : ElectricalPort;
        end sink : ElectricalPort;
    }

    part def Breaker {
        port feed : ElectricalPort;
    }

    part def Outlet {
        port plug : ElectricalPort;
    }

    part def Circuit {
        part breaker : Breaker;
        part outlet : Outlet;
        interface wire : WireInterface connect breaker.feed to outlet.plug;
    }
}
