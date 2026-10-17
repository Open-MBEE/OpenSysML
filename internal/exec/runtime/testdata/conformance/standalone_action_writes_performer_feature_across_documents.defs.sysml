// The action definition of `standalone_action_writes_performer_feature_across_documents`,
// indexed beside it: written on its own, its body resolves in its own namespace, where
// nothing is called `touched`.
package touchDefs {
    private import ScalarValues::*;

    action def Touch {
        action step {
            assign touched := 1;
        }
        first step;
    }
}
