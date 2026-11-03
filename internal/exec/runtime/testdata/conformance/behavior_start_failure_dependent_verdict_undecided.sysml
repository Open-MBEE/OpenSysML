// The machine that cannot start would, had it run, write `rating` from its
// `closed` state: a verdict reading `rating` cannot be told from what the machine
// would have made of it, so it is withheld as undecided rather than passed or failed.
package test {
    private import ScalarValues::*;
    part def Relay {
        attribute rating : Real = 10.0;
        exhibit state contacts {
            state open;
            state closed { entry action derate { assign rating := 0.0; } }
        }
    }
    requirement def RatingReq { subject r : Relay; require constraint { r.rating >= 6.0 } }
    requirement ratingOk : RatingReq;
    part relay : Relay;
    satisfy ratingOk by relay;
}
