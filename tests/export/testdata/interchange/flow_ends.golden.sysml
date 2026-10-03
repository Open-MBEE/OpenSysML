package FlowEnds {
    item def Payload;
    part def PayloadHolder {
        item payload : Payload;
    }
    part def Endpoint {
        part holder : PayloadHolder;
    }
    part def Assembly {
        part source : Endpoint;
        part target : Endpoint;
        flow of Payload from source.holder.payload to target.holder.payload;
    }
}
