package TraceReview {
    private import ScalarValues::*;
    private import SI::*;

    attribute def Open;
    attribute def Submit;
    attribute def Verify;
    attribute def Reload;
    attribute def Timed { attribute attempt : Integer; }
    attribute def Scored { attribute score : Integer; }

    calc def Ord {
        return : Integer;
        attribute y : Integer := 1;
        assign y := y * 10;
        assign y := y + 2;
        y
    }

    part def ReviewSession {
        attribute screen : String = "blank";
        attribute confirmedVisible : Boolean = false;
        attribute score : Integer = 0;
        attribute fraction : Real = 0.5;
        derived attribute derivedScore : Integer = Ord();
        attribute deliveryPath : Integer = 0;

        exhibit state Review {

            entry; then blank;
            state blank;
            state waiting;
            state form { entry action { assign screen := "form"; } }
            state pending {
                entry action { assign screen := "pending"; }
                // Concurrent delivery callbacks settle in either order. Neither path
                // changes the visible pending receipt; Verify distinguishes them.
                do action settle {
                    first start;
                    fork callbacks;
                    action firstChannel { assign deliveryPath := 1; }
                    action secondChannel { assign deliveryPath := 2; }
                    join callbacksDone;
                    succession first start then callbacks;
                    succession first callbacks then firstChannel;
                    succession first callbacks then secondChannel;
                    succession first firstChannel then callbacksDone;
                    succession first secondChannel then callbacksDone;
                    succession first callbacksDone then done;
                }
            }
            state confirmed {
                entry action {
                    assign screen := "confirmed";
                    assign confirmedVisible := true;
                }
            }
            state retry { entry action { assign screen := "retry"; } }

            transition first blank accept Open then form;
            transition first blank accept Timed then waiting;
            transition first blank accept progress : Scored do assign score := progress.score then blank;
            transition first waiting accept after 1 [s] then waiting;
            transition first form accept Submit then pending;
            transition first pending accept Verify if deliveryPath == 1 then confirmed;
            transition first pending accept Verify if deliveryPath == 2 then retry;
            transition first confirmed accept Reload then confirmed;
            transition first retry accept Reload then retry;
        }
    }
    part profile : ReviewSession;
}
