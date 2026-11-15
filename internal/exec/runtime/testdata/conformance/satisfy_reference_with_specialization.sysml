// The requirement a satisfy names is its one ReferenceSubsetting (SysML v2 §8.2.2.21.2
// SatisfyRequirementUsage); a `:>` after it is a further Subsetting, so both requirements
// are checked on the subject under the name of the first, which the margin then violates, and a
// requirement reached through a chain is named the same way.
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
