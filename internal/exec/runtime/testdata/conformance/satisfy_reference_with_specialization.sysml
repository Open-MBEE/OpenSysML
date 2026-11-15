// A satisfy names its requirement by a ReferenceSubsetting (SysML v2 §8.2.2.21.2), directly or
// through a chain; a `:>` after it is a further Subsetting, checked too, which the margin violates.
package test {
    private import ScalarValues::*;
    part def Relay { attribute rating : Real = 10.0; }
    requirement def RatingReq { subject r : Relay; require constraint { r.rating >= 6.0 } }
    requirement def MarginReq { subject r : Relay; require constraint { r.rating >= 20.0 } }
    requirement ratingOk : RatingReq;
    requirement marginOk : MarginReq;
    requirement group { requirement nested : RatingReq; }
    part relay : Relay;
    satisfy ratingOk by relay;
    satisfy group.nested by relay;
    satisfy ratingOk :> marginOk by relay;
}
