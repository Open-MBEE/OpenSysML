// State-machine notation written through the standard metadata extension
// mechanism: pseudostates are state usages carrying a StateMachines metadata
// annotation, and a deferred event is a `#deferred` ref usage.
package StateMetadataPseudostates {
    private import StateMachines::*;

    item def Ping;
    action def SetSpeed;

    state def Controller {
        entry; then start;
        state start;
        state busy {
            #deferred ref : Ping;
            #deferred ref speed : SetSpeed;
        }
        state ready;

        #choice state pick;
        #junction state j;
        #shallowHistory state h;
        #deepHistory state hd;

        succession first start then busy;
        transition first busy then pick;
        transition first pick then ready;
    }
}
