// A behavior a type binds to its objects that cannot start — here an exhibited
// machine with no initial state — does not fail the object's creation: the object
// stands with the failure recorded on that behavior (a performance is the
// performer's, not the performer's existence: SysML v2 §7.17.6, §7.18.4), and a
// verdict reading nothing the failed body may write is reported as it evaluates.
package test {
    private import ScalarValues::*;
    part def Relay {
        attribute rating : Real = 10.0;
        exhibit state contacts { state open; state closed; }
    }
    requirement def RatingReq { subject r : Relay; require constraint { r.rating >= 6.0 } }
    requirement ratingOk : RatingReq;
    part relay : Relay;
    satisfy ratingOk by relay;
}
