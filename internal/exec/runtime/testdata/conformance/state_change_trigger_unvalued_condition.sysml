// A change condition reading a feature that carries no value has no result, so
// it has not become true: the machine rests in its state waiting on it, and the
// object exhibiting the machine is created, so a requirement over the object's
// other features is judged on it.
package test {
    private import ScalarValues::*;
    part def Relay {
        attribute coilVoltage : Real;
        attribute rating : Real = 10.0;
        exhibit state contacts {
            entry; then open;
            state open;
            accept when coilVoltage >= 19.6 then closed;
            state closed;
        }
    }
    requirement def RatingReq { subject r : Relay; require constraint { r.rating >= 6.0 } }
    requirement ratingOk : RatingReq;
    part relay : Relay;
    satisfy ratingOk by relay;
}
