// State-machine notation written through the standard metadata extension
// mechanism: pseudostates are state usages carrying a StateMachines metadata
// annotation.
package StateMetadataPseudostates {
    private import StateMachines::*;

    state def Controller {
        entry; then start;
        state start;
        state busy;
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
