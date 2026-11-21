package ChargingProgress {
    private import ScalarValues::*;

    // One accepted request, with the lock already recognized. The controller
    // can energize under lock or explicitly decline for a local resource reason.
    action serve {
        attribute locked : Boolean = true;
        attribute pending : Boolean = false;
        attribute energized : Boolean = false;
        attribute rejected : Boolean = false;

        first start;
        action acceptRequest { assign pending := true; }
        merge ready;
        action supply { assign energized := true; assign pending := false; }
        action reject { assign rejected := true; assign pending := false; }
        action retry { assign pending := true; }
        merge response;
        done;

        succession first start then acceptRequest;
        succession first acceptRequest then ready;
        succession first ready then respond;
        decide respond;
        if locked then supply;
        if pending then reject;
        if pending then retry;
        succession first retry then ready;
        succession first supply then response;
        succession first reject then response;
        succession first response then done;
    }
}
